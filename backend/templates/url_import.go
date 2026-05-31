package templates

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// URL import — Phase 9 of spec/56-templates-v2.md.
//
// All public knobs and limits are exported so the call site (the Wails
// handler in app_templates_url.go) and tests can refer to them by name
// without re-deriving the constants. The internal fetch helpers accept
// fetchOptions to inject test-only knobs (e.g. an httptest server's
// self-signed certificate pool, an explicit IP-filter bypass for the
// happy-path tests that target a loopback test server). Production
// always calls FetchYAMLFromURL with zero options, which keeps every
// guard active.

const (
	// URLImportMaxBodyBytes mirrors maxYAMLImportBytes in the file
	// import path (1 MiB). Centralising the constant in templates/
	// keeps the URL import layer in sync with the file import layer.
	URLImportMaxBodyBytes int64 = 1 << 20

	// URLImportTotalTimeout is the hard ceiling on the entire fetch —
	// connect + TLS + headers + body. Applied via
	// context.WithTimeout in the public entry point.
	URLImportTotalTimeout = 10 * time.Second

	// URLImportIdleTimeout caps individual stages: response header,
	// TLS handshake, dialer connect. The total budget is bounded by
	// URLImportTotalTimeout above.
	URLImportIdleTimeout = 5 * time.Second

	// URLImportMaxRedirects is the cap enforced by CheckRedirect.
	URLImportMaxRedirects = 3

	// URLImportUserAgent identifies our client honestly. We intentionally
	// do not pretend to be a browser.
	URLImportUserAgent = "EldenRing-SaveForge Templates-v2-URL-import"
)

// IssueCodeURL* — new error codes for URL import surfaced through
// ImportPreviewIssue. Kept in their own const block so the file/library
// allowlist of issue codes (in import.go) stays untouched.
const (
	IssueCodeURLEmpty              = "url_empty"
	IssueCodeURLInvalid            = "url_invalid"
	IssueCodeURLDisallowedScheme   = "url_disallowed_scheme"
	IssueCodeURLForbiddenIP        = "url_forbidden_ip"
	IssueCodeURLDNSFailed          = "url_dns_failed"
	IssueCodeURLTooManyRedirects   = "url_too_many_redirects"
	IssueCodeURLTimeout            = "url_timeout"
	IssueCodeURLConnectionFailed   = "url_connection_failed"
	IssueCodeURLTLSError           = "url_tls_error"
	IssueCodeURLBodyTooLarge       = "url_body_too_large"
	IssueCodeURLUnsupportedContent = "url_unsupported_content_type"
	IssueCodeURLMissingContent     = "url_missing_content_type"
	IssueCodeURLBadStatus          = "url_bad_status"
)

// allowedContentTypes is the closed allowlist of MIME types we accept
// in URL responses. Multipart, octet-stream, html, anything outside
// this map is rejected.
var allowedContentTypes = map[string]bool{
	"application/json":   true,
	"application/yaml":   true,
	"application/x-yaml": true,
	"text/yaml":          true,
	"text/plain":         true,
}

// FetchError carries a precise issue code + user-visible message. The
// caller in app_templates_url.go maps it directly to an
// ImportPreviewIssue, so the codes here must match user-friendly
// rejection reasons the UI can render.
type FetchError struct {
	Code    string
	Message string
}

func (e *FetchError) Error() string { return e.Code + ": " + e.Message }

// FetchYAMLFromURL is the production entry point — public so the Wails
// handler can call it. All guards from spec/56 §12 are active:
// HTTPS-only scheme, IP filter against loopback/private/link-local/
// metadata, redirect re-check (max 3), TLS 1.2+, response header /
// idle / total timeouts, content-type allowlist, body cap.
//
// Returns the body bytes and the canonical final URL (after redirects)
// on success. On any guard violation returns a *FetchError carrying the
// IssueCodeURL* code the UI should surface.
func FetchYAMLFromURL(ctx context.Context, rawURL string) ([]byte, string, *FetchError) {
	return fetchYAMLFromURL(ctx, rawURL, fetchOptions{})
}

// fetchOptions is the test-only injection seam. Production never sets
// any field. Tests use bypassIPFilter so they can target an httptest
// server bound to 127.0.0.1 and rootCAs so they can trust the self-
// signed certificate that httptest.NewTLSServer issues. SSRF / scheme
// rejection tests use zero options against the real network stack and
// expect the guards to fire before any connection is attempted.
type fetchOptions struct {
	rootCAs        *x509.CertPool
	bypassIPFilter bool
}

