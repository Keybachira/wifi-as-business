// Package pfsense integra o controlador com firewalls pfSense.
//
// O pfSense é a alternativa ao MikroTik: o mesmo conjunto de operações
// (aplicar limite de banda e desligar um cliente) é traduzido para as
// tabelas de ARP, os aliases e o traffic shaper do próprio pfSense.
//
// A integração usa a API REST do pfSense (chave de API ou utilizador/senha)
// e, tal como o cliente MikroTik, desativa-se sozinha quando não há
// credenciais configuradas. O MikroTik continua a ser o router principal;
// este módulo só entra em ação quando está explicitamente configurado.
package pfsense

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

// Erros do cliente pfSense.
var (
	ErrNaoConfigurado   = errors.New("o firewall pfSense não está configurado")
	ErrSemAcesso        = errors.New("o pfSense recusou a autenticação")
	ErrNaoEncontrado    = errors.New("o endereço não foi encontrado no pfSense")
	ErrAliasVazio       = errors.New("o alias do pfSense não está definido (WIFIC_PFSENSE_ALIAS)")
	ErrAliasInexistente = errors.New("o alias configurado não existe no pfSense")
)

// Estado de uma interface no pfSense.
type EstadoInterface struct {
	Nome      string `json:"nome"`
	Ativa     bool   `json:"ativa"`
	Descricao string `json:"descricao"`
	RXBytes   int64  `json:"rx_bytes"`
	TXBytes   int64  `json:"tx_bytes"`
}

// EntradaARP é uma linha da tabela ARP do pfSense.
type EntradaARP struct {
	EnderecoIP  string `json:"ip"`
	EnderecoMAC string `json:"mac"`
	Interface   string `json:"ifname"`
}

// Cliente fala com a API REST de um firewall pfSense.
//
// A autenticação prefere a chave de API (cabeçalho pfApiKey); quando não há
// chave, recorre a utilizador e senha por Basic Auth. Ambas as formas são
// aceites pelo pfSense e a segunda é a única disponível sem subscription.
type Cliente struct {
	endereco string
	usuario  string
	senha    string
	chaveAPI string
	// alias é o nome do alias do pfSense onde ficam os IPs autorizados.
	// É removido o IP ao desligar o cliente, o que corta a navegação.
	alias string
	http  *http.Client
}

