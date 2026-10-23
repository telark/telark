package cmd

import "github.com/telark/telark/internal/data/messages"

const (
	BreakGlassFlagSet         = "break-glass"
	BreakGlassFlagEmail       = "email"
	BreakGlassFlagEnroll      = "enroll"
	BreakGlassFlagEmailUsage  = "The bootstrap admin's email (BOOTSTRAP_ADMIN)"
	BreakGlassFlagEnrollUsage = "Get a passkey enrollment link, through a request once the account is pinned"
	BreakGlassUsage           = "Usage: ./main break-glass --email <email> [--enroll]"
	BreakGlassEnrollURL       = "%s/register#enroll=%s&email=%s"
	BreakGlassRequestURL      = "%s/break-glass#request=%s&kind=%s"
	BreakGlassDashboardURL    = "<dashboard-url>"
	BreakGlassKindRecover     = "recover %s"
	BreakGlassKindReplace     = "make %s the bootstrap account"
	BreakGlassCreateStatus    = "status %d: %s"
	BreakGlassNotCreated      = "it was not found once created"
	breakGlassCheckExporter   = "Check that the exporter is running, then run this command again.\n"
)

const (
	BreakGlassDashboardURLHint = "       Replace <dashboard-url> with your dashboard address, such as http://localhost:3000.\n"
	BreakGlassNotBootstrap     = "%s is not the bootstrap admin email (BOOTSTRAP_ADMIN).\n"
	BreakGlassStatusFailed     = "Could not read the bootstrap account: %v\n" + breakGlassCheckExporter
	BreakGlassLookupFailed     = "Could not look up %s: %v\n" + breakGlassCheckExporter
	BreakGlassUserNotFound     = "No account uses %s.\nRun this command again with --enroll to create it.\n"
	BreakGlassCreateFailed     = "Could not create %s: %v\n"
	BreakGlassCreated          = "Created %s with the Admin role.\n"
	BreakGlassPatchFailed      = "Could not promote %s: status %d: %s\n"
	BreakGlassPromoted         = "%s is now the bootstrap admin, with the Admin role.\n"
	BreakGlassAlreadyAdmin     = "Nothing to change: %s is already the bootstrap admin.\n"
	BreakGlassSignIn           = "Sign in with its passkey. If it has none, run this command again with --enroll.\n"
	BreakGlassEnrollFailed     = "Could not create the enrollment link: %v\n"
	BreakGlassEnrollSteps      = "\nRegister a passkey for %s with this link, valid until %s:\n       %s\n%s" +
		"Open it in a browser that is not signed in to Telark, such as a private window.\n"
	BreakGlassPinMissing = "Nothing changed: %s has a passkey but is not pinned.\n" +
		"Upgrade telark-crds if it is older than the app chart, restart the exporter, then run this again.\n"
	BreakGlassPinnedTo      = "The bootstrap account is %s, pinned since %s.\nRun this command again with --enroll to open a request.\n"
	BreakGlassNewestRequest = "Newest request: %s, %s, %s, opened %s.\n"
	BreakGlassOtherRequest  = "Another request is open: %s (%s).\n" +
		"Run this command again once it completes, or the bootstrap account denies it or signs in.\n"
	BreakGlassRequestOpen = "\nRequest %s is open: %s.\n" +
		"Run this command again after %s, or once the bootstrap account approves it at:\n       %s\n%s"
	BreakGlassRecoveryOpen = "\nRequest %s is open: %s.\n" +
		"Run this command again after %s. The bootstrap account can deny it until then at:\n       %s\n%s"
	BreakGlassRecoveryWait = "Waiting for the passkey until %s. Keep this command running.\n"
	BreakGlassStillWaiting = "Still waiting for the passkey, until %s.\n"
	BreakGlassRecovered    = "Request %s completed: %s now has only the new passkey.\n"
	BreakGlassLinkExpired  = "The link expired unused: nothing changed and request %s stays open.\n" +
		"Run this command again for a new link.\n"
	BreakGlassRequestEnded = "Request %s was canceled by a sign-in or a deny: %s keeps its passkeys.\n" +
		"Run this command again to open a new request.\n"
	BreakGlassReplaced        = "Request %s completed: %s is now the bootstrap account.\n"
	BreakGlassAlreadyEnrolled = "%s already has a passkey. Sign in with it.\n"
	BreakGlassOpenFailed      = "Could not open a request: %v\n"
	BreakGlassCompleteFailed  = "Could not finish request %s: %v\n"
	BreakGlassRunFailed       = "Could not run break-glass: %v\n"
)

const (
	LogBackfillFinalizersStarted   messages.Message = "[backfill] finalizers started: batchSize=%d pauseMs=%d"
	LogBackfillFinalizersDone      messages.Message = "[backfill] finalizers done: scanned=%d patched=%d skipped=%d"
	ErrBackfillUnknownResourceType messages.Message = "backfill: unknown resource type %q"
	ErrBackfillListFailed          messages.Message = "backfill: list %s: %w"
	ErrBackfillAddFinalizerFailed  messages.Message = "[backfill] add-finalizer failed: type=%s id=%s status=%d"
	LogFinalizersRemoved           messages.Message = "[remove-finalizers] removed=%d failed=%d"
	ErrRemoveFinalizersListFailed  messages.Message = "[remove-finalizers] list failed: type=%s err=%v"
)
