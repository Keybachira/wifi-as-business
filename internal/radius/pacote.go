package radius

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

// Códigos de pacote RADIUS (RFC 2865, secção 3).
const (
	CodigoAccessRequest      uint8 = 1
	CodigoAccessAccept       uint8 = 2
	CodigoAccessReject       uint8 = 3
	CodigoAccountingRequest  uint8 = 4
	CodigoAccountingResponse uint8 = 5
)

// Tipos de atributo usados pelo captive portal.
const (
	AttrUserName           uint8 = 1
	AttrUserPassword       uint8 = 2
	AttrCHAPPassword       uint8 = 3
	AttrNASIPAddress       uint8 = 4
	AttrNASPort            uint8 = 5
	AttrServiceType        uint8 = 6
	AttrFramedProtocol     uint8 = 7
	AttrFramedIPAddress    uint8 = 8
	AttrFramedIPNetmask    uint8 = 9
	AttrFilterID           uint8 = 11
	AttrReplyMessage       uint8 = 18
	AttrState              uint8 = 24
	AttrClass              uint8 = 25
	AttrVendorSpecific     uint8 = 26
	AttrSessionTimeout     uint8 = 27
	AttrIdleTimeout        uint8 = 28
	AttrTerminationAction  uint8 = 29
	AttrCalledStationID    uint8 = 30
	AttrCallingStationID   uint8 = 31
	AttrNASIdentifier      uint8 = 32
	AttrAcctStatusType     uint8 = 40
	AttrAcctInputOctets    uint8 = 42
	AttrAcctOutputOctets   uint8 = 43
	AttrSessionID          uint8 = 44
	AttrAcctSessionTime    uint8 = 46
	AttrAcctInputPackets   uint8 = 47
	AttrAcctOutputPackets  uint8 = 48
	AttrAcctTerminateCause uint8 = 49
)

// Valores de Service-Type (RFC 2865, secção 5.26).
const ServicoLoginUser uint8 = 1

// Atributo é um par tipo/valor dentro de um pacote RADIUS.
type Atributo struct {
	Tipo  uint8
	Valor []byte
}

// NovoAtributo cria um atributo com valor de texto.
func NovoAtributo(tipo uint8, valor string) Atributo {
	return Atributo{Tipo: tipo, Valor: []byte(valor)}
}

// Pacote é um pacote RADIUS descodificado.
//
// O autenticador é uma chave de autenticação de 16 bytes, não um segredo: só
// quem conhece o segredo partilhado consegue validar uma resposta.
type Pacote struct {
	Codigo        uint8
	Identificador uint8
	Autenticador  [16]byte
	Atributos     []Atributo
}

// Limites do protocolo. O campo de comprimento tem 2 bytes, mas o UDP não
// suporta datagramas tão grandes; 4096 é folgado e evita alocar por ataque.
const (
	tamanhoMinimo = 4 + 16
	tamanhoMaximo = 4096
)

// Descodificar transforma bytes num Pacote.
func Descodificar(dados []byte) (*Pacote, error) {
	if len(dados) < tamanhoMinimo {
		return nil, ErrPacoteCurto
	}

	comprimento := int(binary.BigEndian.Uint16(dados[2:4]))
	// O comprimento declarado tem de bater certo com o que chegou, senão o
	// pacote foi alterado ou truncado no caminho.
	if comprimento < tamanhoMinimo || comprimento > len(dados) || comprimento > tamanhoMaximo {
		return nil, ErrPacoteInvalido
	}

	p := &Pacote{Codigo: dados[0], Identificador: dados[1]}
	copy(p.Autenticador[:], dados[4:20])

	for i := 20; i < comprimento; {
		if i+2 > comprimento {
			return nil, ErrAtributoInvalido
		}
		tipo := dados[i]
		tamAttr := int(dados[i+1])
		// O comprimento inclui os dois bytes do cabeçalho do atributo.
		if tamAttr < 2 || i+tamAttr > comprimento {
			return nil, ErrAtributoInvalido
		}
		valor := make([]byte, tamAttr-2)
		copy(valor, dados[i+2:i+tamAttr])
		p.Atributos = append(p.Atributos, Atributo{Tipo: tipo, Valor: valor})
		i += tamAttr
	}
	return p, nil
}

// Atributo devolve o primeiro atributo do tipo pedido.
func (p *Pacote) Atributo(tipo uint8) ([]byte, bool) {
	for _, a := range p.Atributos {
		if a.Tipo == tipo {
			return a.Valor, true
		}
	}
	return nil, false
}

// Texto devolve um atributo como cadeia, sem espaços nas pontas.
func (p *Pacote) Texto(tipo uint8) string {
	v, ok := p.Atributo(tipo)
	if !ok {
		return ""
	}
	return strings.TrimSpace(string(v))
}

