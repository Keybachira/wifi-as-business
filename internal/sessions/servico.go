package sessions

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Dependências opcionais do serviço (implementadas por outros módulos).
type (
	// AtivadorVale ativa um código de vale e devolve o seu ID e o plano associado.
	AtivadorVale interface {
		AtivarPorCodigo(ctx context.Context, codigo string) (valeID int64, planoID int64, err error)
	}
	// LimitadorRede empurra a remoção da sessão para o router (desligar o cliente).
	LimitadorRede interface {
		DesligarCliente(ctx context.Context, enderecoMAC string) error
	}
	// LeitorPlanos consulta a velocidade configurada num plano.
	LeitorPlanos interface {
		BuscarVelocidade(ctx context.Context, planoID int64) (int, error)
	}
)

// Serviço contém a lógica de negócio das sessões.
type Serviço struct {
	repo      *Repositorio
	vales     AtivadorVale
	planos    LeitorPlanos
	limitador LimitadorRede
	registo   *slog.Logger
}

// NovoServico cria o serviço de sessões.
func NovoServico(repo *Repositorio, registo *slog.Logger) *Serviço {
	return &Serviço{repo: repo, registo: registo}
}

// ComVales liga o serviço de vales (ativação de códigos).
func (s *Serviço) ComVales(v AtivadorVale) *Serviço { s.vales = v; return s }

// ComPlanos liga o serviço de planos (velocidade do acesso).
func (s *Serviço) ComPlanos(p LeitorPlanos) *Serviço { s.planos = p; return s }

// ComLimitador liga o limitador de banda (desligar clientes no router).
func (s *Serviço) ComLimitador(l LimitadorRede) *Serviço { s.limitador = l; return s }

// AutenticarCliente são os dados públicos enviados pelo portal cativo.
type AutenticarCliente struct {
	Codigo          string `json:"codigo"`
	Utilizador      string `json:"utilizador,omitempty"`
	Telefone        string `json:"telefone,omitempty"`
	EnderecoIP      string `json:"endereco_ip"`
	EnderecoMAC     string `json:"endereco_mac"`
	TipoDispositivo string `json:"tipo_dispositivo,omitempty"`
	RedeID          *int64 `json:"rede_id,omitempty"`
}

// RespostaAutenticacao é devolvida ao cliente que se autenticou.
type RespostaAutenticacao struct {
	SessaoID    string    `json:"sessao_id"`
	Utilizador  string    `json:"utilizador"`
	Plano       string    `json:"plano,omitempty"`
	ValidoAte   time.Time `json:"valido_ate"`
	DownloadKbps int      `json:"download_kbps"`
	UploadKbps  int       `json:"upload_kbps"`
}

// AutenticarCliente valida um código de vale e abre a sessão do cliente.
func (s *Serviço) AutenticarCliente(ctx context.Context, dados AutenticarCliente) (RespostaAutenticacao, error) {
	if s.vales == nil {
		return RespostaAutenticacao{}, errors.New("a ativação por vale não está configurada")
	}

	// Consome o vale; a atualização condicional garante que só é usado uma vez.
	valeID, planoID, err := s.vales.AtivarPorCodigo(ctx, dados.Codigo)
	if err != nil {
		return RespostaAutenticacao{}, errors.New("código inválido, expirado ou já utilizado")
	}

	utilizador := strings.TrimSpace(dados.Utilizador)
	if utilizador == "" {
		utilizador = "Guest-" + dados.EnderecoMAC[:min(len(dados.EnderecoMAC), 5)]
	}

	sessao, err := s.Autenticar(ctx, IniciarSessao{
		ValeID:           &valeID,
		RedeID:           dados.RedeID,
		Utilizador:       utilizador,
		Telefone:         dados.Telefone,
		EnderecoIP:       dados.EnderecoIP,
		EnderecoMAC:      dados.EnderecoMAC,
		TipoDispositivo:  dados.TipoDispositivo,
		LimiteVelocidade: s.velocidadeDoPlano(ctx, planoID),
	})
	if err != nil {
		return RespostaAutenticacao{}, err
	}

	return RespostaAutenticacao{
		SessaoID:     strconv.FormatInt(sessao.ID, 10),
		Utilizador:   sessao.Utilizador,
		Plano:        sessao.PlanoNome,
		ValidoAte:    sessao.IniciadaEm.Add(time.Duration(sessao.RestanteMinutos) * time.Minute),
		DownloadKbps: sessao.LimiteVelocidade,
		UploadKbps:   sessao.LimiteVelocidade / 2,
	}, nil
}

