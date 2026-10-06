package plans

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Plano descreve um pacote de acesso vendido aos utilizadores.
type Plano struct {
	ID              int64      `json:"id"`
	Nome            string     `json:"nome"`
	PrecoCentavos   int64      `json:"preco_centavos"`
	DuracaoMinutos  int        `json:"duracao_minutos"`
	VelocidadeKbps  int        `json:"velocidade_kbps"`
	LimiteDadosMB   int        `json:"limite_dados_mb"` // 0 = ilimitado
	MaxDispositivos int        `json:"max_dispositivos"`
	Ativo           bool       `json:"ativo"`
	CriadoEm        time.Time  `json:"criado_em"`
	AtualizadoEm    *time.Time `json:"atualizado_em,omitempty"`
	ValesUsados     int        `json:"vales_usados"`
}

// PrecoFormatado devolve o preço com separadores de milhar.
// A moeda não é imposta pelo sistema: quem formata para exibição escolhe
// o símbolo (configurável via WIFIC_MOEDA).
func (p Plano) PrecoFormatado(moeda string) string {
	if strings.TrimSpace(moeda) == "" {
		return formatarMilhar(p.PrecoCentavos / 100)
	}
	return fmt.Sprintf("%s %s", formatarMilhar(p.PrecoCentavos/100), moeda)
}

// DuracaoFormatada devolve a duração em linguagem natural.
func (p Plano) DuracaoFormatada() string {
	switch {
	case p.DuracaoMinutos%1440 == 0:
		dias := p.DuracaoMinutos / 1440
		return plural(dias, "dia", "dias")
	case p.DuracaoMinutos%60 == 0:
		horas := p.DuracaoMinutos / 60
		return plural(horas, "hora", "horas")
	default:
		return plural(p.DuracaoMinutos, "minuto", "minutos")
	}
}

// DadosFormatados devolve o limite de dados ou "Ilimitado".
func (p Plano) DadosFormatados() string {
	if p.LimiteDadosMB == 0 {
		return "Ilimitado"
	}
	if p.LimiteDadosMB >= 1024 {
		return fmt.Sprintf("%.0f GB", float64(p.LimiteDadosMB)/1024)
	}
	return fmt.Sprintf("%d MB", p.LimiteDadosMB)
}

// SemDadosIndicados indica se o plano não tem limite de tráfego.
func (p Plano) SemDadosIndicados() bool { return p.LimiteDadosMB == 0 }

// NovoPlano são os dados para criar um plano.
type NovoPlano struct {
	Nome            string `json:"nome"`
	PrecoCentavos   int64  `json:"preco_centavos"`
	DuracaoMinutos  int    `json:"duracao_minutos"`
	VelocidadeKbps  int    `json:"velocidade_kbps"`
	LimiteDadosMB   int    `json:"limite_dados_mb"`
	MaxDispositivos int    `json:"max_dispositivos"`
	Ativo           *bool  `json:"ativo,omitempty"`
}

// AtualizarPlano são os campos alteráveis de um plano existente.
type AtualizarPlano struct {
	Nome            *string `json:"nome,omitempty"`
	PrecoCentavos   *int64  `json:"preco_centavos,omitempty"`
	DuracaoMinutos  *int    `json:"duracao_minutos,omitempty"`
	VelocidadeKbps  *int    `json:"velocidade_kbps,omitempty"`
	LimiteDadosMB   *int    `json:"limite_dados_mb,omitempty"`
	MaxDispositivos *int    `json:"max_dispositivos,omitempty"`
	Ativo           *bool   `json:"ativo,omitempty"`
}

// Validar normaliza e valida os dados de criação.
func (n *NovoPlano) Validar() error {
	n.Nome = strings.TrimSpace(n.Nome)

	if n.Nome == "" {
		return errors.New("o nome do plano é obrigatório")
	}
	if len([]rune(n.Nome)) > 60 {
		return errors.New("o nome não pode exceder 60 caracteres")
	}
	if n.PrecoCentavos < 0 {
		return errors.New("o preço não pode ser negativo")
	}
	if n.DuracaoMinutos <= 0 {
		return errors.New("a duração deve ser maior que zero")
	}
	if n.VelocidadeKbps <= 0 {
		return errors.New("a velocidade deve ser maior que zero")
	}
	if n.LimiteDadosMB < 0 {
		return errors.New("o limite de dados não pode ser negativo")
	}
	if n.MaxDispositivos <= 0 {
		n.MaxDispositivos = 1
	}
	if n.Ativo == nil {
		verdadeiro := true
		n.Ativo = &verdadeiro
	}
	return nil
}

// Validar aplica as mesmas regras da criação a uma atualização parcial.
func (a AtualizarPlano) Validar(nomeAntigo string) error {
	if a.Nome != nil {
		n := strings.TrimSpace(*a.Nome)
		if n == "" {
			return errors.New("o nome do plano não pode ficar vazio")
		}
		if len([]rune(n)) > 60 {
			return errors.New("o nome não pode exceder 60 caracteres")
		}
		*a.Nome = n
	}
	if a.PrecoCentavos != nil && *a.PrecoCentavos < 0 {
		return errors.New("o preço não pode ser negativo")
	}
	if a.DuracaoMinutos != nil && *a.DuracaoMinutos <= 0 {
		return errors.New("a duração deve ser maior que zero")
	}
	if a.VelocidadeKbps != nil && *a.VelocidadeKbps <= 0 {
		return errors.New("a velocidade deve ser maior que zero")
	}
	if a.LimiteDadosMB != nil && *a.LimiteDadosMB < 0 {
		return errors.New("o limite de dados não pode ser negativo")
	}
	if a.MaxDispositivos != nil && *a.MaxDispositivos <= 0 {
		return errors.New("deve ser permitido pelo menos um dispositivo")
	}
	_ = nomeAntigo
	return nil
}

func formatarMilhar(v int64) string {
	s := fmt.Sprintf("%d", v)
	var partes []string
	for len(s) > 3 {
		partes = append([]string{s[len(s)-3:]}, partes...)
		s = s[:len(s)-3]
	}
	return s + strings.Join(partes, ",")
}

func plural(n int, singular, pluralForma string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForma)
}