// NovoCliente constrói o cliente a partir da configuração.
//
// endereco deve incluir o esquema e a porta (por exemplo
// "https://192.168.1.1:443"). A chave de API tem precedência sobre
// utilizador/senha quando ambas estão definidas.
func NovoCliente(endereco, usuario, senha, chaveAPI, alias string) *Cliente {
	return &Cliente{
		endereco: strings.TrimSuffix(endereco, "/"),
		usuario:  usuario,
		senha:    senha,
		chaveAPI: chaveAPI,
		alias:    strings.TrimSpace(alias),
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				// O pfSense traz um certificado auto-assinado por omissão.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// Configurado indica se existem credenciais para falar com o firewall.
func (c *Cliente) Configurado() bool {
	if c.endereco == "" {
		return false
	}
	return c.chaveAPI != "" || (c.usuario != "" && c.senha != "")
}

// Endereco devolve o endereço configurado (usado no ecrã de definições).
func (c *Cliente) Endereco() string { return c.endereco }

// Alias devolve o nome do alias usado para cortar a navegação.
func (c *Cliente) Alias() string { return c.alias }

// pedido executa uma chamada autenticada à API REST do pfSense.
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

	pedido, err := http.NewRequestWithContext(ctx, metodo, c.endereco+"/api"+caminho, leitor)
	if err != nil {
		return nil, fmt.Errorf("não foi possível construir o pedido: %w", err)
	}
	if c.chaveAPI != "" {
		pedido.Header.Set("pfApiKey", c.chaveAPI)
	} else {
		pedido.SetBasicAuth(c.usuario, c.senha)
	}
	if corpo != nil {
		pedido.Header.Set("Content-Type", "application/json")
	}

	resposta, err := c.http.Do(pedido)
	if err != nil {
		return nil, fmt.Errorf("o pfSense não respondeu: %w", err)
	}
	defer resposta.Body.Close()

	if resposta.StatusCode == http.StatusUnauthorized || resposta.StatusCode == http.StatusForbidden {
		return nil, ErrSemAcesso
	}
	if resposta.StatusCode >= 400 {
		return nil, fmt.Errorf("o pfSense devolveu o estado %d", resposta.StatusCode)
	}
	return io.ReadAll(resposta.Body)
}

// ListarInterfaces devolve o estado das interfaces do firewall.
func (c *Cliente) ListarInterfaces(ctx context.Context) ([]EstadoInterface, error) {
	dados, err := c.pedido(ctx, http.MethodGet, "/v1/interface/status", nil)
	if err != nil {
		return nil, err
	}

	// A resposta real é {"status": {"em0": {...}}}; normalizamos para a lista.
	var embrulho struct {
		Status map[string]struct {
			Descricao    string `json:"descr"`
			Ativa        bool   `json:"up"`
			RXBytes      int64  `json:"bytesrecv"`
			TXBytes      int64  `json:"bytessent"`
			TrocaRXBytes int64  `json:"bytesrcv"`
		} `json:"status"`
	}
	if err := json.Unmarshal(dados, &embrulho); err != nil {
		return nil, fmt.Errorf("resposta inválida do pfSense: %w", err)
	}

	lista := make([]EstadoInterface, 0, len(embrulho.Status))
	for nome, i := range embrulho.Status {
		lista = append(lista, EstadoInterface{
			Nome:      nome,
			Ativa:     i.Ativa,
			Descricao: i.Descricao,
			RXBytes:   i.RXBytes + i.TrocaRXBytes,
			TXBytes:   i.TXBytes,
		})
	}
	return lista, nil
}

// ResolverIP traduz um endereço MAC para o IP visto na tabela ARP.
func (c *Cliente) ResolverIP(ctx context.Context, enderecoMAC string) (string, error) {
	dados, err := c.pedido(ctx, http.MethodGet, "/v1/diagnostics/arp/table", nil)
	if err != nil {
		return "", err
	}

	var tabela struct {
		ARP []EntradaARP `json:"arp"`
	}
	if err := json.Unmarshal(dados, &tabela); err != nil {
		return "", fmt.Errorf("resposta inválida do pfSense: %w", err)
	}

	alvo := normalizarMAC(enderecoMAC)
	for _, linha := range tabela.ARP {
		if normalizarMAC(linha.EnderecoMAC) == alvo {
			return linha.EnderecoIP, nil
		}
	}
	return "", ErrNaoEncontrado
}

// DesligarCliente corta a navegação de um endereço MAC.
//
// O pfSense não expõe sessões de clientes fora do captive portal, por isso a
// desconexão é feita removendo o IP do alias que autoriza os clientes do
// controlador. É essa regra — e não o estado interno do pfSense — que
// governa quem navega.
func (c *Cliente) DesligarCliente(ctx context.Context, enderecoMAC string) error {
	if c.alias == "" {
		return ErrAliasVazio
	}

	ip, err := c.ResolverIP(ctx, enderecoMAC)
	if err != nil {
		// Sem IP conhecido não há nada a cortar; não é uma falha do pfSense.
		if errors.Is(err, ErrNaoEncontrado) {
			return nil
		}
		return err
	}
	return c.removerDoAlias(ctx, ip)
}

// AplicarLimite cria uma regra de traffic shaper para o endereço indicado.
//
// A API do pfSense cria regras por formulário, com os mesmos nomes de campo
// que a página web do firewall. A pipe é montada com os limites do plano.
func (c *Cliente) AplicarLimite(ctx context.Context, enderecoIP string, downloadKbps, uploadKbps int) error {
	formulario := map[string]string{
		"interface":   "lan",
		"source":      enderecoIP + "/32",
		"destination": "any",
		"protocol":    "any",
		"description": "wificontroller",
		"direction":   "in",
		"pipe":        pipeShaper(downloadKbps, uploadKbps),
		"enabled":     "yes",
		"update":      "save",
	}
	_, err := c.pedido(ctx, http.MethodPost,
		"/v1/firewall/traffic_shaper/rules/add", formulario)
	if err != nil {
		return fmt.Errorf("não foi possível aplicar o limite no pfSense: %w", err)
	}
	return nil
}

// removerDoAlias apaga um endereço do alias configurado.
func (c *Cliente) removerDoAlias(ctx context.Context, enderecoIP string) error {
	leitura, err := c.pedido(ctx, http.MethodGet, "/v1/firewall/alias", nil)
	if err != nil {
		return err
	}

	var atuais struct {
		Alias []struct {
			Nome     string   `json:"name"`
			Conteudo []string `json:"address"`
		} `json:"alias"`
	}
	if err := json.Unmarshal(leitura, &atuais); err != nil {
		return fmt.Errorf("resposta inválida do pfSense: %w", err)
	}

	conteudo := []string{}
	encontrado := false
	for _, a := range atuais.Alias {
		if a.Nome != c.alias {
			continue
		}
		encontrado = true
		for _, item := range a.Conteudo {
			if !strings.EqualFold(item, enderecoIP) {
				conteudo = append(conteudo, item)
			}
		}
	}
	if !encontrado {
		return fmt.Errorf("%w: %q", ErrAliasInexistente, c.alias)
	}

	_, err = c.pedido(ctx, http.MethodPost,
		fmt.Sprintf("/v1/firewall/alias/set/%s", c.alias),
		map[string]any{
			"name":    c.alias,
			"type":    "host",
			"address": conteudo,
		})
	if err != nil {
		return fmt.Errorf("não foi possível atualizar o alias do pfSense: %w", err)
	}
	return nil
}

// pipeShaper monta a pipe de banda com os limites do plano.
//
// O pfSense usa bits por segundo; o controlador trabalha em kbps, por isso a
// conversão é feita aqui. Uma velocidade de 0 significa "sem limite".
func pipeShaper(downloadKbps, uploadKbps int) string {
	normalizar := func(kbps int) string {
		if kbps <= 0 {
			return "0"
		}
		return fmt.Sprintf("%dkbit/s", kbps)
	}
	return fmt.Sprintf("(%s %s)", normalizar(downloadKbps), normalizar(uploadKbps))
}

// normalizarMAC põe o endereço MAC na forma canónica, sem separadores e em
// minúsculas, para comparar com o que a tabela ARP devolve.
func normalizarMAC(valor string) string {
	var b strings.Builder
	for _, r := range valor {
		if r == ':' || r == '-' || r == '.' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
