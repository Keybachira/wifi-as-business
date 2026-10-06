package sessions

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Estados possíveis de uma sessão.
const (
	EstadoAtiva     = "ativa"
	EstadoTerminada = "terminada"
	EstadoExpirada  = "expirada"
)

// Tipos de dispositivo reconhecidos.
const (
	DispositivoDesconhecido = "desconhecido"
	DispositivoAndroid      = "android"
	DispositivoIphone       = "iphone"
	DispositivoComputador   = "computador"
	DispositivoTablet       = "tablet"
)

// Sessão é uma ligação de um cliente à rede, autenticada por um vale.
type Sessão struct {
	ID               int64      `json:"id"`
	ValeID           *int64     `json:"vale_id,omitempty"`
	RedeID           *int64     `json:"rede_id,omitempty"`
	Utilizador       string     `json:"utilizador"`
	Telefone         string     `json:"telefone,omitempty"`
	EnderecoIP       string     `json:"endereco_ip"`
	EnderecoMAC      string     `json:"endereco_mac"`
	TipoDispositivo  string     `json:"tipo_dispositivo"`
	Velocidade       string     `json:"velocidade"`
	LimiteVelocidade int        `json:"limite_velocidade_kbps"`
	LimiteDadosMB    int        `json:"limite_dados_mb"`
	DadosMB          float64    `json:"dados_mb"`
	SessionID        string     `json:"session_id,omitempty"`
	IniciadaEm       time.Time  `json:"iniciada_em"`
	TerminadaEm      *time.Time `json:"terminada_em,omitempty"`
	Estado           string     `json:"estado"`
	MotivoFim        string     `json:"motivo_fim,omitempty"`
	PlanoNome        string     `json:"plano_nome,omitempty"`
	RestanteMinutos  int        `json:"restante_minutos"`
}

// Duracao devolve o tempo de sessão decorrido.
func (s Sessão) Duracao() time.Duration {
	if s.TerminadaEm == nil {
		return time.Since(s.IniciadaEm)
	}
	return s.TerminadaEm.Sub(s.IniciadaEm)
}

// DuracaoFormatada devolve a duração em texto legível.
func (s Sessão) DuracaoFormatada() string {
	return formatarDuracao(s.Duracao())
}

// AindaAtiva indica se a sessão está em curso.
func (s Sessão) AindaAtiva() bool { return s.Estado == EstadoAtiva }

// ExcedeuDados indica se o consumo ultrapassou o limite do plano.
//
// Um limite de 0 significa ilimitado, por isso nunca é excedido.
func (s Sessão) ExcedeuDados() bool {
	return s.LimiteDadosMB > 0 && s.DadosMB >= float64(s.LimiteDadosMB)
}

// RestanteDados devolve os megabytes que faltam consumir, ou -1 se ilimitado.
func (s Sessão) RestanteDados() float64 {
	if s.LimiteDadosMB <= 0 {
		return -1
	}
	restante := float64(s.LimiteDadosMB) - s.DadosMB
	if restante < 0 {
		return 0
	}
	return restante
}

// FiltroSessoes define os parâmetros de pesquisa da listagem.
type FiltroSessoes struct {
	Estado       string
	RedeID       int64
	Utilizador   string
	EnderecoIP   string
	ApenasAtivas bool
	Limite       int
	Deslocar     int
}

// Normalizar aplica valores por omissão seguros.
func (f *FiltroSessoes) Normalizar() {
	if f.Limite <= 0 || f.Limite > 500 {
		f.Limite = 100
	}
	if f.Deslocar < 0 {
		f.Deslocar = 0
	}
	if f.Estado != EstadoAtiva && f.Estado != EstadoTerminada && f.Estado != EstadoExpirada {
		f.Estado = ""
	}
	f.Utilizador = strings.TrimSpace(f.Utilizador)
	f.EnderecoIP = strings.TrimSpace(f.EnderecoIP)
}

// IniciarSessao são os dados registados quando um cliente autentica.
type IniciarSessao struct {
	ValeID           *int64 `json:"vale_id"`
	RedeID           *int64 `json:"rede_id"`
	Utilizador       string `json:"utilizador"`
	Telefone         string `json:"telefone,omitempty"`
	EnderecoIP       string `json:"endereco_ip"`
	EnderecoMAC      string `json:"endereco_mac"`
	TipoDispositivo  string `json:"tipo_dispositivo"`
	Velocidade       string `json:"velocidade"`
	LimiteVelocidade int    `json:"limite_velocidade_kbps"`
	// LimiteDadosMB e SessionID vêm do fluxo RADIUS. São opcionais porque o
	// portal cativo abre sessão sem passar por um router.
	LimiteDadosMB int    `json:"limite_dados_mb"`
	SessionID     string `json:"session_id,omitempty"`
}

// Validar normaliza e valida os dados de arranque de sessão.
func (i *IniciarSessao) Validar() error {
	i.Utilizador = strings.TrimSpace(i.Utilizador)
	i.EnderecoIP = strings.TrimSpace(i.EnderecoIP)
	i.EnderecoMAC = strings.ToUpper(strings.TrimSpace(i.EnderecoMAC))
	i.Telefone = strings.TrimSpace(i.Telefone)
	i.SessionID = strings.TrimSpace(i.SessionID)

	if i.Utilizador == "" {
		return errors.New("o nome do utilizador é obrigatório")
	}
	if i.EnderecoIP == "" {
		return errors.New("o endereço IP é obrigatório")
	}
	if !ipValido(i.EnderecoIP) {
		return errors.New("endereço IP inválido")
	}
	if !macValida(i.EnderecoMAC) {
		return errors.New("endereço MAC inválido (use o formato AA:BB:CC:DD:EE:FF)")
	}
	if i.TipoDispositivo == "" {
		i.TipoDispositivo = DispositivoDesconhecido
	}
	return nil
}

// TerminarSessao descreve o motivo de encerramento.
type TerminarSessao struct {
	Motivo string `json:"motivo,omitempty"`
}

// ipValido faz uma verificação simples de formato IPv4.
func ipValido(ip string) bool {
	partes := strings.Split(ip, ".")
	if len(partes) != 4 {
		return false
	}
	for _, p := range partes {
		if p == "" || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// macValido verifica o formato AA:BB:CC:DD:EE:FF.
func macValida(mac string) bool {
	if len(mac) != 17 {
		return false
	}
	for i, c := range mac {
		if i%3 == 2 {
			if c != ':' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func formatarDuracao(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	horas := int(d.Hours())
	minutos := int(d.Minutes()) % 60
	segundos := int(d.Seconds()) % 60

	switch {
	case horas > 0:
		return fmt.Sprintf("%dh %02dm", horas, minutos)
	case minutos > 0:
		return fmt.Sprintf("%dm %02ds", minutos, segundos)
	default:
		return fmt.Sprintf("%ds", segundos)
	}
}
