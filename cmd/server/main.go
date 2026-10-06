// Command server arranca o WiFi Controller.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Keybachira/wifi-as-business/internal/analytics"
	"github.com/Keybachira/wifi-as-business/internal/auth"
	"github.com/Keybachira/wifi-as-business/internal/database"
	"github.com/Keybachira/wifi-as-business/internal/devices"
	"github.com/Keybachira/wifi-as-business/internal/limitadores"
	"github.com/Keybachira/wifi-as-business/internal/mikrotik"
	"github.com/Keybachira/wifi-as-business/internal/networks"
	"github.com/Keybachira/wifi-as-business/internal/pfsense"
	"github.com/Keybachira/wifi-as-business/internal/plans"
	"github.com/Keybachira/wifi-as-business/internal/portal"
	"github.com/Keybachira/wifi-as-business/internal/radius"
	"github.com/Keybachira/wifi-as-business/internal/sessions"
	"github.com/Keybachira/wifi-as-business/internal/system"
	"github.com/Keybachira/wifi-as-business/internal/users"
	"github.com/Keybachira/wifi-as-business/internal/vouchers"
)

func main() {
	if err := executar(); err != nil {
		fmt.Fprintf(os.Stderr, "erro fatal: %v\n", err)
		os.Exit(1)
	}
}

