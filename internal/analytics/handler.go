package analytics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// Serviço agrega as métricas para consumo pela API.
type Serviço struct{ repo *Repositorio }

// NovoServico cria o serviço de métricas.
func NovoServico(repo *Repositorio) *Serviço { return &Serviço{repo: repo} }

// Resumo devolve os indicadores do dia.
func (s *Serviço) Resumo(ctx context.Context) (MetricasHoje, error) {
	return s.repo.Resumo(ctx)
}

// Painel devolve todo o conjunto de dados do ecrã de análise num único pedido.
func (s *Serviço) Painel(ctx context.Context, duracao time.Duration) (map[string]any, error) {
	intervalo := NovoIntervalo(duracao)

	resumo, err := s.repo.Resumo(ctx)
	if err != nil {
		return nil, err
	}
	porHora, err := s.repo.UtilizadoresPorHora(ctx, intervalo)
	if err != nil {
		return nil, err
	}
	banda, err := s.repo.BandaPorHora(ctx, intervalo)
	if err != nil {
		return nil, err
	}
	porPlano, err := s.repo.PorPlano(ctx, intervalo)
	if err != nil {
		return nil, err
	}
	porDispositivo, err := s.repo.PorDispositivo(ctx, intervalo)
	if err != nil {
		return nil, err
	}
	top, err := s.repo.TopUtilizadores(ctx, intervalo, 10)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"resumo":         resumo,
		"utilizadores_hora": porHora,
		"banda_hora":     banda,
		"por_plano":      porPlano,
		"por_dispositivo": porDispositivo,
		"principais_utilizadores": top,
		"intervalo": map[string]any{
			"de":  intervalo.De,
			"ate": intervalo.Ate,
		},
	}, nil
}

// ServicoHTTP expõe as métricas sobre HTTP.
type ServicoHTTP struct{ srv *Serviço }

// NovoServicoHTTP cria o adaptador HTTP das métricas.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/analises/resumo", exigir(system.Adaptador(h.resumo)))
	mux.Handle("GET /api/v1/analises/painel", exigir(system.Adaptador(h.painel)))
}

func (h *ServicoHTTP) resumo(w http.ResponseWriter, r *http.Request) error {
	resumo, err := h.srv.Resumo(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, resumo)
	return nil
}

func (h *ServicoHTTP) painel(w http.ResponseWriter, r *http.Request) error {
	duracao := 24 * time.Hour
	if v := r.URL.Query().Get("periodo"); v != "" {
		n, _ := strconv.Atoi(v)
		if n > 0 && n <= 365 {
			duracao = time.Duration(n) * 24 * time.Hour
		}
	}

	dados, err := h.srv.Painel(r.Context(), duracao)
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, dados)
	return nil
}
