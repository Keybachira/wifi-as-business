package devices

import (
	"net/http"
	"strconv"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// ServicoHTTP expoe os equipamentos sobre HTTP.
type ServicoHTTP struct{ srv *Servico }

// NovoServicoHTTP cria o adaptador HTTP dos dispositivos.
func NovoServicoHTTP(srv *Servico) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do modulo.
// exigir e o middleware de autenticacao aplicado as rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/dispositivos", exigir(system.Adaptador(h.listar)))
	mux.Handle("POST /api/v1/dispositivos", exigir(system.Adaptador(h.registar)))
	mux.Handle("GET /api/v1/dispositivos/resumo", exigir(system.Adaptador(h.resumo)))
	mux.Handle("GET /api/v1/dispositivos/{id}", exigir(system.Adaptador(h.obter)))
	mux.Handle("PATCH /api/v1/dispositivos/{id}/telemetria", exigir(system.Adaptador(h.telemetria)))
	mux.Handle("DELETE /api/v1/dispositivos/{id}", exigir(system.Adaptador(h.eliminar)))
}

func (h *ServicoHTTP) listar(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	dispositivos, err := h.srv.Listar(r.Context(), q.Get("tipo"), q.Get("estado"))
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, dispositivos)
	return nil
}

func (h *ServicoHTTP) resumo(w http.ResponseWriter, r *http.Request) error {
	resumo, err := h.srv.Resumo(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, resumo)
	return nil
}

func (h *ServicoHTTP) obter(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador invalido")
	}
	d, err := h.srv.Obter(r.Context(), id)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, d)
	return nil
}

func (h *ServicoHTTP) registar(w http.ResponseWriter, r *http.Request) error {
	var dados NovoDispositivo
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	d, err := h.srv.Registar(r.Context(), dados)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusCreated, d)
	return nil
}

func (h *ServicoHTTP) telemetria(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador invalido")
	}
	var dados Telemetria
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	d, err := h.srv.AtualizarTelemetria(r.Context(), id, dados)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, d)
	return nil
}

func (h *ServicoHTTP) eliminar(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador invalido")
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
		return system.ErroNaoEncontrado("Dispositivo")
	case err == ErrNomeEmUso:
		return system.ErroConflito("Ja existe um dispositivo com este nome")
	}
	return system.ErroValidacao("", err.Error())
}