// velocidadeDoPlano devolve a velocidade de descida configurada no plano.
func (s *Serviço) velocidadeDoPlano(ctx context.Context, planoID int64) int {
	if s.planos == nil || planoID == 0 {
		return 0
	}
	velocidade, err := s.planos.BuscarVelocidade(ctx, planoID)
	if err != nil {
		return 0
	}
	return velocidade
}

// Listar devolve as sessões conforme o filtro.
func (s *Serviço) Listar(ctx context.Context, f FiltroSessoes) ([]Sessão, int, error) {
	return s.repo.Listar(ctx, f)
}

// Obter devolve uma sessão pelo identificador.
func (s *Serviço) Obter(ctx context.Context, id int64) (Sessão, error) {
	return s.repo.Buscar(ctx, id)
}

// Ativas devolve apenas as sessões em curso.
func (s *Serviço) Ativas(ctx context.Context) ([]Sessão, error) {
	sessoes, _, err := s.repo.Listar(ctx, FiltroSessoes{ApenasAtivas: true, Limite: 500})
	return sessoes, err
}

// Autenticar regista uma nova sessão. Se o vale for indicado, ativa-o primeiro.
func (s *Serviço) Autenticar(ctx context.Context, d IniciarSessao) (Sessão, error) {
	if err := d.Validar(); err != nil {
		return Sessão{}, err
	}

	// Se o dispositivo já tem sessão, termina a anterior antes de criar a nova.
	if existente, err := s.repo.BuscarAtivaPorMAC(ctx, d.EnderecoMAC); err == nil {
		_ = s.repo.Terminar(ctx, existente.ID, EstadoTerminada, "nova autenticação")
	}

	id, err := s.repo.Iniciar(ctx, d)
	if err != nil {
		return Sessão{}, err
	}
	s.registo.Info("sessão iniciada", "utilizador", d.Utilizador, "ip", d.EnderecoIP, "mac", d.EnderecoMAC)
	return s.repo.Buscar(ctx, id)
}

// Desconectar encerra a sessão a pedido de um administrador.
func (s *Serviço) Desconectar(ctx context.Context, id int64, motivo string) error {
	sessao, err := s.repo.Buscar(ctx, id)
	if err != nil {
		return err
	}
	if !sessao.AindaAtiva() {
		return errors.New("a sessão já está terminada")
	}

	if err := s.repo.Terminar(ctx, id, EstadoTerminada, padrao(motivo, "desconectado pelo operador")); err != nil {
		return err
	}

	// Notifica o router, se existir ligação configurada.
	if s.limitador != nil {
		if err := s.limitador.DesligarCliente(ctx, sessao.EnderecoMAC); err != nil {
			s.registo.Warn("não foi possível desligar o cliente no router",
				"mac", sessao.EnderecoMAC, "erro", err)
		}
	}

	s.registo.Info("sessão terminada", "id", id, "utilizador", sessao.Utilizador)
	return nil
}

// DesconectarPorMAC encerra a sessão ativa de um endereço MAC.
func (s *Serviço) DesconectarPorMAC(ctx context.Context, mac, motivo string) error {
	if err := s.repo.TerminarPorMAC(ctx, mac, padrao(motivo, "desconectado pelo operador")); err != nil {
		return err
	}
	if s.limitador != nil {
		if err := s.limitador.DesligarCliente(ctx, mac); err != nil {
			s.registo.Warn("falha ao desligar cliente no router", "mac", mac, "erro", err)
		}
	}
	return nil
}

// ContarAtivas devolve o número de clientes ligados neste momento.
func (s *Serviço) ContarAtivas(ctx context.Context) (int, error) {
	return s.repo.ContarAtivas(ctx)
}

// ExpirarVencidas encerra sessões cujo plano terminou.
func (s *Serviço) ExpirarVencidas(ctx context.Context) (int64, error) {
	n, err := s.repo.ExpirarVencidas(ctx)
	if err == nil && n > 0 {
		s.registo.Info("sessões expiradas por término do plano", "quantidade", n)
	}
	return n, err
}

func padrao(valor, alternativa string) string {
	if strings.TrimSpace(valor) == "" {
		return alternativa
	}
	return valor
}

var _ = time.Second
