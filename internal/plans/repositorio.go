package plans

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Erros do repositório de planos.
var (
	ErrNaoEncontrado = errors.New("plano não encontrado")
	ErrNomeEmUso     = errors.New("já existe um plano com este nome")
	ErrPlanoEmUso    = errors.New("o plano tem vales associados e não pode ser eliminado")
)

// Repositorio persiste os planos no SQLite.
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório de planos.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

const colunas = `p.id, p.nome, p.preco_centavos, p.duracao_minutos, p.velocidade_kbps,
	p.limite_dados_mb, p.max_dispositivos, p.ativo, p.criado_em, p.atualizado_em`

// Listar devolve todos os planos, ativos primeiro, por preço.
func (r *Repositorio) Listar(ctx context.Context, apenasAtivos bool) ([]Plano, error) {
	query := `SELECT ` + colunas + `, (SELECT COUNT(*) FROM vales v WHERE v.plano_id = p.id)
		FROM planos p`
	if apenasAtivos {
		query += ` WHERE p.ativo = 1`
	}
	query += ` ORDER BY p.ativo DESC, p.preco_centavos ASC`

	linhas, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("não foi possível listar os planos: %w", err)
	}
	defer linhas.Close()

	planos := []Plano{}
	for linhas.Next() {
		p, err := lerPlano(linhas)
		if err != nil {
			return nil, err
		}
		planos = append(planos, p)
	}
	return planos, linhas.Err()
}

// Buscar devolve o plano com o identificador indicado.
func (r *Repositorio) Buscar(ctx context.Context, id int64) (Plano, error) {
	linha := r.db.QueryRowContext(ctx,
		`SELECT `+colunas+`, (SELECT COUNT(*) FROM vales v WHERE v.plano_id = ?) FROM planos p WHERE p.id = ?`, id, id)
	return lerPlano(linha)
}

// BuscarVelocidade devolve apenas a velocidade do plano, evitando carregar a linha inteira.
func (r *Repositorio) BuscarVelocidade(ctx context.Context, id int64) (int, error) {
	var velocidade int
	err := r.db.QueryRowContext(ctx, `SELECT velocidade_kbps FROM planos WHERE id = ?`, id).Scan(&velocidade)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNaoEncontrado
	}
	if err != nil {
		return 0, fmt.Errorf("não foi possível ler a velocidade do plano: %w", err)
	}
	return velocidade, nil
}

// BuscarPorNome devolve o plano com o nome indicado.
func (r *Repositorio) BuscarPorNome(ctx context.Context, nome string) (Plano, error) {
	linha := r.db.QueryRowContext(ctx,
		`SELECT `+colunas+`, 0 FROM planos p WHERE p.nome = ? COLLATE NOCASE`, strings.TrimSpace(nome))
	return lerPlano(linha)
}

// Criar insere um novo plano.
func (r *Repositorio) Criar(ctx context.Context, n NovoPlano) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO planos (nome, preco_centavos, duracao_minutos, velocidade_kbps, limite_dados_mb, max_dispositivos, ativo)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		n.Nome, n.PrecoCentavos, n.DuracaoMinutos, n.VelocidadeKbps, n.LimiteDadosMB, n.MaxDispositivos, boolParaInt(*n.Ativo))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrNomeEmUso
		}
		return 0, fmt.Errorf("não foi possível criar o plano: %w", err)
	}
	return res.LastInsertId()
}

// Atualizar aplica uma atualização parcial ao plano.
func (r *Repositorio) Atualizar(ctx context.Context, id int64, a AtualizarPlano) error {
	query := `UPDATE planos SET `
	args := []any{}
	add := func(coluna string, valor any) {
		if len(args) > 0 {
			query += ", "
		}
		query += coluna + " = ?"
		args = append(args, valor)
	}

	if a.Nome != nil {
		add("nome", *a.Nome)
	}
	if a.PrecoCentavos != nil {
		add("preco_centavos", *a.PrecoCentavos)
	}
	if a.DuracaoMinutos != nil {
		add("duracao_minutos", *a.DuracaoMinutos)
	}
	if a.VelocidadeKbps != nil {
		add("velocidade_kbps", *a.VelocidadeKbps)
	}
	if a.LimiteDadosMB != nil {
		add("limite_dados_mb", *a.LimiteDadosMB)
	}
	if a.MaxDispositivos != nil {
		add("max_dispositivos", *a.MaxDispositivos)
	}
	if a.Ativo != nil {
		add("ativo", boolParaInt(*a.Ativo))
	}
	if len(args) == 0 {
		return nil // nada a alterar
	}

	query += `, atualizado_em = ? WHERE id = ?`
	args = append(args, time.Now().UTC(), id)

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrNomeEmUso
		}
		return fmt.Errorf("não foi possível atualizar o plano: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// Eliminar remove o plano se não tiver vales associados.
func (r *Repositorio) Eliminar(ctx context.Context, id int64) error {
	var usados int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vales WHERE plano_id = ?`, id).Scan(&usados); err != nil {
		return err
	}
	if usados > 0 {
		return ErrPlanoEmUso
	}

	res, err := r.db.ExecContext(ctx, `DELETE FROM planos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("não foi possível eliminar o plano: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

func lerPlano(s scanner) (Plano, error) {
	var p Plano
	var ativo int
	var atualizado sql.NullTime

	if err := s.Scan(&p.ID, &p.Nome, &p.PrecoCentavos, &p.DuracaoMinutos, &p.VelocidadeKbps,
		&p.LimiteDadosMB, &p.MaxDispositivos, &ativo, &p.CriadoEm, &atualizado, &p.ValesUsados); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Plano{}, ErrNaoEncontrado
		}
		return Plano{}, fmt.Errorf("não foi possível ler o plano: %w", err)
	}
	p.Ativo = ativo == 1
	if atualizado.Valid {
		t := atualizado.Time
		p.AtualizadoEm = &t
	}
	return p, nil
}

func boolParaInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
