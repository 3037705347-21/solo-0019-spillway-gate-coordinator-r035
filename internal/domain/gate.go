package domain

import (
	"regexp"
	"strings"
	"time"
)

type GateStatus string

const (
	GateIdle     GateStatus = "idle"
	GateReserved GateStatus = "reserved"
	GateMoving   GateStatus = "moving"
	GateFaulted  GateStatus = "faulted"
)

var gateIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,31}$`)

type Gate struct {
	ID              string
	Name            string
	Status          GateStatus
	Aperture        float64
	MaxAperture     float64
	LimitRevision   int64
	ActiveCommandID string
	UpdatedAt       time.Time
	Version         int64
}

func NewGate(id, name string, maxAperture, initialAperture float64, now time.Time) (Gate, error) {
	id = strings.TrimSpace(id)
	if !gateIDPattern.MatchString(id) {
		return Gate{}, Invalid("invalid_gate_id", "gate id must use lowercase letters, digits, and hyphens")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return Gate{}, Invalid("invalid_gate_name", "gate name is required")
	}
	if maxAperture <= 0 || maxAperture > 1000 {
		return Gate{}, Invalid("invalid_max_aperture", "max aperture must be greater than zero")
	}
	if initialAperture < 0 || initialAperture > maxAperture {
		return Gate{}, Invalid("invalid_initial_aperture", "initial aperture must be within the gate limit")
	}

	return Gate{
		ID:            id,
		Name:          name,
		Status:        GateIdle,
		Aperture:      initialAperture,
		MaxAperture:   maxAperture,
		LimitRevision: InitialLimitRevision,
		UpdatedAt:     now.UTC(),
		Version:       1,
	}, nil
}

func (g *Gate) Reserve(commandID string, now time.Time) error {
	if g.Status != GateIdle {
		return Conflict("gate_not_idle", "gate is not available for a new command")
	}
	if g.ActiveCommandID != "" {
		return Conflict("gate_already_reserved", "gate already references an active command")
	}
	if strings.TrimSpace(commandID) == "" {
		return Invalid("invalid_active_command", "active command id is required")
	}

	g.Status = GateReserved
	g.ActiveCommandID = commandID
	g.UpdatedAt = now.UTC()
	g.Version++
	return nil
}

func (g *Gate) StartMovement(commandID string, now time.Time) error {
	if g.Status != GateReserved {
		return Conflict("gate_not_reserved", "gate must be reserved before movement starts")
	}
	if g.ActiveCommandID != commandID {
		return Conflict("gate_command_mismatch", "gate is reserved by a different command")
	}

	g.Status = GateMoving
	g.UpdatedAt = now.UTC()
	g.Version++
	return nil
}

func (g *Gate) Settle(finalAperture float64, faulted bool, now time.Time) error {
	if g.Status != GateMoving {
		return Conflict("gate_not_moving", "gate must be moving before it can settle")
	}
	if finalAperture < 0 || finalAperture > g.MaxAperture {
		return Invalid("invalid_final_aperture", "final aperture must be within the gate limit")
	}

	g.Aperture = finalAperture
	g.ActiveCommandID = ""
	if faulted {
		g.Status = GateFaulted
	} else {
		g.Status = GateIdle
	}
	g.UpdatedAt = now.UTC()
	g.Version++
	return nil
}

// ReviseLimit changes the maximum allowed aperture after a structural re-review.
// It is only legal while the gate is idle with no active command, and the new
// limit must still cover the gate's current physical aperture. The gate's
// limit revision counter advances on every successful revision.
func (g *Gate) ReviseLimit(newMaxAperture float64, now time.Time) (previousMaxAperture float64, sequence int64, err error) {
	if g.Status != GateIdle || g.ActiveCommandID != "" {
		return 0, 0, Conflict(
			"gate_not_idle",
			"gate must be idle with no active command before its aperture limit can be revised",
		)
	}
	if newMaxAperture <= 0 || newMaxAperture > 1000 {
		return 0, 0, Invalid("invalid_max_aperture", "max aperture must be greater than zero")
	}
	if newMaxAperture == g.MaxAperture {
		return 0, 0, Invalid("limit_unchanged", "new max aperture must differ from the current limit")
	}
	if newMaxAperture < g.Aperture {
		return 0, 0, Conflict(
			"aperture_exceeds_new_limit",
			"current gate aperture exceeds the proposed limit; move the gate within range first",
		)
	}

	previous := g.MaxAperture
	g.MaxAperture = newMaxAperture
	g.LimitRevision++
	g.UpdatedAt = now.UTC()
	g.Version++
	return previous, g.LimitRevision, nil
}
