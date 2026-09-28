// Package devices gere o inventário de equipamentos de rede.
package devices

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Tipos de equipamento suportados.
const (
	TipoRouter      = "router"
	TipoSwitch      = "switch"
	TipoPontoAcesso = "ponto_acesso"
)

// Estados possíveis.
const (
	EstadoOnline  = "online"
	EstadoOffline = "offline"
)

// Erros do repositório.
var (
	ErrNaoEncontrado = errors.New("dispositivo não encontrado")
	ErrNomeEmUso     = errors.New("já existe um dispositivo com este nome")
	ErrIPInvalido    = errors.New("endereço IP inválido")
)

// Dispositivo é um equipamento sob gestão.
type Dispositivo struct {
	ID              int64      `json:"id"`
	Nome            string     `json:"nome"`
	Tipo            string     `json:"tipo"`
	EnderecoIP      string     `json:"endereco_ip"`
	Estado          string     `json:"estado"`
	TempoAtividade  string     `json:"tempo_atividade"`
	Clientes        int        `json:"clientes"`
	Banda           string     `json:"banda"`
	Firmware        string     `json:"firmware"`
	CPU             string     `json:"cpu"`
	Memoria         string     `json:"memoria"`
	VistoEm         *time.Time `json:"visto_em,omitempty"`
	CriadoEm        time.Time  `json:"criado_em"`
}

// Online indica se o equipamento responde.
func (d Dispositivo) Online() bool { return d.Estado == EstadoOnline }

// ÉPontoAcesso indica se é um ponto de acesso sem fios.
func (d Dispositivo) ÉPontoAcesso() bool { return d.Tipo == TipoPontoAcesso }

// NovoDispositivo são os dados para registar um equipamento.
type NovoDispositivo struct {
	Nome       string `json:"nome"`
	Tipo       string `json:"tipo"`
	EnderecoIP string `json:"endereco_ip"`
}

// Validar normaliza e valida os dados de registo.
func (n *NovoDispositivo) Validar() error {
	n.Nome = strings.TrimSpace(n.Nome)
	n.Tipo = strings.ToLower(strings.TrimSpace(n.Tipo))
	n.EnderecoIP = strings.TrimSpace(n.EnderecoIP)

	if n.Nome == "" {
		return errors.New("o nome do dispositivo é obrigatório")
	}
	if len([]rune(n.Nome)) > 80 {
		return errors.New("o nome não pode exceder 80 caracteres")
	}
	switch n.Tipo {
	case TipoRouter, TipoSwitch, TipoPontoAcesso:
	default:
		return errors.New("tipo inválido: use router, switch ou ponto_acesso")
	}
	if net.ParseIP(n.EnderecoIP) == nil {
		return ErrIPInvalido
	}
	return nil
}

// Telemetria são as métricas coletadas do equipamento.
type Telemetria struct {
	Estado   string `json:"estado"`
	Uptime   string `json:"tempo_atividade"`
	Clientes int    `json:"clientes"`
	Banda    string `json:"banda"`
	Firmware string `json:"firmware"`
	CPU      string `json:"cpu"`
	Memoria  string `json:"memoria"`
}

// Repositorio persiste os equipamentos no SQLite.
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório de dispositivos.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

const colunas = `id, nome, tipo, endereco_ip, estado, tempo_actividade, clientes,
	banda, firmware, cpu, memoria, visto_em, criado_em`

// Listar devolve os equipamentos filtrando por tipo e estado.
func (r *Repositorio) Listar(ctx context.Context, tipo, estado string) ([]Dispositivo, error) {
	query := `SELECT ` + colunas + ` FROM dispositivos WHERE 1 = 1`
	var args []any
	if tipo != "" {
		query += ` AND tipo = ?`
		args = append(args, tipo)
	}
	if estado != "" {
		query += ` AND estado = ?`
		args = append(args, estado)
	}
	query += ` ORDER BY tipo, nome`

	linhas, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel listar os dispositivos: %w", err)
	}
	defer linhas.Close()

	dispositivos := []Dispositivo{}
	for linhas.Next() {
		d, err := lerDispositivo(linhas)
		if err != nil {
			return nil, err
		}
		dispositivos = append(dispositivos, d)
	}
	return dispositivos, linhas.Err()
}

// Buscar devolve o equipamento pelo identificador.
func (r *Repositorio) Buscar(ctx context.Context, id int64) (Dispositivo, error) {
	linha := r.db.QueryRowContext(ctx, `SELECT `+colunas+` FROM dispositivos WHERE id = ?`, id)
	return lerDispositivo(linha)
}

// Criar regista um novo equipamento.
func (r *Repositorio) Criar(ctx context.Context, n NovoDispositivo) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO dispositivos (nome, tipo, endereco_ip) VALUES (?, ?, ?)`,
		n.Nome, n.Tipo, n.EnderecoIP)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrNomeEmUso
		}
		return 0, fmt.Errorf("nao foi possivel registar o dispositivo: %w", err)
	}
	return res.LastInsertId()
}

// AtualizarTelemetria grava as metricas mais recentes do equipamento.
func (r *Repositorio) AtualizarTelemetria(ctx context.Context, id int64, t Telemetria) error {
	estado := t.Estado
	if estado != EstadoOnline && estado != EstadoOffline {
		estado = EstadoOffline
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE dispositivos SET estado = ?, tempo_atividade = ?, clientes = ?, banda = ?,
			firmware = ?, cpu = ?, memoria = ?, visto_em = ? WHERE id = ?`,
		estado, t.Uptime, t.Clientes, t.Banda, t.Firmware, t.CPU, t.Memoria, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("nao foi possivel atualizar o dispositivo: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// Eliminar remove o equipamento do inventario.
func (r *Repositorio) Eliminar(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM dispositivos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("nao foi possivel eliminar o dispositivo: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// Resumo devolve contagens por tipo e estado para o painel.
func (r *Repositorio) Resumo(ctx context.Context) (map[string]int, error) {
	linhas, err := r.db.QueryContext(ctx, `SELECT tipo || ':' || estado, COUNT(*) FROM dispositivos GROUP BY tipo, estado`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()

	resumo := map[string]int{}
	for linhas.Next() {
		var chave string
		var total int
		if err := linhas.Scan(&chave, &total); err != nil {
			return nil, err
		}
		resumo[chave] = total
	}
	return resumo, linhas.Err()
}

type scanner interface{ Scan(dest ...any) error }

func lerDispositivo(s scanner) (Dispositivo, error) {
	var d Dispositivo
	var visto sql.NullTime

	if err := s.Scan(&d.ID, &d.Nome, &d.Tipo, &d.EnderecoIP, &d.Estado, &d.TempoAtividade,
		&d.Clientes, &d.Banda, &d.Firmware, &d.CPU, &d.Memoria, &visto, &d.CriadoEm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Dispositivo{}, ErrNaoEncontrado
		}
		return Dispositivo{}, fmt.Errorf("nao foi possivel ler o dispositivo: %w", err)
	}
	if visto.Valid {
		t := visto.Time
		d.VistoEm = &t
	}
	return d, nil
}
