package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// tamanhoHash é o tamanho em bytes do token de sessão.
const tamanhoHash = 32

// hashSenhaFalsa iguala o tempo de resposta quando o email não existe.
const hashSenhaFalsa = "$2a$12$C6UzMDM.H6dfI/f/IKcEe.7Nq7lRR2OKe0dJqk1lEz4kQOWCbS3Ou"

// Erros do serviço de autenticação.
var (
	ErrCredenciaisInvalidas = fmt.Errorf("email ou senha incorretos")
	ErrTokenInvalido        = fmt.Errorf("token inválido ou expirado")
)

// RespostaLogin é o payload devolvido após um login bem-sucedido.
type RespostaLogin struct {
	Token    string    `json:"token"`
	ExpiraEm time.Time `json:"expira_em"`
	Conta    Conta     `json:"conta"`
}

// Serviço implementa a lógica de autenticação e gestão de sessões.
type Serviço struct {
	repo      *Repositorio
	db        *sql.DB
	duracao   time.Duration
	custoHash int
}

// NovoServico constrói o serviço de autenticação.
func NovoServico(repo *Repositorio, db *sql.DB, duracao time.Duration, custoHash int) *Serviço {
	if custoHash < bcrypt.MinCost {
		custoHash = bcrypt.DefaultCost
	}
	return &Serviço{repo: repo, db: db, duracao: duracao, custoHash: custoHash}
}

// IniciarSessao valida as credenciais e emite um token opaco.
func (s *Serviço) IniciarSessao(ctx context.Context, cred Credenciais, agente, ip string) (RespostaLogin, error) {
	conta, err := s.repo.BuscarPorEmail(ctx, cred.Email)
	if err != nil {
		// Compara sempre para não revelar quais emails existem (timing attack).
		_ = bcrypt.CompareHashAndPassword([]byte(hashSenhaFalsa), []byte(cred.Senha))
		return RespostaLogin{}, ErrCredenciaisInvalidas
	}
	if err := bcrypt.CompareHashAndPassword([]byte(conta.SenhaHash), []byte(cred.Senha)); err != nil {
		return RespostaLogin{}, ErrCredenciaisInvalidas
	}
	if !conta.EstaAtiva() {
		return RespostaLogin{}, ErrCredenciaisInvalidas
	}

	token, hash, err := gerarToken()
	if err != nil {
		return RespostaLogin{}, err
	}
	agora := time.Now().UTC()
	expira := agora.Add(s.duracao)

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessoes_admin (token_hash, conta_id, agente, endereco_ip, expira_em) VALUES (?, ?, ?, ?, ?)`,
		hash, conta.ID, trunc(agente, 255), trunc(ip, 45), expira); err != nil {
		return RespostaLogin{}, fmt.Errorf("não foi possível criar a sessão: %w", err)
	}

	_ = s.repo.RegistarAcesso(ctx, conta.ID, agora)
	return RespostaLogin{Token: token, ExpiraEm: expira, Conta: conta}, nil
}

// TerminarSessao invalida o token indicado.
func (s *Serviço) TerminarSessao(ctx context.Context, token string) error {
	hash := hashToken(token)
	if hash == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessoes_admin WHERE token_hash = ?`, hash); err != nil {
		return fmt.Errorf("não foi possível terminar a sessão: %w", err)
	}
	return nil
}

// ValidarToken devolve a conta associada a um token válido.
func (s *Serviço) ValidarToken(ctx context.Context, token string) (Conta, error) {
	hash := hashToken(token)
	if hash == "" {
		return Conta{}, ErrTokenInvalido
	}

	var contaID int64
	var expiraEm time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT conta_id, expira_em FROM sessoes_admin WHERE token_hash = ?`, hash).Scan(&contaID, &expiraEm)
	if err != nil {
		return Conta{}, ErrTokenInvalido
	}
	if time.Now().UTC().After(expiraEm) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessoes_admin WHERE token_hash = ?`, hash)
		return Conta{}, ErrTokenInvalido
	}

	conta, err := s.repo.BuscarPorID(ctx, contaID)
	if err != nil || !conta.EstaAtiva() {
		return Conta{}, ErrTokenInvalido
	}
	return conta, nil
}

