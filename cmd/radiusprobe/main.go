// Envia um Access-Request RADIUS ao servidor indicado e imprime a resposta.
//
// Uso: go run ./cmd/radiusprobe -endereco 127.0.0.1:1812 -segredo x -utilizador AA:BB:CC:DD:EE:FF -vale VA-XXXX-XXXX
//
// Existe para verificar um servidor a correr com um cliente independente do
// código do servidor — é assim que um pfSense real se comporta.
package main

import (
	"crypto/md5"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

// Códigos RADIUS relevantes para a sonda.
const (
	codigoAccessRequest      uint8 = 1
	codigoAccessAccept       uint8 = 2
	codigoAccessReject       uint8 = 3
	codigoAccountingRequest  uint8 = 4
	codigoAccountingResponse uint8 = 5
)

const (
	attrUserName         uint8 = 1
	attrUserPassword     uint8 = 2
	attrSessionTimeout   uint8 = 27
	attrClass            uint8 = 25
	attrAcctStatusType   uint8 = 40
	attrAcctInputOctets  uint8 = 42
	attrAcctOutputOctets uint8 = 43
	attrSessionID        uint8 = 44
)

func main() {
	endereco := flag.String("endereco", "127.0.0.1:1812", "endereço do servidor RADIUS")
	segredo := flag.String("segredo", "", "segredo partilhado")
	utilizador := flag.String("utilizador", "", "User-Name a enviar (normalmente o MAC)")
	vale := flag.String("vale", "", "código do vale")
	contabilizar := flag.Bool("contabilizar", false, "enviar um Accounting-Request em vez de autenticar")
	session := flag.String("session", "", "Session-Id a usar no Accounting-Request")
	entrada := flag.Int("entrada", 0, "MB de download a reportar")
	saida := flag.Int("saida", 0, "MB de upload a reportar")
	flag.Parse()

	// No modo contabilizar o segredo não é preciso: o Accounting-Request não
	// leva senha encriptada, e o servidor responde sempre.
	if *contabilizar {
		if *session == "" {
			fmt.Fprintln(os.Stderr, "falta -session para o Accounting-Request")
			os.Exit(2)
		}
		if err := enviarAccounting(*endereco, *session, *entrada, *saida); err != nil {
			fmt.Fprintf(os.Stderr, "erro: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Accounting-Response recebido (%.1f MB down, %.1f MB up)\n",
			float64(*entrada), float64(*saida))
		return
	}

	if *segredo == "" || *utilizador == "" || *vale == "" {
		fmt.Fprintln(os.Stderr, "uso: radiusprobe -segredo X -utilizador MAC -vale CODIGO")
		fmt.Fprintln(os.Stderr, "     ou: radiusprobe -contabilizar -session ID -entrada MB -saida MB")
		os.Exit(2)
	}

	resposta, err := sondar(*endereco, *segredo, *utilizador, *vale)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}

	switch resposta.codigo {
	case codigoAccessAccept:
		fmt.Println("RESULTADO: Access-Accept")
		if t, ok := resposta.inteiro(attrSessionTimeout); ok {
			fmt.Printf("Session-Timeout: %d segundos (%d minutos)\n", t, t/60)
		}
		if c, ok := resposta.atributo(attrClass); ok {
			fmt.Printf("Class: %s\n", c)
		}
	case codigoAccessReject:
		fmt.Println("RESULTADO: Access-Reject")
	default:
		fmt.Printf("RESULTADO: código %d\n", resposta.codigo)
	}
}

// enviarAccounting envia um Accounting-Request e espera pela resposta.
//
// Os valores são dados em megabytes e convertidos para octetos aqui, que é
// como o RADIUS os transporta.
func enviarAccounting(endereco, sessionID string, entradaMB, saidaMB int) error {
	conexao, err := net.Dial("udp", endereco)
	if err != nil {
		return err
	}
	defer conexao.Close()

	const mb = 1024 * 1024
	corpo := []byte{attrUserName, uint8(len("DC:A6:32:01:77:2E") + 2)}
	corpo = append(corpo, "DC:A6:32:01:77:2E"...)

	// 2 = Interim-Update: o estado intermédio com consumo parcial.
	corpo = append(corpo, attrAcctStatusType, 2, 2)
	corpo = append(corpo, attrSessionID, uint8(len(sessionID)+2))
	corpo = append(corpo, sessionID...)

	corpo = append(corpo, attrAcctInputOctets, 6)
	corpo = append(corpo, inteiroOctetos(entradaMB*mb)...)
	corpo = append(corpo, attrAcctOutputOctets, 6)
	corpo = append(corpo, inteiroOctetos(saidaMB*mb)...)

	pedido := make([]byte, 20, 20+len(corpo))
	pedido[0] = codigoAccountingRequest
	pedido[1] = 2
	binary.BigEndian.PutUint16(pedido[2:4], uint16(20+len(corpo)))
	for i := 4; i < 20; i++ {
		pedido[i] = byte(i)
	}
	pedido = append(pedido, corpo...)

	if _, err := conexao.Write(pedido); err != nil {
		return err
	}
	_ = conexao.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conexao.Read(buffer)
	if err != nil {
		return err
	}

	resposta, err := descodificar(buffer[:n])
	if err != nil {
		return err
	}
	if resposta.codigo != codigoAccountingResponse {
		return fmt.Errorf("esperava Accounting-Response, recebeu código %d", resposta.codigo)
	}
	return nil
}

// inteiroOctetos converte um inteiro em 4 bytes big-endian.
func inteiroOctetos(valor int) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(valor))
	return b
}

