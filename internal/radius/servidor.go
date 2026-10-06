package radius

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"strings"
	"sync"
)

// tamanhoBuffer é o maior datagrama lido de uma vez. O protocolo não permite
// mais do que 4096, por isso ler mais seria aceitar lixo.
const tamanhoBuffer = 4096

// Contabilizador regista o consumo reportado por um router.
type Contabilizador interface {
	// RegistrarConsumo grava os megabytes consumidos e devolve true se a
	// sessão tiver sido encerrada por ter ultrapassado o limite do plano.
	RegistrarConsumo(ctx context.Context, sessionID string, downloadMB, uploadMB float64) (bool, error)
}

// Servidor escuta pedidos RADIUS em UDP e responde com o resultado.
//
// É o que permite a um pfSense (ou MikroTik) validar vouchers contra este
// controlador. Fica desligado enquanto não houver segredo configurado.
type Servidor struct {
	mu             sync.RWMutex
	servico        *Serviço
	contabilizador Contabilizador
	segredo        string
	porta          int
	endereco       string
	conexao        *net.UDPConn
	registro       *slog.Logger
	parar          chan struct{}
	parou          sync.Once
	ativa          bool
	estatisticas   *Estatisticas
}

// Estatisticas conta os pedidos atendidos, para diagnóstico.
type Estatisticas struct {
	mu         sync.Mutex
	Recebidos  int
	Aceites    int
	Rejeitados int
	Erros      int
}

func (e *Estatisticas) registar(rec, ace, rej, erro int) {
	e.mu.Lock()
	e.Recebidos += rec
	e.Aceites += ace
	e.Rejeitados += rej
	e.Erros += erro
	e.mu.Unlock()
}

// NovoServidor cria o servidor RADIUS.
//
// segredo é a chave partilhada com o router (NAS). Sem ela o servidor não
// arranca: sem segredo não há forma de validar um pedido.
func NovoServidor(s *Serviço, segredo string, porta int, endereco string, registo *slog.Logger) *Servidor {
	return &Servidor{
		servico:      s,
		segredo:      segredo,
		porta:        porta,
		endereco:     endereco,
		registro:     registo,
		estatisticas: &Estatisticas{},
	}
}

// ComContabilizador liga quem grava o consumo reportado pelo router.
//
// Sem este vínculo o servidor continua a responder a Accounting-Request, mas
// os octetos não são persistidos: a resposta é sempre enviada ao router.
func (s *Servidor) ComContabilizador(c Contabilizador) *Servidor {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contabilizador = c
	return s
}

// Configurado indica que há segredo e serviço de credenciais.
func (s *Servidor) Configurado() bool {
	return s != nil && s.segredo != "" && s.servico != nil && s.servico.Ativo()
}

// Iniciar abre o socket e começa a atender pedidos em segundo plano.
func (s *Servidor) Iniciar(ctx context.Context) error {
	if !s.Configurado() {
		return ErrNaoConfigurado
	}

	endereco := net.JoinHostPort(s.endereco, fmt.Sprintf("%d", s.porta))
	addr, err := net.ResolveUDPAddr("udp", endereco)
	if err != nil {
		return fmt.Errorf("endereço RADIUS inválido %q: %w", endereco, err)
	}
	conexao, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("não foi possível abrir a porta RADIUS %s: %w", endereco, err)
	}

	s.conexao = conexao
	s.ativa = true
	s.parar = make(chan struct{})

	if s.registro != nil {
		s.registro.Info("servidor RADIUS a escutar", "endereco", conexao.LocalAddr().String())
	}

	go s.atender(ctx)
	return nil
}

// Encerrar fecha o socket e sinaliza a arRoutine de atendimento.
func (s *Servidor) Encerrar() error {
	s.parou.Do(func() {
		if s.parar != nil {
			close(s.parar)
		}
	})
	if s.conexao != nil {
		err := s.conexao.Close()
		s.mu.Lock()
		s.ativa = false
		s.mu.Unlock()
		return err
	}
	return nil
}

// Ativa indica se o socket está aberto.
func (s *Servidor) Ativa() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ativa
}

// Endereco devolve o endereço onde o servidor escuta.
func (s *Servidor) Endereco() string {
	return net.JoinHostPort(s.endereco, fmt.Sprintf("%d", s.porta))
}