func executar() error {
	// 1. Configuração
	cfg, err := system.CarregarConfig()
	if err != nil {
		return err
	}
	registo := system.ConfigurarRegisto(system.NivelPorAmbiente(cfg.Ambiente))
	if cfg.FicheiroConfig != "" {
		registo.Info("configuração local carregada", "ficheiro", cfg.FicheiroConfig)
	}

	// 2. Base de dados e migrações
	banco, err := database.Abrir(cfg.EnderecoBanco())
	if err != nil {
		return err
	}
	defer banco.Fechar()
	registo.Info("base de dados pronta", "ficheiro", banco.Caminho())

	ctx := context.Background()
	if err := database.NovoMigrador(banco, cfg.EnderecoMigracoes(), registo).Aplicar(ctx); err != nil {
		return err
	}

	// 3. Módulos de domínio
	srvAuth := auth.NovoServico(auth.NovoRepositorio(banco.DB), banco.DB, cfg.DuracaoSessao, cfg.HashingCusto)
	httpAuth := auth.NovoServicoHTTP(srvAuth, cfg.NomeSistema)

	srvPlanos := plans.NovoServico(plans.NovoRepositorio(banco.DB), registo)
	httpPlanos := plans.NovoServicoHTTP(srvPlanos)

	srvVales := vouchers.NovoServico(
		vouchers.NovoRepositorio(banco.DB),
		vouchers.NovoRepoPlanos(banco.DB),
		registo, cfg.PrefixoVale)
	httpVales := vouchers.NovoServicoHTTP(srvVales)

	clienteMikrotik := mikrotik.NovoCliente(cfg.EnderecoMikrotik(), cfg.MikrotikUsuario, cfg.MikrotikSenha)
	if !clienteMikrotik.Configurado() {
		registo.Warn("integração MikroTik desativada: defina WIFIC_MIKROTIK_SENHA")
	}

	// O pfSense é a alternativa ao MikroTik. Fica desligado enquanto não
	// houver endereço e credenciais, para não afetar quem só usa MikroTik.
	clientePfSense := pfsense.NovoCliente(
		cfg.PfSenseEndereco, cfg.PfSenseUsuario, cfg.PfSenseSenha,
		cfg.PfSenseChaveAPI, cfg.PfSenseAlias)
	if !clientePfSense.Configurado() {
		registo.Warn("integração pfSense desativada: defina WIFIC_PFSENSE_ENDERECO e uma credencial")
	} else if cfg.PfSenseAlias == "" {
		registo.Warn("pfSense configurado sem alias: a desconexão de clientes não vai funcionar",
			"variavel", "WIFIC_PFSENSE_ALIAS")
	}

	// Ambos os routers recebem a desconexão. O MikroTik é o principal; o
	// pfSense só entra na lista se estiver configurado.
	equipamentos := limitadores.NovoComposto(registo).
		Adicionar("mikrotik", clienteMikrotik, clienteMikrotik.Configurado()).
		Adicionar("pfsense", clientePfSense, clientePfSense.Configurado())

	// A banda é um agregador separado porque os equipamentos funcionam de
	// forma diferente: o pfSense limita por IP e o MikroTik por MAC, o que
	// exige uma tradução antes de aplicar a regra.
	banda := limitadores.NovoComBanda(registo).
		Adicionar("pfsense", clientePfSense, clientePfSense.Configurado()).
		AdicionarPorMAC("mikrotik", clienteMikrotik, clienteMikrotik, clienteMikrotik.Configurado())

	// Repositório de planos partilhado: as sessões leem a velocidade e o
	// RADIUS devolve ao router os limites do plano que o cliente comprou.
	repoPlanos := plans.NovoRepositorio(banco.DB)

	srvSessoes := sessions.NovoServico(sessions.NovoRepositorio(banco.DB), registo).
		ComVales(srvVales).
		ComPlanos(repoPlanos)
	if !equipamentos.Vazio() {
		srvSessoes = srvSessoes.ComLimitador(equipamentos)
	}
	if !banda.Vazio() {
		srvSessoes = srvSessoes.ComBanda(banda)
	}
	httpSessoes := sessions.NovoServicoHTTP(srvSessoes)

	srvRedes := networks.NovoServico(networks.NovoRepositorio(banco.DB), registo)
	httpRedes := networks.NovoServicoHTTP(srvRedes)

	srvDispositivos := devices.NovoServico(devices.NovoRepositorio(banco.DB), registo)
	httpDispositivos := devices.NovoServicoHTTP(srvDispositivos)

	srvUtilizadores := users.NovoServico(users.NovoRepositorio(banco.DB), registo)
	httpUtilizadores := users.NovoServicoHTTP(srvUtilizadores)

	srvPortal := portal.NovoServico(portal.NovoRepositorio(banco.DB), registo)
	httpPortal := portal.NovoServicoHTTP(srvPortal)

	httpAnalises := analytics.NovoServicoHTTP(analytics.NovoServico(analytics.NovoRepositorio(banco.DB)))

	// O serviço RADIUS valida os mesmos vales usados pelo portal cativo, com
	// os limites do plano obtido do repositório partilhado acima.
	srvRadius := radius.NovoServico(cfg.RADIUSPorta, cfg.RADIUSEndereco).
		ComFornecedor(&fornecedorVales{vales: srvVales, planos: repoPlanos})
	if !srvRadius.Ativo() {
		registo.Warn("serviço RADIUS sem fornecedor de credenciais")
	}

	// O servidor UDP só arranca com segredo partilhado: sem ele não há como
	// validar um pedido, e abrir a porta seria apenas uma superfície de ataque.
	//
	// O serviço de sessões é o contabilizador: é quem grava o consumo
	// reportado pelo router e corta o acesso quando o limite é ultrapassado.
	srvRadiusUDP := radius.NovoServidor(srvRadius, cfg.RADIUSSegredo, cfg.RADIUSPorta, cfg.RADIUSEndereco, registo).
		ComContabilizador(srvSessoes)
	if !srvRadiusUDP.Configurado() {
		registo.Warn("servidor RADIUS UDP desativado: defina WIFIC_RADIUS_SEGREDO para ligar um router")
	} else if err := srvRadiusUDP.Iniciar(ctx); err != nil {
		// Uma falha aqui não impede o portal cativo de funcionar: só o router
		// fica sem validação de vouchers.
		registo.Error("não foi possível iniciar o servidor RADIUS UDP", "erro", err)
	} else {
		defer func() {
			if err := srvRadiusUDP.Encerrar(); err != nil {
				registo.Warn("falha ao encerrar o servidor RADIUS", "erro", err)
			}
		}()
	}

	// 4. Rotas
	exigir := httpAuth.ExigirAutenticacao
	registarRotas := func(mux *http.ServeMux) {
		httpAuth.Rotas(mux, exigir)
		httpPlanos.Rotas(mux, exigir)
		httpVales.Rotas(mux, exigir)
		httpSessoes.Rotas(mux, exigir)
		httpRedes.Rotas(mux, exigir)
		httpDispositivos.Rotas(mux, exigir)
		httpUtilizadores.Rotas(mux, exigir)
		httpPortal.Rotas(mux, exigir)
		httpAnalises.Rotas(mux, exigir)
	}

	srv := system.NovoServidor(cfg, registo, registarRotas, pastaFrontend())

	// 5. Dados iniciais
	if err := semear(srvAuth, srvPlanos, cfg, registo); err != nil {
		return err
	}

	// 6. Rotinas periódicas de manutenção
	parar := iniciarManutencao(srvAuth, srvVales, srvSessoes, registo)
	defer parar()

	// 7. Encerramento gracioso
	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sinais
		registo.Info("sinal recebido, a encerrar...")
		contexto, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelar()
		if err := srv.Encerrar(contexto); err != nil {
			registo.Error("falha ao encerrar o servidor", "erro", err)
		}
	}()

	return srv.Iniciar()
}

