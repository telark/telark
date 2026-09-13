package constants

import "github.com/telark/data/errors"

const (
	// General Errors
	ErrInvalidRequest      errors.Error = "invalid request"
	ErrInternalServerError errors.Error = "internal server error"
	ErrUnauthorized        errors.Error = "unauthorized"
	ErrForbidden           errors.Error = "forbidden"
	ErrNotFound            errors.Error = "not found"

	// Authentication Errors
	ErrInvalidUsername        errors.Error = "invalid username"
	ErrInvalidEmail           errors.Error = "invalid email"
	ErrInvalidCredentials     errors.Error = "invalid credentials"
	ErrUserNotFound           errors.Error = "user not found"
	ErrUserAlreadyHasPasskeys errors.Error = "user already has passkeys. please login first"
	ErrNoPasskeysFound        errors.Error = "no passkeys found for user"
	ErrChallengeExpired       errors.Error = "challenge has expired"
	ErrChallengeNotFound      errors.Error = "challenge not found"
	ErrSessionNotFound        errors.Error = "session not found"
	ErrSessionExpired         errors.Error = "session has expired"
	ErrInvalidSessionToken    errors.Error = "invalid session token"
	ErrMissingSessionToken    errors.Error = "missing session token"
	ErrPasskeyNotFound        errors.Error = "passkey not found"

	// WebAuthn Errors
	ErrWebAuthnSetupFailed          errors.Error = "failed to setup WebAuthn %v"
	ErrChallengeGenerationFailed    errors.Error = "failed to generate challenge %v"
	ErrCredentialVerificationFailed errors.Error = "credential verification failed %v"
	ErrInvalidCredential            errors.Error = "invalid credential"
	ErrFailedGetWebAuthnInstance    errors.Error = "failed to get WebAuthn instance: %v"

	// Client Errors
	ErrFailedGetUser         errors.Error = "failed to get user identityHash=%s: %v"
	ErrFailedCreateUser      errors.Error = "failed to create user identityHash=%s: status %d: %s"
	ErrFailedRoleRepair      errors.Error = "failed to repair missing role for identityHash=%s: status %d"
	ErrFailedAttachIdentity  errors.Error = "failed to attach identity to identityHash=%s: status %d"
	ErrFailedGetPasskeys     errors.Error = "failed to get passkeys: %s"
	ErrFailedCreateChallenge errors.Error = "failed to create challenge: %s"
	ErrFailedGetChallenge    errors.Error = "failed to get challenge: %s"
	ErrFailedDeleteChallenge errors.Error = "failed to delete challenge: %s"
	ErrFailedCreateSession   errors.Error = "failed to create session: %s"
	ErrFailedGetSession      errors.Error = "failed to get session: %s"
	ErrFailedDeleteSession   errors.Error = "failed to delete session: %s"
	ErrFailedUpdatePasskey   errors.Error = "failed to update passkey: %s"
	ErrFailedProxyRequest    errors.Error = "failed to proxy request: %s"

	// Encoding/Decoding Errors
	ErrFailedDecodeRequest       errors.Error = "failed to decode request body: %s"
	ErrFailedParseRequestBody    errors.Error = "failed to parse request body: %s"
	ErrFailedEncodeResponse      errors.Error = "failed to encode response: %s"
	ErrFailedReadRequestBody     errors.Error = "failed to read request body: %s"
	ErrFailedDecodePublicKey     errors.Error = "failed to decode public key: %v"
	ErrFailedCloseRequestBody    errors.Error = "failed to close request body: %v"
	ErrFailedGenerateRandomBytes errors.Error = "failed to generate random bytes: %v"
	ErrWebAuthnNotInitialized    errors.Error = "WebAuthn not initialized"
	LogHTTPServerShutdownSuccess errors.Error = "HTTP server shutdown successfully"

	// Server Errors
	ErrServerFailedToStartDetail       errors.Error = "server failed to start: %s"
	ErrServerInitiatingShutdown        errors.Error = "server initiating shutdown"
	ErrFailedShutdownHTTPServer        errors.Error = "failed to shutdown HTTP server: %s"
	ErrServiceHealthCheckFailed        errors.Error = "service health check failed"
	ErrQuitChannelNotAvailable         errors.Error = "quit channel not available"
	ErrFailedSendShutdownSignal        errors.Error = "failed to send shutdown signal"
	ErrServerPanicRecovered            errors.Error = "server panic recovered: %v\nStack: %s"
	ErrMaxPanicRecoveryAttemptsReached errors.Error = "max panic recovery attempts (%d) reached, shutting down"

	// Configuration Errors
	ErrMissingEnvVar             errors.Error = "missing required environment variable: %s"
	ErrInvalidEnvVarValue        errors.Error = "invalid value for environment variable %s: %s"
	ErrFailedLoadConfig          errors.Error = "failed to load configuration: %v"
	ErrFailedLoadBootstrapConfig errors.Error = "failed to load bootstrap configuration: %v"

	// Bootstrap Configuration Errors
	ErrBootstrapNoAdminsAndNoSelfReg errors.Error = "bootstrap config must have at least one admin when self-registration is disabled"
	ErrSelfRegistrationDisabled      errors.Error = "self-registration is disabled. contact your administrator"

	// Validation Errors
	ErrMissingRequiredFields         errors.Error = "credentialId, publicKey, deviceName and deviceType are required"
	ErrFailedGetConfig               errors.Error = "failed to get config: %v"
	ErrFailedGenerateSessionToken    errors.Error = "failed to generate session token: %v"
	ErrManualCredentialParsingFailed errors.Error = "manual credential parsing failed: %v"

	// Passkey Registration Errors
	ErrUsernameRequiredForUnauthenticatedRegistration errors.Error = "username is required for unauthenticated registration"
	ErrInvalidResponseStructure                       errors.Error = "invalid response structure"
	ErrMissingAttestationObject                       errors.Error = "missing attestationObject"
	ErrMissingClientDataJSON                          errors.Error = "missing clientDataJSON"
	ErrMissingCredentialID                            errors.Error = "missing credential id"
	ErrMissingUserID                                  errors.Error = "missing user id"
	ErrFailedDecodeAttestationObject                  errors.Error = "failed to decode attestationObject: %v"
	ErrFailedUnmarshalCBOR                            errors.Error = "failed to unmarshal CBOR: %v"
	ErrAuthDataNotByteArray                           errors.Error = "authData is not []byte"
	ErrAuthDataTooShort                               errors.Error = "authData too short"
	ErrUnsupportedAttestationFormat                   errors.Error = "unsupported attestation format: %v"
	ErrAttStmtMustBeEmpty                             errors.Error = "attStmt must be empty for none format"
	ErrInvalidAttStmtType                             errors.Error = "invalid attStmt type"
	ErrAuthDataTooShortForAAGUID                      errors.Error = "authData too short for AAGUID"
	ErrAuthDataTooShortForCredIDLen                   errors.Error = "authData too short for credential ID length"
	ErrAuthDataTooShortForCredID                      errors.Error = "authData too short for credential ID"
	ErrAuthDataTooShortForPublicKey                   errors.Error = "authData too short for public key"
	ErrFailedDecodeCredentialID                       errors.Error = "failed to decode credential ID: %v"
	ErrCredentialIDMismatch                           errors.Error = "credential ID mismatch"
	ErrFailedDecodeClientDataJSON                     errors.Error = "failed to decode clientDataJSON: %v"
	ErrFailedParseClientDataJSON                      errors.Error = "failed to parse clientDataJSON: %v"
	ErrMissingChallengeInClientData                   errors.Error = "missing challenge in clientDataJSON"
	ErrFailedDecodeChallenge                          errors.Error = "failed to decode challenge: %v"
	ErrFailedDecodeExpectedChallenge                  errors.Error = "failed to decode expected challenge: %v"
	ErrChallengeMismatch                              errors.Error = "challenge mismatch"
	ErrMissingOriginInClientData                      errors.Error = "missing origin in clientDataJSON"
	ErrUserAlreadyHasPasskeysPleaseLoginFirst         errors.Error = "user already has passkeys. please login first"

	// OIDC Errors
	ErrOIDCNotConfigured         errors.Error = "OIDC is not configured"
	ErrOIDCInvalidToken          errors.Error = "invalid OIDC token: %v"
	ErrOIDCUserNotProvisioned    errors.Error = "OIDC user not provisioned"
	ErrOIDCInvalidJWKSet         errors.Error = "invalid JWK set: %v"
	ErrOIDCUnknownKid            errors.Error = "unknown JWK key id: %s"
	ErrOIDCUnexpectedAlg         errors.Error = "unexpected JWK algorithm: %s"
	ErrOIDCMissingEmail          errors.Error = "OIDC token missing email claim"
	ErrOIDCJWKSFetchFailed       errors.Error = "JWKS fetch failed: %v"
	ErrOIDCEmailNotVerified      errors.Error = "OIDC login requires a verified email address"
	ErrOIDCJITProvisioningFailed errors.Error = "OIDC JIT user provisioning failed: %v"
	ErrOIDCNonceMissing          errors.Error = "OIDC nonce is missing from token claims"
	ErrOIDCConfigLoadFailed      errors.Error = "failed to load OIDC config: %v"
	ErrOIDCConfigSaveFailed      errors.Error = "failed to save OIDC config: status %d"
	ErrOIDCClientIDRequired      errors.Error = "googleClientID is required when OIDC is enabled"
	ErrOIDCTrustSourceRequired   errors.Error = "OIDC requires either egressAllowed or a pinned googleJwkJson"
	ErrOIDCJWKUnreachable        errors.Error = "provider signing keys are unreachable: %v"
	ErrOIDCNonceInvalid          errors.Error = "OIDC nonce is invalid or has already been used"
	ErrOIDCNonceStoreFailed      errors.Error = "failed to store OIDC nonce: %v"
	ErrOIDCIdentityLookupFailed  errors.Error = "user identity lookup failed for identityHash=%s: %v"
	ErrOIDCEmailLookupFailed     errors.Error = "email lookup failed for identityHash=%s: %v"
	ErrOIDCAdminPromotionFailed  errors.Error = "failed to promote bootstrap admin identityHash=%s: status %d"

	// Redis Errors
	ErrRedisClientUnavailable errors.Error = "redis client is not available — check REDIS_HOST and REDIS_PORT"
	ErrRedisInitFailed        errors.Error = "failed to initialize Redis: %v"
	ErrRedisGetFailed         errors.Error = "redis GET failed for key %s: %v"
	ErrRedisSetFailed         errors.Error = "redis SET failed for key %s: %v"
	ErrRedisDelFailed         errors.Error = "redis DEL failed for key %s: %v"
	ErrRedisChallengeConflict errors.Error = "concurrent login ceremony detected for identityHash=%s — only one ceremony allowed at a time"

	// Async Worker Warnings
	WarnAsyncWorkerFull         errors.Error = "async worker pool full, dropping housekeeping op"
	WarnAsyncWorkerFailed       errors.Error = "async housekeeping op timed out"
	WarnAsyncWorkerDrainTimeout errors.Error = "async worker drain timed out — some housekeeping ops may not have completed"

	// Credential Verification Errors
	ErrBackupEligibleFlagInconsistency  errors.Error = "backup eligible flag inconsistency: stored (BE:%v, BS:%v) != login (BE:%v, BS:%v)"
	ErrBackupFlag                       errors.Error = "backup flag"
	ErrCredentialNotFoundInAllowed      errors.Error = "credential not found in allowed credentials"
	ErrFailedMarshalCredentialBody      errors.Error = "failed to marshal credential body: %v"
	ErrFailedCreateCredentialRequest    errors.Error = "failed to create credential-only request: %v"
	ErrFailedParseCredentialRequest     errors.Error = "failed to parse credential request: %v"
	ErrManualCredentialValidationFailed errors.Error = "manual credential validation failed: %v"
)
