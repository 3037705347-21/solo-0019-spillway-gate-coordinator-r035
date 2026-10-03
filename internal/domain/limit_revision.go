package domain

import (
	"strings"
	"time"
)

// InitialLimitRevision identifies the limit established when a gate is
// registered. Every subsequent revision increments the gate's counter.
const InitialLimitRevision int64 = 1

// LimitRevisionBasisRegistration is the audit basis recorded for the limit
// snapshot created together with gate registration.
const LimitRevisionBasisRegistration = "gate_registration"

// LimitRevision records one auditable change of a gate's maximum aperture.
// The sequence starts at one for the registration snapshot and is monotonic
// per gate; records are append-only and never rewritten.
type LimitRevision struct {
	ID                  string
	GateID              string
	Sequence            int64
	PreviousMaxAperture float64
	NewMaxAperture      float64
	Basis               string
	Reason              string
	RequestedBy         string
	CreatedAt           time.Time
}

// ReviseLimitInput carries the manager-supplied evidence for a limit
// revision. The resulting sequence and previous limit are determined
// atomically against the persisted gate, not by the caller.
type ReviseLimitInput struct {
	NewMaxAperture float64
	Basis          string
	Reason         string
	RequestedBy    string
}

// NewReviseLimitInput validates the revision evidence before it enters the
// atomic gate update.
func NewReviseLimitInput(newMaxAperture float64, basis, reason, requestedBy string) (ReviseLimitInput, error) {
	basis = strings.TrimSpace(basis)
	if basis == "" {
		return ReviseLimitInput{}, Invalid("invalid_revision_basis", "revision basis is required")
	}
	if basis == LimitRevisionBasisRegistration {
		return ReviseLimitInput{}, Invalid("invalid_revision_basis", "the registration basis is reserved for the initial limit")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ReviseLimitInput{}, Invalid("invalid_revision_reason", "revision reason is required")
	}
	requestedBy = strings.TrimSpace(requestedBy)
	if requestedBy == "" {
		return ReviseLimitInput{}, Invalid("invalid_revision_requested_by", "requested_by is required")
	}
	if newMaxAperture <= 0 || newMaxAperture > 1000 {
		return ReviseLimitInput{}, Invalid("invalid_max_aperture", "max aperture must be greater than zero")
	}

	return ReviseLimitInput{
		NewMaxAperture: newMaxAperture,
		Basis:          basis,
		Reason:         reason,
		RequestedBy:    requestedBy,
	}, nil
}

// NewRegistrationLimitRevision creates the immutable baseline snapshot of the
// gate's limit at registration time.
func NewRegistrationLimitRevision(
	revisionID string,
	gateID string,
	maxAperture float64,
	now time.Time,
) (LimitRevision, error) {
	return newLimitRevision(
		revisionID,
		gateID,
		InitialLimitRevision,
		maxAperture,
		maxAperture,
		LimitRevisionBasisRegistration,
		"gate registration",
		"",
		now,
	)
}

// NewLimitRevision creates an auditable revision backed by a structural
// re-review. Both the structural basis and a human-readable reason must be
// supplied by the manager performing the revision.
func NewLimitRevision(
	revisionID string,
	gateID string,
	sequence int64,
	previousMaxAperture float64,
	newMaxAperture float64,
	basis string,
	reason string,
	requestedBy string,
	now time.Time,
) (LimitRevision, error) {
	basis = strings.TrimSpace(basis)
	if basis == "" {
		return LimitRevision{}, Invalid("invalid_revision_basis", "revision basis is required")
	}
	if basis == LimitRevisionBasisRegistration {
		return LimitRevision{}, Invalid("invalid_revision_basis", "the registration basis is reserved for the initial limit")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return LimitRevision{}, Invalid("invalid_revision_reason", "revision reason is required")
	}
	requestedBy = strings.TrimSpace(requestedBy)
	if requestedBy == "" {
		return LimitRevision{}, Invalid("invalid_revision_requested_by", "requested_by is required")
	}
	if sequence <= InitialLimitRevision {
		return LimitRevision{}, Invalid("invalid_revision_sequence", "revision sequence must advance past the registration baseline")
	}

	return newLimitRevision(
		revisionID,
		gateID,
		sequence,
		previousMaxAperture,
		newMaxAperture,
		basis,
		reason,
		requestedBy,
		now,
	)
}

func newLimitRevision(
	revisionID string,
	gateID string,
	sequence int64,
	previousMaxAperture float64,
	newMaxAperture float64,
	basis string,
	reason string,
	requestedBy string,
	now time.Time,
) (LimitRevision, error) {
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		return LimitRevision{}, Invalid("invalid_revision_id", "revision id is required")
	}
	gateID = strings.TrimSpace(gateID)
	if gateID == "" {
		return LimitRevision{}, Invalid("invalid_gate_reference", "gate id is required")
	}
	if sequence < InitialLimitRevision {
		return LimitRevision{}, Invalid("invalid_revision_sequence", "revision sequence must start at the registration baseline")
	}
	if newMaxAperture <= 0 || newMaxAperture > 1000 {
		return LimitRevision{}, Invalid("invalid_max_aperture", "max aperture must be greater than zero")
	}
	if sequence > InitialLimitRevision && previousMaxAperture == newMaxAperture {
		return LimitRevision{}, Invalid("limit_unchanged", "new max aperture must differ from the current limit")
	}

	return LimitRevision{
		ID:                  revisionID,
		GateID:              gateID,
		Sequence:            sequence,
		PreviousMaxAperture: previousMaxAperture,
		NewMaxAperture:      newMaxAperture,
		Basis:               basis,
		Reason:              strings.TrimSpace(reason),
		RequestedBy:         strings.TrimSpace(requestedBy),
		CreatedAt:           now.UTC(),
	}, nil
}
