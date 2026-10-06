// Package analytics agrega métricas para os painéis do controlador.
package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// Repositorio lê as métricas agregadas do SQLite.
type Repositorio struct{ db *sql.DB }

// NovoRepositorio cria o repositório de métricas.
func NovoRepositorio(db *sql.DB) *Repositorio { return &Repositorio{db: db} }

// Intervalo define a janela temporal de um relatório.
type Intervalo struct {
	De  time.Time
	Ate time.Time
}

// NovoIntervalo cria um intervalo que termina agora e cobre a duração indicada.
func NovoIntervalo(duracao time.Duration) Intervalo {
	fim := time.Now().UTC()
	return Intervalo{De: fim.Add(-duracao), Ate: fim}
}

// SerieTemporal é um ponto de dados de uma série.
type SerieTemporal struct {
	Instante time.Time `json:"instante"`
	Valor    float64   `json:"valor"`
}

// MetricasHoje resume o dia corrente.
type MetricasHoje struct {
	SessoesAtivas      int   `json:"sessoes_ativas"`
	Utilizadores       int   `json:"utilizadores_unicos"`
	ValesVendidos      int   `json:"vales_vendidos"`
	ReceitaCentavos    int64 `json:"receita_centavos"`
	SessaoMediaMin     int   `json:"sessao_media_minutos"`
	DispositivosOnline int   `json:"dispositivos_online"`
	DispositivosTotal  int   `json:"dispositivos_total"`
}

// Distribuicao é uma fatia de um gráfico circular.
type Distribuicao struct {
	Nome        string  `json:"nome"`
	Valor       int     `json:"valor"`
	Percentagem float64 `json:"percentagem"`
}

// TopUtilizador lista os clientes que mais geraram sessões.
type TopUtilizador struct {
	Utilizador string  `json:"utilizador"`
	Sessoes    int     `json:"sessoes"`
	DadosMB    float64 `json:"dados_mb"`
}

// Resumo devolve os indicadores principais do dia.
func (r *Repositorio) Resumo(ctx context.Context) (MetricasHoje, error) {
	var m MetricasHoje
	inicio := time.Now().UTC().Truncate(24 * time.Hour)

	simples := []struct {
		query string
		args  []any
		dest  *int
		nome  string
	}{
		{`SELECT COUNT(DISTINCT utilizador) FROM sessoes WHERE estado = 'ativa'`, nil, &m.SessoesAtivas, "sessões ativas"},
		{`SELECT COUNT(*) FROM vales WHERE usado_em >= ?`, []any{inicio}, &m.ValesVendidos, "vales vendidos"},
		{`SELECT COUNT(*) FROM dispositivos WHERE estado = 'online'`, nil, &m.DispositivosOnline, "dispositivos online"},
		{`SELECT COUNT(*) FROM dispositivos`, nil, &m.DispositivosTotal, "dispositivos totais"},
		{`SELECT COUNT(DISTINCT utilizador) FROM sessoes WHERE iniciada_em >= ?`, []any{inicio}, &m.Utilizadores, "utilizadores"},
	}
	for _, c := range simples {
		if err := r.db.QueryRowContext(ctx, c.query, c.args...).Scan(c.dest); err != nil {
			return m, fmt.Errorf("não foi possível calcular %s: %w", c.nome, err)
		}
	}

	// Receita: soma o preço do plano de cada vale vendido hoje.
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(p.preco_centavos), 0) FROM vales v
		JOIN planos p ON p.id = v.plano_id WHERE v.usado_em >= ?`, inicio).
		Scan(&m.ReceitaCentavos); err != nil {
		return m, err
	}

	var totalMin, totalSessoes int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CAST((julianday(COALESCE(terminada_em, CURRENT_TIMESTAMP))
			- julianday(iniciada_em)) * 1440 AS INTEGER)), 0), COUNT(*)
		FROM sessoes WHERE iniciada_em >= ?`, inicio).Scan(&totalMin, &totalSessoes); err != nil {
		return m, err
	}
	if totalSessoes > 0 {
		m.SessaoMediaMin = totalMin / totalSessoes
	}
	return m, nil
}

// UtilizadoresPorHora devolve o número de sessões iniciadas por hora.
func (r *Repositorio) UtilizadoresPorHora(ctx context.Context, i Intervalo) ([]SerieTemporal, error) {
	linhas, err := r.db.QueryContext(ctx, `
		SELECT strftime('%Y-%m-%dT%H:00:00Z', iniciada_em), COUNT(*)
		FROM sessoes WHERE iniciada_em BETWEEN ? AND ?
		GROUP BY 1 ORDER BY 1`, i.De, i.Ate)
	if err != nil {
		return nil, fmt.Errorf("não foi possível obter as sessões por hora: %w", err)
	}
	defer linhas.Close()

	serie := []SerieTemporal{}
	for linhas.Next() {
		var s SerieTemporal
		var hora string
		if err := linhas.Scan(&hora, &s.Valor); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, hora); err == nil {
			s.Instante = t
		}
		serie = append(serie, s)
	}
	return serie, linhas.Err()
}

