package portal

import (
	"net/http"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// ServicoHTTP expõe o portal sobre HTTP.
type ServicoHTTP struct{ srv *Serviço }

// NovoServicoHTTP cria o adaptador HTTP do portal.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
// A rota /publico não exige autenticação: é usada pelo próprio portal cativo.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/portal", exigir(system.Adaptador(h.obter)))
	mux.Handle("PATCH /api/v1/portal", exigir(system.Adaptador(h.atualizar)))
	mux.Handle("POST /api/v1/portal/publicar", exigir(system.Adaptador(h.publicar)))
	mux.Handle("GET /api/v1/portal/publico", system.Adaptador(h.vistaPublica))
}

func (h *ServicoHTTP) obter(w http.ResponseWriter, r *http.Request) error {
	d, err := h.srv.Obter(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, d)
	return nil
}

func (h *ServicoHTTP) atualizar(w http.ResponseWriter, r *http.Request) error {
	var dados Atualizar
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	d, err := h.srv.Atualizar(r.Context(), dados)
	if err != nil {
		return system.ErroValidacao("", err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, d)
	return nil
}

func (h *ServicoHTTP) publicar(w http.ResponseWriter, r *http.Request) error {
	var dados struct {
		Publicado bool `json:"publicado"`
	}
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	d, err := h.srv.Publicar(r.Context(), dados.Publicado)
	if err != nil {
		return system.ErroValidacao("", err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, d)
	return nil
}

func (h *ServicoHTTP) vistaPublica(w http.ResponseWriter, r *http.Request) error {
	vista, err := h.srv.VistaPublica(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, vista)
	return nil
}
