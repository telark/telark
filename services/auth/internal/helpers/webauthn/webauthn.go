package webauthn

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	sharedhelper "github.com/telark/auth/internal/helpers/shared"
	authdata "github.com/telark/data/auth"
)

var (
	webAuthnConfig    *config.WebAuthnConfig
	webAuthnMutex     sync.RWMutex
	webAuthnInstances sync.Map
	lg                = constants.GetLogger(constants.LoggerPrefixHelper)
)

func InitWebAuthn(cfg *config.WebAuthnConfig) error {
	webAuthnMutex.Lock()
	webAuthnConfig = cfg
	webAuthnMutex.Unlock()
	webAuthnInstances.Clear()

	if cfg.RPID == constants.EmptyString || cfg.RPOrigin == constants.EmptyString {
		return nil
	}
	_, err := instanceFor(cfg, cfg.RPID, splitOrigins(cfg.RPOrigin))
	return err
}

// The relying party follows the host the browser opened, so one instance is
// built per resolved (rpID, origins) pair and reused across the whole ceremony.
func GetWebAuthnFor(r *http.Request) (*webauthn.WebAuthn, error) {
	webAuthnMutex.RLock()
	cfg := webAuthnConfig
	webAuthnMutex.RUnlock()
	if cfg == nil {
		return nil, errors.New(string(constants.ErrWebAuthnNotInitialized))
	}

	rpID, origins, err := resolveRelyingParty(cfg, r)
	if err != nil {
		return nil, err
	}
	return instanceFor(cfg, rpID, origins)
}

// ponytail: unbounded when RP_ID is empty (one entry per distinct Host); pin RP_ID or add an LRU if it ever matters.
func instanceFor(cfg *config.WebAuthnConfig, rpID string, origins []string) (*webauthn.WebAuthn, error) {
	key := rpID + constants.SpaceSeparator + strings.Join(origins, constants.CommaSeparator)
	if cached, ok := webAuthnInstances.Load(key); ok {
		if wa, isInstance := cached.(*webauthn.WebAuthn); isInstance {
			return wa, nil
		}
	}

	timeout := webauthn.TimeoutConfig{
		Enforce:    constants.DefaultWebAuthnEnforce,
		Timeout:    time.Duration(cfg.ChallengeTimeout) * time.Second,
		TimeoutUVD: time.Duration(cfg.ChallengeTimeout) * time.Second,
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.RPName,
		RPID:          rpID,
		RPOrigins:     origins,
		Timeouts:      webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrWebAuthnSetupFailed), err)
	}
	webAuthnInstances.Store(key, wa)
	return wa, nil
}

func resolveRelyingParty(cfg *config.WebAuthnConfig, r *http.Request) (string, []string, error) {
	host := r.Header.Get(constants.HeaderForwardedHost)
	if host == constants.EmptyString {
		host = r.Host
	}
	rpID := cfg.RPID
	if rpID == constants.EmptyString {
		rpID = stripPort(host)
	}

	origin := r.Header.Get(constants.HeaderOrigin)
	if cfg.RPOrigin != constants.EmptyString {
		origins := splitOrigins(cfg.RPOrigin)
		if origin != constants.EmptyString && !slices.Contains(origins, origin) {
			return constants.EmptyString, nil, errors.New(string(constants.ErrOriginNotAllowed))
		}
		return rpID, origins, nil
	}

	if origin == constants.EmptyString {
		origin = requestScheme(r) + constants.SchemeSeparator + host
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Hostname() != rpID && !strings.HasSuffix(parsed.Hostname(), constants.DotSeparator+rpID)) {
		return constants.EmptyString, nil, errors.New(string(constants.ErrOriginNotAllowed))
	}
	return rpID, []string{origin}, nil
}

func splitOrigins(raw string) []string {
	origins := strings.Split(raw, constants.CommaSeparator)
	for i, origin := range origins {
		origins[i] = strings.TrimSpace(origin)
	}
	return origins
}

func stripPort(host string) string {
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		return hostname
	}
	return host
}

func requestScheme(r *http.Request) string {
	if proto := r.Header.Get(constants.HeaderForwardedProto); proto != constants.EmptyString {
		return proto
	}
	if r.TLS != nil {
		return constants.SchemeHTTPS
	}
	return constants.SchemeHTTP
}

func ConvertPasskeysToCredentials(passkeys []*authdata.UserPasskey) []webauthn.Credential {
	credentials := make([]webauthn.Credential, constants.InitialCapacity, len(passkeys))
	for _, pk := range passkeys {
		if pk == nil {
			continue
		}

		credID, err := authhelper.DecodeBase64URLWithFallback(pk.CredentialID)
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrFailedDecodeCredentialID), err))
			continue
		}

		pubKey, err := base64.StdEncoding.DecodeString(pk.PublicKey)
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrFailedDecodePublicKey), err))
			continue
		}

		credentials = append(credentials, webauthn.Credential{
			ID:            credID,
			PublicKey:     pubKey,
			Authenticator: webauthn.Authenticator{},
		})
	}
	return credentials
}

func CreateUser(userID, username, fullname string, credentials []webauthn.Credential) *User {
	return &User{
		ID:          []byte(userHandle(userID)),
		Name:        username,
		DisplayName: fullname,
		Credentials: credentials,
	}
}