// atender é o ciclo de leitura de datagramas.
func (s *Servidor) atender(ctx context.Context) {
	buffer := make([]byte, tamanhoBuffer)
	for {
		n, origem, err := s.conexao.ReadFromUDP(buffer)
		if err != nil {
			// Um socket fechado é o caminho normal de encerramento.
			select {
			case <-s.parar:
				if s.registro != nil {
					s.registro.Info("servidor RADIUS encerrado")
				}
				return
			default:
			}
			if ctx.Err() != nil {
				return
			}
			if s.registro != nil {
				s.registro.Warn("erro a ler pedido RADIUS", "erro", err)
			}
			continue
		}

		// Copia os bytes: o buffer é reutilizado no ciclo seguinte e cada
		// pedido é atendido na sua própria rotina, para que um pedido lento
		// não bloqueie os restantes clientes.
		dados := make([]byte, n)
		copy(dados, buffer[:n])
		go s.processar(dados, origem)
	}
}

// processar descodifica um pedido e encaminha-o para o tratamento correto.
func (s *Servidor) processar(dados []byte, origem *net.UDPAddr) {
	pedido, err := Descodificar(dados)
	if err != nil {
		s.estatisticas.registar(1, 0, 0, 1)
		if s.registro != nil {
			s.registro.Warn("pacote RADIUS inválido", "origem", origem.String(), "erro", err)
		}
		return
	}

	switch pedido.Codigo {
	case CodigoAccessRequest:
		s.tratarAccessRequest(pedido, origem)
	case CodigoAccountingRequest:
		s.tratarAccounting(pedido, origem)
	default:
		s.estatisticas.registar(1, 0, 0, 1)
		if s.registro != nil {
			s.registro.Warn("código RADIUS não suportado",
				"codigo", pedido.Codigo, "origem", origem.String())
		}
	}
}

// tratarAccessRequest valida as credenciais e responde Accept ou Reject.
func (s *Servidor) tratarAccessRequest(pedido *Pacote, origem *net.UDPAddr) {
	utilizador := pedido.Texto(AttrUserName)
	segredoEnviado, temSenha := pedido.Atributo(AttrUserPassword)

	if utilizador == "" || !temSenha {
		s.responder(pedido, origem, CodigoAccessReject, nil)
		return
	}

	// O User-Name transporta o endereço MAC do equipamento (é o que o
	// captive portal conhece) e o User-Password encriptado traz o vale.
	// O serviço valida o par contra a base de dados dos vales.
	informacao, err := s.servico.AutenticarRede(pedido, utilizador, s.segredo, segredoEnviado)
	if err != nil {
		if s.registro != nil {
			s.registro.Warn("credencial RADIUS rejeitada",
				"utilizador", utilizador, "origem", origem.String())
		}
		s.responder(pedido, origem, CodigoAccessReject, nil)
		return
	}

	atributos := []Atributo{
		// Service-Type em binário (1 = Framed), por isso vai directo.
		{Tipo: AttrServiceType, Valor: []byte{ServicoLoginUser}},
		NovoAtributo(AttrClass, informacao.SessionID),
	}
	// O Session-Timeout é o que faz o firewall desligar o cliente sozinho
	// quando o tempo do plano acaba.
	if informacao.LimiteMinutos > 0 {
		segundos := informacao.LimiteMinutos * 60
		if segundos > math.MaxUint32 {
			segundos = math.MaxUint32
		}
		atributos = append(atributos, NovoAtributoInteiro(AttrSessionTimeout, segundos))
	}
	if len(informacao.EnderecoIP) == 4 {
		atributos = append(atributos, Atributo{Tipo: AttrFramedIPAddress, Valor: informacao.EnderecoIP})
	}

	if s.registro != nil {
		s.registro.Info("acesso RADIUS aceite",
			"utilizador", utilizador, "sessao", informacao.SessionID,
			"minutos", informacao.LimiteMinutos)
	}
	s.responder(pedido, origem, CodigoAccessAccept, atributos)
}

// responder envia o pacote de resposta ao router.
func (s *Servidor) responder(pedido *Pacote, origem *net.UDPAddr, codigo uint8, atributos []Atributo) {
	resposta := &Pacote{
		Codigo:        codigo,
		Identificador: pedido.Identificador,
		Atributos:     atributos,
	}

	// O autenticador da resposta é calculado sobre o do pedido e o segredo
	// partilhado: é isso que prova ao router que a resposta é nossa.
	dados, err := resposta.Codificar(s.segredo, &pedido.Autenticador)
	if err != nil {
		s.estatisticas.registar(1, 0, 0, 1)
		return
	}
	if _, err := s.conexao.WriteToUDP(dados, origem); err != nil {
		s.estatisticas.registar(1, 0, 0, 1)
		if s.registro != nil {
			s.registro.Warn("não foi possível responder ao router",
				"origem", origem.String(), "erro", err)
		}
		return
	}

	if codigo == CodigoAccessAccept {
		s.estatisticas.registar(1, 1, 0, 0)
	} else {
		s.estatisticas.registar(1, 0, 1, 0)
	}
}

