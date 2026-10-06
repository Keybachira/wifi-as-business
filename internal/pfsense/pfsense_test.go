package pfsense

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// servidorFalso simula a API REST do pfSense.
type servidorFalso struct {
	ultimaRota  string
	ultimoCorpo string
	cabecalho   string
	respostas   map[string]any
}

func (s *servidorFalso) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.ultimaRota = r.URL.Path
	s.ultimoCorpo = string(body)
	s.cabecalho = r.Header.Get("pfApiKey")

	if resposta, ok := s.respostas[r.URL.Path]; ok {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resposta)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"status":"not found"}`))
}

func TestConfiguradoExigeEnderecoECredencial(t *testing.T) {
	casos := []struct {
		nome    string
		cliente *Cliente
		quer    bool
	}{
		{"sem nada", NovoCliente("", "", "", "", ""), false},
		{"endereco sem credencial", NovoCliente("https://1.1.1.1", "", "", "", ""), false},
		{"so utilizador", NovoCliente("https://1.1.1.1", "admin", "", "", ""), false},
		{"utilizador e senha", NovoCliente("https://1.1.1.1", "admin", "x", "", ""), true},
		{"chave de API", NovoCliente("https://1.1.1.1", "", "", "k", ""), true},
	}
	for _, c := range casos {
		if got := c.cliente.Configurado(); got != c.quer {
			t.Fatalf("%s: esperava %v, recebeu %v", c.nome, c.quer, got)
		}
	}
}

func TestChaveAPITemPrecedencia(t *testing.T) {
	srv := &servidorFalso{respostas: map[string]any{
		"/api/v1/interface/status": map[string]any{"status": map[string]any{}},
	}}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "senha", "chave-secreta", "convidados")
	if _, err := c.ListarInterfaces(context.Background()); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if srv.cabecalho != "chave-secreta" {
		t.Fatalf("esperava o cabeçalho pfApiKey, recebeu %q", srv.cabecalho)
	}
}

func TestListarInterfacesNormalizaResposta(t *testing.T) {
	srv := &servidorFalso{respostas: map[string]any{
		"/api/v1/interface/status": map[string]any{
			"status": map[string]any{
				"em0": map[string]any{"descr": "WAN", "up": true, "bytesrecv": 100, "bytessent": 50},
				"em1": map[string]any{"descr": "LAN", "up": false, "bytesrecv": 10, "bytessent": 5},
			},
		},
	}}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "senha", "", "convidados")
	lista, err := c.ListarInterfaces(context.Background())
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("esperava 2 interfaces, recebeu %d", len(lista))
	}
	// A ordem do mapa é aleatória em Go; confirma-se por conteúdo.
	var wan EstadoInterface
	for _, i := range lista {
		if i.Nome == "em0" {
			wan = i
		}
	}
	if !wan.Ativa || wan.Descricao != "WAN" || wan.RXBytes != 100 {
		t.Fatalf("interface mal normalizada: %+v", wan)
	}
}

func TestCredenciaisRecusadas(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "errada", "", "convidados")
	if _, err := c.ListarInterfaces(context.Background()); err == nil {
		t.Fatal("esperava erro com credenciais recusadas")
	}
}

// O pfSense usa bits por segundo; 0 significa sem limite.
func TestPipeShaper(t *testing.T) {
	casos := []struct {
		down, up int
		quer     string
	}{
		{5000, 2500, "(5000kbit/s 2500kbit/s)"},
		{0, 0, "(0 0)"},
		{1000, 0, "(1000kbit/s 0)"},
	}
	for _, c := range casos {
		if got := pipeShaper(c.down, c.up); got != c.quer {
			t.Fatalf("pipeShaper(%d,%d): esperava %q, recebeu %q", c.down, c.up, c.quer, got)
		}
	}
}

func TestResolverIPNormalizaMAC(t *testing.T) {
	srv := &servidorFalso{respostas: map[string]any{
		"/api/v1/diagnostics/arp/table": map[string]any{
			"arp": []map[string]string{
				{"ip": "192.168.1.50", "mac": "dc:a6:32:01:77:2e", "ifname": "lan"},
			},
		},
	}}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "senha", "", "convidados")
	// O cliente envia o MAC com maiúsculas e dois pontos; o ARP devolve minúsculas.
	ip, err := c.ResolverIP(context.Background(), "DC:A6:32:01:77:2E")
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if ip != "192.168.1.50" {
		t.Fatalf("esperava 192.168.1.50, recebeu %q", ip)
	}
}

// Desligar remove o IP do alias, mantendo os restantes clientes autorizados.
func TestDesligarRemoveApenasODestinatario(t *testing.T) {
	srv := &servidorFalso{respostas: map[string]any{
		"/api/v1/diagnostics/arp/table": map[string]any{
			"arp": []map[string]string{{"ip": "192.168.1.50", "mac": "dc:a6:32:01:77:2e"}},
		},
		"/api/v1/firewall/alias": map[string]any{
			"alias": []map[string]any{
				{"name": "convidados", "address": []string{"192.168.1.50", "192.168.1.51"}},
				{"name": "outro", "address": []string{"10.0.0.1"}},
			},
		},
		"/api/v1/firewall/alias/set/convidados": map[string]any{"saved": true},
	}}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "senha", "", "convidados")
	if err := c.DesligarCliente(context.Background(), "DC:A6:32:01:77:2E"); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}

	if srv.ultimaRota != "/api/v1/firewall/alias/set/convidados" {
		t.Fatalf("esperava escrever no alias, foi para %q", srv.ultimaRota)
	}
	if !strings.Contains(srv.ultimoCorpo, "192.168.1.51") {
		t.Fatalf("o outro cliente devia manter-se: %s", srv.ultimoCorpo)
	}
	if strings.Contains(srv.ultimoCorpo, "192.168.1.50") {
		t.Fatalf("o cliente desligado devia sair: %s", srv.ultimoCorpo)
	}
	if strings.Contains(srv.ultimoCorpo, "10.0.0.1") {
		t.Fatalf("não devia tocar noutros aliases: %s", srv.ultimoCorpo)
	}
}

// Um MAC que não existe na tabela ARP não é erro: não há navegação a cortar.
func TestDesligarMACDesconhecidoNaoFalha(t *testing.T) {
	srv := &servidorFalso{respostas: map[string]any{
		"/api/v1/diagnostics/arp/table": map[string]any{"arp": []map[string]string{}},
	}}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "senha", "", "convidados")
	if err := c.DesligarCliente(context.Background(), "AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("esperava sucesso, recebeu %v", err)
	}
}

func TestDesligarSemAliasFalaClaro(t *testing.T) {
	srv := &servidorFalso{respostas: map[string]any{}}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "senha", "", "")
	if err := c.DesligarCliente(context.Background(), "AA:BB"); !errors.Is(err, ErrAliasVazio) {
		t.Fatalf("esperava ErrAliasVazio, recebeu %v", err)
	}
}

func TestAliasInexistente(t *testing.T) {
	srv := &servidorFalso{respostas: map[string]any{
		"/api/v1/diagnostics/arp/table": map[string]any{
			"arp": []map[string]string{{"ip": "192.168.1.50", "mac": "dc:a6:32:01:77:2e"}},
		},
		"/api/v1/firewall/alias": map[string]any{"alias": []map[string]any{}},
	}}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	c := NovoCliente(ts.URL, "admin", "senha", "", "convidados")
	if err := c.DesligarCliente(context.Background(), "DC:A6:32:01:77:2E"); !errors.Is(err, ErrAliasInexistente) {
		t.Fatalf("esperava ErrAliasInexistente, recebeu %v", err)
	}
}
