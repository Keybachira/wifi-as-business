package networks

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// Serviço contém a lógica de negócio das redes.
type Serviço struct {
	repo    *Repositorio
	registo *slog.Logger
}

// NovoServico cria o serviço de redes.
func NovoServico(repo *Repositorio, registo *slog.Logger) *Serviço {
	return &Serviço{repo: repo, registo: registo}
}

// Listar devolve todas as redes.
func (s *Serviço) Listar(ctx context.Context) ([]Rede, error) { return s.repo.Listar(ctx) }

// Obter devolve uma rede pelo identificador.
func (s *Serviço) Obter(ctx context.Context, id int64) (Rede, error) { return s.repo.Buscar(ctx, id) }

// Criar valida e persiste uma nova rede.
func (s *Serviço) Criar(ctx context.Context, n NovaRede) (Rede, error) {
	if err := n.Validar(); err != nil {
		return Rede{}, err
	}
	id, err := s.repo.Criar(ctx, n)
	if err != nil {
		return Rede{}, err
	}
	return s.repo.Buscar(ctx, id)
}

// AlternarEstado ativa ou desativa a rede.
func (s *Serviço) AlternarEstado(ctx context.Context, id int64) (Rede, error) {
	atual, err := s.repo.Buscar(ctx, id)
	if err != nil {
		return Rede{}, err
	}
	if err := s.repo.AtualizarEstado(ctx, id, !atual.Ativa()); err != nil {
		return Rede{}, err
	}
	return s.repo.Buscar(ctx, id)
}

// Eliminar remove uma rede sem sessões ativas.
func (s *Serviço) Eliminar(ctx context.Context, id int64) error {
	return s.repo.Eliminar(ctx, id)
}

// ServicoHTTP expõe as redes sobre HTTP.
type ServicoHTTP struct{ srv *Serviço }

// NovoServicoHTTP cria o adaptador HTTP das redes.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/redes", exigir(system.Adaptador(h.listar)))
	mux.Handle("POST /api/v1/redes", exigir(system.Adaptador(h.criar)))
	mux.Handle("GET /api/v1/redes/{id}", exigir(system.Adaptador(h.obter)))
	mux.Handle("POST /api/v1/redes/{id}/alternar", exigir(system.Adaptador(h.alternar)))
	mux.Handle("DELETE /api/v1/redes/{id}", exigir(system.Adaptador(h.eliminar)))
}

func (h *ServicoHTTP) listar(w http.ResponseWriter, r *http.Request) error {
	redes, err := h.srv.Listar(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, redes)
	return nil
}

func (h *ServicoHTTP) obter(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	rd, err := h.srv.Obter(r.Context(), id)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, rd)
	return nil
}

func (h *ServicoHTTP) criar(w http.ResponseWriter, r *http.Request) error {
	var dados NovaRede
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	rd, err := h.srv.Criar(r.Context(), dados)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusCreated, rd)
	return nil
}

func (h *ServicoHTTP) alternar(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	rd, err := h.srv.AlternarEstado(r.Context(), id)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, rd)
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
	case err == ErrNaoEncontrada:
		return system.ErroNaoEncontrado("Rede")
	case err == ErrNomeEmUso:
		return system.ErroConflito("Já existe uma rede com este nome")
	case err == ErrRedeEmUso:
		return system.ErroConflito("A rede tem sessões ativas")
	}
	return system.ErroValidacao("", err.Error())
}