// tratarAccounting regista o consumo reportado pelo router.
//
// O router envia três tipos de estado: Start (1), Interim-Update (2) e
// Stop (7). Todos trazem os contadores acumulados desde o início da sessão,
// por isso basta gravar o valor absoluto — somar outra vez contaria o mesmo
// tráfego várias vezes.
//
// Responder é obrigatório mesmo quando o consumo não pode ser gravado: um
// router que não recebe Accounting-Response tende a repetir o pedido e a
// perder a sessão. Por isso o erro é registado, mas não se propaga.
func (s *Servidor) tratarAccounting(pedido *Pacote, origem *net.UDPAddr) {
	estado, temEstado := pedido.Atributo(AttrAcctStatusType)
	status := uint8(0)
	if temEstado && len(estado) == 1 {
		status = estado[0]
	}

	s.mu.RLock()
	contabilizador := s.contabilizador
	s.mu.RUnlock()

	if contabilizador != nil {
		// O Session-Id (32 bytes) identifica a sessão que criámos no
		// Access-Accept. Sem ele não há a que atribuir o consumo.
		sessionID := lerSessionID(pedido)
		if sessionID != "" {
			entrada, saida := lerOctetos(pedido)
			encerrada, err := contabilizador.RegistrarConsumo(
				context.Background(), sessionID, entrada, saida)
			if err != nil {
				if s.registro != nil {
					s.registro.Warn("não foi possível gravar o consumo da sessão",
						"session_id", sessionID, "erro", err)
				}
			} else if encerrada && s.registro != nil {
				s.registro.Info("sessão cortada por excesso de dados",
					"session_id", sessionID,
					"consumido_mb", entrada+saida)
			}
		}
	}

	if s.registro != nil {
		s.registro.Debug("pedido de accounting recebido",
			"utilizador", pedido.Texto(AttrUserName),
			"estado", status, "origem", origem.String())
	}
	s.responder(pedido, origem, CodigoAccountingResponse, nil)
}

// lerSessionID extrai o Session-Id, que pode vir como texto ou em binário.
//
// O pfSense envia 32 bytes binários; outros routers enviam o mesmo valor como
// texto. Os dois são aceitos para que o servidor funcione com qualquer um.
func lerSessionID(pedido *Pacote) string {
	valor, ok := pedido.Atributo(AttrSessionID)
	if !ok || len(valor) == 0 {
		return ""
	}
	// Binário costuma vir preenchido com zeros no fim.
	if texto := strings.TrimRight(string(valor), "\x00"); isImprimivel(texto) {
		return texto
	}
	return fmt.Sprintf("%x", valor)
}

// isImprimivel indica se o texto tem só caracteres visíveis.
func isImprimivel(texto string) bool {
	if texto == "" {
		return false
	}
	for _, r := range texto {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

// lerOctetos devolve o consumo em megabytes, de download e de upload.
//
// Os atributos são inteiros de 4 bytes em octetos (RFC 2866). O cálculo usa
// megabytes binários (1 MB = 1024*1024 octetos), que é a unidade em que os
// planos expressam o limite.
func lerOctetos(pedido *Pacote) (entrada, saida float64) {
	const octetosPorMB = 1024 * 1024

	if v, ok := pedido.AtributoInteiro(AttrAcctInputOctets); ok && v > 0 {
		entrada = float64(v) / octetosPorMB
	}
	if v, ok := pedido.AtributoInteiro(AttrAcctOutputOctets); ok && v > 0 {
		saida = float64(v) / octetosPorMB
	}
	return entrada, saida
}

// Estatisticas devolve os contadores de pedidos.
func (s *Servidor) Estatisticas() (recebidos, aceites, rejeitados, erros int) {
	s.estatisticas.mu.Lock()
	defer s.estatisticas.mu.Unlock()
	return s.estatisticas.Recebidos, s.estatisticas.Aceites,
		s.estatisticas.Rejeitados, s.estatisticas.Erros
}