// fornecedorVales adapta o serviço de vales à interface do RADIUS.
type fornecedorVales struct {
	vales  *vouchers.Serviço
	planos *plans.Repositorio
}

// Validar confirma um código de vale e devolve os limites do plano associado.
func (f *fornecedorVales) Validar(ctx context.Context, utilizador, codigo string) (radius.InfoAcesso, error) {
	vale, err := f.vales.Ativar(ctx, codigo, utilizador)
	if err != nil {
		return radius.InfoAcesso{}, radius.CredencialInvalida
	}

	acesso := radius.InfoAcesso{
		Utilizador: utilizador,
		// O Session-Id identifica a sessão para o router. O ID do vale serve,
		// porque um vale é de uso único e só dá origem a uma sessão.
		SessionID:     fmt.Sprintf("%d", vale.ID),
		DownloadKbps:  5000,
		UploadKbps:    2500,
		LimiteMinutos: 60,
	}

	// Os limites vêm do plano do vale, e não de valores fixos: o RADIUS tem de
	// fazer exactamente o mesmo que o portal cativo, senão um cliente pagava
	// por um plano e recebia as condições de outro.
	//
	// Se o plano não for encontrado, mantêm-se os valores por omissão: é
	// melhor uma sessão com limites genéricos do que recusar o acesso.
	if f.planos != nil {
		if plano, err := f.planos.Buscar(ctx, vale.PlanoID); err == nil {
			acesso.DownloadKbps = plano.VelocidadeKbps
			acesso.UploadKbps = plano.VelocidadeKbps / 2
			acesso.LimiteMinutos = plano.DuracaoMinutos
			acesso.LimiteDadosMB = plano.LimiteDadosMB
			acesso.MaxDispositivos = plano.MaxDispositivos
		}
	}
	return acesso, nil
}

// pastaFrontend devolve a pasta com a aplicação compilada.
//
// Só aceita a pasta de build (dist). Servir a pasta de código-fonte
// exporia ficheiros .tsx e a pasta node_modules, por isso é ignorada.
func pastaFrontend() string {
	dist := filepath.Join("web", "frontend", "dist")
	if info, err := os.Stat(dist); err == nil && info.IsDir() {
		return dist
	}
	return ""
}

