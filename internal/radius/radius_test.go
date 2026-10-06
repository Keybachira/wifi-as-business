package radius

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestAccountingRecebeResposta(t *testing.T) {
	srv := montarServidor(t, "segredo-partilhado", &fornecedorFalso{valido: true})

	cliente, err := net.Dial("udp", srv.Endereco())
	if err != nil {
		t.Fatalf("não foi possível falar com o servidor: %v", err)
	}
	defer cliente.Close()

	pedido := &Pacote{
		Codigo:        CodigoAccountingRequest,
		Identificador: 9,
		Atributos: []Atributo{
			NovoAtributo(AttrUserName, "DC:A6:32:01:77:2E"),
			{Tipo: AttrAcctStatusType, Valor: []byte{15}},
			NovoAtributoInteiro(AttrAcctSessionTime, 600),
		},
	}
	dados, _ := pedido.Codificar("", nil)
	if _, err := cliente.Write(dados); err != nil {
		t.Fatalf("não foi possível enviar: %v", err)
	}

	_ = cliente.SetReadDeadline(time.Now().Add(3 * time.Second))
	buffer := make([]byte, tamanhoBuffer)
	n, err := cliente.Read(buffer)
	if err != nil {
		t.Fatalf("sem resposta ao accounting: %v", err)
	}
	resposta, err := Descodificar(buffer[:n])
	if err != nil {
		t.Fatalf("resposta inválida: %v", err)
	}
	if resposta.Codigo != CodigoAccountingResponse {
		t.Fatalf("esperava Accounting-Response, recebeu %d", resposta.Codigo)
	}
}

// Bytes aleatórios não podem provocar pânico nem tirar o servidor do ar.
func TestLixoNaoDerrubaServidor(t *testing.T) {
	srv := montarServidor(t, "segredo-partilhado", &fornecedorFalso{valido: true})

	cliente, err := net.Dial("udp", srv.Endereco())
	if err != nil {
		t.Fatalf("não foi possível falar com o servidor: %v", err)
	}
	if _, err := cliente.Write([]byte("isto nao e um pacote radius")); err != nil {
		t.Fatalf("não foi possível enviar: %v", err)
	}
	_ = cliente.Close()

	// A seguir, um pedido válido tem de ser atendido normalmente.
	resposta := enviarAccessRequest(t, srv, "segredo-partilhado", "DC:A6:32:01:77:2E", "ZG-ABCD-1234")
	if resposta.Codigo != CodigoAccessAccept {
		t.Fatalf("o servidor deixou de responder após lixo: %d", resposta.Codigo)
	}
}

func TestServidorDesativadoSemSegredo(t *testing.T) {
	porta := portaLivre(t)
	srv := NovoServidor(
		NovoServico(porta, "127.0.0.1").ComFornecedor(&fornecedorFalso{valido: true}),
		"", porta, "127.0.0.1", nil)

	if srv.Configurado() {
		t.Fatal("sem segredo o servidor não devia estar configurado")
	}
	if err := srv.Iniciar(context.Background()); !errors.Is(err, ErrNaoConfigurado) {
		t.Fatalf("esperava ErrNaoConfigurado, recebeu %v", err)
	}
	if srv.Ativa() {
		t.Fatal("o servidor não devia estar activo")
	}
}

// A porta 1812 é a padrão do RADIUS; 1813 é a de accounting.
func TestPortasUsadasPorOmissao(t *testing.T) {
	if got := NovoServico(1812, "127.0.0.1").Endereco(); got != "127.0.0.1:1812" {
		t.Fatalf("endereço inesperado: %q", got)
	}
}

// Um pacote com comprimento declarado maior do que o datagrama é truncamento
// ou ataque: tem de ser recusado sem alocar nada.
func TestPacoteTruncadoRecusado(t *testing.T) {
	if _, err := Descodificar([]byte{1, 2, 0, 200}); !errors.Is(err, ErrPacoteCurto) {
		t.Fatalf("esperava ErrPacoteCurto, recebeu %v", err)
	}

	// 20 bytes de cabeçalho mas o campo de comprimento promete 200.
	curto := make([]byte, 20)
	curto[0] = CodigoAccessRequest
	curto[3] = 200
	if _, err := Descodificar(curto); !errors.Is(err, ErrPacoteInvalido) {
		t.Fatalf("esperava ErrPacoteInvalido, recebeu %v", err)
	}
}

// A senha tem de sobreviver à ida e volta pela encriptação do RADIUS.
func TestPasswordSobreviveArcoIris(t *testing.T) {
	aut := [16]byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5, 6}
	for _, senha := range []string{"ZG-ABCD-1234", "a", "1234567890123456789"} {
		cifrada := codificarPassword("segredo", aut, senha)
		if descodificarPassword("segredo", aut, cifrada) != senha {
			t.Fatalf("a senha %q não sobreviveu à encriptação", senha)
		}
	}
}