func userHandle(userID string) string {
	randomBytes := make([]byte, constants.RandomBytesLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf(
			constants.UserHandleFormat, userID, uuid.New().String()[:constants.RandomBytesLength])
	}

	suffix := base64.RawURLEncoding.EncodeToString(randomBytes)
	handle := fmt.Sprintf(constants.UserHandleFormat, userID, suffix)
	if len(handle) <= constants.MaxUserHandleLength {
		return handle
	}

	maxSuffixLen := constants.MaxUserHandleLength - len(userID) - constants.DefaultColonSeparatorLength
	if maxSuffixLen <= constants.DefaultInitValue {
		return userID
	}
	return fmt.Sprintf(constants.UserHandleFormat, userID, suffix[:maxSuffixLen])
}

func StartRegistration(
	userID, username, fullname string,
	existingCredentials []webauthn.Credential,
	r *http.Request,
) (*protocol.CredentialCreation, string, error) {
	wa, err := GetWebAuthnFor(r)
	if err != nil {
		return nil, constants.EmptyString, err
	}

	webAuthnUser := CreateUser(userID, username, fullname, existingCredentials)
	options, sessionData, err := wa.BeginRegistration(webAuthnUser)
	if err != nil {
		return nil, constants.EmptyString, fmt.Errorf(string(constants.ErrChallengeGenerationFailed), err)
	}

	options.Response.AuthenticatorSelection.RequireResidentKey = protocol.ResidentKeyRequired()
	options.Response.AuthenticatorSelection.ResidentKey = protocol.ResidentKeyRequirementRequired
	options.Response.CredentialExcludeList = []protocol.CredentialDescriptor{}

	if err := StoreChallenge(userID, sessionData.Challenge); err != nil {
		return nil, constants.EmptyString, err
	}
	if err := StoreRegistrationChallengeOwner(sessionData.Challenge, userID); err != nil {
		CleanupChallenge(userID)
		return nil, constants.EmptyString, err
	}

	return options, sessionData.Challenge, nil
}

func extractRegistrationData(bodyBytes []byte) (
	attestationObjB64, clientDataJSONB64, credentialIDB64 string, err error,
) {
	var credMap map[string]any
	if err := json.Unmarshal(bodyBytes, &credMap); err != nil {
		return constants.EmptyString, constants.EmptyString, constants.EmptyString,
			fmt.Errorf(string(constants.ErrFailedParseRequestBody), err)
	}

	credentialIDB64, ok := credMap[constants.WebAuthnKeyID].(string)
	if !ok {
		return constants.EmptyString, constants.EmptyString, constants.EmptyString,
			errors.New(string(constants.ErrMissingCredentialID))
	}

	response, ok := credMap[constants.WebAuthnKeyResponse].(map[string]any)
	if !ok {
		return constants.EmptyString, constants.EmptyString, constants.EmptyString,
			errors.New(string(constants.ErrInvalidResponseStructure))
	}

	attestationObjB64, ok = response[constants.WebAuthnKeyAttestationObject].(string)
	if !ok {
		return constants.EmptyString, constants.EmptyString, constants.EmptyString,
			errors.New(string(constants.ErrMissingAttestationObject))
	}

	clientDataJSONB64, ok = response[constants.WebAuthnKeyClientDataJSON].(string)
	if !ok {
		return constants.EmptyString, constants.EmptyString, constants.EmptyString,
			errors.New(string(constants.ErrMissingClientDataJSON))
	}

	return attestationObjB64, clientDataJSONB64, credentialIDB64, nil
}

func FinishRegistration(
	userID, username, fullname string,
	r *http.Request,
) (credential *webauthn.Credential, backupEligible, backupState bool, err error) {
	wa, err := GetWebAuthnFor(r)
	if err != nil {
		return nil, false, false, err
	}

	challenge, err := ValidateAndGetChallenge(userID)
	if err != nil {
		return nil, false, false, errors.New(string(constants.ErrChallengeNotFound))
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, false, false, fmt.Errorf(string(constants.ErrFailedReadRequestBody), err)
	}

	attestationObjB64, clientDataJSONB64, credentialIDB64, err := extractRegistrationData(bodyBytes)
	if err != nil {
		return nil, false, false, err
	}

	webAuthnUser := CreateUser(userID, username, fullname, nil)
	sessionData := &webauthn.SessionData{
		Challenge: challenge.Challenge,
		UserID:    webAuthnUser.ID,
	}

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	credential, err = wa.FinishRegistration(webAuthnUser, *sessionData, r)

	if err != nil {
		credential, backupEligible, backupState, err = ParseAttestationObjectManually(
			attestationObjB64, clientDataJSONB64, credentialIDB64, challenge.Challenge)
		if err != nil {
			return nil, false, false, fmt.Errorf(string(constants.ErrManualCredentialParsingFailed), err)
		}
	} else {
		backupEligible, backupState = ExtractBackupFlagsFromAttestation(attestationObjB64)
	}

	CleanupChallenge(userID)
	cleanupRegistrationChallengeOwner(challenge.Challenge)
	lg.Info(fmt.Sprintf(string(constants.LogRegistrationVerifiedSuccessfully),
		sharedhelper.IdentityHash(userID)))
	return credential, backupEligible, backupState, nil
}