// TerminarTodasSessões invalida todos os tokens de uma conta (ex.: ao desativar).
func (s *Serviço) TerminarTodasSessões(ctx context.Context, contaID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessoes_admin WHERE conta_id = ?`, contaID)
	return err
}

// LimparSessoesExpiradas remove tokens caducados. Deve correr periodicamente.
func (s *Serviço) LimparSessoesExpiradas(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessoes_admin WHERE expira_em < ?`, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CriarConta regista uma nova conta de administração.
func (s *Serviço) CriarConta(ctx context.Context, dados CriarConta) (Conta, error) {
	if err := dados.Validar(); err != nil {
		return Conta{}, err
	}
	if err := ValidarSenha(dados.Senha); err != nil {
		return Conta{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(dados.Senha), s.custoHash)
	if err != nil {
		return Conta{}, fmt.Errorf("não foi possível processar a senha: %w", err)
	}

	id, err := s.repo.Criar(ctx, dados, string(hash))
	if err != nil {
		return Conta{}, err
	}
	return s.repo.BuscarPorID(ctx, id)
}

// AlterarSenha define uma nova senha para a conta indicada.
func (s *Serviço) AlterarSenha(ctx context.Context, contaID int64, senhaAtual, novaSenha string) error {
	conta, err := s.repo.BuscarPorID(ctx, contaID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(conta.SenhaHash), []byte(senhaAtual)); err != nil {
		return fmt.Errorf("a senha atual está incorreta")
	}
	if err := ValidarSenha(novaSenha); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(novaSenha), s.custoHash)
	if err != nil {
		return fmt.Errorf("não foi possível processar a senha: %w", err)
	}
	if err := s.repo.AtualizarSenha(ctx, contaID, string(hash)); err != nil {
		return err
	}
	// Por segurança, invalida as outras sessões após a troca de senha.
	return s.TerminarTodasSessões(ctx, contaID)
}

// AlterarEstado ativa ou desativa uma conta.
// Sistema genérico, sem dono: não há conta que não possa ser desativada.
// Como autodesproteção mínima, impede desativar a própria conta em sessão.
func (s *Serviço) AlterarEstado(ctx context.Context, contaID int64, ativo bool, contaSessaoID int64) error {
	if !ativo && contaID == contaSessaoID {
		return fmt.Errorf("não é possível desativar a própria conta com sessão iniciada")
	}

	if err := s.repo.AtualizarEstado(ctx, contaID, ativo); err != nil {
		return err
	}
	if !ativo {
		return s.TerminarTodasSessões(ctx, contaID)
	}
	return nil
}

// AlterarPapel muda o rótulo da conta. Sem papéis de privilégio, é apenas
// organização: qualquer valor não vazio é aceite como texto.
func (s *Serviço) AlterarPapel(ctx context.Context, contaID int64, papel Papel) error {
	papel = Papel(strings.ToLower(strings.TrimSpace(string(papel))))
	if papel == "" {
		papel = PapelOperador
	}
	if len(papel) > 30 {
		return fmt.Errorf("o papel não pode exceder 30 caracteres")
	}
	return s.repo.AtualizarPapel(ctx, contaID, papel)
}

// Listar devolve todas as contas.
func (s *Serviço) Listar(ctx context.Context) ([]Conta, error) { return s.repo.Listar(ctx) }

// Obter devolve uma conta pelo identificador.
func (s *Serviço) Obter(ctx context.Context, id int64) (Conta, error) {
	return s.repo.BuscarPorID(ctx, id)
}

// ExisteAlgumaConta indica se o sistema já tem contas registadas.
func (s *Serviço) ExisteAlgumaConta(ctx context.Context) (bool, error) {
	return s.repo.Existe(ctx)
}

// gerarToken devolve um token opaco de 32 bytes e o seu SHA-256 em hexadecimal.
func gerarToken() (token, hash string, err error) {
	buffer := make([]byte, tamanhoHash)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", fmt.Errorf("não foi possível gerar o token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(buffer)
	return token, hashToken(token), nil
}

// hashToken calcula o SHA-256 hexadecimal do token (é o que fica guardado).
func hashToken(token string) string {
	if token == "" {
		return ""
	}
	soma := sha256.Sum256([]byte(token))
	return hex.EncodeToString(soma[:])
}

// ExtrairTokenBearer lê o token do cabeçalho Authorization.
func ExtrairTokenBearer(cabecalho string) string {
	const prefixo = "Bearer "
	if len(cabecalho) <= len(prefixo) || !strings.EqualFold(cabecalho[:len(prefixo)], prefixo) {
		return ""
	}
	return strings.TrimSpace(cabecalho[len(prefixo):])
}

// trunc limita o tamanho de um texto para caber na coluna.
func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
