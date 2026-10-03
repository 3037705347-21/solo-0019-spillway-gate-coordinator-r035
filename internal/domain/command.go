package domain

import (
	"strings"
	"time"
)

type CommandStatus string

const (
	CommandPending   CommandStatus = "pending"
	CommandApproved  CommandStatus = "approved"
	CommandRejected  CommandStatus = "rejected"
	CommandExecuting CommandStatus = "executing"
	CommandCompleted CommandStatus = "completed"
	CommandAborted   CommandStatus = "aborted"
)

type GateCommand struct {
	ID                 string
	GateID             string
	TargetAperture     float64
	StartingAperture   float64
	BasisLimitRevision int64
	BasisMaxAperture   float64
	Reason             string
	RequestedBy        string
	Status             CommandStatus
	Interlock          *InterlockDecision
	ExecutionToken     string
	ExecutedBy         string
	TerminalReason     string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Version            int64
}

func NewGateCommand(
	commandID string,
	gateID string,
	targetAperture float64,
	startingAperture float64,
	maxAperture float64,
	limitRevision int64,
	reason string,
	requestedBy string,
	now time.Time,
) (GateCommand, error) {
	commandID = strings.TrimSpace(commandID)
	if commandID == "" {
		return GateCommand{}, Invalid("invalid_command_id", "command id is required")
	}

	gateID = strings.TrimSpace(gateID)
	if gateID == "" {
		return GateCommand{}, Invalid("invalid_gate_reference", "gate id is required")
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		return GateCommand{}, Invalid("invalid_command_reason", "reason is required")
	}

	requestedBy = strings.TrimSpace(requestedBy)
	if requestedBy == "" {
		return GateCommand{}, Invalid("invalid_requested_by", "requested_by is required")
	}

	if startingAperture < 0 || startingAperture > maxAperture {
		return GateCommand{}, Invalid("invalid_starting_aperture", "starting aperture is outside the gate limit")
	}
	if targetAperture < 0 || targetAperture > maxAperture {
		return GateCommand{}, Invalid("invalid_target_aperture", "target aperture is outside the gate limit")
	}
	if targetAperture == startingAperture {
		return GateCommand{}, Invalid("no_aperture_change", "target aperture must differ from the current aperture")
	}
	if limitRevision <= 0 {
		return GateCommand{}, Invalid("invalid_limit_basis", "limit revision is required")
	}

	return GateCommand{
		ID:                 commandID,
		GateID:             gateID,
		TargetAperture:     targetAperture,
		StartingAperture:   startingAperture,
		BasisLimitRevision: limitRevision,
		BasisMaxAperture:   maxAperture,
		Reason:             reason,
		RequestedBy:        requestedBy,
		Status:             CommandPending,
		CreatedAt:          now.UTC(),
		UpdatedAt:          now.UTC(),
		Version:            1,
	}, nil
}

// BasisIsCurrent reports whether the command was requested against the limit
// revision currently in force. A stale basis must be reconfirmed before a
// clear review can reserve the gate.
func (c *GateCommand) BasisIsCurrent(currentRevision int64) bool {
	return c.BasisLimitRevision == currentRevision
}

// ReconfirmBasis rebases a still-pending command onto the gate's current
// limit revision after a manager re-verified the request. The frozen target
// must remain valid under the new limit, and a command already on the
// current revision needs no reconfirmation.
func (c *GateCommand) ReconfirmBasis(currentRevision int64, currentMax float64, confirmedBy string, now time.Time) error {
	if c.Status != CommandPending {
		return Conflict("command_not_pending", "only pending commands can reconfirm their limit basis")
	}
	confirmedBy = strings.TrimSpace(confirmedBy)
	if confirmedBy == "" {
		return Invalid("invalid_confirmed_by", "confirmed_by is required")
	}
	if currentRevision <= 0 {
		return Invalid("invalid_limit_basis", "current limit revision is unknown")
	}
	if currentRevision == c.BasisLimitRevision {
		return Conflict("limit_basis_current", "command already references the current limit revision")
	}
	if c.TargetAperture > currentMax || c.TargetAperture < 0 {
		return Invalid("invalid_target_aperture", "target aperture is outside the revised gate limit")
	}
	if c.StartingAperture > currentMax || c.StartingAperture < 0 {
		return Invalid("invalid_starting_aperture", "starting aperture is outside the revised gate limit")
	}

	c.BasisLimitRevision = currentRevision
	c.BasisMaxAperture = currentMax
	c.UpdatedAt = now.UTC()
	c.Version++
	return nil
}

func (c *GateCommand) Review(
	decision InterlockDecision,
	token string,
	currentLimitRevision int64,
	now time.Time,
) error {
	if c.Status != CommandPending {
		return Conflict("command_not_pending", "only pending commands can be reviewed")
	}
	// A clear verdict is a safety acceptance that reserves the gate; it must
	// always be checked against the limit currently in force. A blocked
	// verdict only rejects the command and may proceed on any basis.
	if decision.IsClear() && !c.BasisIsCurrent(currentLimitRevision) {
		return Conflict(
			"command_limit_basis_stale",
			"the gate limit changed after this command was requested; reconfirm the command before a clear review",
		)
	}

	decisionCopy := decision
	c.Interlock = &decisionCopy
	c.UpdatedAt = now.UTC()
	c.Version++

	if decision.IsClear() {
		token = strings.TrimSpace(token)
		if token == "" {
			return Invalid("missing_execution_token", "clear decisions require an execution token")
		}
		c.ExecutionToken = token
		c.Status = CommandApproved
		return nil
	}

	c.ExecutionToken = ""
	c.Status = CommandRejected
	return nil
}

func (c *GateCommand) BeginExecution(token, operator string, now time.Time) error {
	if c.Status != CommandApproved {
		return Conflict("command_not_executable", "only approved commands can execute")
	}
	if strings.TrimSpace(operator) == "" {
		return Invalid("invalid_operator", "operator is required")
	}
	if strings.TrimSpace(token) == "" || token != c.ExecutionToken {
		return Conflict("invalid_execution_token", "execution token is invalid")
	}

	c.ExecutedBy = strings.TrimSpace(operator)
	c.Status = CommandExecuting
	c.UpdatedAt = now.UTC()
	c.Version++
	return nil
}

func (c *GateCommand) Complete(now time.Time) error {
	if c.Status != CommandExecuting {
		return Conflict("command_not_executing", "command is not executing")
	}

	c.ExecutionToken = ""
	c.Status = CommandCompleted
	c.UpdatedAt = now.UTC()
	c.Version++
	return nil
}

func (c *GateCommand) Abort(reason string, now time.Time) error {
	if c.Status != CommandExecuting {
		return Conflict("command_not_executing", "command is not executing")
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "gate_execution_failed"
	}
	c.ExecutionToken = ""
	c.TerminalReason = reason
	c.Status = CommandAborted
	c.UpdatedAt = now.UTC()
	c.Version++
	return nil
}
