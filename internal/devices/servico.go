package devices

import (
	"context"
	"log/slog"
)

// Servico contem a logica de negocio dos equipamentos.
type Servico struct {
	repo    *Repositorio
	registo *slog.Logger
}

// NovoServico cria o servico de dispositivos.
func NovoServico(repo *Repositorio, registo *slog.Logger) *Servico {
	return &Servico{repo: repo, registo: registo}
}

// Listar devolve os equipamentos filtrando por tipo e estado.
func (s *Servico) Listar(ctx context.Context, tipo, estado string) ([]Dispositivo, error) {
	return s.repo.Listar(ctx, tipo, estado)
}

// Obter devolve um equipamento pelo identificador.
func (s *Servico) Obter(ctx context.Context, id int64) (Dispositivo, error) {
	return s.repo.Buscar(ctx, id)
}

// Registar valida e persiste um novo equipamento.
func (s *Servico) Registar(ctx context.Context, n NovoDispositivo) (Dispositivo, error) {
	if err := n.Validar(); err != nil {
		return Dispositivo{}, err
	}
	id, err := s.repo.Criar(ctx, n)
	if err != nil {
		return Dispositivo{}, err
	}
	s.registo.Info("dispositivo registado", "nome", n.Nome, "tipo", n.Tipo, "ip", n.EnderecoIP)
	return s.repo.Buscar(ctx, id)
}

// AtualizarTelemetria grava as metricas coletadas do equipamento.
func (s *Servico) AtualizarTelemetria(ctx context.Context, id int64, t Telemetria) (Dispositivo, error) {
	if err := s.repo.AtualizarTelemetria(ctx, id, t); err != nil {
		return Dispositivo{}, err
	}
	return s.repo.Buscar(ctx, id)
}

// Eliminar remove o equipamento do inventario.
func (s *Servico) Eliminar(ctx context.Context, id int64) error {
	return s.repo.Eliminar(ctx, id)
}

// Resumo devolve as contagens para o painel.
func (s *Servico) Resumo(ctx context.Context) (map[string]int, error) {
	return s.repo.Resumo(ctx)
}