// semear prepara os dados iniciais. Sistema genérico e totalmente local:
// não cria contas (a primeira conta é criada pelo operador via
// POST /api/v1/auth/primeira-conta, ou por WIFIC_CRIAR_ADMIN=true).
// Planos só são criados se o operador pedir (WIFIC_PLANOS_INICIAIS).
func semear(srvAuth *auth.Serviço, srvPlanos *plans.Serviço, cfg *system.Config, registo *slog.Logger) error {
	ctx := context.Background()

	// Conta opcional por configuração — para arranques desacompanhados.
	if cfg.AdminEmail != "" && cfg.AdminSenha != "" {
		existe, err := srvAuth.ExisteAlgumaConta(ctx)
		if err != nil {
			return err
		}
		if !existe {
			if _, err := srvAuth.CriarConta(ctx, auth.CriarConta{
				Nome:  "Administrador",
				Email: cfg.AdminEmail,
				Senha: cfg.AdminSenha,
				Papel: auth.PapelOperador,
			}); err != nil {
				return fmt.Errorf("não foi possível criar a conta inicial: %w", err)
			}
			registo.Info("conta inicial criada por configuração", "email", cfg.AdminEmail)
		}
	}

	existentes, err := srvPlanos.Listar(ctx, false)
	if err != nil {
		return err
	}
	if len(existentes) > 0 {
		return nil
	}

	// Sem WIFIC_PLANOS_INICIAIS, o sistema arranca vazio: o operador cria
	// os planos que quiser. Com a variável, os planos são criados no arranque.
	if strings.TrimSpace(cfg.PlanosIniciais) == "" {
		registo.Info("sem planos iniciais — crie-os via API ou defina WIFIC_PLANOS_INICIAIS")
		return nil
	}

	// Formato: "Nome;preco_centavos;duracao_minutos;velocidade_kbps;dados_mb;max_dispositivos", separado por vírgulas.
	// Ex.: "1 Hora;100;60;5000;1024;1, 1 Dia;500;1440;20000;8192;2"
	for _, item := range strings.Split(cfg.PlanosIniciais, ",") {
		campos := strings.Split(strings.TrimSpace(item), ";")
		if len(campos) < 4 {
			return fmt.Errorf("WIFIC_PLANOS_INICIAIS: item inválido %q (use Nome;preco;duracao;velocidade[,dados][,dispositivos])", item)
		}
		preco, err := strconv.ParseInt(strings.TrimSpace(campos[1]), 10, 64)
		if err != nil {
			return fmt.Errorf("WIFIC_PLANOS_INICIAIS: preço inválido em %q", item)
		}
		duracao, err := strconv.Atoi(strings.TrimSpace(campos[2]))
		if err != nil {
			return fmt.Errorf("WIFIC_PLANOS_INICIAIS: duração inválida em %q", item)
		}
		velocidade, err := strconv.Atoi(strings.TrimSpace(campos[3]))
		if err != nil {
			return fmt.Errorf("WIFIC_PLANOS_INICIAIS: velocidade inválida em %q", item)
		}
		n := plans.NovoPlano{Nome: strings.TrimSpace(campos[0]), PrecoCentavos: preco, DuracaoMinutos: duracao, VelocidadeKbps: velocidade}
		if len(campos) > 4 {
			dados, err := strconv.Atoi(strings.TrimSpace(campos[4]))
			if err != nil {
				return fmt.Errorf("WIFIC_PLANOS_INICIAIS: dados inválidos em %q", item)
			}
			n.LimiteDadosMB = dados
		}
		if len(campos) > 5 {
			disp, err := strconv.Atoi(strings.TrimSpace(campos[5]))
			if err != nil {
				return fmt.Errorf("WIFIC_PLANOS_INICIAIS: dispositivos inválidos em %q", item)
			}
			n.MaxDispositivos = disp
		}
		if _, err := srvPlanos.Criar(ctx, n); err != nil {
			return fmt.Errorf("não foi possível criar o plano %q: %w", n.Nome, err)
		}
		registo.Info("plano inicial criado", "nome", n.Nome)
	}
	return nil
}

// iniciarManutencao corre as tarefas periódicas e devolve a função de paragem.
func iniciarManutencao(srvAuth *auth.Serviço, srvVales *vouchers.Serviço, srvSessoes *sessions.Serviço, registo *slog.Logger) func() {
	terminar := make(chan struct{})

	go func() {
		// Primeira execução aos 30s para não competir com o arranque.
		temporizador := time.NewTimer(30 * time.Second)
		defer temporizador.Stop()

		for {
			select {
			case <-terminar:
				return
			case <-temporizador.C:
				t := context.Background()
				if _, err := srvAuth.LimparSessoesExpiradas(t); err != nil {
					registo.Warn("falha ao limpar sessões de acesso expiradas", "erro", err)
				}
				if _, err := srvVales.ExpirarVencidos(t); err != nil {
					registo.Warn("falha ao expirar vales", "erro", err)
				}
				if _, err := srvSessoes.ExpirarVencidas(t); err != nil {
					registo.Warn("falha ao expirar sessões", "erro", err)
				}
				// Rede de segurança da cobrança: apanha o consumo que o router
				// não reportou, por estar desligado ou mal configurado.
				if _, err := srvSessoes.ExcederDados(t); err != nil {
					registo.Warn("falha ao cortar sessões por excesso de dados", "erro", err)
				}
				temporizador.Reset(5 * time.Minute)
			}
		}
	}()

	return func() { close(terminar) }
}
