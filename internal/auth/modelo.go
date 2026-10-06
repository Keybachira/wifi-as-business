package auth

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

// Papel é o nível de permissão de uma conta. O sistema é genérico: não há
// dono. Todos os papeis têm os mesmos direitos; o campo existe para
// rotular contas (ex.: operador, suporte) e nada mais.
type Papel string

const (
	PapelOperador Papel = "operador"
	PapelSuporte  Papel = "suporte"
)

// Estado da conta.
const (
	EstadoAtivo      = "ativo"
	EstadoDesativado = "desativado"
)

// Conta representa um utilizador administrativo do painel.
type Conta struct {
	ID           int64      `json:"id"`
	Nome         string     `json:"nome"`
	Email        string     `json:"email"`
	SenhaHash    string     `json:"-"`
	Papel        Papel      `json:"papel"`
	Estado       string     `json:"estado"`
	UltimoAcesso *time.Time `json:"ultimo_acesso,omitempty"`
	CriadoEm     time.Time  `json:"criado_em"`
}

// PodeEditarValores indica se a conta pode criar/alterar planos e definições.
// Sistema genérico: qualquer conta ativa pode.
func (c Conta) PodeEditarValores() bool { return true }

// PodeGerirContas indica se a conta pode administrar outras contas.
// Sistema genérico, sem dono: qualquer conta ativa pode.
func (c Conta) PodeGerirContas() bool { return true }

// EstaAtiva indica se a conta pode iniciar sessão.
func (c Conta) EstaAtiva() bool { return c.Estado == EstadoAtivo }

// Credenciais são os dados de entrada do formulário de login.
type Credenciais struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// CriarConta são os dados para registar uma nova conta.
type CriarConta struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
	Papel Papel  `json:"papel"`
	Ativo *bool  `json:"ativo,omitempty"`
}

// Validar normaliza e valida os dados de criação de conta.
func (c *CriarConta) Validar() error {
	c.Nome = strings.TrimSpace(c.Nome)
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))

	if c.Nome == "" {
		return errors.New("o nome é obrigatório")
	}
	if len(c.Nome) > 120 {
		return errors.New("o nome não pode exceder 120 caracteres")
	}
	if !emailValido(c.Email) {
		return errors.New("o email indicado é inválido")
	}
	if len(c.Senha) < 8 {
		return errors.New("a senha deve ter no mínimo 8 caracteres")
	}
	if c.Papel == "" {
		c.Papel = PapelOperador
	}
	if !c.PapelValido() {
		return errors.New("papel inválido")
	}
	return nil
}

func (c CriarConta) PapelValido() bool {
	switch c.Papel {
	case PapelOperador, PapelSuporte:
		return true
	}
	return false
}

// AtivoOuPadrao devolve o estado ativo, assumindo "ativo" quando não especificado.
func (c CriarConta) AtivoOuPadrao() bool { return c.Ativo == nil || *c.Ativo }

// emailValido faz uma verificação mínima mas rigorosa do formato.
func emailValido(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	partes := strings.Split(email, "@")
	if len(partes) != 2 || partes[0] == "" || partes[1] == "" {
		return false
	}
	if strings.Contains(partes[1], ".") == false {
		return false
	}
	if strings.ContainsAny(email, " \t\n") {
		return false
	}
	return true
}

// RequisitosSenha descreve os critérios mínimos de uma senha aceite.
type RequisitosSenha struct {
	MinimoCaracteres int  `json:"minimo_caracteres"`
	ExigirMaiuscula  bool `json:"exigir_maiuscula"`
	ExigirNumero     bool `json:"exigir_numero"`
}

// RequisitosPadrao é a política de senhas aplicada ao sistema.
var RequisitosPadrao = RequisitosSenha{MinimoCaracteres: 8, ExigirMaiuscula: true, ExigirNumero: true}

// ValidarSenha verifica a senha contra a política configurada.
func ValidarSenha(senha string) error {
	if len([]rune(senha)) < RequisitosPadrao.MinimoCaracteres {
		return errors.New("a senha deve ter no mínimo 8 caracteres")
	}
	var temMaiuscula, temNumero bool
	for _, r := range senha {
		if unicode.IsUpper(r) {
			temMaiuscula = true
		}
		if unicode.IsDigit(r) {
			temNumero = true
		}
	}
	if RequisitosPadrao.ExigirMaiuscula && !temMaiuscula {
		return errors.New("a senha deve conter pelo menos uma letra maiúscula")
	}
	if RequisitosPadrao.ExigirNumero && !temNumero {
		return errors.New("a senha deve conter pelo menos um número")
	}
	return nil
}
