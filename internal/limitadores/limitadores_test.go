package limitadores

import (
	"context"
	"errors"
	"testing"
)

// limitadorFalso é um equipamento de teste que registra o que recebeu.
type limitadorFalso struct {
	chamados []string
	erro     error
}

func (l *limitadorFalso) DesligarCliente(_ context.Context, enderecoMAC string) error {
	l.chamados = append(l.chamados, enderecoMAC)
	return l.erro
}

func TestCompostoVazioNaoConfigura(t *testing.T) {
	c := NovoComposto(nil)
	if !c.Vazio() {
		t.Fatal("um composto sem equipamentos devia estar vazio")
	}
	if err := c.DesligarCliente(context.Background(), "AA:BB"); !errors.Is(err, NaoConfigurado) {
		t.Fatalf("esperava NaoConfigurado, recebeu %v", err)
	}
}

func TestAdicionarIgnoraNaoConfigurados(t *testing.T) {
	c := NovoComposto(nil)
	c.Adicionar("mikrotik", &limitadorFalso{}, false)
	if !c.Vazio() {
		t.Fatal("um equipamento sem credenciais não devia entrar na lista")
	}
}

func TestDesligarNotificaTodos(t *testing.T) {
	mikrotik := &limitadorFalso{}
	pfsense := &limitadorFalso{}

	c := NovoComposto(nil).
		Adicionar("mikrotik", mikrotik, true).
		Adicionar("pfsense", pfsense, true)

	if c.Vazio() {
		t.Fatal("devia haver dois equipamentos")
	}
	if err := c.DesligarCliente(context.Background(), "DC:A6:32:01:77:2E"); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	for _, l := range []*limitadorFalso{mikrotik, pfsense} {
		if len(l.chamados) != 1 || l.chamados[0] != "DC:A6:32:01:77:2E" {
			t.Fatalf("equipamento não notificado corretamente: %v", l.chamados)
		}
	}
}

// Um router em baixo não pode impedir que o outro corte a navegação.
func TestFalhaParcialNaoPropaga(t *testing.T) {
	caido := &limitadorFalso{erro: errors.New("router fora do ar")}
	funciona := &limitadorFalso{}

	c := NovoComposto(nil).
		Adicionar("mikrotik", caido, true).
		Adicionar("pfsense", funciona, true)

	if err := c.DesligarCliente(context.Background(), "AA:BB"); err != nil {
		t.Fatalf("com um equipamento a funcionar não devia haver erro: %v", err)
	}
	if len(funciona.chamados) != 1 {
		t.Fatal("o equipamento saudável devia ter sido notificado")
	}
}

func TestFalhaTotalDevolveErro(t *testing.T) {
	primeiro := errors.New("sem rede")
	c := NovoComposto(nil).
		Adicionar("mikrotik", &limitadorFalso{erro: primeiro}, true).
		Adicionar("pfsense", &limitadorFalso{erro: errors.New("sem rede")}, true)

	err := c.DesligarCliente(context.Background(), "AA:BB")
	if !errors.Is(err, primeiro) {
		t.Fatalf("esperava a primeira falha, recebeu %v", err)
	}
}
