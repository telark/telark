package constants

const (
	AuthUse   = "auth"
	AuthShort = "Enroll or recover the bootstrap admin through an AuthRequest"
	AuthLong  = "telark auth works with AuthRequest objects in the release namespace, which needs the telark-auth role " +
		"(app.auth.request.subjects). It needs no Telark login."
	EnrollUse   = "enroll"
	EnrollShort = "Enroll the bootstrap admin, or recover it, and print its enrollment link"
	EnrollLong  = "telark auth enroll creates an AuthRequest with your Kubernetes username and a one-time key, and " +
		"follows it: it prints the request's page and completion time when a request has to wait, or the enrollment " +
		"link, which only this command can open, then waits for the passkey. Run it again after a wait to continue " +
		"the same request."
	AuthStatusUse   = "status"
	AuthStatusShort = "Show the bootstrap account's pin and its newest request"
	FlagEmail       = "email"
	FlagEmailUsage  = "The bootstrap admin's email (app.auth.bootstrap.admin)"
)

const (
	AuthRequestPrefix    = "auth-request-"
	KeySpec              = "spec"
	KeyStatus            = "status"
	DashboardPlaceholder = "<dashboard-url>"
	SealedKeyBytes       = 32
	// The command a release older than 0.3.0 runs instead, in the auth pod.
	OldEnrollCommand = "kubectl exec -n %s deploy/telark-auth-service -- ./main break-glass --email %s --enroll"
	OldStatusCommand = "kubectl exec -n %s deploy/telark-auth-service -- ./main break-glass --email " + PlaceholderEmail
)

const (
	MsgRequestCreated  = "Created AuthRequest %s as %s. Waiting for auth to run it\n"
	MsgRequestStep     = "%s: %s"
	MsgApprovalURL     = "  approvalUrl:   %s\n"
	MsgCompletesAt     = "  completesAt:   %s\n"
	MsgLink            = "  link:          %s\n"
	MsgLinkExpiresAt   = "  linkExpiresAt: %s\n"
	MsgPlaceholder     = "  Replace " + DashboardPlaceholder + " with the address you open the dashboard at\n"
	MsgWaitingPasskey  = "Waiting for the passkey\n"
	MsgStatusPinned    = "The bootstrap account is %s, pinned since %s.\n"
	MsgStatusUnpinned  = "The bootstrap account isn't pinned yet: run `telark auth enroll --email " + PlaceholderEmail + "`.\n"
	MsgStatusRequest   = "Newest request: %s, %s, %s, opened %s.\n"
	MsgStatusRequestBy = "Newest request: %s, %s, %s, opened %s by %s.\n"
	MsgStatusNoRequest = "No request has been opened.\n"
	MsgKindRecover     = "recover %s"
	MsgKindReplace     = "make %s the bootstrap account"
	ErrNoAuthRequests  = "this Telark serves no AuthRequest objects, so it is older than 0.3.0: run `%s` instead"
	ErrReadAPI         = "read the telark.io API: %w"
	ErrWhoAmI          = "read your Kubernetes username: %w"
	ErrCreateRequest   = "create the AuthRequest: %w"
	ErrReadStatus      = "read TelarkConfig default: %w"
	ErrRequestEnded    = "AuthRequest %s ended " + MsgRequestStep
	ErrRequestDeleted  = "AuthRequest %s was deleted"
	ErrSealedLink      = "AuthRequest %s: the link isn't sealed to this command's key"
)
