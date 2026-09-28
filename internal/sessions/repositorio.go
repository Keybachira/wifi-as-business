package sessions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Erros do repositório de sessões.
var (
	ErrNaoEncontrada = errors.New("sessão não encontrada")
	ErrJaAtiva      = errors.New("este dispositivo já tem uma sessão ativa")
)

// Repositorio persiste as sessões no SQLite.
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório de sessões.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

const colunas = `s.id, s.vale_id, s.rede_id, s.utilizador, COALESCE(s.telefone, ''), s.endereco_ip,
	s.endereco_mac, s.tipo_dispositivo, s.velocidade, s.limite_velocidade, s.dados_mb,
	s.iniciada_em, s.terminada_em, s.estado, COALESCE(s.motivo_fim, ''), COALESCE(p.nome, ''), COALESCE(p.duracao_minutos, 0)
	FROM sessoes s LEFT JOIN vales v ON v.id = s.vale_id LEFT JOIN planos p ON p.id = v.plano_id`

// Listar devolve as sessões conforme o filtro.
func (r *Repositorio) Listar(ctx context.Context, f FiltroSessoes) ([]Sessão, int, error) {
	f.Normalizar()

	where := []string{"1 = 1"}
	args := []any{}

	if f.Estado != "" {
		where = append(where, "s.estado = ?")
		args = append(args, f.Estado)
	} else if f.ApenasAtivas {
		where = append(where, "s.estado = ?")
		args = append(args, EstadoAtiva)
	}
	if f.RedeID > 0 {
		where = append(where, "s.rede_id = ?")
		args = append(args, f.RedeID)
	}
	if f.Utilizador != "" {
		where = append(where, "s.utilizador LIKE ?")
		args = append(args, "%"+f.Utilizador+"%")
	}
	if f.EnderecoIP != "" {
		where = append(where, "s.endereco_ip = ?")
		args = append(args, f.EnderecoIP)
	}
	clausula := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessoes s`+clausula, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("não foi possível contar as sessões: %w", err)
	}

	query := `SELECT ` + colunas + clausula + ` ORDER BY s.iniciada_em DESC, s.id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limite, f.Deslocar)

	linhas, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("não foi possível listar as sessões: %w", err)
	}
	defer linhas.Close()

	sessoes := []Sessão{}
	for linhas.Next() {
		s, err := lerSessao(linhas)
		if err != nil {
			return nil, 0, err
		}
		sessoes = append(sessoes, s)
	}
	return sessoes, total, linhas.Err()
}

// Iniciar regista uma nova sessão ativa.
// A velocidade é herdada do plano associado ao vale, salvo se for fornecida.
func (r *Repositorio) Iniciar(ctx context.Context, d IniciarSessao) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO sessoes (vale_id, rede_id, utilizador, telefone, endereco_ip, endereco_mac,
			tipo_dispositivo, velocidade, limite_velocidade)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, 0), (
			SELECT p.velocidade_kbps FROM vales v
			JOIN planos p ON p.id = v.plano_id WHERE v.id = ?), 0))`,
		d.ValeID, d.RedeID, d.Utilizador, nullSeVazio(d.Telefone), d.EnderecoIP, d.EnderecoMAC,
		d.TipoDispositivo, d.Velocidade, d.LimiteVelocidade, d.ValeID)
	if err != nil {
		return 0, fmt.Errorf("não foi possível iniciar a sessão: %w", err)
	}
	return res.LastInsertId()
}

// Terminar encerra a sessão com o estado e motivo indicados.
func (r *Repositorio) Terminar(ctx context.Context, id int64, estado, motivo string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE sessoes SET estado = ?, terminada_em = ?, motivo_fim = ? WHERE id = ? AND estado = ?`,
		estado, time.Now().UTC(), motivo, id, EstadoAtiva)
	if err != nil {
		return fmt.Errorf("não foi possível terminar a sessão: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrada
	}
	return nil
}

