package httpapi

import (
	"time"

	"example.com/spillway-gate-coordinator/internal/contracts"
	"example.com/spillway-gate-coordinator/internal/domain"
)

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func gateView(gate domain.Gate) contracts.GateView {
	return contracts.GateView{
		ID:              gate.ID,
		Name:            gate.Name,
		Status:          string(gate.Status),
		Aperture:        gate.Aperture,
		MaxAperture:     gate.MaxAperture,
		LimitRevision:   gate.LimitRevision,
		ActiveCommandID: gate.ActiveCommandID,
		UpdatedAt:       formatTime(gate.UpdatedAt),
		Version:         gate.Version,
	}
}

func limitRevisionViews(revisions []domain.LimitRevision) []contracts.LimitRevisionView {
	views := make([]contracts.LimitRevisionView, 0, len(revisions))
	for _, revision := range revisions {
		views = append(views, contracts.LimitRevisionView{
			Revision:       revision.Revision,
			MaxAperture:    revision.MaxAperture,
			Source:         string(revision.Source),
			BasisReference: revision.BasisReference,
			Reason:         revision.Reason,
			RevisedBy:      revision.RevisedBy,
			RevisedAt:      formatTime(revision.RevisedAt),
		})
	}
	return views
}

func commandView(command domain.GateCommand, includeToken bool, basisCurrent bool) contracts.CommandView {
	view := contracts.CommandView{
		ID:                 command.ID,
		GateID:             command.GateID,
		TargetAperture:     command.TargetAperture,
		StartingAperture:   command.StartingAperture,
		BasisLimitRevision: command.BasisLimitRevision,
		BasisMaxAperture:   command.BasisMaxAperture,
		BasisCurrent:       basisCurrent,
		Reason:             command.Reason,
		RequestedBy:        command.RequestedBy,
		Status:             string(command.Status),
		ExecutedBy:         command.ExecutedBy,
		TerminalReason:     command.TerminalReason,
		CreatedAt:          formatTime(command.CreatedAt),
		UpdatedAt:          formatTime(command.UpdatedAt),
		Version:            command.Version,
	}

	if includeToken && command.Status == domain.CommandApproved {
		view.ExecutionToken = command.ExecutionToken
	}
	if command.Interlock != nil {
		view.Interlock = &contracts.InterlockDecisionView{
			Verdict:    string(command.Interlock.Verdict),
			Verifier:   command.Interlock.Verifier,
			Note:       command.Interlock.Note,
			ReviewedAt: formatTime(command.Interlock.ReviewedAt),
		}
	}
	return view
}
