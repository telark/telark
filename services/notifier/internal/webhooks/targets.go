package webhooks

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"

	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/notifier/internal/constants"
)

var (
	namePattern = regexp.MustCompile(constants.WebhookNamePattern)

	// None of these wraps a parse error or quotes a value: a URL or a secret never reaches the log.
	errDuplicateName  = errors.New(constants.ErrWebhookDuplicateName)
	errBadName        = errors.New(constants.ErrWebhookBadName)
	errNoEvents       = errors.New(constants.ErrWebhookNoEvents)
	errBadURL         = errors.New(constants.ErrWebhookBadURL)
	errURLScheme      = errors.New(constants.ErrWebhookURLScheme)
	errURLUserInfo    = errors.New(constants.ErrWebhookURLUserInfo)
	errURLPrivateHost = errors.New(constants.ErrWebhookURLPrivateHost)
	errBadSecret      = errors.New(constants.ErrWebhookBadSecret)
)

// The whole file or nothing: one bad target refuses it, so a typo never drops a target's
// group and the deliveries it still owes.
func ParseTargets(raw []byte) ([]Target, error) {
	var configs []TargetConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configs); err != nil {
		return nil, fmt.Errorf(constants.ErrWebhookTargetsJSON, err)
	}
	targets := make([]Target, constants.DefaultInitValue, len(configs))
	seen := map[string]bool{}
	for i, config := range configs {
		keys, err := validate(config)
		if err == nil && seen[config.Name] {
			err = errDuplicateName
		}
		if err != nil {
			return nil, fmt.Errorf(constants.ErrWebhookTarget, i, config.Name, err)
		}
		seen[config.Name] = true
		targets = append(targets, Target{Config: config, keys: keys})
	}
	return targets, nil
}

func validate(config TargetConfig) ([][]byte, error) {
	if !namePattern.MatchString(config.Name) {
		return nil, errBadName
	}
	if len(config.Events) == constants.DefaultInitValue {
		return nil, errNoEvents
	}
	for _, event := range config.Events {
		if !slices.Contains(notifclient.WebhookEventTypes, event) {
			return nil, fmt.Errorf(constants.ErrWebhookBadEvent, event)
		}
	}
	if err := checkURL(config.URL, config.AllowPrivate); err != nil {
		return nil, err
	}
	return signingKeys(config)
}

// Names are vetted here; the address a name resolves to is vetted when it is dialed.
func checkURL(raw string, allowPrivate bool) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == constants.EmptyString {
		return errBadURL
	}
	if parsed.User != nil {
		return errURLUserInfo
	}
	if parsed.Scheme != constants.WebhookSchemeHTTPS && (parsed.Scheme != constants.WebhookSchemeHTTP || !allowPrivate) {
		return errURLScheme
	}
	if !allowPrivate && localName(parsed.Hostname()) {
		return errURLPrivateHost
	}
	return nil
}

// An IP literal is left to the dial-time check, which sees every address alike.
func localName(hostname string) bool {
	host := strings.TrimSuffix(strings.ToLower(hostname), constants.WebhookHostDot)
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	return host == constants.WebhookHostLocal || !strings.Contains(host, constants.WebhookHostDot) ||
		strings.HasSuffix(host, constants.WebhookHostSuffixSvc) || strings.HasSuffix(host, constants.WebhookHostSuffixCluster)
}

// The current secret signs first; a previous one, kept while receivers rotate, adds a second signature.
func signingKeys(config TargetConfig) ([][]byte, error) {
	secrets := []string{config.Secret}
	if config.PreviousSecret != constants.EmptyString {
		secrets = append(secrets, config.PreviousSecret)
	}
	keys := make([][]byte, constants.DefaultInitValue, len(secrets))
	for _, secret := range secrets {
		encoded, ok := strings.CutPrefix(secret, constants.WebhookSecretPrefix)
		key, err := base64.StdEncoding.DecodeString(encoded)
		if !ok || err != nil || len(strings.TrimRight(encoded, constants.WebhookBase64Padding)) < constants.WebhookMinSecretChars {
			return nil, errBadSecret
		}
		keys = append(keys, key)
	}
	return keys, nil
}
