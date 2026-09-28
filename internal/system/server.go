package system

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Servidor agrega as dependências partilhadas por todos os módulos.
type Servidor struct {
	cfg     *Config
	registo *slog.Logger
	http    *http.Server
}

// NovoServidor constrói o servidor com as rotas de API e ficheiros estáticos.
func NovoServidor(cfg *Config, registo *slog.Logger, registarRotas func(*http.ServeMux), pastaEstatica string) *Servidor {
	mux := http.NewServeMux()
	registarRotas(mux)

	if pastaEstatica != "" {
		if info, err := os.Stat(pastaEstatica); err == nil && info.IsDir() {
			mux.Handle("/", ServirSPA(pastaEstatica))
		} else {
			slog.Warn("pasta estática não encontrada, a servir apenas a API", "pasta", pastaEstatica)
		}
	}

	return &Servidor{
		cfg:     cfg,
		registo: registo,
		http: &http.Server{
			Addr:              cfg.EnderecoEscuta(),
			Handler:           middlewareCabecalhos(mux),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

// Iniciar começa a escutar requests. Bloqueia até o servidor terminar.
func (s *Servidor) Iniciar() error {
	s.registo.Info("servidor iniciado", "endereco", s.cfg.EnderecoEscuta(), "ambiente", s.cfg.Ambiente)

	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		s.registo.Info("servidor encerrado com sucesso")
		return nil
	}
	return err
}

// Encerrar desliga o servidor de forma graciosa.
func (s *Servidor) Encerrar(ctx context.Context) error {
	s.registo.Info("a encerrar o servidor...")
	return s.http.Shutdown(ctx)
}

// AguardarSinal bloqueia até receber SIGINT/SIGTERM e devolve o contexto associado.
func AguardarSinal() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sinais
		cancel()
		signal.Stop(sinais)
	}()

	return ctx, func() { signal.Stop(sinais) }
}

// middlewareCabecalhos aplica cabeçalhos de segurança e CORS a todas as respostas.
func middlewareCabecalhos(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if origem := r.Header.Get("Origin"); origem != "" {
			w.Header().Set("Access-Control-Allow-Origin", origem)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		proximo.ServeHTTP(w, r)
	})
}

// ServirSPA serve a aplicação de página única a partir de uma pasta de build.
//
// Comportamento:
//   - rotas de API não passam por aqui (são registadas antes no mux);
//   - ficheiros existentes são servidos com cache longo, porque o nome
//     inclui o hash do conteúdo;
//   - rotas desconhecidas devolvem index.html, para que o roteamento do
//     lado do cliente funcione ao recarregar a página;
//   - caminhos que escapam da pasta (ex.: ../) são recusados.
func ServirSPA(pasta string) http.Handler {
	raiz, err := filepath.Abs(pasta)
	if err != nil {
		raiz = pasta
	}
	ficheiros := http.FileServer(http.Dir(raiz))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Só se servem ficheiros, nunca diretórios (a raiz é tratada abaixo).
		caminho := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if caminho != "" && caminho != "." && !strings.HasPrefix(caminho, "..") {
			completo := filepath.Join(raiz, filepath.FromSlash(caminho))
			if info, err := os.Stat(completo); err == nil && !info.IsDir() {
				// Os assets têm hash no nome, por isso podem ser cacheados para sempre.
				if strings.HasPrefix(caminho, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				ficheiros.ServeHTTP(w, r)
				return
			}
			// Não existe (ou é uma tentativa de travessia): cai no index.html.
		}

		// O index.html nunca pode ser cacheado, senão novos builds não chegam ao cliente.
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filepath.Join(raiz, "index.html"))
	})
}

// WithTimeout limita a duração de um handler.
func WithTimeout(duracao time.Duration, proximo http.Handler) http.Handler {
	return http.TimeoutHandler(proximo, duracao, "{\"erro\":{\"codigo\":\"tempo_esgotado\",\"mensagem\":\"O pedido demorou demasiado tempo\"}}")
}
