// Package networks gere as redes WiFi administradas pelo controlador.
package networks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Estado de uma rede.
const (
	EstadoAtiva   = "ativa"
	EstadoInativa = "inativa"
)

// Erros do repositório.
var (
	ErrNaoEncontrada = errors.New("rede não encontrada")
	ErrNomeEmUso     = errors.New("já existe uma rede com este nome")
	ErrRedeEmUso     = errors.New("a rede tem sessões ou dispositivos associados")
)

// Rede é um segmento WiFi gerido pelo controlador.
type Rede struct {
	ID          int64     `json:"id"`
	Nome        string    `json:"nome"`
	Descricao   string    `json:"descricao"`
	Subnet      string    `json:"subnet"`
	Estado      string    `json:"estado"`
	CriadoEm    time.Time `json:"criado_em"`
	SessoesAtivas int     `json:"sessoes_ativas"`
	Dispositivos int      `json:"dispositivos"`
}

// Ativa indica se a rede está operacional.
func (r Rede) Ativa() bool { return r.Estado == EstadoAtiva }

// NovaRede são os dados para criar uma rede.
type NovaRede struct {
	Nome      string `json:"nome"`
	Descricao string `json:"descricao"`
	Subnet    string `json:"subnet"`
	Ativa     *bool  `json:"ativa,omitempty"`
}

// Validar normaliza e valida os dados de criação.
func (n *NovaRede) Validar() error {
	n.Nome = strings.TrimSpace(n.Nome)
	n.Descricao = strings.TrimSpace(n.Descricao)
	n.Subnet = strings.TrimSpace(n.Subnet)

	if n.Nome == "" {
		return errors.New("o nome da rede é obrigatório")
	}
	if len([]rune(n.Nome)) > 80 {
		return errors.New("o nome não pode exceder 80 caracteres")
	}
	if n.Subnet != "" {
		if _, _, err := net.ParseCIDR(n.Subnet); err != nil {
			return errors.New("a sub-rede deve estar no formato CIDR, por exemplo 192.168.1.0/24")
		}
	}
	if n.Ativa == nil {
		v := true
		n.Ativa = &v
	}
	return nil
}

// Repositorio persiste as redes no SQLite.
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório de redes.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

// Listar devolve todas as redes com contagens de sessões e dispositivos.
func (r *Repositorio) Listar(ctx context.Context) ([]Rede, error) {
	linhas, err := r.db.QueryContext(ctx, `
		SELECT n.id, n.nome, n.descricao, n.subnet, n.estado, n.criado_em,
			(SELECT COUNT(*) FROM sessoes s WHERE s.rede_id = n.id AND s.estado = 'ativa'),
			(SELECT COUNT(*) FROM dispositivos d WHERE d.endereco_ip LIKE substr(n.subnet, 1, instr(n.subnet || '/', '/') - 1) || '%')
		FROM redes n ORDER BY n.nome`)
	if err != nil {
		return nil, fmt.Errorf("não foi possível listar as redes: %w", err)
	}
	defer linhas.Close()

	redes := []Rede{}
	for linhas.Next() {
		var rd Rede
		if err := linhas.Scan(&rd.ID, &rd.Nome, &rd.Descricao, &rd.Subnet, &rd.Estado,
			&rd.CriadoEm, &rd.SessoesAtivas, &rd.Dispositivos); err != nil {
			return nil, err
		}
		redes = append(redes, rd)
	}
	return redes, linhas.Err()
}

// Buscar devolve a rede pelo identificador.
func (r *Repositorio) Buscar(ctx context.Context, id int64) (Rede, error) {
	var rd Rede
	err := r.db.QueryRowContext(ctx, `SELECT id, nome, descricao, subnet, estado, criado_em FROM redes WHERE id = ?`, id).
		Scan(&rd.ID, &rd.Nome, &rd.Descricao, &rd.Subnet, &rd.Estado, &rd.CriadoEm)
	if errors.Is(err, sql.ErrNoRows) {
		return Rede{}, ErrNaoEncontrada
	}
	if err != nil {
		return Rede{}, fmt.Errorf("não foi possível ler a rede: %w", err)
	}
	return rd, nil
}

// Criar insere uma nova rede.
func (r *Repositorio) Criar(ctx context.Context, n NovaRede) (int64, error) {
	estado := EstadoInativa
	if *n.Ativa {
		estado = EstadoAtiva
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO redes (nome, descricao, subnet, estado) VALUES (?, ?, ?, ?)`,
		n.Nome, n.Descricao, n.Subnet, estado)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrNomeEmUso
		}
		return 0, fmt.Errorf("não foi possível criar a rede: %w", err)
	}
	return res.LastInsertId()
}

// AtualizarEstado ativa ou desativa a rede.
func (r *Repositorio) AtualizarEstado(ctx context.Context, id int64, ativa bool) error {
	estado := EstadoInativa
	if ativa {
		estado = EstadoAtiva
	}
	res, err := r.db.ExecContext(ctx, `UPDATE redes SET estado = ? WHERE id = ?`, estado, id)
	if err != nil {
		return fmt.Errorf("não foi possível atualizar a rede: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrada
	}
	return nil
}

// Eliminar remove a rede se não tiver sessões associadas.
func (r *Repositorio) Eliminar(ctx context.Context, id int64) error {
	var ativas int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessoes WHERE rede_id = ? AND estado = 'ativa'`, id).Scan(&ativas); err != nil {
		return err
	}
	if ativas > 0 {
		return ErrRedeEmUso
	}

	res, err := r.db.ExecContext(ctx, `DELETE FROM redes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("não foi possível eliminar a rede: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrada
	}
	return nil
}
