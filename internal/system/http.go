package system

import (
	"encoding/json"
	"io"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// ErroAPI representa um erro de negócio com código HTTP e mensagem em português.
type ErroAPI struct {
	Estado   int    `json:"-"`
	Codigo   string `json:"codigo"`
	Mensagem string `json:"mensagem"`
	Campo    string `json:"campo,omitempty"`
}

func (e *ErroAPI) Error() string { return e.Mensagem }

// Construtores de erros comuns.
func ErroValidacao(campo, mensagem string) *ErroAPI {
	return &ErroAPI{Estado: http.StatusBadRequest, Codigo: "validacao_invalida", Mensagem: mensagem, Campo: campo}
}

func ErroNaoEncontrado(recurso string) *ErroAPI {
	return &ErroAPI{Estado: http.StatusNotFound, Codigo: "nao_encontrado", Mensagem: recurso + " não encontrado"}
}

func ErroConflito(mensagem string) *ErroAPI {
	return &ErroAPI{Estado: http.StatusConflict, Codigo: "conflito", Mensagem: mensagem}
}

func ErroNaoAutorizado(mensagem string) *ErroAPI {
	return &ErroAPI{Estado: http.StatusUnauthorized, Codigo: "nao_autorizado", Mensagem: mensagem}
}

func ErroProibido(mensagem string) *ErroAPI {
	return &ErroAPI{Estado: http.StatusForbidden, Codigo: "proibido", Mensagem: mensagem}
}

func ErroInterno(mensagem string) *ErroAPI {
	return &ErroAPI{Estado: http.StatusInternalServerError, Codigo: "erro_interno", Mensagem: mensagem}
}

// ResponderJSON escreve uma resposta JSON com o estado HTTP indicado.
func ResponderJSON(w http.ResponseWriter, estado int, dados any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(estado)
	if dados == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(dados); err != nil {
		slog.Error("falha ao escrever resposta JSON", "erro", err)
	}
}

// ResponderErro converte qualquer erro numa resposta JSON padronizada.
func ResponderErro(w http.ResponseWriter, err error) {
	var apiErr *ErroAPI
	if !errors.As(err, &apiErr) {
		apiErr = ErroInterno("Ocorreu um erro inesperado")
	}
	if apiErr.Estado >= http.StatusInternalServerError {
		slog.Error("erro interno", "erro", err)
	}
	ResponderJSON(w, apiErr.Estado, map[string]any{
		"erro": map[string]any{
			"codigo":   apiErr.Codigo,
			"mensagem": apiErr.Mensagem,
			"campo":    apiErr.Campo,
		},
	})
}

// Manipulador é um http.HandlerFunc que pode devolver erro.
type Manipulador func(http.ResponseWriter, *http.Request) error

// Adaptador converte um Manipulador num http.Handler, lidando com erros.
func Adaptador(h Manipulador) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			ResponderErro(w, err)
		}
	}
}

// DecodificarJSON lê e valida o corpo JSON de um pedido.
// Campos desconhecidos são rejeitados para apanhar erros de digitação nos clientes.
func DecodificarJSON(r *http.Request, destino any) error {
	if r.Body == nil {
		return ErroValidacao("", "Corpo do pedido vazio")
	}
	// Limita o corpo a 1 MB para evitar consumo excessivo de memória.
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(destino); err != nil {
		return ErroValidacao("", "JSON inválido: "+err.Error())
	}
	return nil
}

// ConfigurarRegisto cria o logger estruturado da aplicação.
func ConfigurarRegisto(nivel slog.Level) *slog.Logger {
	saida := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: nivel})
	return slog.New(saida)
}

// NivelPorAmbiente traduz o ambiente para o nível de log correspondente.
func NivelPorAmbiente(ambiente string) slog.Level {
	if ambiente == "producao" {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}

// HoraAtual retorna o instante atual em UTC truncado ao segundo.
// Centralizar isto evita variações de formatação de tempo entre camadas.
func HoraAtual() time.Time { return time.Now().UTC().Truncate(time.Second) }
