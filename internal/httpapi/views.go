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

func limitRevisionView(revision domain.LimitRevision) contracts.LimitRevisionView {
	return contracts.LimitRevisionView{
		ID:                  revision.ID,
		GateID:              revision.GateID,
		Sequence:            revision.Sequence,
		PreviousMaxAperture: revision.PreviousMaxAperture,
		NewMaxAperture:      revision.NewMaxAperture,
		Basis:               revision.Basis,
		Reason:              revision.Reason,
		RequestedBy:         revision.RequestedBy,
		CreatedAt:           formatTime(revision.CreatedAt),
	}
}

// commandView renders a command together with the gate's current limit so
// reads can tell whether the command's review basis is still current.
func commandView(command domain.GateCommand, gate domain.Gate, includeToken bool) contracts.CommandView {
	view := contracts.CommandView{
		ID:                 command.ID,
		GateID:             command.GateID,
		TargetAperture:     command.TargetAperture,
		StartingAperture:   command.StartingAperture,
		LimitMaxAperture:   command.LimitMaxAperture,
		LimitRevision:      command.LimitRevision,
		CurrentMaxAperture: gate.MaxAperture,
		LimitCurrent:       command.LimitRevision == gate.LimitRevision,
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
			Verdict:       string(command.Interlock.Verdict),
			Verifier:      command.Interlock.Verifier,
			Note:          command.Interlock.Note,
			LimitRevision: command.Interlock.LimitRevision,
			ReviewedAt:    formatTime(command.Interlock.ReviewedAt),
		}
	}
	return view
}
