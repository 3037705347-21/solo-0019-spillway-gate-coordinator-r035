package storage

import (
	"context"
	"time"

	"example.com/spillway-gate-coordinator/internal/domain"
)

type Repository interface {
	CreateGate(ctx context.Context, gate domain.Gate, baseline domain.LimitRevision) error
	Gate(ctx context.Context, id string) (domain.Gate, error)
	ReviseGateLimit(
		ctx context.Context,
		gateID string,
		revisionID string,
		input domain.ReviseLimitInput,
		now time.Time,
	) (domain.Gate, domain.LimitRevision, error)
	ListLimitRevisions(ctx context.Context, gateID string) ([]domain.LimitRevision, error)
	CreateCommand(ctx context.Context, command domain.GateCommand) error
	Command(ctx context.Context, id string) (domain.GateCommand, error)
	CommandDetail(ctx context.Context, id string) (domain.GateCommand, domain.Gate, error)
	ReviewCommand(
		ctx context.Context,
		commandID string,
		verdict domain.InterlockVerdict,
		verifier string,
		note string,
		acknowledgedRevision int64,
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
