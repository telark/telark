package webauthn

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
)

func ExtractBackupFlagsFromAttestation(attObjB64 string) (backupEligible, backupState bool) {
	attBytes, err := authhelper.DecodeBase64URLWithFallback(attObjB64)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.LogFailedDecodeAttestationForBackupFlags), err))
		return false, false
	}

	var attMap map[string]any
	if err := cbor.Unmarshal(attBytes, &attMap); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.LogFailedUnmarshalCBORForBackupFlags), err))
		return false, false
	}

	authData, ok := attMap[constants.WebAuthnKeyAuthData].([]byte)
	if !ok {
		lg.Warn(string(constants.LogAuthDataNotByteForBackupFlags))
		return false, false
	}

	if len(authData) < constants.AuthDataMinLengthForFlags {
		lg.Warn(string(constants.LogAuthDataTooShortForBackupFlags))
		return false, false
	}

	return backupFlags(authData)
}

func backupFlags(authData []byte) (backupEligible, backupState bool) {
	flags := authData[constants.AuthDataOffsetFlags]
	return flags&constants.BackupEligibleFlag != constants.DefaultInitValue,
		flags&constants.BackupStateFlag != constants.DefaultInitValue
}

func validateAttestationFormat(attMap map[string]any) error {
	fmtVal, ok := attMap[constants.WebAuthnKeyFormat].(string)
	if !ok || fmtVal != constants.AttestationFormatNone {
		return fmt.Errorf(string(constants.ErrUnsupportedAttestationFormat), attMap[constants.WebAuthnKeyFormat])
	}

	// A missing key decodes to a nil interface, so absent and explicitly-null
	// attStmt both land on the nil case.
	var attStmtLen int
	switch attStmt := attMap[constants.AttStmtKey].(type) {
	case nil:
		return nil
	case map[any]any:
		attStmtLen = len(attStmt)
	case map[string]any:
		attStmtLen = len(attStmt)
	default:
		return errors.New(string(constants.ErrInvalidAttStmtType))
	}

	if attStmtLen != constants.DefaultInitValue {
		return errors.New(string(constants.ErrAttStmtMustBeEmpty))
	}
	return nil
}

func parseAuthData(authData []byte) (
	credID, aaguid, coseKey []byte,
	signCount uint32,
	backupEligible, backupState bool,
	err error,
) {
	if len(authData) < constants.AuthDataMinLengthForCredIDLen {
		return nil, nil, nil, constants.DefaultInitValue, false, false,
			errors.New(string(constants.ErrAuthDataTooShort))
	}

	backupEligible, backupState = backupFlags(authData)

	signCount = uint32(authData[constants.AuthDataOffsetSignCount])<<constants.SignCountShift24 |
		uint32(authData[constants.AuthDataOffsetSignCount+1])<<constants.SignCountShift16 |
		uint32(authData[constants.AuthDataOffsetSignCount+2])<<constants.SignCountShift8 |
		uint32(authData[constants.AuthDataOffsetSignCount+3])

	aaguid = authData[constants.AuthDataOffsetAAGUID : constants.AuthDataOffsetAAGUID+constants.AAGUIDLength]
	const credIDLenShift = 8
	credIDLen := int(authData[constants.AuthDataOffsetCredIDLen])<<credIDLenShift | int(authData[constants.AuthDataOffsetCredIDLen+1])

	minRequiredLen := constants.AuthDataOffsetCredID + credIDLen
	if len(authData) < minRequiredLen {
		return nil, nil, nil, constants.DefaultInitValue, false, false,
			errors.New(string(constants.ErrAuthDataTooShortForCredID))
	}

	credID = authData[constants.AuthDataOffsetCredID : constants.AuthDataOffsetCredID+credIDLen]
	coseKey = authData[constants.AuthDataOffsetCredID+credIDLen:]

	return credID, aaguid, coseKey, signCount, backupEligible, backupState, nil
}

func parseClientData(clientDataJSONB64 string) (map[string]any, error) {
	clientDataJSON, err := authhelper.DecodeBase64URLWithFallback(clientDataJSONB64)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedDecodeClientDataJSON), err)
	}

	var clientData map[string]any
	if err := json.Unmarshal(clientDataJSON, &clientData); err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedParseClientDataJSON), err)
	}
	return clientData, nil
}

