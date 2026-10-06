package radius

import (
	"context"
	"net"
	"testing"
	"time"
)

// fornecedorFalso devolve limites fixos, para testar sem base de dados.
type fornecedorFalso struct {
	valido     bool
	utilizador string
	codigo     string
}

func (f *fornecedorFalso) Validar(_ context.Context, utilizador, segredo string) (InfoAcesso, error) {
	f.utilizador = utilizador
	f.codigo = segredo
	if !f.valido {
		return InfoAcesso{}, CredencialInvalida
	}
	return InfoAcesso{
		Utilizador:    utilizador,
		SessionID:     "42",
		DownloadKbps:  5000,
		UploadKbps:    2500,
		LimiteMinutos: 60,
	}, nil
}

// portaLivre devolve uma porta UDP livre para o teste.
func portaLivre(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("não foi possível reservar uma porta: %v", err)
	}
	porta := c.LocalAddr().(*net.UDPAddr).Port
	_ = c.Close()
	return porta
}

// montarServidor arranca um servidor RADIUS real e devolve o seu endereço.
func montarServidor(t *testing.T, segredo string, f Fornecedor) *Servidor {
	t.Helper()
	porta := portaLivre(t)
	s := NovoServico(porta, "127.0.0.1").ComFornecedor(f)
	srv := NovoServidor(s, segredo, porta, "127.0.0.1", nil)
	if err := srv.Iniciar(context.Background()); err != nil {
		t.Fatalf("não foi possível iniciar o servidor: %v", err)
	}
	t.Cleanup(func() { _ = srv.Encerrar() })
	return srv
}

