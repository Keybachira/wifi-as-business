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
// permitiendo sobrescrita por um ficheiro .env.
type Config struct {
	Endereco           string
	Porta              int
	BancoDeDados       string
	DiretorioMigracoes string
	SegredoJWT         []byte
	DuracaoSessao      time.Duration
	Ambiente           string
	HashingCusto       int
	// Integrações externas
	MikrotikEndereco string
	MikrotikUsuario  string
	MikrotikSenha    string
	MikrotikPortaAPI int
	RADIUSEndereco   string
	RADIUSPorta      int
	RADIUSSegredo    string
}

// CarregarConfig lê a configuração do ambiente aplicando valores padrão seguros.
func CarregarConfig() (*Config, error) {
	cfg := &Config{
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
	}

	cfg.Porta = obterEnvInteiro("WIFIC_PORTA", 8080)

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
