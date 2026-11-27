package authrequest

const (
	FieldEmail        = "email"
	FieldRequestedBy  = "requestedBy"
	FieldRecipientKey = "recipientKey"
)

const (
	PhasePending            = "Pending"
	PhaseWaitingForApproval = "WaitingForApproval"
	PhaseRecovering         = "Recovering"
	PhaseLinkReady          = "LinkReady"
	PhaseCompleted          = "Completed"
	PhaseDenied             = "Denied"
	PhaseCanceled           = "Canceled"
	PhaseExpired            = "Expired"
	PhaseFailed             = "Failed"
)

type Spec struct {
	Email        string `json:"email"`
	RequestedBy  string `json:"requestedBy"`
	RecipientKey string `json:"recipientKey"`
}

// Only the fields a step sets: the exporter merges them into the stored status.
type Status struct {
	Phase         string `json:"phase,omitempty"`
	Reason        string `json:"reason,omitempty"`
	RequestID     string `json:"requestId,omitempty"`
	ApprovalURL   string `json:"approvalUrl,omitempty"`
	CompletesAt   string `json:"completesAt,omitempty"`
	SealedLink    string `json:"sealedLink,omitempty"`
	LinkExpiresAt string `json:"linkExpiresAt,omitempty"`
}

// The REST view of one object: its name, its spec and its status, flattened.
type AuthRequest struct {
	ID string `json:"id"`
	Spec
	Status
	CreatedAt string `json:"createdAt,omitempty"`
}

// A run started and left unfinished: only a run that stopped halfway leaves one behind.
func (r *AuthRequest) InFlight() bool {
	return r.Phase == PhaseRecovering || r.Phase == PhaseLinkReady
}
