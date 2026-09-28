// Package radius implementa a validação de acessos no formato RADIUS.
//
// O servidor expõe um Authenticator que valida credenciais contra um
// Fornecedor de credenciais — normalmente o módulo de vales. Esta separação
// permite usar um RADIUS externo (FreeRADIUS, MikroTik) sem alterar o código.
package radius

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Resultados possíveis de uma autenticação.
const (
	ResultadoAceite     = "aceite"
	ResultadoRejeitado  = "rejeitado"
	ResultadoExpirado   = "expirado"
	ResultadoDesconhecido = "desconhecido"
)

// Erros do módulo.
var (
	ErrNaoConfigurado = errors.New("o serviço RADIUS não está configurado")
	CredencialInvalida = errors.New("credenciais inválidas")
)

// Fornecedor valida as credenciais de um acesso.
// A implementação real (vales) é injetada pelo arranque da aplicação.
type Fornecedor interface {
	// Validar devolve nil quando a credencial é aceite.
	Validar(ctx context.Context, utilizador, segredo string) (InfoAcesso, error)
}

// InfoAcesso descreve o que o utilizador ganha ao autenticar.
type InfoAcesso struct {
	Utilizador      string
	DownloadKbps    int
	UploadKbps      int
	LimiteMinutos   int
	LimiteDadosMB   int
	MaxDispositivos int
	SessionID       string
}

// Pacote autenticado é a resposta enviada ao cliente RADIUS.
type PacoteAutenticado struct {
	Utilizador    string `json:"utilizador"`
	SessionID     string `json:"session_id"`
	DownloadKbps  int    `json:"download_kbps"`
	UploadKbps    int    `json:"upload_kbps"`
	LimiteMinutos int    `json:"limite_minutos"`
	ValidoAte     time.Time `json:"valido_ate"`
}

// Serviço coordena a validação de credenciais e emite sessões temporárias.
type Serviço struct {
	mu         sync.RWMutex
	fornecedor Fornecedor
	ativo      bool
	porta      int
	endereco   string
}

// NovoServico cria o serviço RADIUS.
func NovoServico(porta int, endereco string) *Serviço {
	return &Serviço{porta: porta, endereco: endereco}
}

// ComFornecedor define a fonte de verdade das credenciais.
func (s *Serviço) ComFornecedor(f Fornecedor) *Serviço {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fornecedor = f
	s.ativo = f != nil
	return s
}

// Ativo indica se há um fornecedor de credenciais configurado.
func (s *Serviço) Ativo() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ativo
}

// Endereco devolve o endereço onde o serviço escuta.
func (s *Serviço) Endereco() string { return s.endereco + ":" + strconv.Itoa(s.porta) }

// Autenticar valida as credenciais e devolve o pacote de sessão.
func (s *Serviço) Autenticar(ctx context.Context, utilizador, segredo string) (PacoteAutenticado, error) {
	s.mu.RLock()
	f := s.fornecedor
	ativo := s.ativo
	s.mu.RUnlock()

	if !ativo || f == nil {
		return PacoteAutenticado{}, ErrNaoConfigurado
	}

	utilizador = strings.TrimSpace(utilizador)
	segredo = strings.ToUpper(strings.TrimSpace(segredo))
	if utilizador == "" || segredo == "" {
		return PacoteAutenticado{}, CredencialInvalida
	}

	info, err := f.Validar(ctx, utilizador, segredo)
	if err != nil {
		return PacoteAutenticado{}, err
	}

	return PacoteAutenticado{
		Utilizador:    info.Utilizador,
		SessionID:     info.SessionID,
		DownloadKbps:  info.DownloadKbps,
		UploadKbps:    info.UploadKbps,
		LimiteMinutos: info.LimiteMinutos,
		ValidoAte:     time.Now().UTC().Add(time.Duration(info.LimiteMinutos) * time.Minute),
	}, nil
}
