package storage

import (
	"context"
	"time"

	"example.com/spillway-gate-coordinator/internal/domain"
)

type Repository interface {
	CreateGate(ctx context.Context, gate domain.Gate) error
	Gate(ctx context.Context, id string) (domain.Gate, error)
	ReviseGateLimit(
		ctx context.Context,
		gateID string,
		maxAperture float64,
		basisReference string,
		reason string,
		revisedBy string,
		now time.Time,
	) (domain.Gate, error)
	GateLimitRevisions(ctx context.Context, gateID string) ([]domain.LimitRevision, error)
	CreateCommand(ctx context.Context, command domain.GateCommand) error
	// CommandDetail returns a command together with its gate under a single
	// read lock so the limit-basis comparison is consistent.
	CommandDetail(ctx context.Context, id string) (domain.GateCommand, domain.Gate, error)
	ReconfirmCommand(
		ctx context.Context,
		commandID string,
		confirmedBy string,
		now time.Time,
	) (domain.GateCommand, domain.Gate, error)
	ReviewCommand(
		ctx context.Context,
		commandID string,
		decision domain.InterlockDecision,
		token string,
		now time.Time,
	) (domain.GateCommand, domain.Gate, error)
	ExecuteCommand(
		ctx context.Context,
		commandID string,
		token string,
		operator string,
		simulateFault bool,
		now time.Time,
	) (domain.GateCommand, domain.Gate, error)
}
