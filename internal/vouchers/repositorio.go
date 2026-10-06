package vouchers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Erros do repositório de vales.
var (
	ErrNaoEncontrado = errors.New("vale não encontrado")
	ErrJaExiste      = errors.New("já existe um vale com este código")
)

// Repositorio persiste os vales no SQLite.
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório de vales.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

const colunas = `v.id, v.codigo, v.plano_id, p.nome, v.estado, v.expira_em, v.usado_em, v.usado_por, v.criado_em`

// Listar devolve os vales conforme o filtro, dos mais recentes para os mais antigos.
func (r *Repositorio) Listar(ctx context.Context, f FiltroVales) ([]Vale, int, error) {
	f.Normalizar()

	where := []string{"1 = 1"}
	args := []any{}

	if f.Estado != "" {
		where = append(where, "v.estado = ?")
		args = append(args, f.Estado)
	}
	if f.PlanoID > 0 {
		where = append(where, "v.plano_id = ?")
		args = append(args, f.PlanoID)
	}
	if f.Busca != "" {
		where = append(where, "(v.codigo LIKE ? OR v.usado_por LIKE ?)")
		alvo := "%" + f.Busca + "%"
		args = append(args, alvo, alvo)
	}
	clausula := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM vales v JOIN planos p ON p.id = v.plano_id`+clausula, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("não foi possível contar os vales: %w", err)
	}

	query := `SELECT ` + colunas + ` FROM vales v JOIN planos p ON p.id = v.plano_id` +
		clausula + ` ORDER BY v.criado_em DESC, v.id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limite, f.Deslocar)

	linhas, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("não foi possível listar os vales: %w", err)
	}
	defer linhas.Close()

	vales := []Vale{}
	for linhas.Next() {
		v, err := lerVale(linhas)
		if err != nil {
			return nil, 0, err
		}
		vales = append(vales, v)
	}
	return vales, total, linhas.Err()
}

// BuscarPorCodigo devolve o vale com o código indicado (ignorando maiúsculas).
func (r *Repositorio) BuscarPorCodigo(ctx context.Context, codigo string) (Vale, error) {
	linha := r.db.QueryRowContext(ctx,
		`SELECT `+colunas+` FROM vales v JOIN planos p ON p.id = v.plano_id WHERE v.codigo = ? COLLATE NOCASE`,
		strings.TrimSpace(codigo))
	return lerVale(linha)
}

// Buscar devolve o vale pelo identificador.
func (r *Repositorio) Buscar(ctx context.Context, id int64) (Vale, error) {
	linha := r.db.QueryRowContext(ctx,
		`SELECT `+colunas+` FROM vales v JOIN planos p ON p.id = v.plano_id WHERE v.id = ?`, id)
	return lerVale(linha)
}

// InserirMany grava um lote de vales numa única transação.
func (r *Repositorio) InserirMany(ctx context.Context, codigos []string, planoID int64, expiraEm *time.Time) error {
	if len(codigos) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("não foi possível iniciar a transação: %w", err)
	}
	defer tx.Rollback() // no-op se a transação já foi confirmada

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO vales (codigo, plano_id, expira_em) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, codigo := range codigos {
		if _, err := stmt.ExecContext(ctx, codigo, planoID, expiraEm); err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return ErrJaExiste
			}
			return err
		}
	}
	return tx.Commit()
}

// AtualizarEstado muda o estado do vale.
func (r *Repositorio) AtualizarEstado(ctx context.Context, id int64, estado string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE vales SET estado = ? WHERE id = ?`, estado, id)
	if err != nil {
		return fmt.Errorf("não foi possível atualizar o vale: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// AtivarCondicional só ativa o vale se ainda estiver disponível.
// affected=0 significa que outro pedido consumiu o vale primeiro (corrida).
func (r *Repositorio) AtivarCondicional(ctx context.Context, codigo, utilizador string) (Vale, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM vales WHERE codigo = ? COLLATE NOCASE AND estado = ? AND (expira_em IS NULL OR expira_em > ?)`,
		strings.TrimSpace(codigo), EstadoDisponivel, time.Now().UTC()).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Vale{}, ErrNaoEncontrado
		}
		return Vale{}, err
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE vales SET estado = ?, usado_em = ?, usado_por = ? WHERE id = ? AND estado = ?`,
		EstadoAtivo, time.Now().UTC(), utilizador, id, EstadoDisponivel)
	if err != nil {
		return Vale{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Vale{}, ErrNaoEncontrado
	}
	return r.Buscar(ctx, id)
}

// Eliminar remove o vale.
func (r *Repositorio) Eliminar(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM vales WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("não foi possível eliminar o vale: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// ExpirarVencidos marca como expirados os vales disponíveis cujo prazo terminou.
func (r *Repositorio) ExpirarVencidos(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE vales SET estado = ? WHERE estado = ? AND expira_em IS NOT NULL AND expira_em <= ?`,
		EstadoExpirado, EstadoDisponivel, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ContarPorEstado devolve a contagem de vales agrupada por estado.
func (r *Repositorio) ContarPorEstado(ctx context.Context) (map[string]int, error) {
	linhas, err := r.db.QueryContext(ctx, `SELECT estado, COUNT(*) FROM vales GROUP BY estado`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()

	contagem := map[string]int{}
	for linhas.Next() {
		var estado string
		var total int
		if err := linhas.Scan(&estado, &total); err != nil {
			return nil, err
		}
		contagem[estado] = total
	}
	return contagem, linhas.Err()
}

type scanner interface{ Scan(dest ...any) error }

func lerVale(s scanner) (Vale, error) {
	var v Vale
	var expira, usado sql.NullTime
	var usadoPor sql.NullString

	if err := s.Scan(&v.ID, &v.Codigo, &v.PlanoID, &v.PlanoNome, &v.Estado,
		&expira, &usado, &usadoPor, &v.CriadoEm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Vale{}, ErrNaoEncontrado
		}
		return Vale{}, fmt.Errorf("não foi possível ler o vale: %w", err)
	}
	if expira.Valid {
		t := expira.Time
		v.ExpiraEm = &t
	}
	if usado.Valid {
		t := usado.Time
		v.UsadoEm = &t
	}
	v.UsadoPor = usadoPor.String
	return v, nil
}
