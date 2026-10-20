package constants

const (
	JSONKeyError                  = "error"
	JSONKeySuccess                = "success"
	JSONKeyMessage                = "message"
	JSONKeyData                   = "data"
	JSONKeyCode                   = "code"
	JSONKeySelfRegEnabled         = "selfRegistrationEnabled"
	JSONKeyOIDCEnabled            = "oidcEnabled"
	JSONKeyGoogleClientID         = "googleClientID"
	ConfigMessage                 = "auth bootstrap config"
	HealthMessageHealthy          = "Service is healthy"
	HealthMessageNotReady         = "Service is not ready"
	PasskeyFieldDeviceName        = "deviceName"
	PasskeyFieldLastUsedTimestamp = "lastUsedTimestamp"
	RequestFieldUsername          = "username"
	DefaultForceLastDelete        = false
	DefaultCleanupOrphaned        = false
	DefaultWebAuthnEnforce        = true
	WebAuthnKeyID                 = "id"
	WebAuthnKeyResponse           = "response"
	WebAuthnKeyAttestationObject  = "attestationObject"
	WebAuthnKeyClientDataJSON     = "clientDataJSON"
	WebAuthnKeyAuthenticatorData  = "authenticatorData"
	WebAuthnKeyChallenge          = "challenge"
	Base64URLMinus                = "-"
	Base64URLPlus                 = "+"
	Base64URLUnderscore           = "_"
	Base64URLSlash                = "/"
	Base64URLPadding              = "="
	BackupFlagErrorCapitalized    = "Backup Eligible flag inconsistency"
	BackupFlagErrorLowercase      = "backup eligible flag inconsistency"
	BackupFlagErrorGeneric        = "backup flag"
	TimeFormatRFC3339             = "2006-01-02T15:04:05Z07:00"
	GoogleIssuer                  = "https://accounts.google.com"
	OIDCJWKKeyType                = "RSA"
	OIDCJWKUse                    = "sig"
	OIDCJWKSFetchURL              = "https://www.googleapis.com/oauth2/v3/certs"
	IdentityProviderGoogle        = "google"
	IdentityProviderPasskey       = "passkey"
	UserFieldIdentities           = "identities"
	UserFieldBootstrap            = "bootstrap"
	UserFieldStatus               = "status"
	UserStatusFieldLastLoginAt    = "lastLoginAt"
	UserStatusFieldInvite         = "invite"
	UserStatusFieldInviteAccepted = "inviteAcceptedAt"
	IdentityHashLength            = 8
	IdentityHashUnknown           = "unknown"
	TokenStatusValid              = "valid"
	TokenStatusExpired            = "expired"
	TokenStatusInvalid            = "invalid"
	TokenStatusRevokeFailed       = "revoke-failed"
	RoleSourceDirect              = "direct"
	RoleSourceInherited           = "inherited"
	JWTHeaderAlg                  = "alg"
	JWTHeaderKid                  = "kid"
	NotFoundStatusMarker          = "status: 404"
	JSONObjectStart               = "{"
	SyntheticRequestPath          = "/"
	BootstrapActionApprove        = "approve"
	BootstrapActionDeny           = "deny"
	BreakGlassFlagSet             = "break-glass"
	BreakGlassFlagEmail           = "email"
	BreakGlassFlagEmailUsage      = "The bootstrap admin's email (BOOTSTRAP_ADMIN)"
	BreakGlassFlagEnroll          = "enroll"
	BreakGlassFlagEnrollUsage     = "Create the user when missing and print a one-time passkey enrollment link"
	BreakGlassUsage               = "usage: ./main break-glass --email <email> [--enroll]"
	BreakGlassUserNotFound        = "No account uses %s yet. Run this command again with --enroll to create it and get an enrollment link.\n"
	BreakGlassLookupFailed        = "Could not read %s from the exporter: %v\nCheck that the exporter is running, then run this command again.\n"
	BreakGlassNotBootstrap        = "not the bootstrap admin email (BOOTSTRAP_ADMIN): %s\n"
	BreakGlassCreated             = "Created the bootstrap admin %s with the Admin role.\n"
	BreakGlassCreateFailed        = "create failed: status %d: %s\n"
	BreakGlassEnrollFailed        = "Could not create the enrollment link: %v\n"
	BreakGlassAlreadyAdmin        = "Nothing to change: %s is already the bootstrap admin with the Admin role.\n"
	BreakGlassPatchFailed         = "patch failed: status %d: %s\n"
	BreakGlassPromoted            = "%s is now the bootstrap admin, with the Admin role and passkey sign-in only.\n"
	BreakGlassSignIn              = "Sign in with its passkey. If it has none, run this command again with --enroll.\n"
	BreakGlassEnrollURL           = "%s/register#enroll=%s&email=%s"
	BreakGlassDashboardURL        = "<dashboard-url>"
	BreakGlassRequestFailed       = "The break-glass request failed: %v\n"
	BreakGlassNewestRequest       = "Newest request: %s, %s, %s, opened %s.\n"
	BreakGlassKindRecover         = "recover %s"
	BreakGlassKindReplace         = "make %s the bootstrap account"
	BreakGlassApproveURL          = "%s/break-glass#request=%s"
	BreakGlassRecovered           = "Request %s completed: the earlier passkeys, sessions and enrollment links of %s are removed.\n"
	BreakGlassReplaced            = "Request %s completed: %s is now the bootstrap account.\n"
	BreakGlassAlreadyEnrolled     = "%s already has a passkey. Sign in with it.\n"
)

