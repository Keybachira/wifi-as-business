package sessions

import (
	"context"
	"errors"
	"fmt"
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
	// LimitadorBanda aplica a velocidade do plano no equipamento de rede.
	//
	// É separado do LimitadorRede porque nem todos os equipamentos precisam de
	// estar configurados para os dois: desligar um cliente e limitar a banda
	// são operações com requisitos diferentes.
	LimitadorBanda interface {
		AplicarLimite(ctx context.Context, enderecoIP string, downloadKbps, uploadKbps int) error
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
	banda     LimitadorBanda
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

// ComBanda liga quem aplica a velocidade do plano no equipamento de rede.
func (s *Serviço) ComBanda(l LimitadorBanda) *Serviço { s.banda = l; return s }

// aplicarVelocidade empurra a velocidade do plano para o equipamento.
//
// Uma falha aqui não invalida a sessão: o cliente está ligado e o plano foi
// consumido. A velocidade é uma garantia de serviço, não uma condição de acesso.
func (s *Serviço) aplicarVelocidade(ctx context.Context, sessao Sessão) {
	if s.banda == nil || sessao.LimiteVelocidade <= 0 {
		return
	}
	download := sessao.LimiteVelocidade
	upload := sessao.LimiteVelocidade / 2
	if err := s.banda.AplicarLimite(ctx, sessao.EnderecoIP, download, upload); err != nil {
		s.registo.Warn("não foi possível aplicar a velocidade no equipamento",
			"sessao", sessao.ID, "ip", sessao.EnderecoIP, "erro", err)
	}
}

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
	SessaoID     string    `json:"sessao_id"`
	Utilizador   string    `json:"utilizador"`
	Plano        string    `json:"plano,omitempty"`
	ValidoAte    time.Time `json:"valido_ate"`
	DownloadKbps int       `json:"download_kbps"`
	UploadKbps   int       `json:"upload_kbps"`
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

// RegistarConsumo grava o consumo reportado pelo router e corta a sessão se
// o limite do plano tiver sido ultrapassado.
//
// É chamado a cada Accounting-Request do RADIUS, e também pelo portal quando
// o consumo é conhecido por outra via. Devolve true se a sessão foi encerrada.
func (s *Serviço) RegistrarConsumo(ctx context.Context, sessionID string, downloadMB, uploadMB float64) (bool, error) {
	if s.repo == nil {
		return false, nil
	}

	sessao, err := s.repo.BuscarPorSessionID(ctx, sessionID)
	if err != nil {
		// Uma sessão que não existe localmente é normal: o router pode
		// reportar consumo de sessões abertas antes do último arranque.
		if errors.Is(err, ErrNaoEncontrada) {
			return false, nil
		}
		return false, err
	}

	total := downloadMB + uploadMB
	if err := s.repo.AtualizarConsumo(ctx, sessao.ID, total); err != nil {
		return false, fmt.Errorf("não foi possível gravar o consumo: %w", err)
	}

	// Só uma sessão ativa e com limite definido pode ser cortada por dados.
	if !sessao.AindaAtiva() || sessao.LimiteDadosMB <= 0 {
		return false, nil
	}
	guardada := sessao
	guardada.DadosMB = total
	if !guardada.ExcedeuDados() {
		return false, nil
	}

	if err := s.cortarPorDados(ctx, guardada); err != nil {
		return false, err
	}
	return true, nil
}

// cortarPorDados termina a sessão e avisa o equipamento de rede.
func (s *Serviço) cortarPorDados(ctx context.Context, sessao Sessão) error {
	if err := s.repo.Terminar(ctx, sessao.ID, EstadoExpirada, "limite de dados atingido"); err != nil {
		if !errors.Is(err, ErrNaoEncontrada) {
			return err
		}
	}
	s.registo.Info("sessão encerrada por excesso de dados",
		"sessao", sessao.ID, "utilizador", sessao.Utilizador,
		"consumido_mb", sessao.DadosMB, "limite_mb", sessao.LimiteDadosMB)

	if s.limitador != nil {
		if err := s.limitador.DesligarCliente(ctx, sessao.EnderecoMAC); err != nil {
			s.registo.Warn("não foi possível desligar o cliente do equipamento",
				"mac", sessao.EnderecoMAC, "erro", err)
		}
	}
	return nil
}

// ExcederDados encerra todas as sessões ativas que ultrapassaram o limite.
//
// Corre a cada cinco minutos, para apanhar o consumo que o router não reportou.
func (s *Serviço) ExcederDados(ctx context.Context) (int64, error) {
	ids, err := s.repo.ExcedeuDados(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	// Recolhe os MAC antes de terminar, para poder avisar os equipamentos.
	macs := make([]string, 0, len(ids))
	for _, id := range ids {
		if sessao, err := s.repo.Buscar(ctx, id); err == nil {
			macs = append(macs, sessao.EnderecoMAC)
		}
	}

	if err := s.repo.MarcarExcedidas(ctx, ids); err != nil {
		return 0, err
	}
	if s.registo != nil {
		s.registo.Info("sessões encerradas por excesso de dados", "quantidade", len(ids))
	}
	if s.limitador != nil {
		for _, mac := range macs {
			if err := s.limitador.DesligarCliente(ctx, mac); err != nil {
				s.registo.Warn("não foi possível desligar o cliente do equipamento",
					"mac", mac, "erro", err)
			}
		}
	}
	return int64(len(ids)), nil
}
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
	sessao, err := s.repo.Buscar(ctx, id)
	if err != nil {
		return Sessão{}, err
	}
	s.registo.Info("sessão iniciada", "utilizador", d.Utilizador, "ip", d.EnderecoIP, "mac", d.EnderecoMAC)

	// A velocidade só é aplicada depois de a sessão estar registada, para que
	// uma falha do equipamento não deixe o cliente sem vale consumido.
	s.aplicarVelocidade(ctx, sessao)
	return sessao, nil
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
