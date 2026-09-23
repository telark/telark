package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/telark/auth/internal/constants"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	globalconfigresource "github.com/telark/data/resources/globalconfig"
)

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

type GoogleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`
	jwt.RegisteredClaims
}

type keyStore struct {
	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	lastRefresh time.Time
	egressMode  bool
	staticJSON  string
	stopRefresh chan struct{}
	stopOnce    sync.Once
}

var (
	store   *keyStore
	storeMu sync.Mutex
)

var lg = constants.GetLogger(constants.LoggerPrefixOIDC)

func (s *keyStore) stop() {
	s.stopOnce.Do(func() { close(s.stopRefresh) })
}

func (s *keyStore) matches(oidc globalconfigresource.OIDCConfig) bool {
	return s.egressMode == oidc.EgressAllowed && s.staticJSON == oidc.GoogleJWKJSON
}

// Rebuilt whenever the admin changes the trust settings, so a key rotation takes
// effect on the next login rather than on the next restart.
func getStore(oidc globalconfigresource.OIDCConfig) (*keyStore, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	if store != nil && store.matches(oidc) {
		return store, nil
	}

	if store != nil {
		store.stop()
		store = nil
	}

	s := &keyStore{
		egressMode:  oidc.EgressAllowed,
		staticJSON:  oidc.GoogleJWKJSON,
		keys:        make(map[string]*rsa.PublicKey),
		stopRefresh: make(chan struct{}),
	}

	if err := s.doRefresh(); err != nil {
		if !oidc.EgressAllowed {
			return nil, err
		}
		// Egress mode: log and continue — background refresh will populate keys
		lg.Warn(fmt.Sprintf(string(constants.ErrOIDCJWKSFetchFailed), err))
	}

	store = s

	if oidc.EgressAllowed {
		go s.runBackgroundRefresh()
	}

	return s, nil
}

func (s *keyStore) getKey(kid string) (*rsa.PublicKey, error) {
	s.mu.RLock()
	key, ok := s.keys[kid]
	s.mu.RUnlock()

	if !ok && s.egressMode {
		// kid unknown — Google may have rotated; attempt immediate refresh with cooldown
		_ = s.refreshOnKidMiss()

		s.mu.RLock()
		key, ok = s.keys[kid]
		s.mu.RUnlock()
	}

	if !ok {
		return nil, fmt.Errorf(string(constants.ErrOIDCUnknownKid), kid)
	}
	return key, nil
}

func (s *keyStore) refreshOnKidMiss() error {
	s.mu.RLock()
	age := time.Since(s.lastRefresh)
	s.mu.RUnlock()

	if age < constants.OIDCJWKSMinRefreshInterval {
		return nil
	}
	return s.doRefresh()
}

func (s *keyStore) doRefresh() error {
	var raw []byte

	if s.egressMode {
		if cached := loadJWKSFromRedis(); len(cached) > constants.DefaultInitValue {
			raw = cached
		} else {
			fetched, err := fetchJWKS()
			if err != nil {
				lg.Warn(fmt.Sprintf(string(constants.ErrOIDCJWKSFetchFailed), err))
				s.mu.Lock()
				s.lastRefresh = time.Now()
				s.mu.Unlock()
				return nil // keep old keys; fail-safe
			}
			saveJWKSToRedis(fetched)
			raw = fetched
		}
	} else {
		raw = []byte(s.staticJSON)
	}

	parsed, err := parseKeys(raw)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.keys = parsed
	s.lastRefresh = time.Now()
	s.mu.Unlock()
	return nil
}

func (s *keyStore) runBackgroundRefresh() {
	ticker := time.NewTicker(constants.OIDCJWKSRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = s.doRefresh()
		case <-s.stopRefresh:
			return
		}
	}
}

func StopJWKSRefresh() {
	storeMu.Lock()
	defer storeMu.Unlock()
	if store != nil {
		store.stop()
	}
}

func loadJWKSFromRedis() []byte {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisAsyncWorkerTimeout)
	defer cancel()
	val, err := rdb.Get(ctx, constants.RedisKeyJWKS).Bytes()
	if err != nil {
		return nil
	}
	return val
}

func saveJWKSToRedis(raw []byte) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisAsyncWorkerTimeout)
	defer cancel()
	if err := rdb.Set(ctx, constants.RedisKeyJWKS, raw,
		time.Duration(constants.RedisTTLJWKS)*time.Hour).Err(); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.ErrOIDCJWKSCacheFailed), err))
	}
}

func fetchJWKS() ([]byte, error) {
	client := &http.Client{Timeout: constants.OIDCJWKSFetchTimeout}
	resp, err := client.Get(constants.OIDCJWKSFetchURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			lg.Debug(fmt.Sprintf(string(constants.ErrFailedCloseRequestBody), cerr))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ErrOIDCJWKSBadStatus), resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func parseKeys(raw []byte) (map[string]*rsa.PublicKey, error) {
	var set jwkSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCInvalidJWKSet), err)
	}

	result := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != constants.OIDCJWKKeyType || k.Use != constants.OIDCJWKUse {
			continue
		}
		pub, err := buildRSAPublicKey(k.N, k.E)
		if err != nil {
			lg.Warn(fmt.Sprintf(string(constants.ErrOIDCInvalidJWKSet), err))
			continue
		}
		result[k.Kid] = pub
	}
	return result, nil
}

func buildRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

func ValidateGoogleIDToken(rawToken string, oidc globalconfigresource.OIDCConfig) (*GoogleClaims, error) {
	s, err := getStore(oidc)
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(rawToken, &GoogleClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf(string(constants.ErrOIDCUnexpectedAlg), t.Header[constants.JWTHeaderAlg])
		}
		kid, ok := t.Header[constants.JWTHeaderKid].(string)
		if !ok {
			return nil, fmt.Errorf(string(constants.ErrOIDCUnknownKid), constants.EmptyString)
		}
		return s.getKey(kid)
	},
		jwt.WithAudience(oidc.GoogleClientID),
		jwt.WithIssuer(constants.GoogleIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCInvalidToken), err)
	}

	claims, ok := token.Claims.(*GoogleClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf(string(constants.ErrOIDCInvalidToken), string(constants.ErrOIDCTokenInvalidDetail))
	}

	if claims.Email == constants.EmptyString {
		return nil, errors.New(string(constants.ErrOIDCMissingEmail))
	}

	return claims, nil
}
