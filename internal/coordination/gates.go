package coordination

import (
	"context"

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
	if err := s.repository.CreateGate(ctx, gate); err != nil {
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
	request contracts.ReviseLimitRequest,
) (domain.Gate, error) {
	return s.repository.ReviseGateLimit(
		ctx,
		gateID,
		request.MaxAperture,
		request.BasisReference,
		request.Reason,
		request.RevisedBy,
		s.now(),
	)
}

func (s *Service) GateLimitRevisions(ctx context.Context, gateID string) (string, []domain.LimitRevision, error) {
	revisions, err := s.repository.GateLimitRevisions(ctx, gateID)
	if err != nil {
		return "", nil, err
	}
	return gateID, revisions, nil
}