func fetchYAMLFromURL(ctx context.Context, rawURL string, opts fetchOptions) ([]byte, string, *FetchError) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, "", &FetchError{Code: IssueCodeURLEmpty, Message: "URL is required."}
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, "", &FetchError{Code: IssueCodeURLInvalid, Message: "Could not parse URL: " + err.Error()}
	}
	if u.Scheme != "https" {
		return nil, "", &FetchError{Code: IssueCodeURLDisallowedScheme, Message: "Only https:// URLs are allowed."}
	}
	if u.Host == "" {
		return nil, "", &FetchError{Code: IssueCodeURLInvalid, Message: "URL host is missing."}
	}
	if u.User != nil {
		return nil, "", &FetchError{Code: IssueCodeURLInvalid, Message: "URL must not embed credentials."}
	}

	client := buildSafeClient(opts)

	ctx, cancel := context.WithTimeout(ctx, URLImportTotalTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmed, nil)
	if err != nil {
		return nil, "", &FetchError{Code: IssueCodeURLInvalid, Message: "Could not build request: " + err.Error()}
	}
	req.Header.Set("User-Agent", URLImportUserAgent)
	req.Header.Set("Accept", "application/json, application/yaml, application/x-yaml, text/yaml, text/plain")

	resp, doErr := client.Do(req)
	if doErr != nil {
		return nil, "", classifyDoError(doErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", &FetchError{Code: IssueCodeURLBadStatus, Message: fmt.Sprintf("Server returned HTTP %d.", resp.StatusCode)}
	}

	ct := resp.Header.Get("Content-Type")
	if strings.TrimSpace(ct) == "" {
		return nil, "", &FetchError{Code: IssueCodeURLMissingContent, Message: "Server did not send a Content-Type header."}
	}
	mediaType, _, mtErr := mime.ParseMediaType(ct)
	if mtErr != nil {
		return nil, "", &FetchError{Code: IssueCodeURLUnsupportedContent, Message: "Invalid Content-Type: " + ct}
	}
	if !allowedContentTypes[mediaType] {
		return nil, "", &FetchError{Code: IssueCodeURLUnsupportedContent, Message: "Content-Type " + mediaType + " is not allowed."}
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, URLImportMaxBodyBytes+1))
	if readErr != nil {
		if errors.Is(readErr, context.DeadlineExceeded) {
			return nil, "", &FetchError{Code: IssueCodeURLTimeout, Message: "Body read timed out."}
		}
		return nil, "", &FetchError{Code: IssueCodeURLConnectionFailed, Message: "Body read failed: " + readErr.Error()}
	}
	if int64(len(body)) > URLImportMaxBodyBytes {
		return nil, "", &FetchError{Code: IssueCodeURLBodyTooLarge, Message: fmt.Sprintf("Body exceeds %d byte limit.", URLImportMaxBodyBytes)}
	}

	finalURL := trimmed
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return body, finalURL, nil
}

