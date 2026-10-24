package webhooks

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"syscall"

	"github.com/telark/telark/services/notifier/internal/constants"
)

var (
	errAddressNotAllowed = errors.New(constants.ErrWebhookAddress)
	carrierGradeNAT      = netip.MustParsePrefix(constants.CIDRCarrierGradeNAT)
	thisNetwork          = netip.MustParsePrefix(constants.CIDRThisNetwork)
	broadcast            = netip.MustParseAddr(constants.IPBroadcast)
)

// The check runs on the address actually dialed, after DNS, so a name that resolves (or
// rebinds) to a refused address is caught. No proxy from the environment: the guard would vet
// the proxy instead of the target. Redirects are answers, never followed.
func newClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout: constants.WebhookDialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			addrPort, err := netip.ParseAddrPort(address)
			if err != nil || refused(addrPort.Addr().Unmap(), allowPrivate) {
				return errAddressNotAllowed
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: constants.WebhookAttemptTimeout,
		Transport: &http.Transport{
			Proxy:             nil,
			DialContext:       dialer.DialContext,
			ForceAttemptHTTP2: true,
			IdleConnTimeout:   constants.WebhookIdleConnTimeout,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func refused(ip netip.Addr, allowPrivate bool) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsUnspecified() || thisNetwork.Contains(ip) || ip == broadcast {
		return true
	}
	return !allowPrivate && (ip.IsPrivate() || carrierGradeNAT.Contains(ip))
}
