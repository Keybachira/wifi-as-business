package vouchers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Estados possíveis de um vale.
const (
	EstadoDisponivel = "disponivel"
	EstadoAtivo      = "ativo"
	EstadoExpirado   = "expirado"
	EstadoRevogado   = "revogado"
)

// alfabetoVais exclui caracteres ambíguos (0/O, 1/I) para leitura manual.
const alfabetoVais = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// formatoCodigo é o padrão ZG-XXXX-XXXX.
const formatoCodigo = "ZG-%s-%s"

// Vale é um código de acesso vendido a um cliente.
type Vale struct {
	ID         int64      `json:"id"`
	Codigo     string     `json:"codigo"`
	PlanoID    int64      `json:"plano_id"`
	PlanoNome  string     `json:"plano_nome,omitempty"`
	Estado     string     `json:"estado"`
	ExpiraEm   *time.Time `json:"expira_em,omitempty"`
	UsadoEm    *time.Time `json:"usado_em,omitempty"`
	UsadoPor   string     `json:"usado_por,omitempty"`
	CriadoEm   time.Time  `json:"criado_em"`
}

// PodeUsar indica se o vale está disponível para ativação.
func (v Vale) PodeUsar() bool {
	if v.Estado != EstadoDisponivel {
		return false
	}
	return v.ExpiraEm == nil || time.Now().UTC().Before(*v.ExpiraEm)
}

// GerarLote são os dados para gerar um lote de vales.
type GerarLote struct {
	PlanoID   int64  `json:"plano_id"`
	Quantidade int   `json:"quantidade"`
	ValidadeDias int `json:"validade_dias"`
	Prefixo   string `json:"prefixo,omitempty"`
}

// Validar normaliza e valida os parâmetros do lote.
func (g *GerarLote) Validar() error {
	if g.PlanoID <= 0 {
		return errors.New("deve indicar um plano válido")
	}
	if g.Quantidade < 1 {
		return errors.New("a quantidade deve ser no mínimo 1")
	}
	if g.Quantidade > 1000 {
		return errors.New("a quantidade máxima por lote é 1000")
	}
	if g.ValidadeDias < 0 {
		return errors.New("a validade não pode ser negativa")
	}
	if g.ValidadeDias == 0 {
		g.ValidadeDias = 365
	}
	g.Prefixo = strings.ToUpper(strings.TrimSpace(g.Prefixo))
	if len(g.Prefixo) > 4 {
		return errors.New("o prefixo não pode exceder 4 caracteres")
	}
	return nil
}

// FiltroVales define os parâmetros de pesquisa da listagem.
type FiltroVales struct {
	Estado   string
	PlanoID  int64
	Busca    string
	Limite   int
	Deslocar int
}

// Normalizar aplica limites por omissão e valores seguros.
func (f *FiltroVales) Normalizar() {
	if f.Limite <= 0 || f.Limite > 500 {
		f.Limite = 100
	}
	if f.Deslocar < 0 {
		f.Deslocar = 0
	}
	if f.Estado != "" && !estadoValido(f.Estado) {
		f.Estado = ""
	}
	f.Busca = strings.TrimSpace(f.Busca)
}

func estadoValido(e string) bool {
	switch e {
	case EstadoDisponivel, EstadoAtivo, EstadoExpirado, EstadoRevogado:
		return true
	}
	return false
}

// GerarCodigo devolve um código de vale aleatório no formato ZG-XXXX-XXXX.
func GerarCodigo(prefijo string) (string, error) {
	partes := make([]string, 2)
	for i := range partes {
		bloco, err := randomDoAlfabeto(lenAlfabeto)
		if err != nil {
			return "", err
		}
		partes[i] = bloco
	}
	return fmt.Sprintf(formatoCodigo, partes[0], partes[1]), nil
}

const lenAlfabeto = 4

func randomDoAlfabeto(n int) (string, error) {
	var sb strings.Builder
	limite := big.NewInt(int64(len(alfabetoVais)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, limite)
		if err != nil {
			return "", fmt.Errorf("não foi possível gerar o código: %w", err)
		}
		sb.WriteByte(alfabetoVais[idx.Int64()])
	}
	return sb.String(), nil
}
