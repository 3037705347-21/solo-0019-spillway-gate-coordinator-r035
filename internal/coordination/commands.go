package coordination

import (
	"context"

	"example.com/spillway-gate-coordinator/internal/contracts"
	"example.com/spillway-gate-coordinator/internal/domain"
)

func (s *Service) RequestCommand(
	ctx context.Context,
	request contracts.RequestCommandRequest,
) (domain.GateCommand, domain.Gate, error) {
	gate, err := s.repository.Gate(ctx, request.GateID)
	if err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	commandID, err := s.newID("cmd")
	if err != nil {
		return domain.GateCommand{}, domain.Gate{}, domain.Internal("command_id_generation_failed", "command id could not be generated")
	}

	command, err := domain.NewGateCommand(
		commandID,
		gate.ID,
		request.TargetAperture,
		gate.Aperture,
		gate.MaxAperture,
		gate.LimitRevision,
		request.Reason,
		request.RequestedBy,
		s.now(),
	)
	if err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}
	if err := s.repository.CreateCommand(ctx, command); err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}
	return command, gate, nil
}

// CommandDetail returns the command together with the current gate snapshot
// so reads can interpret the command against the gate's present limit.
func (s *Service) CommandDetail(ctx context.Context, id string) (domain.GateCommand, domain.Gate, error) {
	return s.repository.CommandDetail(ctx, id)
}

func (s *Service) ReviewInterlock(
	ctx context.Context,
	commandID string,
	request contracts.ReviewInterlockRequest,
) (domain.GateCommand, domain.Gate, error) {
	verdict := domain.NormalizeInterlockVerdict(request.Verdict)
	if verdict != domain.InterlockClear && verdict != domain.InterlockBlocked {
		return domain.GateCommand{}, domain.Gate{}, domain.Invalid(
			"invalid_interlock_verdict",
			"verdict must be clear or blocked",
		)
	}

	command, _, err := s.repository.CommandDetail(ctx, commandID)
	if err != nil {
		return domain.GateCommand{}, domain.Gate{}, err
	}

	// A reviewer that does not name a limit revision is assumed to stand by
	// the basis captured when the command was requested. Any later revision
	// therefore surfaces as the same stable conflict as a stale basis.
	acknowledged := request.LimitRevision
	if acknowledged == 0 {
		acknowledged = command.LimitRevision
	}

	token := ""
	if verdict == domain.InterlockClear {
		token, err = s.newToken()
		if err != nil {
			return domain.GateCommand{}, domain.Gate{}, domain.Internal(
				"execution_token_generation_failed",
				"execution token could not be generated",
			)
		}
	}

	return s.repository.ReviewCommand(
		ctx,
		commandID,
		verdict,
		request.Verifier,
		request.Note,
		acknowledged,
		token,
		s.now(),
	)
}

func (s *Service) ExecuteCommand(
	ctx context.Context,
	commandID string,
	request contracts.ExecuteCommandRequest,
) (domain.GateCommand, domain.Gate, error) {
	return s.repository.ExecuteCommand(
		ctx,
		commandID,
		request.Token,
		request.Operator,
		request.SimulateFault,
		s.now(),
	)
}