// AtributoInteiro devolve um atributo de 4 bytes como inteiro.
func (p *Pacote) AtributoInteiro(tipo uint8) (int, bool) {
	v, ok := p.Atributo(tipo)
	if !ok || len(v) != 4 {
		return 0, false
	}
	return int(binary.BigEndian.Uint32(v)), true
}

// Codificar serializa o pacote.
//
// Numa resposta, o autenticador é MD5(segredo, autenticador do pedido,
// atributos) — é assim que o cliente confirma que a resposta vem do servidor
// legítimo. Numa autenticação, geramos um Request Authenticator aleatório.
func (p *Pacote) Codificar(segredo string, autenticadorPedido *[16]byte) ([]byte, error) {
	corpo := make([]byte, 0, 64)
	for _, a := range p.Atributos {
		// O comprimento do atributo é 1 byte e inclui o próprio cabeçalho.
		if len(a.Valor) > 253 {
			return nil, ErrAtributoInvalido
		}
		corpo = append(corpo, a.Tipo, uint8(len(a.Valor)+2))
		corpo = append(corpo, a.Valor...)
	}

	comprimento := tamanhoMinimo + len(corpo)
	if comprimento > tamanhoMaximo {
		return nil, ErrPacoteInvalido
	}

	saida := make([]byte, tamanhoMinimo, comprimento)
	saida[0] = p.Codigo
	saida[1] = p.Identificador
	binary.BigEndian.PutUint16(saida[2:4], uint16(comprimento))
	saida = append(saida, corpo...)

	if segredo != "" && autenticadorPedido != nil {
		// Resposta: MD5(segredo, autenticador do pedido, atributos).
		h := md5.New()
		h.Write([]byte(segredo))
		h.Write(autenticadorPedido[:])
		h.Write(saida[4:])
		copy(saida[4:20], h.Sum(nil))
	} else {
		// Pedido: preserva o Request Authenticator já escolhido pelo cliente.
		// Só o geramos se o cliente não o tiver definido.
		if p.Autenticador == [16]byte{} {
			if _, err := rand.Read(saida[4:20]); err != nil {
				return nil, fmt.Errorf("não foi possível gerar o autenticador: %w", err)
			}
		} else {
			copy(saida[4:20], p.Autenticador[:])
		}
	}
	return saida, nil
}

// codificarPassword encripta a senha no formato exigido pelo RADIUS.
//
// A senha é bloqueada em 16 bytes, terminada em zeros, e cada bloco é
// encriptado com MD5(segredo + autenticador do bloco anterior).
func codificarPassword(segredo string, autenticador [16]byte, senha string) []byte {
	if senha == "" {
		return nil
	}
	if len(senha) > 128 {
		senha = senha[:128]
	}

	bruto := make([]byte, ((len(senha)+15)/16)*16)
	copy(bruto, senha)

	saida := make([]byte, len(bruto))
	anterior := autenticador
	for i := 0; i < len(bruto); i += 16 {
		h := md5.New()
		h.Write([]byte(segredo))
		h.Write(anterior[:])
		bloco := h.Sum(nil)
		for j := 0; j < 16; j++ {
			saida[i+j] = bruto[i+j] ^ bloco[j]
		}
		var seguinte [16]byte
		copy(seguinte[:], saida[i:i+16])
		anterior = seguinte
	}
	return saida
}

// descodificarPassword desfaz a encriptação de um User-Password.
//
// É a operação inversa de codificarPassword: cada bloco é XOR com
// MD5(segredo + autenticador do bloco anterior).
func descodificarPassword(segredo string, autenticador [16]byte, dados []byte) string {
	if len(dados) == 0 || len(dados)%16 != 0 {
		return ""
	}

	bruto := make([]byte, len(dados))
	anterior := autenticador
	for i := 0; i < len(dados); i += 16 {
		h := md5.New()
		h.Write([]byte(segredo))
		h.Write(anterior[:])
		bloco := h.Sum(nil)
		for j := 0; j < 16; j++ {
			bruto[i+j] = dados[i+j] ^ bloco[j]
		}
		var seguinte [16]byte
		copy(seguinte[:], dados[i:i+16])
		anterior = seguinte
	}
	// O padding é feito com zeros, que não podem fazer parte de uma senha.
	return strings.TrimRight(string(bruto), "\x00")
}

// AtributoEndereco devolve um atributo de 4 bytes como endereço IPv4.
func (p *Pacote) AtributoEndereco(tipo uint8) (net.IP, bool) {
	v, ok := p.Atributo(tipo)
	if !ok || len(v) != 4 {
		return nil, false
	}
	return net.IPv4(v[0], v[1], v[2], v[3]), true
}

// NovoAtributoInteiro cria um atributo inteiro de 4 bytes.
func NovoAtributoInteiro(tipo uint8, valor int) Atributo {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(valor))
	return Atributo{Tipo: tipo, Valor: b}
}
