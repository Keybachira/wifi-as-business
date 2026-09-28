// Command server arranca o WiFi Controller.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Keybachira/wifi-as-business/internal/analytics"
	"github.com/Keybachira/wifi-as-business/internal/auth"
	"github.com/Keybachira/wifi-as-business/internal/database"
	"github.com/Keybachira/wifi-as-business/internal/devices"
	"github.com/Keybachira/wifi-as-business/internal/mikrotik"
	"github.com/Keybachira/wifi-as-business/internal/networks"
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
	httpAuth := auth.NovoServicoHTTP(srvAuth)

	srvPlanos := plans.NovoServico(plans.NovoRepositorio(banco.DB), registo)
	httpPlanos := plans.NovoServicoHTTP(srvPlanos)

	srvVales := vouchers.NovoServico(
		vouchers.NovoRepositorio(banco.DB),
		vouchers.NovoRepoPlanos(banco.DB),
		registo)
	httpVales := vouchers.NovoServicoHTTP(srvVales)

	clienteMikrotik := mikrotik.NovoCliente(cfg.EnderecoMikrotik(), cfg.MikrotikUsuario, cfg.MikrotikSenha)
	if !clienteMikrotik.Configurado() {
		registo.Warn("integração MikroTik desativada: defina WIFIC_MIKROTIK_SENHA")
	}

	srvSessoes := sessions.NovoServico(sessions.NovoRepositorio(banco.DB), registo).
		ComVales(srvVales).
		ComPlanos(plans.NovoRepositorio(banco.DB))
	if clienteMikrotik.Configurado() {
		srvSessoes = srvSessoes.ComLimitador(clienteMikrotik)
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

	// O serviço RADIUS valida os mesmos vales usados pelo portal cativo.
	srvRadius := radius.NovoServico(cfg.RADIUSPorta, cfg.RADIUSEndereco).
		ComFornecedor(&fornecedorVales{vales: srvVales})
	if !srvRadius.Ativo() {
		registo.Warn("serviço RADIUS sem fornecedor de credenciais")
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
	if err := semear(srvAuth, srvPlanos, registo); err != nil {
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
type fornecedorVales struct{ vales *vouchers.Serviço }

// Validar confirma um código de vale e devolve os limites do plano associado.
func (f *fornecedorVales) Validar(ctx context.Context, utilizador, codigo string) (radius.InfoAcesso, error) {
	vale, err := f.vales.Ativar(ctx, codigo, utilizador)
	if err != nil {
		return radius.InfoAcesso{}, radius.CredencialInvalida
	}
	return radius.InfoAcesso{
		Utilizador:    utilizador,
		SessionID:     fmt.Sprintf("%d", vale.ID),
		DownloadKbps:  5000,
		UploadKbps:    2500,
		LimiteMinutos: 60,
	}, nil
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

// semear cria os dados mínimos para o sistema arrancar utilizável.
func semear(srvAuth *auth.Serviço, srvPlanos *plans.Serviço, registo *slog.Logger) error {
	ctx := context.Background()

	existe, err := srvAuth.ExisteAlgumaConta(ctx)
	if err != nil {
		return err
	}
	if !existe {
		email, senha := credenciaisIniciais()
		if _, err := srvAuth.CriarConta(ctx, auth.CriarConta{
			Nome:  "Administrador",
			Email: email,
			Senha: senha,
			Papel: auth.PapelDono,
		}); err != nil {
			return fmt.Errorf("não foi possível criar a conta inicial: %w", err)
		}
		registo.Warn("conta de dono criada — anote estas credenciais",
			"email", email, "senha", senha)
	}

	existentes, err := srvPlanos.Listar(ctx, false)
	if err != nil {
		return err
	}
	if len(existentes) > 0 {
		return nil
	}

	// Planos padrão (valores em centavos do Kwanza).
	semente := []plans.NovoPlano{
		{Nome: "1 Hora", PrecoCentavos: 10000, DuracaoMinutos: 60, VelocidadeKbps: 5000, LimiteDadosMB: 1024, MaxDispositivos: 1},
		{Nome: "3 Horas", PrecoCentavos: 20000, DuracaoMinutos: 180, VelocidadeKbps: 10000, LimiteDadosMB: 3072, MaxDispositivos: 1},
		{Nome: "1 Dia", PrecoCentavos: 50000, DuracaoMinutos: 1440, VelocidadeKbps: 20000, LimiteDadosMB: 8192, MaxDispositivos: 2},
		{Nome: "7 Dias", PrecoCentavos: 200000, DuracaoMinutos: 10080, VelocidadeKbps: 20000, LimiteDadosMB: 30720, MaxDispositivos: 2},
	}
	for _, p := range semente {
		if _, err := srvPlanos.Criar(ctx, p); err != nil {
			return fmt.Errorf("não foi possível criar o plano %q: %w", p.Nome, err)
		}
	}
	registo.Info("planos padrão criados", "quantidade", len(semente))
	return nil
}

// credenciaisIniciais lê as variáveis de ambiente ou gera uma senha forte.
func credenciaisIniciais() (email, senha string) {
	email = os.Getenv("WIFIC_ADMIN_EMAIL")
	senha = os.Getenv("WIFIC_ADMIN_SENHA")
	if email != "" && senha != "" {
		return email, senha
	}
	if email == "" {
		email = "admin@wificontroller.ao"
	}
	return email, senhaAleatoria()
}

// senhaAleatoria gera uma senha que cumpre a política do módulo de auth.
func senhaAleatoria() string {
	const letras = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	const numeros = "23456789"

	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "WifController7"
	}

	var sb strings.Builder
	sb.WriteByte('W')
	for i, b := range buffer {
		base := letras
		if i >= 8 {
			base = numeros
		}
		sb.WriteByte(base[int(b)%len(base)])
	}
	sb.WriteByte('7')
	return sb.String()
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
				temporizador.Reset(5 * time.Minute)
			}
		}
	}()

	return func() { close(terminar) }
}