const (
	BreakGlassDashboardURLHint = "     Replace <dashboard-url> with the address you open the dashboard at,\n" +
		"     such as http://localhost:3000 with the port-forward from the install notes.\n"
	BreakGlassEnrollSteps = "\nNext, register a passkey for %s:\n" +
		"  1. Open this link in a browser where you are not signed in to Telark (a private window works):\n" +
		"       %s\n" +
		"%s" +
		"  2. Name the device, select Register Passkey, and confirm with your fingerprint, face or screen lock.\n\n" +
		"The link adds one passkey and expires in 10 minutes, at %s. Running this command again replaces it.\n" +
		"The enrollment token is the value after enroll= in the link. If you type the address by hand,\n" +
		"put the token there: the page has no field for it.\n"
	BreakGlassStatusFailed = "Could not read the bootstrap account from the exporter: %v\n" +
		"Check that the exporter is running, then run this command again.\n"
	BreakGlassPinMissing = "%s has a passkey, but the exporter has not pinned it, so this command changes nothing.\n" +
		"Upgrade the telark-crds chart if it is older than the app chart, restart the exporter, then run this command again.\n"
	BreakGlassPinnedTo = "The bootstrap account is %s, pinned since %s.\n" +
		"Run this command again with --enroll to open a break-glass request.\n"
	BreakGlassOtherRequest = "Another break-glass request is open: %s, %s.\n" +
		"A new one opens once it completes, or once the bootstrap account denies it or signs in.\n"
	BreakGlassRequestOpen = "\nBreak-glass request %s is open: %s.\n" +
		"It completes when the bootstrap account approves it with a passkey, signed in or not, at:\n" +
		"       %s\n" +
		"%s" +
		"or at %s, 72 hours after it opened, unless the bootstrap account signs in first: a sign-in cancels it.\n" +
		"Run this command again after the approval, or after that time, to get the enrollment link.\n"
)

// A refusal's code is what clients branch on, so its message can be reworded freely.
const (
	RefusalSelfDelete                  = "self_delete"
	RefusalBootstrapManaged            = "bootstrap_managed"
	RefusalRoleAboveCallerLevel        = "role_above_caller_level"
	RefusalEnrollLinkSelf              = "enroll_link_self"
	RefusalEnrollLinkBootstrap         = "enroll_link_bootstrap"
	RefusalEnrollLinkAboveLevel        = "enroll_link_above_level"
	RefusalEnrollLinkRecovery          = "enroll_link_recovery"
	RefusalSignInSettingsBootstrapOnly = "sign_in_settings_bootstrap_only"
	RefusalEmailMismatch               = "email_mismatch"
	RefusalEmailReserved               = "email_reserved"
	RefusalSelfRegistrationDisabled    = "self_registration_disabled"
	RefusalAccountSuspended            = "account_suspended"
	RefusalAccountNotActive            = "account_not_active"
	RefusalBootstrapPasskeyOnly        = "bootstrap_passkey_only"
	RefusalUserNotFound                = "user_not_found"
)