// pacote é o mínimo necessário para descodificar uma resposta.
type pacote struct {
	codigo    uint8
	atributos map[uint8][]byte
}

func (p pacote) atributo(t uint8) ([]byte, bool) {
	v, ok := p.atributos[t]
	return v, ok
}

func (p pacote) inteiro(t uint8) (int, bool) {
	v, ok := p.atributos[t]
	if !ok || len(v) != 4 {
		return 0, false
	}
	return int(binary.BigEndian.Uint32(v)), true
}

// sondar envia o pedido e espera pela resposta.
func sondar(endereco, segredo, utilizador, vale string) (pacote, error) {
	conexao, err := net.Dial("udp", endereco)
	if err != nil {
		return pacote{}, fmt.Errorf("não foi possível falar com %s: %w", endereco, err)
	}
	defer conexao.Close()

	var autenticador [16]byte
	copy(autenticador[:], []byte("sonda-probe-1234"))
	senha := encriptar(segredo, autenticador, vale)

	corpo := []byte{attrUserName, uint8(len(utilizador) + 2)}
	corpo = append(corpo, utilizador...)
	corpo = append(corpo, attrUserPassword, uint8(len(senha)+2))
	corpo = append(corpo, senha...)

	pedido := make([]byte, 20, 20+len(corpo))
	pedido[0] = codigoAccessRequest
	pedido[1] = 1 // identificador
	binary.BigEndian.PutUint16(pedido[2:4], uint16(20+len(corpo)))
	copy(pedido[4:20], autenticador[:])
	pedido = append(pedido, corpo...)

	if _, err := conexao.Write(pedido); err != nil {
		return pacote{}, fmt.Errorf("não foi possível enviar: %w", err)
	}

	_ = conexao.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conexao.Read(buffer)
	if err != nil {
		return pacote{}, fmt.Errorf("sem resposta em 5s: %w", err)
	}
	return descodificar(buffer[:n])
}

// encriptar aplica a cifra do User-Password do RADIUS.
func encriptar(segredo string, autenticador [16]byte, senha string) []byte {
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

func descodificar(dados []byte) (pacote, error) {
	if len(dados) < 20 {
		return pacote{}, fmt.Errorf("resposta demasiado curta (%d bytes)", len(dados))
	}
	comprimento := int(binary.BigEndian.Uint16(dados[2:4]))
	if comprimento > len(dados) {
		return pacote{}, fmt.Errorf("comprimento declarado %d, recebido %d", comprimento, len(dados))
	}

	p := pacote{codigo: dados[0], atributos: map[uint8][]byte{}}
	for i := 20; i < comprimento; {
		if i+2 > comprimento {
			break
		}
		tipo, tam := dados[i], int(dados[i+1])
		if tam < 2 || i+tam > comprimento {
			break
		}
		valor := make([]byte, tam-2)
		copy(valor, dados[i+2:i+tam])
		if _, existe := p.atributos[tipo]; !existe {
			p.atributos[tipo] = valor
		}
		i += tam
	}
	return p, nil
}
