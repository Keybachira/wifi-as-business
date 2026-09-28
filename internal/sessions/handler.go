package sessions

import (
	"net/http"
	"strconv"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// ServicoHTTP expõe as sessões sobre HTTP.
type ServicoHTTP struct{ srv *Serviço }

// NovoServicoHTTP cria o adaptador HTTP das sessões.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/sessoes", exigir(system.Adaptador(h.listar)))
	mux.Handle("GET /api/v1/sessoes/ativas", exigir(system.Adaptador(h.ativas)))
	mux.Handle("GET /api/v1/sessoes/{id}", exigir(system.Adaptador(h.obter)))
	mux.Handle("DELETE /api/v1/sessoes/{id}", exigir(system.Adaptador(h.desconectar)))

	// Rota pública: usada pelo portal cativo para autenticar um cliente.
	mux.Handle("POST /api/v1/publico/sessao", system.Adaptador(h.autenticar))
}

func (h *ServicoHTTP) autenticar(w http.ResponseWriter, r *http.Request) error {
	var dados AutenticarCliente
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}
	sessao, err := h.srv.AutenticarCliente(r.Context(), dados)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, sessao)
	return nil
}

func (h *ServicoHTTP) listar(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	filtro := FiltroSessoes{
		Estado:     q.Get("estado"),
		Utilizador: q.Get("utilizador"),
		EnderecoIP: q.Get("ip"),
		Limite:     intDeQuery(q.Get("limite")),
		Deslocar:   intDeQuery(q.Get("deslocar")),
	}
	if id := intDeQuery(q.Get("rede_id")); id > 0 {
		filtro.RedeID = int64(id)
	}

	sessoes, total, err := h.srv.Listar(r.Context(), filtro)
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, map[string]any{
		"itens":    sessoes,
		"total":    total,
		"limite":   filtro.Limite,
		"deslocar": filtro.Deslocar,
	})
	return nil
}

func (h *ServicoHTTP) ativas(w http.ResponseWriter, r *http.Request) error {
	sessoes, err := h.srv.Ativas(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, sessoes)
	return nil
}

func (h *ServicoHTTP) obter(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	sessao, err := h.srv.Obter(r.Context(), id)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, sessao)
	return nil
}

func (h *ServicoHTTP) desconectar(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	if err := h.srv.Desconectar(r.Context(), id, ""); err != nil {
		return traduzirErro(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func traduzirErro(err error) error {
	if err == ErrNaoEncontrada {
		return system.ErroNaoEncontrado("Sessão")
	}
	return system.ErroConflito(err.Error())
}

func intDeQuery(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}
