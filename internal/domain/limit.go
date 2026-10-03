package domain

import (
	"strings"
	"time"
)

type LimitRevisionSource string

const (
	LimitFromRegistration     LimitRevisionSource = "registration"
	LimitFromStructuralReview LimitRevisionSource = "structural_review"
)

// LimitRevision is an immutable record of an aperture limit in force at a
// point in time. Revision 1 is always created together with gate
// registration; every later revision must cite a structural review.
type LimitRevision struct {
	Revision       int64
	MaxAperture    float64
	Source         LimitRevisionSource
	BasisReference string
	Reason         string
	RevisedBy      string
	RevisedAt      time.Time
}

func newRegistrationLimit(maxAperture float64, now time.Time) LimitRevision {
	return LimitRevision{
		Revision:    1,
		MaxAperture: maxAperture,
		Source:      LimitFromRegistration,
		Reason:      "gate registration",
		RevisedAt:   now.UTC(),
	}
}

func validateRevisionEvidence(basisReference, reason, revisedBy string) (string, string, string, error) {
	basisReference = strings.TrimSpace(basisReference)
	if basisReference == "" {
		return "", "", "", Invalid("invalid_revision_basis", "structural review reference is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", "", "", Invalid("invalid_revision_reason", "revision reason is required")
	}
	revisedBy = strings.TrimSpace(revisedBy)
	if revisedBy == "" {
		return "", "", "", Invalid("invalid_revised_by", "revised_by is required")
	}
	return basisReference, reason, revisedBy, nil
}
