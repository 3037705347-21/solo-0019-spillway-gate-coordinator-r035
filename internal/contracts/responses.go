package contracts

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InterlockDecisionView struct {
	Verdict    string `json:"verdict"`
	Verifier   string `json:"verifier"`
	Note       string `json:"note"`
	ReviewedAt string `json:"reviewed_at"`
}

type LimitRevisionView struct {
	Revision       int64   `json:"revision"`
	MaxAperture    float64 `json:"max_aperture"`
	Source         string  `json:"source"`
	BasisReference string  `json:"basis_reference,omitempty"`
	Reason         string  `json:"reason"`
	RevisedBy      string  `json:"revised_by,omitempty"`
	RevisedAt      string  `json:"revised_at"`
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
	BasisLimitRevision int64                  `json:"basis_limit_revision"`
	BasisMaxAperture   float64                `json:"basis_max_aperture"`
	BasisCurrent       bool                   `json:"basis_current"`
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

type ReviseLimitResponse struct {
	Gate GateView `json:"gate"`
}

type LimitRevisionsResponse struct {
	GateID    string              `json:"gate_id"`
	Revisions []LimitRevisionView `json:"revisions"`
}

type CommandResponse struct {
	Command CommandView `json:"command"`
}

type ReconfirmCommandResponse struct {
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