// BandaPorHora devolve o tráfego agregado por hora.
func (r *Repositorio) BandaPorHora(ctx context.Context, i Intervalo) ([]SerieTemporal, error) {
	linhas, err := r.db.QueryContext(ctx, `
		SELECT instante, download_kbps FROM metricas_hora
		WHERE instante BETWEEN ? AND ? ORDER BY instante`, i.De, i.Ate)
	if err != nil {
		return nil, fmt.Errorf("não foi possível obter a banda por hora: %w", err)
	}
	defer linhas.Close()

	serie := []SerieTemporal{}
	for linhas.Next() {
		var s SerieTemporal
		if err := linhas.Scan(&s.Instante, &s.Valor); err != nil {
			return nil, err
		}
		serie = append(serie, s)
	}
	return serie, linhas.Err()
}

// PorPlano devolve a distribuição de sessões por plano.
func (r *Repositorio) PorPlano(ctx context.Context, i Intervalo) ([]Distribuicao, error) {
	return r.distribuicao(ctx, `
		SELECT COALESCE(p.nome, 'Sem plano'), COUNT(*)
		FROM sessoes s
		LEFT JOIN vales v ON v.id = s.vale_id
		LEFT JOIN planos p ON p.id = v.plano_id
		WHERE s.iniciada_em BETWEEN ? AND ?
		GROUP BY p.nome ORDER BY 2 DESC`, i)
}

// PorDispositivo devolve a distribuição de sessões por tipo de equipamento.
func (r *Repositorio) PorDispositivo(ctx context.Context, i Intervalo) ([]Distribuicao, error) {
	return r.distribuicao(ctx, `
		SELECT tipo_dispositivo, COUNT(*) FROM sessoes
		WHERE iniciada_em BETWEEN ? AND ?
		GROUP BY tipo_dispositivo ORDER BY 2 DESC`, i)
}

// distribuicao executa uma consulta de agrupamento e converte em percentagens.
func (r *Repositorio) distribuicao(ctx context.Context, query string, i Intervalo) ([]Distribuicao, error) {
	linhas, err := r.db.QueryContext(ctx, query, i.De, i.Ate)
	if err != nil {
		return nil, fmt.Errorf("não foi possível obter a distribuição: %w", err)
	}
	defer linhas.Close()

	bruto := []Distribuicao{}
	var total int
	for linhas.Next() {
		var d Distribuicao
		if err := linhas.Scan(&d.Nome, &d.Valor); err != nil {
			return nil, err
		}
		total += d.Valor
		bruto = append(bruto, d)
	}
	if err := linhas.Err(); err != nil {
		return nil, err
	}
	for i := range bruto {
		if total > 0 {
			bruto[i].Percentagem = math.Round(float64(bruto[i].Valor)/float64(total)*1000) / 10
		}
	}
	return bruto, nil
}

// TopUtilizadores devolve os clientes com mais sessões no intervalo.
func (r *Repositorio) TopUtilizadores(ctx context.Context, i Intervalo, limite int) ([]TopUtilizador, error) {
	if limite <= 0 || limite > 100 {
		limite = 10
	}
	linhas, err := r.db.QueryContext(ctx, `
		SELECT utilizador, COUNT(*), COALESCE(SUM(dados_mb), 0)
		FROM sessoes WHERE iniciada_em BETWEEN ? AND ?
		GROUP BY utilizador ORDER BY 2 DESC LIMIT ?`, i.De, i.Ate, limite)
	if err != nil {
		return nil, fmt.Errorf("não foi possível obter os principais utilizadores: %w", err)
	}
	defer linhas.Close()

	lista := []TopUtilizador{}
	for linhas.Next() {
		var t TopUtilizador
		if err := linhas.Scan(&t.Utilizador, &t.Sessoes, &t.DadosMB); err != nil {
			return nil, err
		}
		lista = append(lista, t)
	}
	return lista, linhas.Err()
}

// GravarMetricas regista a telemetria de uma hora.
func (r *Repositorio) GravarMetricas(ctx context.Context, instante time.Time, downloadKbps, uploadKbps, utilizadores int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO metricas_hora (instante, download_kbps, upload_kbps, utilizadores)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(instante) DO UPDATE SET
			download_kbps = excluded.download_kbps,
			upload_kbps = excluded.upload_kbps,
			utilizadores = excluded.utilizadores`,
		instante.Truncate(time.Hour), downloadKbps, uploadKbps, utilizadores)
	return err
}
