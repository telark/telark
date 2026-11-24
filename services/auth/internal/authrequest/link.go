package authrequest

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"

	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"golang.org/x/crypto/nacl/box"
)

// Checked before anything is written: a key nobody can open would spend the request and its link for nothing.
func (r *run) readKey() bool {
	raw, err := base64.StdEncoding.DecodeString(r.request.RecipientKey)
	if err != nil || len(raw) != constants.AuthRequestKeyBytes {
		r.failf(string(constants.ErrAuthRequestRecipientKey))
		return false
	}
	r.key = (*[constants.AuthRequestKeyBytes]byte)(raw)
	return true
}

// Issued as an invite so the request leaves a trace: one live token, the record and the owner's notices. Its lifetime
// is fixed, so the chart's member-link setting never stretches a token that opens the bootstrap account.
func (r *run) issueLink(userID string) (token string, expiresAt time.Time, ok bool) {
	ttl := time.Duration(constants.RedisTTLEnrollToken) * time.Second
	token, expiresAt, err := authhelper.IssueInvite(userID, constants.EmptyString, ttl)
	if err != nil {
		r.failf(string(constants.ErrAuthRequestLinkFailed), err)
	}
	return token, expiresAt, err == nil
}

// The link is a credential: whoever opens it first gets the passkey. Sealed to the requester's one-time key, the
// object, a backup of it or an audit log shows only ciphertext.
func (r *run) linkReady(token string, expiresAt time.Time) bool {
	// The dashboard's Members dialog builds the same link: the email only fills in the register form.
	link := fmt.Sprintf(constants.AuthRequestEnrollURL, dashboardOrigin(), url.QueryEscape(token), url.QueryEscape(r.email))
	sealed, err := box.SealAnonymous(nil, []byte(link), r.key, rand.Reader)
	if err != nil {
		r.failf(string(constants.ErrAuthRequestLinkFailed), err)
		return false
	}
	r.report(authrequest.Status{
		Phase:         authrequest.PhaseLinkReady,
		Reason:        fmt.Sprintf(constants.AuthRequestLinkReady, r.email),
		SealedLink:    base64.StdEncoding.EncodeToString(sealed),
		LinkExpiresAt: expiresAt.UTC().Format(constants.TimeFormatRFC3339),
	})
	return true
}

// Without a public address the links carry a placeholder the reader replaces.
func dashboardOrigin() string {
	if origin := config.DashboardOrigin(); origin != constants.EmptyString {
		return origin
	}
	return constants.AuthRequestDashboardURL
}
