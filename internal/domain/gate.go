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
	LimitHistory    []LimitRevision
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

	initialLimit := newRegistrationLimit(maxAperture, now)
	return Gate{
		ID:            id,
		Name:          name,
		Status:        GateIdle,
		Aperture:      initialAperture,
		MaxAperture:   maxAperture,
		LimitRevision: initialLimit.Revision,
		LimitHistory:  []LimitRevision{initialLimit},
		UpdatedAt:     now.UTC(),
		Version:       1,
	}, nil
}

// ReviseLimit replaces the maximum aperture based on a cited structural
// review. It is only safe on an idle gate without an active command, and the
// new limit must stay at or above the physical aperture the gate is at.
func (g *Gate) ReviseLimit(
	maxAperture float64,
	basisReference, reason, revisedBy string,
	now time.Time,
) error {
	basisReference, reason, revisedBy, err := validateRevisionEvidence(basisReference, reason, revisedBy)
	if err != nil {
		return err
	}
	if maxAperture <= 0 || maxAperture > 1000 {
		return Invalid("invalid_max_aperture", "max aperture must be greater than zero")
	}
	if g.Status != GateIdle || g.ActiveCommandID != "" {
		return Conflict("gate_not_idle", "limit can only be revised while the gate is idle and unreserved")
	}
	if maxAperture < g.Aperture {
		return Invalid("limit_below_current_aperture", "revised limit must not be below the current aperture")
	}
	if maxAperture == g.MaxAperture {
		return Conflict("limit_unchanged", "revised limit is identical to the current limit")
	}

	entry := LimitRevision{
		Revision:       g.LimitRevision + 1,
		MaxAperture:    maxAperture,
		Source:         LimitFromStructuralReview,
		BasisReference: basisReference,
		Reason:         reason,
		RevisedBy:      revisedBy,
		RevisedAt:      now.UTC(),
	}
	g.MaxAperture = maxAperture
	g.LimitRevision = entry.Revision
	g.LimitHistory = append(append([]LimitRevision(nil), g.LimitHistory...), entry)
	g.UpdatedAt = now.UTC()
	g.Version++
	return nil
}

// LimitHistoryCopy returns a defensive copy of the immutable revision trail.
func (g *Gate) LimitHistoryCopy() []LimitRevision {
	return append([]LimitRevision(nil), g.LimitHistory...)
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
