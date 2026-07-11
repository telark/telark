package webauthn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
)

func ExtractBackupFlagsFromAttestation(attObjB64 string) (backupEligible, backupState bool) {
	attBytes, err := authhelper.DecodeBase64URLWithFallback(attObjB64)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.LogFailedDecodeAttestationForBackupFlags), err))
		return false, false
	}

	var attMap map[string]any
	if err := cbor.Unmarshal(attBytes, &attMap); err != nil {
		lg.Error(fmt.Sprintf(string(constants.LogFailedUnmarshalCBORForBackupFlags), err))
		return false, false
	}

	authData, ok := attMap[constants.WebAuthnKeyAuthData].([]byte)
	if !ok {
		lg.Error(string(constants.LogAuthDataNotByteForBackupFlags))
		return false, false
	}

	const minAuthDataLengthForFlags = constants.AuthDataOffsetFlags + 1
	if len(authData) < minAuthDataLengthForFlags {
		lg.Error(string(constants.LogAuthDataTooShortForBackupFlags))
		return false, false
	}

	const zeroValue = 0
	flags := authData[constants.AuthDataOffsetFlags]
	backupEligible = (flags & constants.BackupEligibleFlag) != zeroValue
	backupState = (flags & constants.BackupStateFlag) != zeroValue

	return backupEligible, backupState
}

func validateAttestationFormat(attMap map[string]any) error {
	fmtVal, ok := attMap[constants.WebAuthnKeyFormat].(string)
	if !ok || fmtVal != constants.AttestationFormatNone {
		return fmt.Errorf(string(constants.ErrUnsupportedAttestationFormat), attMap[constants.WebAuthnKeyFormat])
	}

	attStmtVal, exists := attMap[constants.AttStmtKey]
	if !exists {
		return nil
	}

	if attStmtVal == nil {
		return nil
	}

	attStmt, ok := attStmtVal.(map[any]any)
	if ok {
		if len(attStmt) != constants.DefaultInitValue {
			return errors.New(string(constants.ErrAttStmtMustBeEmpty))
		}
		return nil
	}

	attStmtStr, ok := attStmtVal.(map[string]any)
	if ok {
		if len(attStmtStr) != constants.DefaultInitValue {
			return errors.New(string(constants.ErrAttStmtMustBeEmpty))
		}
		return nil
	}

	return errors.New(string(constants.ErrInvalidAttStmtType))
}

//nolint:revive // function-result-limit: 7 return values needed for authData parsing
func parseAuthData(authData []byte) ([]byte, []byte, []byte, uint32, bool, bool, error) {
	if len(authData) < constants.AuthDataMinLengthForCredIDLen {
		return nil, nil, nil, 0, false, false, errors.New(string(constants.ErrAuthDataTooShort))
	}

	flags := authData[constants.AuthDataOffsetFlags]
	backupEligible := (flags & constants.BackupEligibleFlag) != 0
	backupState := (flags & constants.BackupStateFlag) != 0

	signCount := uint32(authData[constants.AuthDataOffsetSignCount])<<constants.SignCountShift24 |
		uint32(authData[constants.AuthDataOffsetSignCount+1])<<constants.SignCountShift16 |
		uint32(authData[constants.AuthDataOffsetSignCount+2])<<constants.SignCountShift8 |
		uint32(authData[constants.AuthDataOffsetSignCount+3])

	aaguid := authData[constants.AuthDataOffsetAAGUID : constants.AuthDataOffsetAAGUID+constants.AAGUIDLength]
	const credIDLenShift = 8
	credIDLen := int(authData[constants.AuthDataOffsetCredIDLen])<<credIDLenShift | int(authData[constants.AuthDataOffsetCredIDLen+1])

	minRequiredLen := constants.AuthDataOffsetCredID + credIDLen
	if len(authData) < minRequiredLen {
		return nil, nil, nil, 0, false, false, errors.New(string(constants.ErrAuthDataTooShortForCredID))
	}

	credID := authData[constants.AuthDataOffsetCredID : constants.AuthDataOffsetCredID+credIDLen]
	coseKey := authData[constants.AuthDataOffsetCredID+credIDLen:]

	return credID, aaguid, coseKey, signCount, backupEligible, backupState, nil
}

func validateClientData(clientDataJSONB64, expectedChallenge string) error {
	clientDataJSON, err := authhelper.DecodeBase64URLWithFallback(clientDataJSONB64)
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedDecodeClientDataJSON), err)
	}

	var clientData map[string]any
	if err := json.Unmarshal(clientDataJSON, &clientData); err != nil {
		return fmt.Errorf(string(constants.ErrFailedParseClientDataJSON), err)
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

	if _, ok := clientData[constants.WebAuthnKeyOrigin].(string); !ok {
		return errors.New(string(constants.ErrMissingOriginInClientData))
	}

	return nil
}

func ParseAttestationObjectManually(
	attObjB64, clientDataJSONB64, credentialIDB64, expectedChallenge string,
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

	if err := validateClientData(clientDataJSONB64, expectedChallenge); err != nil {
		return nil, false, false, err
	}

	lg.Info(fmt.Sprintf(string(constants.LogExtractedBackupFlags), backupEligible, backupState))

	return &webauthn.Credential{
		ID:        credID,
		PublicKey: coseKey,
		Authenticator: webauthn.Authenticator{
			AAGUID:    aaguid,
			SignCount: signCount,
		},
	}, backupEligible, backupState, nil
}
