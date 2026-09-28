// Package mikrotik integra o controlador com routers MikroTik via API.
//
// A integração usa a API REST oficial (RouterOS 7.x). Quando a credencial
// não está configurada, o cliente devolve um erro claro em vez de falhar
// silenciosamente, e o restante sistema continua a funcionar.
package mikrotik

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Erros do cliente MikroTik.
var (
	ErrNaoConfigurado = errors.New("o router MikroTik não está configurado")
	ErrSemAcesso      = errors.New("o router recusou a autenticação")
)

// Estado de um interface no router.
type EstadoInterface struct {
	Nome      string  `json:"nome"`
	Ativa     bool    `json:"ativa"`
	Comment   string  `json:"comment"`
	RXMbps    float64 `json:"rx_mbps"`
	TXMbps    float64 `json:"tx_mbps"`
	Clientes  int     `json:"clientes"`
}

// Regra de limitação aplicada a um cliente.
type RegraLimitacao struct {
	EnderecoMAC   string `json:"endereco_mac"`
	DownloadKbps  int    `json:"download_kbps"`
	UploadKbps    int    `json:"upload_kbps"`
	LimiteMinutos int    `json:"limite_minutos"`
	Ativa         bool   `json:"ativa"`
}

// Cliente fala com a API REST de um router MikroTik.
type Cliente struct {
	endereco string
	usuario  string
	senha    string
	http     *http.Client
}

// NovoCliente constrói o cliente a partir da configuração.
func NovoCliente(endereco, usuario, senha string) *Cliente {
	return &Cliente{
		endereco: strings.TrimSuffix(endereco, "/"),
		usuario:  usuario,
		senha:    senha,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // routers usam certificados auto-assinados
			},
		},
	}
}

// Configurado indica se existem credenciais para falar com o router.
func (c *Cliente) Configurado() bool {
	return c.endereco != "" && c.usuario != "" && c.senha != ""
}

// Endereco devolve o endereço configurado (usado no ecrã de definições).
func (c *Cliente) Endereco() string { return c.endereco }

// pedido executa uma chamada autenticada à API REST.
func (c *Cliente) pedido(ctx context.Context, metodo, caminho string, corpo any) ([]byte, error) {
	if !c.Configurado() {
		return nil, ErrNaoConfigurado
	}

	var leitor io.Reader
	if corpo != nil {
		dados, err := json.Marshal(corpo)
		if err != nil {
			return nil, err
		}
		leitor = strings.NewReader(string(dados))
	}

	pedido, err := http.NewRequestWithContext(ctx, metodo, c.endereco+"/rest"+caminho, leitor)
	if err != nil {
		return nil, fmt.Errorf("não foi possível construir o pedido: %w", err)
	}
	pedido.SetBasicAuth(c.usuario, c.senha)
	if corpo != nil {
		pedido.Header.Set("Content-Type", "application/json")
	}

	resposta, err := c.http.Do(pedido)
	if err != nil {
		return nil, fmt.Errorf("o router não respondeu: %w", err)
	}
	defer resposta.Body.Close()

	if resposta.StatusCode == http.StatusUnauthorized || resposta.StatusCode == http.StatusForbidden {
		return nil, ErrSemAcesso
	}
	if resposta.StatusCode >= 400 {
		return nil, fmt.Errorf("o router devolveu o estado %d", resposta.StatusCode)
	}
	return io.ReadAll(resposta.Body)
}

// ListarInterfaces devolve o estado das interfaces do router.
func (c *Cliente) ListarInterfaces(ctx context.Context) ([]EstadoInterface, error) {
	dados, err := c.pedido(ctx, http.MethodGet, "/interface", nil)
	if err != nil {
		return nil, err
	}
	var lista []EstadoInterface
	if err := json.Unmarshal(dados, &lista); err != nil {
		return nil, fmt.Errorf("resposta inválida do router: %w", err)
	}
	return lista, nil
}

// AtivarCliente liga a navegação de um endereço MAC.
func (c *Cliente) AtivarCliente(ctx context.Context, enderecoMAC string, limiteMinutos int) error {
	comentario := fmt.Sprintf("wificontroller:%d", limiteMinutos)
	_, err := c.pedido(ctx, http.MethodPut,
		fmt.Sprintf("/ip/hotspot/active?=.id=%s", enderecoMAC),
		map[string]string{"comment": comentario})
	return err
}

// DesligarCliente remove a sessão de um endereço MAC.
func (c *Cliente) DesligarCliente(ctx context.Context, enderecoMAC string) error {
	_, err := c.pedido(ctx, http.MethodDelete,
		"/ip/hotspot/active?=.id="+enderecoMAC, nil)
	return err
}

// AplicarLimite define a velocidade de um cliente na fila do router.
func (c *Cliente) AplicarLimite(ctx context.Context, r RegraLimitacao) error {
	corpo := map[string]any{
		"address":    r.EnderecoMAC,
		"rate-limit": fmt.Sprintf("%dk/%dk", r.DownloadKbps, r.UploadKbps),
		"comment":    "wificontroller",
	}
	_, err := c.pedido(ctx, http.MethodPut,
		"/ip/hotspot/user?=.id="+r.EnderecoMAC, corpo)
	return err
}
