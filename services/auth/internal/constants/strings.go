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
	BreakGlassEnrollURL           = "%s/register?enroll=%s&email=%s"
	BreakGlassDashboardURL        = "<dashboard-url>"
)

const (
	BreakGlassDashboardURLHint = "     Replace <dashboard-url> with the address you open the dashboard at,\n" +
		"     such as http://localhost:3000 with the port-forward from the install notes.\n"
	BreakGlassEnrollSteps = "\nNext, register a passkey for %s:\n" +
		"  1. Open this link in a browser where you are not signed in to Telark (a private window works):\n" +
		"       %s\n" +
		"%s" +
		"  2. Name the device, select Register Passkey, and confirm with your fingerprint, face or screen lock.\n\n" +
		"The link adds one passkey and expires at %s. Running this command again replaces it.\n" +
		"The enrollment token is the value after enroll= in the link. If you type the address by hand,\n" +
		"put the token there: the page has no field for it.\n"
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
