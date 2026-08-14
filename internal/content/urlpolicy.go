package content

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// URLPolicy is the egress fence for content sync (M7-014 / BL-004). Every
// source URL is checked against it twice: once when an admin writes the URL
// (Registry.UpdateSource) and again immediately before any fetch
// (HTTPSource.Open). The policy allows https only by default, rejects every
// private or reserved destination, and refuses to follow a redirect that would
// break those rules.
type URLPolicy struct {
	// AllowHTTP permits plain http:// URLs. Only a development deployment may
	// set it (config.Environment.IsDevelopment); the zero value is production.
	AllowHTTP bool

	// LookupIP resolves a hostname to the addresses Validate checks. nil uses
	// net.DefaultResolver. Tests inject a fake so validation never touches the
	// network.
	LookupIP func(ctx context.Context, host string) ([]net.IP, error)
}

const (
	// fetchDialTimeout bounds TCP/TLS connection establishment for a content
	// fetch. The response body is bounded separately by the request context
	// (the job's own timeout), not by a whole-request client timeout: a
	// legitimate multi-hundred-MiB catalog can take minutes on a slow link.
	fetchDialTimeout = 10 * time.Second

	// fetchResponseHeaderTimeout bounds how long a fetch waits for the upstream
	// to start answering. A private host that accepts TCP but never responds
	// fails here instead of hanging until the job timeout.
	fetchResponseHeaderTimeout = 30 * time.Second
)

// metadataHosts are cloud instance-metadata endpoints, matched on the hostname
// before any resolution. 169.254.169.254 is an IP literal caught by the private
// range check too, but is listed here for a clearer error message.
var metadataHosts = map[string]struct{}{
	"169.254.169.254":          {},
	"metadata.google.internal": {},
	"metadata":                 {},
	"instance-data":            {},
}

// Validate checks raw before any request is made. A nil error means the URL is
// a public http(s) destination this policy permits.
func (p URLPolicy) Validate(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("source URL is not a valid URL: %w", err)
	}
	if u.Scheme == "" {
		return errors.New("source URL must be an absolute http(s) URL")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !p.AllowHTTP {
			return errors.New("source URL must use https (plain http is only allowed in development)")
		}
	default:
		return fmt.Errorf("source URL scheme %q is not allowed (only http and https)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("source URL must include a host")
	}
	if _, err := p.resolve(ctx, host); err != nil {
		return err
	}
	return nil
}

// resolve returns the public addresses of host, or an error when host is a
// metadata endpoint, resolves to any private or reserved address, or cannot be
// resolved. It is the dial-time half of the fence: a connection is pinned to one
// of the returned addresses, so a later DNS answer cannot smuggle a private
// target past the check.
func (p URLPolicy) resolve(ctx context.Context, host string) ([]net.IP, error) {
	if _, ok := metadataHosts[strings.ToLower(strings.TrimSuffix(host, "."))]; ok {
		return nil, fmt.Errorf("source host %q is a cloud metadata endpoint", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("source host %q is not a public address", host)
		}
		return []net.IP{ip}, nil
	}
	lookup := p.LookupIP
	if lookup == nil {
		lookup = defaultLookupIP
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("source host %q cannot be resolved: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("source host %q has no addresses", host)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("source host %q resolves to %s, which is not a public address", host, ip)
		}
	}
	return ips, nil
}

func defaultLookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// isPrivateIP reports whether ip is loopback, link-local, RFC1918, unique-local
// IPv6, unspecified, or multicast. None of these is ever a legitimate
// content-sync destination.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// NewClient returns an *http.Client for content fetch that enforces the policy:
// the initial URL is validated before it is dialed (HTTPSource.Open), every
// redirect is re-validated, and the TCP connection is pinned to an address
// validated at dial time. It carries dial and response-header timeouts; the
// response body is bounded by the request context, not a whole-request client
// timeout.
func (p URLPolicy) NewClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   fetchDialTimeout,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           p.dialContext(dialer),
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: fetchResponseHeaderTimeout,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   fetchDialTimeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := p.Validate(req.Context(), req.URL.String()); err != nil {
				return fmt.Errorf("redirect refused: %w", err)
			}
			return nil
		},
	}
}

func (p URLPolicy) dialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := p.resolve(ctx, host)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}
