package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config contém todas as configurações do servidor.
// Os valores são lidos de variáveis de ambiente com o prefixo WIFIC_,
// com opção de ficheiro de configuração local (config.env ou .env).
type Config struct {
	Endereco           string
	Porta              int
	BancoDeDados       string
	DiretorioMigracoes string
	SegredoJWT         []byte
	DuracaoSessao      time.Duration
	Ambiente           string
	HashingCusto       int
	FicheiroConfig     string
	// Identidade e apresentação, tudo ajustável pelo operador.
	NomeSistema    string
	Moeda          string
	PrefixoVale    string
	PlanosIniciais string
	// Primeira conta (só usada se WIFIC_CRIAR_ADMIN=true).
	AdminEmail string
	AdminSenha string
	// Integrações externas
	MikrotikEndereco string
	MikrotikUsuario  string
	MikrotikSenha    string
	MikrotikPortaAPI int
	RADIUSEndereco   string
	RADIUSPorta      int
	RADIUSSegredo    string
	// Integração alternativa com pfSense (opcional)
	PfSenseEndereco string
	PfSenseUsuario  string
	PfSenseSenha    string
	PfSenseChaveAPI string
	PfSenseAlias    string
}

// CarregarConfig lê a configuração do ambiente aplicando valores padrão seguros.
// Antes de ler o ambiente, carrega um ficheiro local (WIFIC_FICHEIRO, ou
// config.env / .env na pasta de execução) sem sobrepor variáveis já definidas.
func CarregarConfig() (*Config, error) {
	ficheiro, err := carregarFicheiroLocal()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		FicheiroConfig:     ficheiro,
		NomeSistema:        obterEnv("WIFIC_NOME", "WiFi Controller"),
		Moeda:              obterEnv("WIFIC_MOEDA", ""),
		PrefixoVale:        strings.ToUpper(obterEnv("WIFIC_PREFIXO_VALE", "VA")),
		PlanosIniciais:     obterEnv("WIFIC_PLANOS_INICIAIS", ""),
		AdminEmail:         obterEnv("WIFIC_ADMIN_EMAIL", ""),
		AdminSenha:         obterEnv("WIFIC_ADMIN_SENHA", ""),
		Endereco:           obterEnv("WIFIC_ENDERECO", "0.0.0.0"),
		BancoDeDados:       obterEnv("WIFIC_BANCO", filepath.Join("data", "controller.db")),
		DiretorioMigracoes: obterEnv("WIFIC_MIGRACOES", "migrations"),
		SegredoJWT:         []byte(obterEnv("WIFIC_SEGREDO_JWT", "troque-este-segredo-em-producao")),
		DuracaoSessao:      obterEnvDuracao("WIFIC_DURACAO_SESSAO", 12*time.Hour),
		Ambiente:           obterEnv("WIFIC_AMBIENTE", "desenvolvimento"),
		HashingCusto:       obterEnvInteiro("WIFIC_CUSTO_HASH", 12),
		MikrotikEndereco:   obterEnv("WIFIC_MIKROTIK_ENDERECO", "192.168.88.1"),
		MikrotikUsuario:    obterEnv("WIFIC_MIKROTIK_USUARIO", "admin"),
		MikrotikSenha:      obterEnv("WIFIC_MIKROTIK_SENHA", ""),
		MikrotikPortaAPI:   obterEnvInteiro("WIFIC_MIKROTIK_PORTA_API", 8728),
		RADIUSEndereco:     obterEnv("WIFIC_RADIUS_ENDERECO", "127.0.0.1"),
		RADIUSPorta:        obterEnvInteiro("WIFIC_RADIUS_PORTA", 1812),
		RADIUSSegredo:      obterEnv("WIFIC_RADIUS_SEGREDO", ""),
		// Sem omissão de propósito: uma chave ou senha vazia desativa o
		// pfSense, e um endereço por omissão apontaria para um firewall
		// que o operador não pediu para integrar.
		PfSenseEndereco: obterEnv("WIFIC_PFSENSE_ENDERECO", ""),
		PfSenseUsuario:  obterEnv("WIFIC_PFSENSE_USUARIO", "admin"),
		PfSenseSenha:    obterEnv("WIFIC_PFSENSE_SENHA", ""),
		PfSenseChaveAPI: obterEnv("WIFIC_PFSENSE_CHAVE_API", ""),
		PfSenseAlias:    obterEnv("WIFIC_PFSENSE_ALIAS", ""),
	}

	cfg.Porta = obterEnvInteiro("WIFIC_PORTA", 8080)
	if len(cfg.PrefixoVale) > 4 {
		return nil, fmt.Errorf("WIFIC_PREFIXO_VALE não pode exceder 4 caracteres")
	}
	if cfg.AdminEmail != "" || cfg.AdminSenha != "" {
		if cfg.AdminEmail == "" || cfg.AdminSenha == "" {
			return nil, fmt.Errorf("WIFIC_ADMIN_EMAIL e WIFIC_ADMIN_SENHA devem ser definidos juntos")
		}
	}

	if cfg.Ambiente == "producao" {
		if string(cfg.SegredoJWT) == "troque-este-segredo-em-producao" {
			return nil, fmt.Errorf("WIFIC_SEGREDO_JWT é obrigatório em produção")
		}
		if cfg.HashingCusto < 12 {
			return nil, fmt.Errorf("WIFIC_CUSTO_HASH deve ser no mínimo 12 em produção")
		}
	}
	return cfg, nil
}

