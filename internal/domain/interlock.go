package domain

import (
	"strings"
	"time"
)

type InterlockVerdict string

const (
	InterlockClear   InterlockVerdict = "clear"
	InterlockBlocked InterlockVerdict = "blocked"
)

type InterlockDecision struct {
	Verdict       InterlockVerdict
	Verifier      string
	Note          string
	LimitRevision int64
	ReviewedAt    time.Time
}

func NormalizeInterlockVerdict(verdict string) InterlockVerdict {
	return InterlockVerdict(strings.ToLower(strings.TrimSpace(verdict)))
}

func NewInterlockDecision(verdict, verifier, note string, limitRevision int64, now time.Time) (InterlockDecision, error) {
	normalized := NormalizeInterlockVerdict(verdict)
	if normalized != InterlockClear && normalized != InterlockBlocked {
		return InterlockDecision{}, Invalid("invalid_interlock_verdict", "verdict must be clear or blocked")
	}

	verifier = strings.TrimSpace(verifier)
	if verifier == "" {
		return InterlockDecision{}, Invalid("invalid_interlock_verifier", "verifier is required")
	}

	note = strings.TrimSpace(note)
	if normalized == InterlockBlocked && note == "" {
		return InterlockDecision{}, Invalid("invalid_interlock_note", "blocked decisions require a note")
	}

	if limitRevision < InitialLimitRevision {
		return InterlockDecision{}, Invalid("invalid_limit_revision", "interlock decisions must reference the gate's current limit")
	}

	return InterlockDecision{
		Verdict:       normalized,
		Verifier:      verifier,
		Note:          note,
		LimitRevision: limitRevision,
		ReviewedAt:    now.UTC(),
	}, nil
}

func (d InterlockDecision) IsClear() bool {
	return d.Verdict == InterlockClear
}
