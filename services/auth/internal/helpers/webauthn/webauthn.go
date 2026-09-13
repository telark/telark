package webauthn

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	webAuthnInstance *webauthn.WebAuthn
	webAuthnMutex    sync.RWMutex
	lg               = constants.GetLogger(constants.LoggerPrefixHelper)
)

func InitWebAuthn(cfg *config.WebAuthnConfig) error {
	wconfig := &webauthn.Config{
		RPDisplayName: cfg.RPName,
		RPID:          cfg.RPID,
		RPOrigins:     []string{cfg.RPOrigin},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    constants.DefaultWebAuthnEnforce,
				Timeout:    time.Duration(cfg.ChallengeTimeout) * time.Second,
				TimeoutUVD: time.Duration(cfg.ChallengeTimeout) * time.Second,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    constants.DefaultWebAuthnEnforce,
				Timeout:    time.Duration(cfg.ChallengeTimeout) * time.Second,
				TimeoutUVD: time.Duration(cfg.ChallengeTimeout) * time.Second,
			},
		},
	}

	wa, err := webauthn.New(wconfig)
	if err != nil {
		return err
	}

	webAuthnMutex.Lock()
	webAuthnInstance = wa
	webAuthnMutex.Unlock()
	return nil
}

func GetWebAuthn() (*webauthn.WebAuthn, error) {
	webAuthnMutex.RLock()
	defer webAuthnMutex.RUnlock()

	if webAuthnInstance == nil {
		return nil, errors.New(string(constants.ErrWebAuthnNotInitialized))
	}
	return webAuthnInstance, nil
}

func ConvertPasskeysToCredentials(passkeys []*authdata.UserPasskey) []webauthn.Credential {
	if len(passkeys) == constants.InitialCapacity {
		return []webauthn.Credential{}
	}

	validCount := constants.InitialCapacity
	for _, pk := range passkeys {
		if pk != nil {
			validCount++
		}
	}

	if validCount == constants.InitialCapacity {
		return []webauthn.Credential{}
	}

	credentials := make([]webauthn.Credential, constants.InitialCapacity, validCount)
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
	randomBytes := make([]byte, constants.RandomBytesLength)
	if _, err := rand.Read(randomBytes); err != nil {
		uniqueUserHandle := fmt.Sprintf(
			constants.UserHandleFormat, userID, uuid.New().String()[:constants.RandomBytesLength])
		return &User{
			ID:          []byte(uniqueUserHandle),
			Name:        username,
			DisplayName: fullname,
			Credentials: credentials,
		}
	}

	suffix := base64.RawURLEncoding.EncodeToString(randomBytes)
	uniqueUserHandle := fmt.Sprintf(constants.UserHandleFormat, userID, suffix)

	if len(uniqueUserHandle) > constants.MaxUserHandleLength {
		maxSuffixLen := constants.MaxUserHandleLength - len(userID) - constants.DefaultColonSeparatorLength
		if maxSuffixLen > constants.DefaultInitValue {
			suffix = suffix[:maxSuffixLen]
			uniqueUserHandle = fmt.Sprintf(constants.UserHandleFormat, userID, suffix)
		} else {
			uniqueUserHandle = userID
		}
	}

	return &User{
		ID:          []byte(uniqueUserHandle),
		Name:        username,
		DisplayName: fullname,
		Credentials: credentials,
	}
}

func ExtractBaseUserID(uniqueUserHandle string) string {
	parts := strings.Split(uniqueUserHandle, ":")
	if len(parts) > constants.DefaultInitValue {
		return parts[constants.DefaultInitValue]
	}
	return uniqueUserHandle
}

func StartRegistration(userID, username, fullname string, existingCredentials []webauthn.Credential) (
	*protocol.CredentialCreation, string, error,
) {
	wa, err := GetWebAuthn()
	if err != nil {
		return nil, constants.EmptyString, fmt.Errorf(string(constants.ErrWebAuthnSetupFailed), err)
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
	wa, err := GetWebAuthn()
	if err != nil {
		return nil, false, false, fmt.Errorf(string(constants.ErrFailedGetWebAuthnInstance), err)
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