// EnderecoEscuta devolve o endereço "host:porta" usado pelo servidor HTTP.
func (c *Config) EnderecoEscuta() string {
	return fmt.Sprintf("%s:%d", c.Endereco, c.Porta)
}

// EnderecoMigracoes devolve o caminho absoluto do diretório de migrações.
func (c *Config) EnderecoMigracoes() string {
	abs, err := filepath.Abs(c.DiretorioMigracoes)
	if err != nil {
		return c.DiretorioMigracoes
	}
	return abs
}

// EnderecoBanco devolve o caminho absoluto do ficheiro SQLite.
func (c *Config) EnderecoBanco() string {
	abs, err := filepath.Abs(c.BancoDeDados)
	if err != nil {
		return c.BancoDeDados
	}
	return abs
}

// EnderecoMikrotik devolve o endereço completo do cliente MikroTik.
func (c *Config) EnderecoMikrotik() string {
	return fmt.Sprintf("%s:%d", c.MikrotikEndereco, c.MikrotikPortaAPI)
}

// carregarFicheiroLocal procura e aplica um ficheiro de configuração local.
// Ordem: WIFIC_FICHEIRO, config.env, .env. O primeiro que existir é usado.
// O ficheiro pedido explicitamente via WIFIC_FICHEIRO é obrigatório: se não
// existir, devolve erro em vez de continuar com outra configuração.
// Variáveis já definidas no ambiente NÃO são sobrescritas.
func carregarFicheiroLocal() (string, error) {
	candidatos := []string{os.Getenv("WIFIC_FICHEIRO"), "config.env", ".env"}
	for _, f := range candidatos {
		if strings.TrimSpace(f) == "" {
			continue
		}
		dados, err := os.ReadFile(f)
		if err != nil {
			if os.Getenv("WIFIC_FICHEIRO") != "" {
				return "", fmt.Errorf("ficheiro de configuração indicado em WIFIC_FICHEIRO não foi lido (%s): %w", f, err)
			}
			// Candidatos por omissão são opcionais: tenta o seguinte.
			continue
		}
		aplicarLinhasEnv(string(dados))
		return f, nil
	}
	return "", nil
}

// aplicarLinhasEnv interpreta pares CHAVE=valor, ignorando comentários.
// Não sobrescreve variáveis já presentes no ambiente.
func aplicarLinhasEnv(conteudo string) {
	for _, linha := range strings.Split(conteudo, "\n") {
		linha = strings.TrimSpace(linha)
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		linha = strings.TrimPrefix(linha, "export ")
		i := strings.Index(linha, "=")
		if i <= 0 {
			continue
		}
		chave := strings.TrimSpace(linha[:i])
		valor := strings.TrimSpace(linha[i+1:])
		valor = strings.Trim(valor, `"'`)
		if chave == "" {
			continue
		}
		if _, existe := os.LookupEnv(chave); !existe {
			_ = os.Setenv(chave, valor)
		}
	}
}

func obterEnv(chave, padrao string) string {
	if v, ok := os.LookupEnv(chave); ok && v != "" {
		return v
	}
	return padrao
}

func obterEnvInteiro(chave string, padrao int) int {
	if v, ok := os.LookupEnv(chave); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return padrao
}

func obterEnvDuracao(chave string, padrao time.Duration) time.Duration {
	if v, ok := os.LookupEnv(chave); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return padrao
}