// buildSafeClient assembles the http.Client with all SSRF guards wired
// in. Tests inject opts; production calls with the zero value.
func buildSafeClient(opts fetchOptions) *http.Client {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    opts.rootCAs, // nil → system roots
	}

	netDialer := &net.Dialer{
		Timeout:   URLImportIdleTimeout,
		KeepAlive: -1,
	}

	safeDial := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		// If the host is already a literal IP we still apply the
		// filter — this is what catches https://127.0.0.1/foo even
		// before any DNS lookup happens.
		if literal, ok := netip.AddrFromSlice(net.ParseIP(host)); ok && literal.IsValid() {
			if !opts.bypassIPFilter && !isAllowedAddr(literal) {
				return nil, &FetchError{Code: IssueCodeURLForbiddenIP, Message: "Resolved IP " + literal.String() + " is not allowed (loopback / private / link-local / metadata)."}
			}
			return netDialer.DialContext(ctx, network, net.JoinHostPort(literal.String(), port))
		}
		ips, lookupErr := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if lookupErr != nil || len(ips) == 0 {
			return nil, &FetchError{Code: IssueCodeURLDNSFailed, Message: "DNS resolution failed for " + host + "."}
		}
		var chosen net.IP
		for _, ip := range ips {
			a, ok := netip.AddrFromSlice(ip)
			if !ok {
				continue
			}
			a = a.Unmap()
			if !opts.bypassIPFilter && !isAllowedAddr(a) {
				return nil, &FetchError{Code: IssueCodeURLForbiddenIP, Message: "Resolved IP " + a.String() + " is not allowed (loopback / private / link-local / metadata)."}
			}
			chosen = ip
			break
		}
		if chosen == nil {
			return nil, &FetchError{Code: IssueCodeURLDNSFailed, Message: "No usable IP for " + host + "."}
		}
		return netDialer.DialContext(ctx, network, net.JoinHostPort(chosen.String(), port))
	}

	transport := &http.Transport{
		DialContext:           safeDial,
		TLSClientConfig:       tlsCfg,
		TLSHandshakeTimeout:   URLImportIdleTimeout,
		ResponseHeaderTimeout: URLImportIdleTimeout,
		IdleConnTimeout:       URLImportIdleTimeout,
		DisableKeepAlives:     true,
		DisableCompression:    true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   URLImportTotalTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= URLImportMaxRedirects {
				return &FetchError{Code: IssueCodeURLTooManyRedirects, Message: fmt.Sprintf("Too many redirects (max %d).", URLImportMaxRedirects)}
			}
			if req.URL.Scheme != "https" {
				return &FetchError{Code: IssueCodeURLDisallowedScheme, Message: "Redirect to non-https URL is blocked."}
			}
			// Re-resolve and re-filter the redirect target so a
			// 30x → https://127.0.0.1 attack fails fast with a
			// precise error code, before DialContext gets a chance.
			if opts.bypassIPFilter {
				return nil
			}
			host := req.URL.Hostname()
			if literal, ok := netip.AddrFromSlice(net.ParseIP(host)); ok && literal.IsValid() {
				if !isAllowedAddr(literal) {
					return &FetchError{Code: IssueCodeURLForbiddenIP, Message: "Redirect target IP " + literal.String() + " is not allowed."}
				}
				return nil
			}
			ips, err := net.DefaultResolver.LookupIP(req.Context(), "ip", host)
			if err != nil || len(ips) == 0 {
				return &FetchError{Code: IssueCodeURLDNSFailed, Message: "DNS resolution failed for redirect target " + host + "."}
			}
			for _, ip := range ips {
				a, ok := netip.AddrFromSlice(ip)
				if !ok {
					continue
				}
				a = a.Unmap()
				if !isAllowedAddr(a) {
					return &FetchError{Code: IssueCodeURLForbiddenIP, Message: "Redirect target IP " + a.String() + " is not allowed."}
				}
			}
			return nil
		},
	}
}

// classifyDoError maps the heterogeneous errors http.Client.Do returns
// into our FetchError codes. The url.Error wrapper hides the real
// cause behind .Err; we unwrap and inspect.
func classifyDoError(err error) *FetchError {
	var fe *FetchError
	if errors.As(err, &fe) {
		return fe
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &FetchError{Code: IssueCodeURLTimeout, Message: "Request timed out."}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return &FetchError{Code: IssueCodeURLTimeout, Message: "Request timed out."}
		}
		inner := urlErr.Err
		if errors.As(inner, &fe) {
			return fe
		}
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(low, "x509") || strings.Contains(low, "tls") || strings.Contains(low, "certificate") {
		return &FetchError{Code: IssueCodeURLTLSError, Message: "TLS error: " + msg}
	}
	return &FetchError{Code: IssueCodeURLConnectionFailed, Message: "Connection failed: " + msg}
}

// isAllowedAddr is the pure SSRF predicate. Returns true only for
// addresses that are safe to connect to from the user's machine to a
// public host — loopback, RFC1918, link-local, ULA, multicast,
// unspecified, broadcast, and known cloud metadata endpoints all
// return false.
//
// netip.Addr.IsPrivate covers RFC1918 (10/8, 172.16/12, 192.168/16)
// and IPv6 ULA fc00::/7. IsLinkLocalUnicast covers 169.254/16 and
// fe80::/10. IsLoopback covers 127/8 and ::1. IsMulticast covers
// 224/4 and ff00::/8. Cloud metadata endpoints are listed explicitly
// because the IPv6 endpoint (fd00:ec2::254) is in ULA — already caught
// by IsPrivate — but the IPv4 endpoint (169.254.169.254) is already
// caught by IsLinkLocalUnicast. Both are kept here for an exact match
// belt-and-braces.
func isAllowedAddr(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() {
		return false
	}
	if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() ||
		a.IsLinkLocalMulticast() || a.IsMulticast() || a.IsUnspecified() ||
		a.IsInterfaceLocalMulticast() {
		return false
	}
	// IPv4 broadcast.
	if a.Is4() && a == netip.MustParseAddr("255.255.255.255") {
		return false
	}
	// AWS / GCE / Azure IMDS — IPv4 (169.254.169.254) is already
	// caught by IsLinkLocalUnicast; IPv6 (fd00:ec2::254) is already
	// caught by IsPrivate (ULA). Belt-and-braces: explicit match.
	if a == netip.MustParseAddr("169.254.169.254") {
		return false
	}
	if a == netip.MustParseAddr("fd00:ec2::254") {
		return false
	}
	return true
}
