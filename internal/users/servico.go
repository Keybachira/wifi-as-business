package users

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// Serviço contém a lógica de negócio dos utilizadores.
type Serviço struct {
	repo    *Repositorio
	registo *slog.Logger
}

// NovoServico cria o serviço de utilizadores.
func NovoServico(repo *Repositorio, registo *slog.Logger) *Serviço {
	return &Serviço{repo: repo, registo: registo}
}

// Listar devolve os utilizadores filtrando opcionalmente por estado.
func (s *Serviço) Listar(ctx context.Context, estado string) ([]Utilizador, error) {
	return s.repo.Listar(ctx, estado)
}

// Obter devolve um utilizador pelo identificador.
func (s *Serviço) Obter(ctx context.Context, id int64) (Utilizador, error) {
	return s.repo.Buscar(ctx, id)
}

// Registar valida e persiste um novo utilizador.
func (s *Serviço) Registar(ctx context.Context, n NovoUtilizador) (Utilizador, error) {
	if err := n.Validar(); err != nil {
		return Utilizador{}, err
	}
	id, err := s.repo.Criar(ctx, n)
	if err != nil {
		return Utilizador{}, err
	}
	return s.repo.Buscar(ctx, id)
}

// AlternarEstado ativa ou desativa um utilizador.
func (s *Serviço) AlternarEstado(ctx context.Context, id int64, ativo bool) (Utilizador, error) {
	if err := s.repo.AtualizarEstado(ctx, id, ativo); err != nil {
		return Utilizador{}, err
	}
	return s.repo.Buscar(ctx, id)
}

// Eliminar remove um utilizador.
func (s *Serviço) Eliminar(ctx context.Context, id int64) error {
	return s.repo.Eliminar(ctx, id)
}

// ContarAtivos devolve o número de utilizadores ativos.
func (s *Serviço) ContarAtivos(ctx context.Context) (int, error) {
	return s.repo.ContarAtivos(ctx)
}

// ServicoHTTP expõe os utilizadores sobre HTTP.
type ServicoHTTP struct{ srv *Serviço }

// NovoServicoHTTP cria o adaptador HTTP dos utilizadores.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/utilizadores", exigir(system.Adaptador(h.listar)))
	mux.Handle("POST /api/v1/utilizadores", exigir(system.Adaptador(h.registar)))
	mux.Handle("GET /api/v1/utilizadores/{id}", exigir(system.Adaptador(h.obter)))
	mux.Handle("PATCH /api/v1/utilizadores/{id}/estado", exigir(system.Adaptador(h.alterarEstado)))
	mux.Handle("DELETE /api/v1/utilizadores/{id}", exigir(system.Adaptador(h.eliminar)))
}

func (h *ServicoHTTP) listar(w http.ResponseWriter, r *http.Request) error {
	utilizadores, err := h.srv.Listar(r.Context(), r.URL.Query().Get("estado"))
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, utilizadores)
	return nil
}

func (h *ServicoHTTP) obter(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	u, err := h.srv.Obter(r.Context(), id)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, u)
	return nil
}

func (h *ServicoHTTP) registar(w http.ResponseWriter, r *http.Request) error {
	var dados NovoUtilizador
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	u, err := h.srv.Registar(r.Context(), dados)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusCreated, u)
	return nil
}

func (h *ServicoHTTP) alterarEstado(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	var dados AlternarEstado
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	u, err := h.srv.AlternarEstado(r.Context(), id, dados.Ativo)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, u)
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

func traduzirErro(err error) error {
	switch {
	case err == ErrNaoEncontrado:
		return system.ErroNaoEncontrado("Utilizador")
	case err == ErrEmailEmUso:
		return system.ErroConflito("Este email já está registado")
	}
	return system.ErroValidacao("", err.Error())
}