// TerminarPorMAC encerra a sessão ativa de um dispositivo.
func (r *Repositorio) TerminarPorMAC(ctx context.Context, mac, motivo string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessoes SET estado = ?, terminada_em = ?, motivo_fim = ?
		 WHERE endereco_mac = ? AND estado = ?`,
		EstadoTerminada, time.Now().UTC(), motivo, strings.ToUpper(strings.TrimSpace(mac)), EstadoAtiva)
	return err
}

// ExpirarVencidas encerra sessões ativas cujo vale já não é válido.
func (r *Repositorio) ExpirarVencidas(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE sessoes SET estado = ?, terminada_em = ?, motivo_fim = 'plano expirado'
		 WHERE estado = ? AND vale_id IN (
			SELECT v.id FROM vales v JOIN planos p ON p.id = v.plano_id
			WHERE v.estado IN (?, ?) OR v.expira_em <= ?
		 )`,
		EstadoExpirada, time.Now().UTC(), EstadoAtiva, "expirado", "revogado", time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ContarAtivas devolve o número de sessões ativas.
func (r *Repositorio) ContarAtivas(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessoes WHERE estado = ?`, EstadoAtiva).Scan(&total)
	return total, err
}

// AtualizarConsumo regista o tráfego consumido por uma sessão.
func (r *Repositorio) AtualizarConsumo(ctx context.Context, id int64, dadosMB float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessoes SET dados_mb = ? WHERE id = ?`, dadosMB, id)
	return err
}

type scanner interface{ Scan(dest ...any) error }

func lerSessao(s scanner) (Sessão, error) {
	var sess Sessão
	var valeID, redeID sql.NullInt64
	var terminada sql.NullTime

	if err := s.Scan(&sess.ID, &valeID, &redeID, &sess.Utilizador, &sess.Telefone, &sess.EnderecoIP,
		&sess.EnderecoMAC, &sess.TipoDispositivo, &sess.Velocidade, &sess.LimiteVelocidade, &sess.DadosMB,
		&sess.IniciadaEm, &terminada, &sess.Estado, &sess.MotivoFim, &sess.PlanoNome, &sess.RestanteMinutos); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Sessão{}, ErrNaoEncontrada
		}
		return Sessão{}, fmt.Errorf("não foi possível ler a sessão: %w", err)
	}
	if valeID.Valid {
		v := valeID.Int64
		sess.ValeID = &v
	}
	if redeID.Valid {
		r := redeID.Int64
		sess.RedeID = &r
	}
	if terminada.Valid {
		t := terminada.Time
		sess.TerminadaEm = &t
	}

	// Converte a duração total do plano em minutos restantes.
	if sess.RestanteMinutos > 0 {
		restante := time.Duration(sess.RestanteMinutos)*time.Minute - time.Since(sess.IniciadaEm)
		if restante < 0 {
			restante = 0
		}
		sess.RestanteMinutos = int(restante.Minutes())
	}
	return sess, nil
}

func nullSeVazio(s string) any {
	if s == "" {
		return nil
	}
	return s
}


// Buscar devolve a sessão pelo identificador.
func (r *Repositorio) Buscar(ctx context.Context, id int64) (Sessão, error) {
	linha := r.db.QueryRowContext(ctx, `SELECT `+colunas+` WHERE s.id = ?`, id)
	return lerSessao(linha)
}

// BuscarAtivaPorMAC devolve a sessão ativa do dispositivo, se existir.
func (r *Repositorio) BuscarAtivaPorMAC(ctx context.Context, mac string) (Sessão, error) {
	linha := r.db.QueryRowContext(ctx,
		`SELECT `+colunas+` WHERE s.endereco_mac = ? AND s.estado = ? ORDER BY s.iniciada_em DESC LIMIT 1`,
		strings.ToUpper(strings.TrimSpace(mac)), EstadoAtiva)
	return lerSessao(linha)
}
