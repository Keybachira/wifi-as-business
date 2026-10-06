package radius

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// contabilizadorFalso regista as chamadas de consumo.
type contabilizadorFalso struct {
	sessionID string
	entrada   float64
	saida     float64
	chamadas  int
	encerrada bool
	err       error
}

func (c *contabilizadorFalso) RegistrarConsumo(
	_ context.Context, sessionID string, entrada, saida float64) (bool, error) {
	c.chamadas++
	c.sessionID = sessionID
	c.entrada, c.saida = entrada, saida
	return c.encerrada, c.err
}

// enviarAccounting envia um Accounting-Request com os contadores indicados.
func enviarAccounting(t *testing.T, srv *Servidor, sessionID string, entrada, saida int) error {
	t.Helper()

	cliente, err := net.Dial("udp", srv.Endereco())
	if err != nil {
		t.Fatalf("não foi possível falar com o servidor: %v", err)
	}
	defer cliente.Close()

	atributos := []Atributo{
		NovoAtributo(AttrUserName, "DC:A6:32:01:77:2E"),
		// 2 = Interim-Update: o estado intermédio que traz o consumo parcial.
		{Tipo: AttrAcctStatusType, Valor: []byte{2}},
		{Tipo: AttrSessionID, Valor: []byte(sessionID)},
		NovoAtributoInteiro(AttrAcctInputOctets, entrada),
		NovoAtributoInteiro(AttrAcctOutputOctets, saida),
	}
	dados, err := (&Pacote{
		Codigo: CodigoAccountingRequest, Identificador: 11, Atributos: atributos,
	}).Codificar("", nil)
	if err != nil {
		t.Fatalf("não foi possível codificar: %v", err)
	}
	if _, err := cliente.Write(dados); err != nil {
		t.Fatalf("não foi possível enviar: %v", err)
	}

	// O servidor tem de responder mesmo quando não consegue gravar.
	_ = cliente.SetReadDeadline(time.Now().Add(3 * time.Second))
	buffer := make([]byte, tamanhoBuffer)
	_, err = cliente.Read(buffer)
	return err
}

// O consumo reportado pelo router é convertido de octetos para megabytes e
// entregue ao contabilizador.
func TestAccountingGravaConsumoEmMB(t *testing.T) {
	contabilizador := &contabilizadorFalso{}
	srv := montarServidor(t, "segredo-partilhado", &fornecedorFalso{valido: true}).
		ComContabilizador(contabilizador)

	const mb = 1024 * 1024
	if err := enviarAccounting(t, srv, "sessao-42", 10*mb, 5*mb); err != nil {
		t.Fatalf("sem resposta: %v", err)
	}

	if contabilizador.chamadas != 1 {
		t.Fatalf("esperava 1 chamada, obteve %d", contabilizador.chamadas)
	}
	if contabilizador.sessionID != "sessao-42" {
		t.Errorf("Session-ID errado: %q", contabilizador.sessionID)
	}
	if contabilizador.entrada != 10 || contabilizador.saida != 5 {
		t.Errorf("conversão para MB errada: %v/%v",
			contabilizador.entrada, contabilizador.saida)
	}
}

// Mesmo quando a gravação falha, o router recebe Accounting-Response: um
// router sem resposta tende a repetir o pedido e a perder a sessão.
func TestAccountingRespondeMesmoComErro(t *testing.T) {
	contabilizador := &contabilizadorFalso{err: errors.New("base de dados indisponível")}
	srv := montarServidor(t, "segredo-partilhado", &fornecedorFalso{valido: true}).
		ComContabilizador(contabilizador)

	if err := enviarAccounting(t, srv, "sessao-42", 1024, 1024); err != nil {
		t.Fatalf("o servidor devia responder mesmo com erro na gravação: %v", err)
	}
	if contabilizador.chamadas != 1 {
		t.Fatalf("o contabilizador devia ter sido chamado, obteve %d", contabilizador.chamadas)
	}
}

// O Session-Id do pfSense chega em 32 bytes binários; tem de ser reconhecido,
// e bytes genuinamente binários convertidos para hexadecimal.
func TestSessionIDBinarioDoPfSense(t *testing.T) {
	binario := make([]byte, 32)
	copy(binario, []byte("sessao-binaria"))
	pedido := &Pacote{Atributos: []Atributo{{Tipo: AttrSessionID, Valor: binario}}}
	if got := lerSessionID(pedido); got != "sessao-binaria" {
		t.Errorf("esperava 'sessao-binaria', recebeu %q", got)
	}

	aleatorio := []byte{0x00, 0xff, 0x10, 0xab}
	pedido = &Pacote{Atributos: []Atributo{{Tipo: AttrSessionID, Valor: aleatorio}}}
	if got := lerSessionID(pedido); got != "00ff10ab" {
		t.Errorf("esperava hexadecimal, recebeu %q", got)
	}
}

// Sem Session-Id não há consumo a atribuir, mas a resposta continua a ser dada.
func TestAccountingSemSessionIDNaoFalha(t *testing.T) {
	contabilizador := &contabilizadorFalso{}
	srv := montarServidor(t, "segredo-partilhado", &fornecedorFalso{valido: true}).
		ComContabilizador(contabilizador)

	cliente, err := net.Dial("udp", srv.Endereco())
	if err != nil {
		t.Fatalf("não foi possível falar com o servidor: %v", err)
	}
	defer cliente.Close()

	dados, _ := (&Pacote{
		Codigo:        CodigoAccountingRequest,
		Identificador: 12,
		Atributos: []Atributo{
			NovoAtributo(AttrUserName, "DC:A6:32:01:77:2E"),
			{Tipo: AttrAcctStatusType, Valor: []byte{7}},
			NovoAtributoInteiro(AttrAcctInputOctets, 5*1024*1024),
		},
	}).Codificar("", nil)
	if _, err := cliente.Write(dados); err != nil {
		t.Fatalf("não foi possível enviar: %v", err)
	}

	_ = cliente.SetReadDeadline(time.Now().Add(3 * time.Second))
	buffer := make([]byte, tamanhoBuffer)
	if _, err := cliente.Read(buffer); err != nil {
		t.Fatalf("sem resposta: %v", err)
	}
	if contabilizador.chamadas != 0 {
		t.Error("sem Session-Id o contabilizador não devia ser chamado")
	}
}
