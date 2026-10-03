package coordination

import (
	"context"
	"strings"

	"example.com/spillway-gate-coordinator/internal/contracts"
	"example.com/spillway-gate-coordinator/internal/domain"
)

func (s *Service) RegisterGate(ctx context.Context, request contracts.RegisterGateRequest) (domain.Gate, error) {
	gate, err := domain.NewGate(
		request.ID,
		request.Name,
		request.MaxAperture,
		request.InitialAperture,
		s.now(),
	)
	if err != nil {
		return domain.Gate{}, err
	}

	// Every gate starts with an auditable baseline limit snapshot.
	baseline, err := domain.NewRegistrationLimitRevision(
		gate.ID+"-r1",
		gate.ID,
		gate.MaxAperture,
		s.now(),
	)
	if err != nil {
		return domain.Gate{}, err
	}

	if err := s.repository.CreateGate(ctx, gate, baseline); err != nil {
		return domain.Gate{}, err
	}
	return gate, nil
}

func (s *Service) Gate(ctx context.Context, id string) (domain.Gate, error) {
	return s.repository.Gate(ctx, id)
}

func (s *Service) ReviseGateLimit(
	ctx context.Context,
	gateID string,
	request contracts.ReviseGateLimitRequest,
) (domain.Gate, domain.LimitRevision, error) {
	input, err := domain.NewReviseLimitInput(
		request.NewMaxAperture,
		request.Basis,
		request.Reason,
		request.RequestedBy,
	)
	if err != nil {
		return domain.Gate{}, domain.LimitRevision{}, err
	}

	revisionID, err := s.newID("rev")
	if err != nil {
		return domain.Gate{}, domain.LimitRevision{}, domain.Internal(
			"revision_id_generation_failed",
			"limit revision id could not be generated",
		)
	}

	return s.repository.ReviseGateLimit(ctx, strings.TrimSpace(gateID), revisionID, input, s.now())
}

func (s *Service) LimitRevisions(ctx context.Context, gateID string) ([]domain.LimitRevision, error) {
	return s.repository.ListLimitRevisions(ctx, strings.TrimSpace(gateID))
}
