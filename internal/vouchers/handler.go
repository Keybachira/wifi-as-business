package vouchers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

// ServicoHTTP expõe os vales sobre HTTP.
type ServicoHTTP struct{ srv *Serviço }

// NovoServicoHTTP cria o adaptador HTTP dos vales.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/vales", exigir(system.Adaptador(h.listar)))
	mux.Handle("POST /api/v1/vales/lote", exigir(system.Adaptador(h.gerar)))
	mux.Handle("GET /api/v1/vales/resumo", exigir(system.Adaptador(h.resumo)))
	mux.Handle("GET /api/v1/vales/{id}", exigir(system.Adaptador(h.obter)))
	mux.Handle("POST /api/v1/vales/{id}/revogar", exigir(system.Adaptador(h.revogar)))

	// Rotas públicas: usadas pelo portal cativo e pelo servidor RADIUS.
	mux.Handle("GET /api/v1/publico/portal", system.Adaptador(h.vistaPublica))
	mux.Handle("POST /api/v1/publico/validar", system.Adaptador(h.validarCodigo))
}

func (h *ServicoHTTP) validarCodigo(w http.ResponseWriter, r *http.Request) error {
	var corpo struct {
		Codigo string `json:"codigo"`
	}
	if err := system.DecodificarJSON(r, &corpo); err != nil {
		return err
	}
	vale, err := h.srv.ValidarCodigo(r.Context(), corpo.Codigo)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, vale)
	return nil
}

func (h *ServicoHTTP) vistaPublica(w http.ResponseWriter, r *http.Request) error {
	vale, err := h.srv.ValidarCodigo(r.Context(), r.URL.Query().Get("codigo"))
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, vale)
	return nil
}

func (h *ServicoHTTP) listar(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	filtro := FiltroVales{
		Estado:   q.Get("estado"),
		Busca:    q.Get("busca"),
		Limite:   intDeQuery(q.Get("limite")),
		Deslocar: intDeQuery(q.Get("deslocar")),
	}
	if id := intDeQuery(q.Get("plano_id")); id > 0 {
		filtro.PlanoID = int64(id)
	}

	vales, total, err := h.srv.Listar(r.Context(), filtro)
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, map[string]any{
		"itens":    vales,
		"total":    total,
		"limite":   filtro.Limite,
		"deslocar": filtro.Deslocar,
	})
	return nil
}

func (h *ServicoHTTP) resumo(w http.ResponseWriter, r *http.Request) error {
	contagem, err := h.srv.ContarPorEstado(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, contagem)
	return nil
}

func (h *ServicoHTTP) obter(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	vale, err := h.srv.Obter(r.Context(), id)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusOK, vale)
	return nil
}

func (h *ServicoHTTP) gerar(w http.ResponseWriter, r *http.Request) error {
	var lote GerarLote
	if err := system.DecodificarJSON(r, &lote); err != nil {
		return err
	}
	resultado, err := h.srv.Gerar(r.Context(), lote)
	if err != nil {
		return traduzirErro(err)
	}
	system.ResponderJSON(w, http.StatusCreated, resultado)
	return nil
}

func (h *ServicoHTTP) revogar(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}
	if err := h.srv.Revogar(r.Context(), id); err != nil {
		return traduzirErro(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func traduzirErro(err error) error {
	switch {
	case err == ErrNaoEncontrado:
		return system.ErroNaoEncontrado("Vale")
	case err == ErrJaExiste:
		return system.ErroConflito("Já existe um vale com este código")
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "inativo"), strings.Contains(msg, "quantidade"),
		strings.Contains(msg, "plano"), strings.Contains(msg, "prefixo"),
		strings.Contains(msg, "validade"), strings.Contains(msg, "código"):
		return system.ErroValidacao("", capitalizar(msg))
	}
	return system.ErroInterno(msg)
}

func intDeQuery(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func capitalizar(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
