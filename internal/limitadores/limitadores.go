// Package limitadores agrega os routers sob gestão num único LimitadorRede.
//
// O WiFi Controller fala com mais do que um tipo de equipamento: o MikroTik
// continua a ser o router principal, mas o pfSense pode estar configurado em
// paralelo. O Composto junta ambos para que o resto do sistema continue a
// depender de uma única interface, sem saber qual equipamento está activo.
package limitadores

import (
	"context"
	"errors"
	"log/slog"
)

// NaoConfigurado é devolvido por um equipamento sem credenciais. Não conta
// como falha: o equipamento simplesmente não existe para esta instalação.
var NaoConfigurado = errors.New("limitador não configurado")

// Limitador é um equipamento capaz de desligar um cliente da rede.
type Limitador interface {
	DesligarCliente(ctx context.Context, enderecoMAC string) error
}

// LimitadorBanda é um equipamento capaz de aplicar a velocidade de um plano.
type LimitadorBanda interface {
	AplicarLimite(ctx context.Context, enderecoIP string, downloadKbps, uploadKbps int) error
}

// ResolvedorIP encontra o endereço MAC associado a um IP.
//
// O MikroTik identifica clientes por MAC, mas a sessão guarda o IP. Sem esta
// tradução não é possível limitar a banda pelo router principal.
type ResolvedorIP interface {
	ResolverIP(ctx context.Context, enderecoIP string) (string, error)
}

// ComBanda agrega os equipamentos que aplicam a velocidade do plano.
//
// A tolerância a falhas é a mesma do Composto: se um equipamento falhar, os
// restantes são ainda notificados e a sessão continua válida.
type ComBanda struct {
	registos  []banda
	porMAC    []bandaMAC
	registo   *slog.Logger
	resolutor ResolvedorIP
}

type banda struct {
	nome      string
	limitador LimitadorBanda
}

type bandaMAC struct {
	nome      string
	limitador interface {
		AplicarLimite(ctx context.Context, enderecoMAC string, downloadKbps, uploadKbps int) error
	}
}

// NovoComBanda cria o agregador de banda.
func NovoComBanda(registo *slog.Logger) *ComBanda {
	return &ComBanda{registo: registo}
}

// Adicionar regista um equipamento que trabalha por IP (pfSense).
func (c *ComBanda) Adicionar(nome string, l LimitadorBanda, configurado bool) *ComBanda {
	if l == nil || !configurado {
		return c
	}
	c.registos = append(c.registos, banda{nome: nome, limitador: l})
	return c
}

// AdicionarPorMAC regista um equipamento que trabalha por MAC (MikroTik) e o
// resolvedor usado para traduzir o IP da sessão.
func (c *ComBanda) AdicionarPorMAC(
	nome string,
	l interface {
		AplicarLimite(ctx context.Context, enderecoMAC string, downloadKbps, uploadKbps int) error
	},
	resolvedor ResolvedorIP,
	configurado bool,
) *ComBanda {
	if l == nil || !configurado {
		return c
	}
	c.porMAC = append(c.porMAC, bandaMAC{nome: nome, limitador: l})
	c.resolutor = resolvedor
	return c
}

// Vazio indica que não há nenhum equipamento de banda configurado.
func (c *ComBanda) Vazio() bool { return len(c.registos) == 0 && len(c.porMAC) == 0 }

// AplicarLimite distribui a velocidade por todos os equipamentos configurados.
func (c *ComBanda) AplicarLimite(ctx context.Context, enderecoIP string, downloadKbps, uploadKbps int) error {
	if c.Vazio() {
		return NaoConfigurado
	}

	var (
		falhas   []error
		primeiro error
	)
	registar := func(nome string, err error) {
		falhas = append(falhas, err)
		if primeiro == nil {
			primeiro = err
		}
		if c.registo != nil {
			c.registo.Warn("falha ao aplicar a velocidade no equipamento",
				"equipamento", nome, "ip", enderecoIP, "erro", err)
		}
	}

	for _, r := range c.registos {
		if err := r.limitador.AplicarLimite(ctx, enderecoIP, downloadKbps, uploadKbps); err != nil {
			registar(r.nome, err)
		}
	}

	// O MikroTik precisa do MAC; sem correspondência não há o que limitar.
	if len(c.porMAC) > 0 && c.resolutor != nil {
		mac, err := c.resolutor.ResolverIP(ctx, enderecoIP)
		switch {
		case err != nil:
			for _, r := range c.porMAC {
				registar(r.nome, err)
			}
		default:
			for _, r := range c.porMAC {
				if err := r.limitador.AplicarLimite(ctx, mac, downloadKbps, uploadKbps); err != nil {
					registar(r.nome, err)
				}
			}
		}
	}

	total := len(c.registos) + len(c.porMAC)
	if len(falhas) == total {
		return primeiro
	}
	return nil
}

// Composto distribui a desconexão por todos os limitadores registados.
//
// Nenhum equipamento é essencial: se um falhar, os restantes são ainda
// notificados e a falha só é devolvida se todos falharem. Um router em baixo
// não pode deixar de desligar clientes nos routers que funcionam.
type Composto struct {
	registos []registo
	registo  *slog.Logger
}

type registo struct {
	nome      string
	limitador Limitador
}

// NovoComposto cria o agregador com o registo indicado.
func NovoComposto(registo *slog.Logger) *Composto {
	return &Composto{registo: registo}
}

// Adicionar regista um equipamento. Só entram os que estiverem configurados,
// pelo que os desativados não são silenciosamente notificados.
func (c *Composto) Adicionar(nome string, l Limitador, configurado bool) *Composto {
	if l == nil || !configurado {
		return c
	}
	c.registos = append(c.registos, registo{nome: nome, limitador: l})
	if c.registo != nil {
		c.registo.Info("router sob gestão", "tipo", nome)
	}
	return c
}

// Vazio indica que não há nenhum equipamento configurado.
func (c *Composto) Vazio() bool { return len(c.registos) == 0 }

// DesligarCliente notifica todos os equipamentos configurados.
//
// Devolve nil quando pelo menos um teve sucesso, porque a sessão já está
// terminada na base de dados e a navegação foi cortada em algum sítio.
func (c *Composto) DesligarCliente(ctx context.Context, enderecoMAC string) error {
	if len(c.registos) == 0 {
		return NaoConfigurado
	}

	var (
		falhas   []error
		primeiro error
	)
	for _, r := range c.registos {
		if err := r.limitador.DesligarCliente(ctx, enderecoMAC); err != nil {
			falhas = append(falhas, err)
			if primeiro == nil {
				primeiro = err
			}
			if c.registo != nil {
				c.registo.Warn("falha ao desligar cliente no equipamento",
					"equipamento", r.nome, "mac", enderecoMAC, "erro", err)
			}
		}
	}

	if len(falhas) == len(c.registos) {
		return primeiro
	}
	return nil
}
