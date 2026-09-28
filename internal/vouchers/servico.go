package vouchers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// RepoPlanos é o contrato mínimo que o serviço de vales precisa do módulo de planos.
type RepoPlanos interface {
	Buscar(ctx context.Context, id int64) (PlanoReferencia, error)
}

// PlanoReferencia são os dados do plano necessários para descrever um vale.
type PlanoReferencia struct {
	ID             int64
	Nome           string
	DuracaoMinutos int
	Ativo          bool
	PrecoCentavos  int64
}

// repoPlanosSQLite adapta o repositório real de planos a esta interface.
type repoPlanosSQLite struct{ db *sql.DB }

func (r repoPlanosSQLite) Buscar(ctx context.Context, id int64) (PlanoReferencia, error) {
	var p PlanoReferencia
	var ativo int
	err := r.db.QueryRowContext(ctx,
		`SELECT id, nome, duracao_minutos, ativo, preco_centavos FROM planos WHERE id = ?`, id).
		Scan(&p.ID, &p.Nome, &p.DuracaoMinutos, &ativo, &p.PrecoCentavos)
	if errors.Is(err, sql.ErrNoRows) {
		return PlanoReferencia{}, errors.New("plano não encontrado")
	}
	if err != nil {
		return PlanoReferencia{}, err
	}
	p.Ativo = ativo == 1
	return p, nil
}

// NovoRepoPlanos cria o adaptador sobre uma conexão de base de dados.
func NovoRepoPlanos(db *sql.DB) RepoPlanos { return repoPlanosSQLite{db: db} }

// ResultadoGeracao devolve os vales criados num lote.
type ResultadoGeracao struct {
	Quantidade int      `json:"quantidade"`
	Codigos     []string `json:"codigos"`
	PlanoID     int64    `json:"plano_id"`
	ExpiraEm    string   `json:"expira_em"`
}

// Serviço contém a lógica de negócio dos vales.
type Serviço struct {
	repo     *Repositorio
	planos   RepoPlanos
	registo  *slog.Logger
}

// NovoServico cria o serviço de vales.
func NovoServico(repo *Repositorio, planos RepoPlanos, registo *slog.Logger) *Serviço {
	return &Serviço{repo: repo, planos: planos, registo: registo}
}

// Listar devolve os vales e o total encontrado.
func (s *Serviço) Listar(ctx context.Context, f FiltroVales) ([]Vale, int, error) {
	return s.repo.Listar(ctx, f)
}

// Obter devolve um vale pelo identificador.
func (s *Serviço) Obter(ctx context.Context, id int64) (Vale, error) {
	return s.repo.Buscar(ctx, id)
}

// Gerar cria um lote de vales únicos para o plano indicado.
func (s *Serviço) Gerar(ctx context.Context, lote GerarLote) (ResultadoGeracao, error) {
	if err := lote.Validar(); err != nil {
		return ResultadoGeracao{}, err
	}

	plano, err := s.planos.Buscar(ctx, lote.PlanoID)
	if err != nil {
		return ResultadoGeracao{}, err
	}
	if !plano.Ativo {
		return ResultadoGeracao{}, errors.New("não é possível gerar vales de um plano inativo")
	}

	expiraEm := time.Now().UTC().AddDate(0, 0, lote.ValidadeDias)

	// Gera com tentativas extras para absorver colisões de códigos.
	codigos := make([]string, 0, lote.Quantidade)
	existentes := map[string]bool{}
	tentativas := lote.Quantidade * 3
	for len(codigos) < lote.Quantidade && tentativas > 0 {
		tentativas--
		codigo, err := GerarCodigo(lote.Prefixo)
		if err != nil {
			return ResultadoGeracao{}, err
		}
		if existentes[codigo] {
			continue
		}
		existentes[codigo] = true
		codigos = append(codigos, codigo)
	}

	if err := s.repo.InserirMany(ctx, codigos, lote.PlanoID, &expiraEm); err != nil {
		return ResultadoGeracao{}, err
	}

	s.registo.Info("lote de vales gerado", "quantidade", len(codigos), "plano", plano.Nome)
	return ResultadoGeracao{
		Quantidade: len(codigos),
		Codigos:    codigos,
		PlanoID:    lote.PlanoID,
		ExpiraEm:   expiraEm.Format(time.RFC3339),
	}, nil
}

// Revogar impede o uso de um vale disponível.
func (s *Serviço) Revogar(ctx context.Context, id int64) error {
	vale, err := s.repo.Buscar(ctx, id)
	if err != nil {
		return err
	}
	if vale.Estado == EstadoAtivo {
		return errors.New("não é possível revogar um vale já ativado")
	}
	return s.repo.AtualizarEstado(ctx, id, EstadoRevogado)
}

// Ativar marca o vale como usado por um utilizador (usado pelo portal/RADIUS).
func (s *Serviço) Ativar(ctx context.Context, codigo, utilizador string) (Vale, error) {
	codigo = strings.ToUpper(strings.TrimSpace(codigo))
	if codigo == "" {
		return Vale{}, errors.New("indique o código do vale")
	}
	return s.repo.AtivarCondicional(ctx, codigo, utilizador)
}

// ResultadoValidacao é a resposta pública à validação de um código.
type ResultadoValidacao struct {
	Valido    bool   `json:"valido"`
	PlanoNome string `json:"plano_nome,omitempty"`
	Mensagem  string `json:"mensagem"`
}

// ValidarCodigo consulta um código sem o consumir, usado pelo portal cativo.
// Devolve sempre 200 com `valido: false` para não revelar quais códigos existem.
func (s *Serviço) ValidarCodigo(ctx context.Context, codigo string) (ResultadoValidacao, error) {
	codigo = strings.ToUpper(strings.TrimSpace(codigo))
	if codigo == "" {
		return ResultadoValidacao{Valido: false, Mensagem: "Introduza o seu código de acesso"}, nil
	}

	vale, err := s.repo.BuscarPorCodigo(ctx, codigo)
	if err != nil {
		return ResultadoValidacao{Valido: false, Mensagem: "Código inválido ou já utilizado"}, nil
	}
	if !vale.PodeUsar() {
		return ResultadoValidacao{Valido: false, Mensagem: "Código inválido ou já utilizado"}, nil
	}
	return ResultadoValidacao{Valido: true, PlanoNome: vale.PlanoNome, Mensagem: "Código válido"}, nil
}

// AtivarPorCodigo satisfaz a interface de sessões.AtivadorVale.
func (s *Serviço) AtivarPorCodigo(ctx context.Context, codigo string) (int64, int64, error) {
	vale, err := s.Ativar(ctx, codigo, "")
	if err != nil {
		return 0, 0, err
	}
	return vale.ID, vale.PlanoID, nil
}

// ContarPorEstado devolve as contagens para o painel.
func (s *Serviço) ContarPorEstado(ctx context.Context) (map[string]int, error) {
	return s.repo.ContarPorEstado(ctx)
}

// ExpirarVencidos executa a rotina de manutenção dos vales.
func (s *Serviço) ExpirarVencidos(ctx context.Context) (int64, error) {
	n, err := s.repo.ExpirarVencidos(ctx)
	if err == nil && n > 0 {
		s.registo.Info("vales expirados", "quantidade", n)
	}
	return n, err
}
