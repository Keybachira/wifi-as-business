// Package portal gere a aparência e o comportamento do portal cativo.
package portal

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

// CorHex valida cores no formato #RRGGBB.
var padraoCor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Definições são os parâmetros do portal cativo (registo único).
type Definições struct {
	NomeRede        string    `json:"nome_rede"`
	Mensagem        string    `json:"mensagem"`
	CorFundo        string    `json:"cor_fundo"`
	CorDestaque     string    `json:"cor_destaque"`
	Termos          string    `json:"termos"`
	MensagemSucesso string    `json:"mensagem_sucesso"`
	Publicado       bool      `json:"publicado"`
	AtualizadoEm    time.Time `json:"atualizado_em"`
}

// Publico é a vista devolvida aos utilizadores do portal, sem dados internos.
type Publico struct {
	NomeRede        string `json:"nome_rede"`
	Mensagem        string `json:"mensagem"`
	CorFundo        string `json:"cor_fundo"`
	CorDestaque     string `json:"cor_destaque"`
	Termos          string `json:"termos"`
	MensagemSucesso string `json:"mensagem_sucesso"`
}

// Atualizar são os campos alteráveis do portal.
type Atualizar struct {
	NomeRede        *string `json:"nome_rede,omitempty"`
	Mensagem        *string `json:"mensagem,omitempty"`
	CorFundo        *string `json:"cor_fundo,omitempty"`
	CorDestaque     *string `json:"cor_destaque,omitempty"`
	Termos          *string `json:"termos,omitempty"`
	MensagemSucesso *string `json:"mensagem_sucesso,omitempty"`
	Publicado       *bool   `json:"publicado,omitempty"`
}

// Validar normaliza e valida uma atualização parcial.
func (a Atualizar) Validar() error {
	cor := func(valor *string, campo string) error {
		if valor == nil {
			return nil
		}
		*valor = strings.TrimSpace(*valor)
		if !padraoCor.MatchString(*valor) {
			return errors.New("a " + campo + " deve estar no formato #RRGGBB")
		}
		return nil
	}
	if err := cor(a.CorFundo, "cor de fundo"); err != nil {
		return err
	}
	if err := cor(a.CorDestaque, "cor de destaque"); err != nil {
		return err
	}

	if a.NomeRede != nil {
		v := strings.TrimSpace(*a.NomeRede)
		if v == "" {
			return errors.New("o nome da rede não pode ficar vazio")
		}
		*a.NomeRede = v
	}
	return nil
}

// Repositorio persiste as definições do portal (registo único id=1).
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório do portal.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

const colunas = `nome_rede, mensagem, cor_fundo, cor_destaque, termos, mensagem_sucesso, publicado, atualizado_em`

// Obter devolve as definições atuais, criando-as se necessário.
func (r *Repositorio) Obter(ctx context.Context) (Definições, error) {
	var d Definições
	var publicado int
	err := r.db.QueryRowContext(ctx, `SELECT `+colunas+` FROM portal WHERE id = 1`).
		Scan(&d.NomeRede, &d.Mensagem, &d.CorFundo, &d.CorDestaque,
			&d.Termos, &d.MensagemSucesso, &publicado, &d.AtualizadoEm)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := r.db.ExecContext(ctx, `INSERT OR IGNORE INTO portal (id) VALUES (1)`); err != nil {
			return Definições{}, err
		}
		return r.Obter(ctx)
	}
	if err != nil {
		return Definições{}, err
	}
	d.Publicado = publicado == 1
	return d, nil
}

// Atualizar aplica uma atualização parcial às definições.
func (r *Repositorio) Atualizar(ctx context.Context, a Atualizar) error {
	query := `UPDATE portal SET `
	args := []any{}
	add := func(coluna string, valor any) {
		if len(args) > 0 {
			query += ", "
		}
		query += coluna + " = ?"
		args = append(args, valor)
	}

	if a.NomeRede != nil {
		add("nome_rede", *a.NomeRede)
	}
	if a.Mensagem != nil {
		add("mensagem", strings.TrimSpace(*a.Mensagem))
	}
	if a.CorFundo != nil {
		add("cor_fundo", *a.CorFundo)
	}
	if a.CorDestaque != nil {
		add("cor_destaque", *a.CorDestaque)
	}
	if a.Termos != nil {
		add("termos", strings.TrimSpace(*a.Termos))
	}
	if a.MensagemSucesso != nil {
		add("mensagem_sucesso", strings.TrimSpace(*a.MensagemSucesso))
	}
	if a.Publicado != nil {
		v := 0
		if *a.Publicado {
			v = 1
		}
		add("publicado", v)
	}
	if len(args) == 0 {
		return nil
	}

	query += `, atualizado_em = ? WHERE id = 1`
	args = append(args, time.Now().UTC())

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// Serviço contém a lógica de negócio do portal.
type Serviço struct {
	repo    *Repositorio
	registo *slog.Logger
}

// NovoServico cria o serviço do portal.
func NovoServico(repo *Repositorio, registo *slog.Logger) *Serviço {
	return &Serviço{repo: repo, registo: registo}
}

// Obter devolve as definições atuais.
func (s *Serviço) Obter(ctx context.Context) (Definições, error) { return s.repo.Obter(ctx) }

// Atualizar valida e grava as alterações ao portal.
func (s *Serviço) Atualizar(ctx context.Context, a Atualizar) (Definições, error) {
	if err := a.Validar(); err != nil {
		return Definições{}, err
	}
	if err := s.repo.Atualizar(ctx, a); err != nil {
		return Definições{}, err
	}
	return s.repo.Obter(ctx)
}

// Publicar marca o portal como publicado.
func (s *Serviço) Publicar(ctx context.Context, publicado bool) (Definições, error) {
	return s.Atualizar(ctx, Atualizar{Publicado: &publicado})
}

// VistaPublica devolve apenas os dados que os utilizadores do portal podem ver.
// Se o portal não estiver publicado, devolve os valores padrão.
func (s *Serviço) VistaPublica(ctx context.Context) (Publico, error) {
	d, err := s.repo.Obter(ctx)
	if err != nil {
		return Publico{}, err
	}
	if !d.Publicado {
		return Publico{
			NomeRede:        "WiFi Controller",
			Mensagem:        "Introduza o seu código de acesso",
			CorFundo:        "#0b1220",
			CorDestaque:     "#3b82f6",
			MensagemSucesso: "Está ligado. Aproveite a sua sessão.",
		}, nil
	}
	return Publico{
		NomeRede:        d.NomeRede,
		Mensagem:        d.Mensagem,
		CorFundo:        d.CorFundo,
		CorDestaque:     d.CorDestaque,
		Termos:          d.Termos,
		MensagemSucesso: d.MensagemSucesso,
	}, nil
}
