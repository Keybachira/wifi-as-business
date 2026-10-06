package limitadores

import (
	"context"
	"errors"
	"testing"
)

// bandaFalsa registra os limites recebidos, trabalhando por IP.
type bandaFalsa struct {
	enderecoIP string
	download   int
	upload     int
	erro       error
}

func (b *bandaFalsa) AplicarLimite(_ context.Context, ip string, down, up int) error {
	b.enderecoIP, b.download, b.upload = ip, down, up
	return b.erro
}

// bandaMACFalsa é a variante MikroTik, que trabalha por endereço MAC.
type bandaMACFalsa struct {
	enderecoMAC string
	erro        error
}

func (b *bandaMACFalsa) AplicarLimite(_ context.Context, mac string, _, _ int) error {
	b.enderecoMAC = mac
	return b.erro
}

// resolvedorFalso traduz um IP para um MAC fixo.
type resolvedorFalso struct {
	mac string
	err error
}

func (r *resolvedorFalso) ResolverIP(_ context.Context, _ string) (string, error) {
	return r.mac, r.err
}

func TestBandaVaziaNaoConfigurada(t *testing.T) {
	c := NovoComBanda(nil)
	if !c.Vazio() {
		t.Fatal("um agregador sem equipamentos devia estar vazio")
	}
	err := c.AplicarLimite(context.Background(), "192.168.1.50", 5000, 2500)
	if !errors.Is(err, NaoConfigurado) {
		t.Fatalf("esperava NaoConfigurado, recebeu %v", err)
	}
}

func TestBandaPorIP(t *testing.T) {
	pfsense := &bandaFalsa{}
	c := NovoComBanda(nil).Adicionar("pfsense", pfsense, true)

	if err := c.AplicarLimite(context.Background(), "192.168.1.50", 5000, 2500); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if pfsense.enderecoIP != "192.168.1.50" {
		t.Fatalf("IP errado: %q", pfsense.enderecoIP)
	}
	if pfsense.download != 5000 || pfsense.upload != 2500 {
		t.Fatalf("limites errados: %d/%d", pfsense.download, pfsense.upload)
	}
}

// O MikroTik recebe o MAC, não o IP: é o resolvedor que faz a tradução.
func TestBandaPorMACTraduzEndereco(t *testing.T) {
	mikrotik := &bandaMACFalsa{}
	c := NovoComBanda(nil).
		AdicionarPorMAC("mikrotik", mikrotik,
			&resolvedorFalso{mac: "DC:A6:32:01:77:2E"}, true)

	if err := c.AplicarLimite(context.Background(), "192.168.1.50", 5000, 2500); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if mikrotik.enderecoMAC != "DC:A6:32:01:77:2E" {
		t.Fatalf("o MAC não foi traduzido: %q", mikrotik.enderecoMAC)
	}
}

// Sem correspondência no ARP não se aplica a regra, mas o erro é reportado.
func TestBandaSemMACCorrespondente(t *testing.T) {
	mikrotik := &bandaMACFalsa{}
	falha := errors.New("sem correspondência")
	c := NovoComBanda(nil).
		AdicionarPorMAC("mikrotik", mikrotik, &resolvedorFalso{err: falha}, true)

	if err := c.AplicarLimite(context.Background(), "192.168.1.50", 5000, 2500); !errors.Is(err, falha) {
		t.Fatalf("esperava a falha do resolvedor, recebeu %v", err)
	}
	if mikrotik.enderecoMAC != "" {
		t.Fatal("não devia aplicar a regra sem MAC conhecido")
	}
}

// Os dois equipamentos recebem o limite, cada um no seu formato.
func TestBandaNotificaOsDois(t *testing.T) {
	pfsense := &bandaFalsa{}
	mikrotik := &bandaMACFalsa{}
	c := NovoComBanda(nil).
		Adicionar("pfsense", pfsense, true).
		AdicionarPorMAC("mikrotik", mikrotik,
			&resolvedorFalso{mac: "AA:BB:CC:DD:EE:FF"}, true)

	if err := c.AplicarLimite(context.Background(), "192.168.1.50", 10000, 5000); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if pfsense.enderecoIP != "192.168.1.50" {
		t.Fatalf("o pfSense não recebeu o limite: %q", pfsense.enderecoIP)
	}
	if mikrotik.enderecoMAC != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("o MikroTik não recebeu o limite: %q", mikrotik.enderecoMAC)
	}
}

// Uma falha num equipamento não impede o outro de receber o limite.
func TestBandaFalhaParcialNaoPropaga(t *testing.T) {
	caido := &bandaFalsa{erro: errors.New("pfSense fora do ar")}
	mikrotik := &bandaMACFalsa{}
	c := NovoComBanda(nil).
		Adicionar("pfsense", caido, true).
		AdicionarPorMAC("mikrotik", mikrotik,
			&resolvedorFalso{mac: "AA:BB:CC:DD:EE:FF"}, true)

	if err := c.AplicarLimite(context.Background(), "192.168.1.50", 5000, 2500); err != nil {
		t.Fatalf("com um equipamento a funcionar não devia haver erro: %v", err)
	}
	if mikrotik.enderecoMAC == "" {
		t.Fatal("o equipamento saudável devia ter recebido o limite")
	}
}

func TestBandaFalhaTotalDevolveErro(t *testing.T) {
	primeiro := errors.New("sem rede")
	c := NovoComBanda(nil).
		Adicionar("pfsense", &bandaFalsa{erro: primeiro}, true).
		AdicionarPorMAC("mikrotik", &bandaMACFalsa{erro: errors.New("sem rede")},
			&resolvedorFalso{mac: "AA:BB"}, true)

	if err := c.AplicarLimite(context.Background(), "192.168.1.50", 5000, 2500); !errors.Is(err, primeiro) {
		t.Fatalf("esperava a primeira falha, recebeu %v", err)
	}
}

func TestBandaIgnoraNaoConfigurados(t *testing.T) {
	c := NovoComBanda(nil).
		Adicionar("pfsense", &bandaFalsa{}, false).
		AdicionarPorMAC("mikrotik", &bandaMACFalsa{}, &resolvedorFalso{}, false)
	if !c.Vazio() {
		t.Fatal("equipamentos sem credenciais não deviam entrar na lista")
	}
}
