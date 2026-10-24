package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/rest/base"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var (
	lg = constants.GetLogger(constants.LoggerPrefixAuthService)
	// The public starts: each stores a challenge, a nonce or a pending user before the caller proves anything.
	limited = []string{
		router.Key(base.Post, autheps.StartLogin),
		router.Key(base.Post, autheps.StartRegister),
		router.Key(base.Post, bootstrapeps.StartAssertion),
		router.Key(base.Post, autheps.OIDCGoogleNonce),
	}
	window             = time.Duration(constants.RateLimitWindowSec) * time.Second
	errTooManyAttempts = errors.New(string(constants.ErrTooManyAttempts))
)

// Mux middleware: it needs the matched route. Without Redis it lets every start through, which then fails at its own
// write: refusing would report an outage as too many attempts.
func New() func(http.Handler) http.Handler {
	perClient, global, hops := config.RateLimitPerClient(), config.RateLimitGlobal(), config.RateLimitTrustedHops()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := router.KeyFromRequest(r)
			if !slices.Contains(limited, route) || allowed(r.Context(), route, clientOf(r, hops), perClient, global) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set(constants.HeaderRetryAfter, strconv.Itoa(constants.RateLimitWindowSec))
			shared.SendErrorResponse(w, http.StatusTooManyRequests, errTooManyAttempts)
		})
	}
}

// Fixed windows. A start refused at the route's cap writes nothing, and one refused by its own client's limit spends
// none of the route's. The cap is read before the counts move, so a burst can pass it by a few.
func allowed(ctx context.Context, route, client string, perClient, global int64) bool {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return true
	}
	routeKey := constants.RedisKeyPrefixRateLimit + route
	spent, err := rdb.Get(ctx, routeKey).Int64()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return true
	}
	if spent >= global {
		return false
	}
	attempts, err := count(ctx, rdb, routeKey+constants.ColonSeparator+client)
	if err != nil {
		return true
	}
	if attempts > perClient {
		return false
	}
	if spent, err = count(ctx, rdb, routeKey); err == nil && spent == global {
		lg.Warn(fmt.Sprintf(string(constants.WarnRateLimitGlobalReached), route, global))
	}
	return true
}

func count(ctx context.Context, rdb *goredis.Client, key string) (int64, error) {
	var attempts *goredis.IntCmd
	_, err := rdb.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		attempts = pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, window)
		return nil
	})
	return attempts.Val(), err
}

// The ui's nginx appends the address it saw to X-Forwarded-For, as does each trusted proxy in front of it, so the
// entry that many hops from the right is the client; anything left of it is the client's own, and may be forged.
func clientOf(r *http.Request, hops int) string {
	forwarded := strings.Split(strings.Join(r.Header.Values(constants.HeaderForwardedFor), constants.CommaSeparator),
		constants.CommaSeparator)
	entry := forwarded[max(len(forwarded)-constants.DefaultIncrementValue-hops, constants.DefaultInitValue)]
	if addr, err := netip.ParseAddr(strings.TrimSpace(entry)); err == nil {
		return keyOf(addr)
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if addr, err := netip.ParseAddr(host); err == nil {
		return keyOf(addr)
	}
	return r.RemoteAddr
}

func keyOf(addr netip.Addr) string {
	if unmapped := addr.Unmap(); unmapped.Is4() {
		return unmapped.String()
	}
	prefix, _ := addr.Prefix(constants.RateLimitIPv6PrefixBits)
	return prefix.String()
}
