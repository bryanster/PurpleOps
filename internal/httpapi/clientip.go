package httpapi

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/bryanster/purpleops/internal/config"
)

// The forwarding headers a reverse proxy sets. Only ever read from a peer in
// PURPLEOPS_TRUSTED_PROXIES — see [realIP].
const (
	forwardedForHeader = "X-Forwarded-For"
	realIPHeader       = "X-Real-Ip"
)

// contextKey is this package's context key type. Unexported and comparable only
// to itself, so nothing outside can collide with it or overwrite what it holds.
type contextKey int

const clientIPKey contextKey = iota

// realIP resolves the address the request came from and puts it in the context.
//
// When trusted is empty — the default — the answer is always the peer this
// process is talking to. That is the only address nobody can forge, and a
// server that believed X-Forwarded-For unconditionally would let any client
// choose which address gets throttled (M1-004) and which one appears in the
// activity log (M1-015).
//
// When the peer is a proxy the operator listed, the forwarded chain is read
// right to left and the first address that is not itself a listed proxy is the
// client: the entries to the left of it were written by hops this deployment
// does not control, and a client can put anything it likes there.
//
// The request's RemoteAddr is left alone, on purpose. Rewriting it — which is
// what chi's RealIP does — means every later reader of RemoteAddr is told
// something the transport did not say, and loses the port with it. Read the
// answer with [ClientIP].
func realIP(trusted config.CIDRs) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := clientAddr(r, trusted)

			ctx := r.Context()
			if client.IsValid() {
				ctx = context.WithValue(ctx, clientIPKey, client.String())
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClientIP returns the address the request came from, as resolved by the
// middleware: the peer, or the client behind a trusted proxy. It is "" only
// when the peer address did not parse — a test's synthetic request, or a
// listener that is not TCP.
//
// It exists so that throttling, the activity log and the request log all read
// one answer rather than each interpreting the headers again.
func ClientIP(ctx context.Context) string {
	ip, ok := ctx.Value(clientIPKey).(string)
	if !ok {
		return ""
	}
	return ip
}

// clientAddr implements the rule described on [realIP].
func clientAddr(r *http.Request, trusted config.CIDRs) netip.Addr {
	peer := parseAddr(r.RemoteAddr)
	if !peer.IsValid() || !trusted.Contains(peer) {
		return peer
	}

	// Right to left: the rightmost entry was written by the proxy we trust, and
	// each step left is written by the hop before it. The first non-proxy is as
	// far back as this deployment has any reason to believe.
	for _, hop := range forwardedChain(r) {
		if addr := parseAddr(hop); addr.IsValid() && !trusted.Contains(addr) {
			return addr
		}
	}

	// Every hop is a trusted proxy, or the header is absent or unparseable.
	// The peer is the truthful answer.
	return peer
}

// forwardedChain returns the forwarded hops, nearest first.
func forwardedChain(r *http.Request) []string {
	var hops []string
	// A proxy may send several X-Forwarded-For headers rather than one comma
	// separated list; net/http keeps them in order, and the chain is their
	// concatenation.
	for _, header := range r.Header.Values(forwardedForHeader) {
		hops = append(hops, strings.Split(header, ",")...)
	}
	if len(hops) == 0 {
		// X-Real-IP carries a single address and no chain. Nginx sets it by
		// default while X-Forwarded-For has to be configured, so a deployment
		// behind a stock proxy has this and nothing else.
		if realIP := strings.TrimSpace(r.Header.Get(realIPHeader)); realIP != "" {
			hops = append(hops, realIP)
		}
	}

	nearestFirst := make([]string, 0, len(hops))
	for i := len(hops) - 1; i >= 0; i-- {
		nearestFirst = append(nearestFirst, strings.TrimSpace(hops[i]))
	}
	return nearestFirst
}

// parseAddr reads an address that may or may not carry a port, and may be an
// IPv6 address in brackets. An unparseable value — "unknown", which RFC 7239
// permits, or an obfuscated identifier — returns the zero Addr, which is not
// valid and not contained by any range.
func parseAddr(value string) netip.Addr {
	if value == "" {
		return netip.Addr{}
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	// A zone ("fe80::1%eth0") is meaningful to the machine that produced it and
	// to nothing else here; comparing it against a configured range needs it
	// gone.
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap().WithZone("")
}
