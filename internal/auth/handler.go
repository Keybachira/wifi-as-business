package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Keybachira/wifi-as-business/internal/system"
)

type chaveContexto string

const chaveConta chaveContexto = "conta"

// contaDoContexto devolve a conta colocada pelo middleware.
func contaDoContexto(ctx context.Context) (Conta, bool) {
	c, ok := ctx.Value(chaveConta).(Conta)
	return c, ok
}

// ServicoHTTP expõe o módulo de autenticação sobre HTTP.
type ServicoHTTP struct {
	srv *Serviço
}

// NovoServicoHTTP cria o adaptador HTTP.
func NovoServicoHTTP(srv *Serviço) *ServicoHTTP { return &ServicoHTTP{srv: srv} }

// Rotas regista os caminhos do módulo no mux.
// exigir é o middleware de autenticação aplicado às rotas protegidas.
func (h *ServicoHTTP) Rotas(mux *http.ServeMux, exigir func(http.Handler) http.Handler) {
	// Rotas públicas
	mux.Handle("POST /api/v1/auth/login", system.Adaptador(h.login))
	mux.Handle("GET /api/v1/auth/saude", system.Adaptador(h.saudePublica))

	// Rotas protegidas
	mux.Handle("POST /api/v1/auth/logout", exigir(system.Adaptador(h.logout)))
	mux.Handle("GET /api/v1/auth/sessao", exigir(system.Adaptador(h.sessaoAtual)))
	mux.Handle("GET /api/v1/contas", exigir(system.Adaptador(h.listarContas)))
	mux.Handle("POST /api/v1/contas", exigir(system.Adaptador(h.criarConta)))
	mux.Handle("PATCH /api/v1/contas/{id}/estado", exigir(system.Adaptador(h.alterarEstado)))
	mux.Handle("PATCH /api/v1/contas/{id}/papel", exigir(system.Adaptador(h.alterarPapel)))
}

func (h *ServicoHTTP) saudePublica(w http.ResponseWriter, r *http.Request) error {
	system.ResponderJSON(w, http.StatusOK, map[string]any{
		"estado":  "ok",
		"servico": "WiFi Controller",
		"versao":  "1.0.0",
		"horario": time.Now().UTC(),
	})
	return nil
}

func (h *ServicoHTTP) login(w http.ResponseWriter, r *http.Request) error {
	var cred Credenciais
	if err := system.DecodificarJSON(r, &cred); err != nil {
		return err
	}

	resposta, err := h.srv.IniciarSessao(r.Context(), cred, r.UserAgent(), enderecoRemoto(r))
	if err != nil {
		if err == ErrCredenciaisInvalidas {
			return system.ErroNaoAutorizado("Email ou senha incorretos")
		}
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, resposta)
	return nil
}

func (h *ServicoHTTP) logout(w http.ResponseWriter, r *http.Request) error {
	token := ExtrairTokenBearer(r.Header.Get("Authorization"))
	if err := h.srv.TerminarSessao(r.Context(), token); err != nil {
		return system.ErroInterno(err.Error())
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *ServicoHTTP) sessaoAtual(w http.ResponseWriter, r *http.Request) error {
	conta, ok := contaDoContexto(r.Context())
	if !ok {
		return system.ErroNaoAutorizado("Sessão inválida")
	}
	system.ResponderJSON(w, http.StatusOK, conta)
	return nil
}

func (h *ServicoHTTP) listarContas(w http.ResponseWriter, r *http.Request) error {
	contas, err := h.srv.Listar(r.Context())
	if err != nil {
		return system.ErroInterno(err.Error())
	}
	system.ResponderJSON(w, http.StatusOK, contas)
	return nil
}

func (h *ServicoHTTP) criarConta(w http.ResponseWriter, r *http.Request) error {
	contaAtual, _ := contaDoContexto(r.Context())
	if !contaAtual.PodeGerirContas() {
		return system.ErroProibido("Apenas o dono pode criar contas")
	}

	var dados CriarConta
	if err := system.DecodificarJSON(r, &dados); err != nil {
		return err
	}

	conta, err := h.srv.CriarConta(r.Context(), dados)
	if err != nil {
		switch err {
		case ErrEmailEmUso:
			return system.ErroConflito("Este email já está registado")
		case ErrContaNaoEncontrada:
			return system.ErroNaoEncontrado("Conta")
		}
		return system.ErroValidacao("", err.Error())
	}
	system.ResponderJSON(w, http.StatusCreated, conta)
	return nil
}

func (h *ServicoHTTP) alterarEstado(w http.ResponseWriter, r *http.Request) error {
	contaAtual, _ := contaDoContexto(r.Context())
	if !contaAtual.PodeGerirContas() {
		return system.ErroProibido("Apenas o dono pode gerir contas")
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}

	var corpo struct {
		Ativo bool `json:"ativo"`
	}
	if err := system.DecodificarJSON(r, &corpo); err != nil {
		return err
	}

	if err := h.srv.AlterarEstado(r.Context(), id, corpo.Ativo); err != nil {
		if err == ErrContaNaoEncontrada {
			return system.ErroNaoEncontrado("Conta")
		}
		return system.ErroConflito(err.Error())
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *ServicoHTTP) alterarPapel(w http.ResponseWriter, r *http.Request) error {
	contaAtual, _ := contaDoContexto(r.Context())
	if !contaAtual.PodeGerirContas() {
		return system.ErroProibido("Apenas o dono pode gerir contas")
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return system.ErroValidacao("id", "Identificador inválido")
	}

	var corpo struct {
		Papel Papel `json:"papel"`
	}
	if err := system.DecodificarJSON(r, &corpo); err != nil {
		return err
	}

	if err := h.srv.AlterarPapel(r.Context(), id, corpo.Papel); err != nil {
		if err == ErrContaNaoEncontrada {
			return system.ErroNaoEncontrado("Conta")
		}
		return system.ErroValidacao("papel", err.Error())
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ExigirAutenticacao valida o token Bearer e coloca a conta no contexto.
func (h *ServicoHTTP) ExigirAutenticacao(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ExtrairTokenBearer(r.Header.Get("Authorization"))
		if token == "" {
			system.ResponderErro(w, system.ErroNaoAutorizado("Token de acesso em falta"))
			return
		}

		conta, err := h.srv.ValidarToken(r.Context(), token)
		if err != nil {
			system.ResponderErro(w, system.ErroNaoAutorizado("Sessão inválida ou expirada"))
			return
		}

		proximo.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), chaveConta, conta)))
	})
}

// enderecoRemoto devolve o IP do cliente, respeitando o cabeçalho X-Forwarded-For.
func enderecoRemoto(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}