// enviarAccessRequest monta e envia um Access-Request, devolvendo a resposta.
func enviarAccessRequest(t *testing.T, srv *Servidor, segredo, utilizador, vale string) *Pacote {
	t.Helper()

	cliente, err := net.Dial("udp", srv.Endereco())
	if err != nil {
		t.Fatalf("não foi possível falar com o servidor: %v", err)
	}
	defer cliente.Close()

	// O Request Authenticator tem de ser escolhido ANTES de encriptar a
	// senha: a cifra depende dele. Numa sessão real o router gera-o.
	var autenticador [16]byte
	copy(autenticador[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	pedido := &Pacote{
		Codigo:        CodigoAccessRequest,
		Identificador: 7,
		Autenticador:  autenticador,
		Atributos: []Atributo{
			NovoAtributo(AttrUserName, utilizador),
			{Tipo: AttrUserPassword, Valor: codificarPassword(segredo, autenticador, vale)},
			{Tipo: AttrCalledStationID, Valor: []byte{192, 168, 1, 50}},
		},
	}
	dados, err := pedido.Codificar("", nil)
	if err != nil {
		t.Fatalf("não foi possível codificar o pedido: %v", err)
	}
	if _, err := cliente.Write(dados); err != nil {
		t.Fatalf("não foi possível enviar o pedido: %v", err)
	}

	_ = cliente.SetReadDeadline(time.Now().Add(3 * time.Second))
	buffer := make([]byte, tamanhoBuffer)
	n, err := cliente.Read(buffer)
	if err != nil {
		t.Fatalf("não foi possível ler a resposta: %v", err)
	}
	resposta, err := Descodificar(buffer[:n])
	if err != nil {
		t.Fatalf("resposta inválida: %v", err)
	}
	return resposta
}

func TestServidorAceitaValeValido(t *testing.T) {
	f := &fornecedorFalso{valido: true}
	srv := montarServidor(t, "segredo-partilhado", f)

	resposta := enviarAccessRequest(t, srv, "segredo-partilhado", "DC:A6:32:01:77:2E", "ZG-ABCD-1234")

	if resposta.Codigo != CodigoAccessAccept {
		t.Fatalf("esperava Access-Accept, recebeu código %d", resposta.Codigo)
	}
	// O identificador tem de ser copiado do pedido, senão o router descarta.
	if resposta.Identificador != 7 {
		t.Fatalf("identificador não copiado: %d", resposta.Identificador)
	}
	// 60 minutos = 3600 segundos: é o Session-Timeout que corta a sessão.
	segundos, ok := resposta.AtributoInteiro(AttrSessionTimeout)
	if !ok || segundos != 3600 {
		t.Fatalf("Session-Timeout errado: %d (ok=%v)", segundos, ok)
	}
	if resposta.Texto(AttrClass) != "42" {
		t.Fatalf("Class devia trazer a sessão, recebeu %q", resposta.Texto(AttrClass))
	}
	ip, ok := resposta.AtributoEndereco(AttrFramedIPAddress)
	if !ok || ip.String() != "192.168.1.50" {
		t.Fatalf("Framed-IP-Address errado: %v", ip)
	}
	// O vale tem de ter chegado ao fornecedor em claro, e o MAC como utilizador.
	if f.codigo != "ZG-ABCD-1234" {
		t.Fatalf("o vale não foi descodificado: %q", f.codigo)
	}
	if f.utilizador != "DC:A6:32:01:77:2E" {
		t.Fatalf("utilizador errado: %q", f.utilizador)
	}
}

func TestServidorRejeitaValeInvalido(t *testing.T) {
	srv := montarServidor(t, "segredo-partilhado", &fornecedorFalso{valido: false})

	resposta := enviarAccessRequest(t, srv, "segredo-partilhado", "DC:A6:32:01:77:2E", "ZG-XXXX-XXXX")

	if resposta.Codigo != CodigoAccessReject {
		t.Fatalf("esperava Access-Reject, recebeu código %d", resposta.Codigo)
	}
	// Uma rejeição nunca pode revelar que o vale existe.
	if resposta.Texto(AttrReplyMessage) != "" {
		t.Fatalf("a rejeição não devia incluir mensagem: %q", resposta.Texto(AttrReplyMessage))
	}
}

// Com o segredo errado a senha não se descodifica: o que chega ao fornecedor
// é lixo, e é o fornecedor que o rejeita. O RADIUS não tem forma de detectar
// sozinho o segredo trocado — a protecção é o segredo estar bem configurado
// nos dois lados.
func TestSegredoErradoCorrompeASenha(t *testing.T) {
	f := &fornecedorFalso{valido: true}
	srv := montarServidor(t, "segredo-correto", f)

	enviarAccessRequest(t, srv, "segredo-errado", "DC:A6:32:01:77:2E", "ZG-ABCD-1234")

	if f.codigo == "ZG-ABCD-1234" {
		t.Fatal("com o segredo errado o vale não devia chegar legível")
	}
	if f.codigo == "" {
		t.Fatal("esperava lixo descodificado, não uma senha vazia")
	}
}

// Um fornecedor que conhece os códigos válidos rejeita o que chega corrompido,
// exatamente como acontece com a base de dados dos vales.
func TestValeCorrompidoERejeitado(t *testing.T) {
	s := NewfornecedorEstrito("segredo-partilhado")
	srv := montarServidor(t, "segredo-partilhado", s)

	resposta := enviarAccessRequest(t, srv, "segredo-errado", "DC:A6:32:01:77:2E", "ZG-ABCD-1234")

	if resposta.Codigo != CodigoAccessReject {
		t.Fatalf("esperava Access-Reject para um vale corrompido, recebeu %d", resposta.Codigo)
	}
}

// fornecedorEstrito só aceita um código exacto, como a tabela de vales.
type fornecedorEstrito struct{ codigoValido string }

func NewfornecedorEstrito(codigo string) *fornecedorEstrito {
	return &fornecedorEstrito{codigoValido: codigo}
}

func (f *fornecedorEstrito) Validar(_ context.Context, _, codigo string) (InfoAcesso, error) {
	if codigo != f.codigoValido {
		return InfoAcesso{}, CredencialInvalida
	}
	return InfoAcesso{SessionID: "7", LimiteMinutos: 60}, nil
}

func TestEstatisticasContamPedidos(t *testing.T) {
	srv := montarServidor(t, "segredo-partilhado", &fornecedorFalso{valido: true})
	enviarAccessRequest(t, srv, "segredo-partilhado", "DC:A6:32:01:77:2E", "ZG-ABCD-1234")

	recebidos, aceites, _, _ := srv.Estatisticas()
	if recebidos != 1 {
		t.Fatalf("esperava 1 pedido recebido, recebeu %d", recebidos)
	}
	if aceites != 1 {
		t.Fatalf("esperava 1 aceite, recebeu %d", aceites)
	}
}
