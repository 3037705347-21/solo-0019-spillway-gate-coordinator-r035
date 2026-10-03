package contracts

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InterlockDecisionView struct {
	Verdict       string `json:"verdict"`
	Verifier      string `json:"verifier"`
	Note          string `json:"note"`
	LimitRevision int64  `json:"limit_revision"`
	ReviewedAt    string `json:"reviewed_at"`
}

type LimitRevisionView struct {
	ID                  string  `json:"id"`
	GateID              string  `json:"gate_id"`
	Sequence            int64   `json:"sequence"`
	PreviousMaxAperture float64 `json:"previous_max_aperture"`
	NewMaxAperture      float64 `json:"new_max_aperture"`
	Basis               string  `json:"basis"`
	Reason              string  `json:"reason,omitempty"`
	RequestedBy         string  `json:"requested_by,omitempty"`
	CreatedAt           string  `json:"created_at"`
}

type GateView struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Aperture        float64 `json:"aperture"`
	MaxAperture     float64 `json:"max_aperture"`
	LimitRevision   int64   `json:"limit_revision"`
	ActiveCommandID string  `json:"active_command_id,omitempty"`
	UpdatedAt       string  `json:"updated_at"`
	Version         int64   `json:"version"`
}

type CommandView struct {
	ID                 string                 `json:"id"`
	GateID             string                 `json:"gate_id"`
	TargetAperture     float64                `json:"target_aperture"`
	StartingAperture   float64                `json:"starting_aperture"`
	LimitMaxAperture   float64                `json:"limit_max_aperture"`
	LimitRevision      int64                  `json:"limit_revision"`
	CurrentMaxAperture float64                `json:"current_max_aperture"`
	LimitCurrent       bool                   `json:"limit_current"`
	Reason             string                 `json:"reason"`
	RequestedBy        string                 `json:"requested_by"`
	Status             string                 `json:"status"`
	Interlock          *InterlockDecisionView `json:"interlock,omitempty"`
	ExecutionToken     string                 `json:"execution_token,omitempty"`
	ExecutedBy         string                 `json:"executed_by,omitempty"`
	TerminalReason     string                 `json:"terminal_reason,omitempty"`
	CreatedAt          string                 `json:"created_at"`
	UpdatedAt          string                 `json:"updated_at"`
	Version            int64                  `json:"version"`
}

type RegisterGateResponse struct {
	Gate GateView `json:"gate"`
}

type GateResponse struct {
	Gate GateView `json:"gate"`
}

type CommandResponse struct {
	Command CommandView `json:"command"`
	Gate    GateView    `json:"gate"`
}

type ReviewCommandResponse struct {
	Command CommandView `json:"command"`
	Gate    GateView    `json:"gate"`
}

type ExecuteCommandResponse struct {
	Command CommandView `json:"command"`
	Gate    GateView    `json:"gate"`
}

type ReviseGateLimitResponse struct {
	Gate     GateView          `json:"gate"`
	Revision LimitRevisionView `json:"revision"`
}

type LimitRevisionsResponse struct {
	Revisions []LimitRevisionView `json:"revisions"`
}