// The manual path replaces the library's checks, so it must verify everything
// the library would: ceremony type, challenge and an allowed origin.
func validateClientData(clientDataJSONB64, expectedChallenge string, origins []string) error {
	clientData, err := parseClientData(clientDataJSONB64)
	if err != nil {
		return err
	}
	if ceremony, ok := clientData[constants.WebAuthnKeyType].(string); !ok || ceremony != constants.WebAuthnTypeCreate {
		return errors.New(string(constants.ErrClientDataTypeMismatch))
	}

	challenge, ok := clientData[constants.WebAuthnKeyChallenge].(string)
	if !ok {
		return errors.New(string(constants.ErrMissingChallengeInClientData))
	}

	challengeDecoded, err := authhelper.DecodeBase64URLWithFallback(challenge)
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedDecodeChallenge), err)
	}

	expectedChallengeDecoded, err := authhelper.DecodeBase64URLWithFallback(expectedChallenge)
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedDecodeExpectedChallenge), err)
	}

	if !bytes.Equal(challengeDecoded, expectedChallengeDecoded) {
		return errors.New(string(constants.ErrChallengeMismatch))
	}

	origin, ok := clientData[constants.WebAuthnKeyOrigin].(string)
	if !ok {
		return errors.New(string(constants.ErrMissingOriginInClientData))
	}
	if !slices.Contains(origins, origin) {
		return errors.New(string(constants.ErrClientDataOriginNotAllowed))
	}

	return nil
}

func validateAuthDataBinding(authData []byte, rpID string) error {
	rpIDHash := sha256.Sum256([]byte(rpID))
	if !bytes.Equal(authData[:constants.RPIDHashLength], rpIDHash[:]) {
		return errors.New(string(constants.ErrRPIDHashMismatch))
	}
	if authData[constants.AuthDataOffsetFlags]&constants.UserPresentFlag == constants.DefaultInitValue {
		return errors.New(string(constants.ErrUserPresenceMissing))
	}
	return nil
}

func ParseAttestationObjectManually(
	attObjB64, clientDataJSONB64, credentialIDB64, expectedChallenge, rpID string, origins []string,
) (credential *webauthn.Credential, backupEligible, backupState bool, err error) {
	attBytes, err := authhelper.DecodeBase64URLWithFallback(attObjB64)
	if err != nil {
		return nil, false, false, fmt.Errorf(string(constants.ErrFailedDecodeAttestationObject), err)
	}

	var attMap map[string]any
	if err := cbor.Unmarshal(attBytes, &attMap); err != nil {
		return nil, false, false, fmt.Errorf(string(constants.ErrFailedUnmarshalCBOR), err)
	}

	if err := validateAttestationFormat(attMap); err != nil {
		return nil, false, false, err
	}

	authData, ok := attMap[constants.WebAuthnKeyAuthData].([]byte)
	if !ok {
		return nil, false, false, errors.New(string(constants.ErrAuthDataNotByteArray))
	}

	credID, aaguid, coseKey, signCount, backupEligible, backupState, err := parseAuthData(authData)
	if err != nil {
		return nil, false, false, err
	}

	credIDDecoded, err := authhelper.DecodeBase64URLWithFallback(credentialIDB64)
	if err != nil {
		return nil, false, false, fmt.Errorf(string(constants.ErrFailedDecodeCredentialID), err)
	}

	if !bytes.Equal(credID, credIDDecoded) {
		return nil, false, false, errors.New(string(constants.ErrCredentialIDMismatch))
	}

	if err := validateAuthDataBinding(authData, rpID); err != nil {
		return nil, false, false, err
	}
	if err := validateClientData(clientDataJSONB64, expectedChallenge, origins); err != nil {
		return nil, false, false, err
	}

	lg.Debug(fmt.Sprintf(string(constants.LogExtractedBackupFlags), backupEligible, backupState))

	return &webauthn.Credential{
		ID:        credID,
		PublicKey: coseKey,
		Authenticator: webauthn.Authenticator{
			AAGUID:    aaguid,
			SignCount: signCount,
		},
	}, backupEligible, backupState, nil
}
