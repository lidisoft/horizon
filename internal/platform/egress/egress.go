// Package egress builds the HTTP clients Horizon calls other servers with
// on someone's behalf (webhooks, forge APIs): no redirects, a bounded time,
// and, unless allowed, no destinations on the instance's own networks.
package egress

import (
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrPrivate refuses connections to loopback, private and link-local
// addresses.
var ErrPrivate = errors.New("address not allowed: private, loopback or link-local")

// Client returns a client with the timeout. Unless allowPrivate, it refuses
// private destinations, checked on the address actually dialled, so DNS
// can't point elsewhere after a check.
func Client(timeout time.Duration, allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	if !allowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
				ip.IsUnspecified() || ip.IsMulticast() {
				return ErrPrivate
			}
			return nil
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	transport.Proxy = nil
	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
