package plans

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// Serviço contém a lógica de negócio dos planos.
type Serviço struct {
	repo    *Repositorio
	registo *slog.Logger
}

// NovoServico cria o serviço de planos.
func NovoServico(repo *Repositorio, registo *slog.Logger) *Serviço {
	return &Serviço{repo: repo, registo: registo}
}

// Listar devolve os planos filtrando opcionalmente pelos ativos.
func (s *Serviço) Listar(ctx context.Context, apenasAtivos bool) ([]Plano, error) {
	return s.repo.Listar(ctx, apenasAtivos)
}

// Obter devolve um plano pelo identificador.
func (s *Serviço) Obter(ctx context.Context, id int64) (Plano, error) {
	return s.repo.Buscar(ctx, id)
}

// Criar valida e persiste um novo plano.
func (s *Serviço) Criar(ctx context.Context, n NovoPlano) (Plano, error) {
	if err := n.Validar(); err != nil {
		return Plano{}, err
	}
	id, err := s.repo.Criar(ctx, n)
	if err != nil {
		return Plano{}, err
	}
	return s.repo.Buscar(ctx, id)
}

// Atualizar aplica uma atualização parcial, bloqueando o nome duplicado.
func (s *Serviço) Atualizar(ctx context.Context, id int64, a AtualizarPlano) (Plano, error) {
	plano, err := s.repo.Buscar(ctx, id)
	if err != nil {
		return Plano{}, err
	}
	if err := a.Validar(plano.Nome); err != nil {
		return Plano{}, err
	}
	if err := s.repo.Atualizar(ctx, id, a); err != nil {
		return Plano{}, err
	}
	return s.repo.Buscar(ctx, id)
}

// Eliminar remove um plano sem vales associados.
func (s *Serviço) Eliminar(ctx context.Context, id int64) error {
	return s.repo.Eliminar(ctx, id)
}

// ServicoHTTP expõe os planos sobre HTTP.
type ServicoHTTP struct{ srv *Serviço }

// NovoServicoHTTP cria o adaptador HTTP dos planos.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo no mux.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/planos", exigir(system.Adaptador(h.listar)))
	mux.Handle("POST /api/v1/planos", exigir(system.Adaptador(h.criar)))
	mux.Handle("GET /api/v1/planos/{id}", exigir(system.Adaptador(h.obter)))
	mux.Handle("PATCH /api/v1/planos/{id}", exigir(system.Adaptador(h.atualizar)))
	mux.Handle("DELETE /api/v1/planos/{id}", exigir(system.Adaptador(h.eliminar)))
}

func (h *ServicoHTTP) listar(w http.ResponseWriter, r *http.Request) error {
	apenasAtivos := r.URL.Query().Get("ativos") == "true"
	planos, err := h.srv.Listar(r.Context(), apenasAtivos)
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, planos)
	return nil
}

func (h *ServicoHTTP) obter(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	plano, err := h.srv.Obter(r.Context(), id)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, plano)
	return nil
}

func (h *ServicoHTTP) criar(w http.ResponseWriter, r *http.Request) error {
	var dados NovoPlano
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	plano, err := h.srv.Criar(r.Context(), dados)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusCreated, plano)
	return nil
}

func (h *ServicoHTTP) atualizar(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	var dados AtualizarPlano
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	plano, err := h.srv.Atualizar(r.Context(), id, dados)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, plano)
	return nil
}

func (h *ServicoHTTP) eliminar(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	if err := h.srv.Eliminar(r.Context(), id); err != nil {
		return traduzirErro(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// traduzirErro converte erros do domínio em respostas HTTP.
func traduzirErro(err error) error {
	switch {
	case err == ErrNaoEncontrado:
		return system.ErroNaoEncontrado("Plano")
	case err == ErrNomeEmUso:
		return system.ErroConflito("Já existe um plano com este nome")
	case err == ErrPlanoEmUso:
		return system.ErroConflito("O plano tem vales associados; desative-o em vez de eliminar")
	case strings.Contains(err.Error(), "não pode"):
		return system.ErroValidacao("", err.Error())
	}
	return system.ErroInterno(err.Error())
}
