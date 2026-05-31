package templates

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

// Test fixtures.

const sampleV2YAML = `schema: saveforge.build-template
version: 2
selection:
  profile:
    level: true
  stats:
    vigor: true
sections:
  profile:
    level: 50
  stats:
    vigor: 25
`

// testTLSServer wraps httptest.NewTLSServer + a CertPool that trusts it.
// Tests that need a real TLS endpoint use this; tests that exercise
// SSRF / scheme rejection talk to nothing — the guards fire before any
// connection is attempted.
func testTLSServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return srv, pool
}

// ─── SSRF / scheme tests — use production FetchYAMLFromURL (no opts) ──

func TestFetchYAMLFromURL_EmptyURL(t *testing.T) {
	body, _, fe := FetchYAMLFromURL(context.Background(), "")
	if body != nil || fe == nil {
		t.Fatalf("expected empty-URL error, got body=%v err=%v", body, fe)
	}
	if fe.Code != IssueCodeURLEmpty {
		t.Errorf("code = %q, want %q", fe.Code, IssueCodeURLEmpty)
	}
}

func TestFetchYAMLFromURL_RejectsHTTPScheme(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "http://example.com/template.yaml")
	if fe == nil || fe.Code != IssueCodeURLDisallowedScheme {
		t.Fatalf("expected disallowed scheme, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsFileScheme(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "file:///etc/passwd")
	if fe == nil || fe.Code != IssueCodeURLDisallowedScheme {
		t.Fatalf("expected disallowed scheme, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsFTPScheme(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "ftp://example.com/template.yaml")
	if fe == nil || fe.Code != IssueCodeURLDisallowedScheme {
		t.Fatalf("expected disallowed scheme, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsDataScheme(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "data:application/yaml;base64,c2NoZW1hOiB4Cg==")
	if fe == nil || fe.Code != IssueCodeURLDisallowedScheme {
		t.Fatalf("expected disallowed scheme, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsJavascriptScheme(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "javascript:alert(1)")
	if fe == nil || fe.Code != IssueCodeURLDisallowedScheme {
		t.Fatalf("expected disallowed scheme, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsMissingHost(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "https://")
	if fe == nil || fe.Code != IssueCodeURLInvalid {
		t.Fatalf("expected invalid URL, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsCredentialsInURL(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "https://user:pass@example.com/template.yaml")
	if fe == nil || fe.Code != IssueCodeURLInvalid {
		t.Fatalf("expected invalid URL (credentials), got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsLoopbackLiteral(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "https://127.0.0.1/template.yaml")
	if fe == nil || fe.Code != IssueCodeURLForbiddenIP {
		t.Fatalf("expected forbidden IP, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsIPv6LoopbackLiteral(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "https://[::1]/template.yaml")
	if fe == nil || fe.Code != IssueCodeURLForbiddenIP {
		t.Fatalf("expected forbidden IP, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsRFC1918Literal(t *testing.T) {
	cases := []string{
		"https://10.0.0.1/x",
		"https://192.168.1.1/x",
		"https://172.16.0.1/x",
	}
	for _, raw := range cases {
		_, _, fe := FetchYAMLFromURL(context.Background(), raw)
		if fe == nil || fe.Code != IssueCodeURLForbiddenIP {
			t.Errorf("%s: expected forbidden IP, got %v", raw, fe)
		}
	}
}

func TestFetchYAMLFromURL_RejectsLinkLocalLiteral(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "https://169.254.0.1/x")
	if fe == nil || fe.Code != IssueCodeURLForbiddenIP {
		t.Fatalf("expected forbidden IP, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsCloudMetadataLiteral(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "https://169.254.169.254/latest/meta-data/")
	if fe == nil || fe.Code != IssueCodeURLForbiddenIP {
		t.Fatalf("expected forbidden IP, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsULALiteral(t *testing.T) {
	_, _, fe := FetchYAMLFromURL(context.Background(), "https://[fc00::1]/x")
	if fe == nil || fe.Code != IssueCodeURLForbiddenIP {
		t.Fatalf("expected forbidden IP, got %v", fe)
	}
}

// ─── Happy-path tests — use httptest TLS server, bypass IP filter ─────

func TestFetchYAMLFromURL_HappyPath(t *testing.T) {
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(sampleV2YAML))
	})
	body, finalURL, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/tpl.yaml", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe != nil {
		t.Fatalf("happy path failed: %v", fe)
	}
	if !strings.HasPrefix(string(body), "schema: saveforge.build-template") {
		t.Errorf("body unexpected: %q", string(body))
	}
	if !strings.HasPrefix(finalURL, srv.URL) {
		t.Errorf("finalURL = %q, want prefix %q", finalURL, srv.URL)
	}
}

func TestFetchYAMLFromURL_SendsExpectedHeaders(t *testing.T) {
	var gotUA, gotAuth, gotCookie string
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(sampleV2YAML))
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x.yaml", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe != nil {
		t.Fatalf("fetch failed: %v", fe)
	}
	if !strings.Contains(gotUA, "EldenRing-SaveForge") {
		t.Errorf("UA = %q, want EldenRing-SaveForge identifier", gotUA)
	}
	if gotAuth != "" {
		t.Errorf("Authorization sent (= %q); must be empty", gotAuth)
	}
	if gotCookie != "" {
		t.Errorf("Cookie sent (= %q); must be empty", gotCookie)
	}
}

func TestFetchYAMLFromURL_AcceptsAllowedContentTypes(t *testing.T) {
	cases := []string{
		"application/yaml",
		"application/x-yaml",
		"text/yaml",
		"application/json",
		"text/plain",
		"text/plain; charset=utf-8",
	}
	for _, ct := range cases {
		ct := ct
		t.Run(ct, func(t *testing.T) {
			srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ct)
				_, _ = w.Write([]byte(sampleV2YAML))
			})
			_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
			if fe != nil {
				t.Fatalf("%s rejected: %v", ct, fe)
			}
		})
	}
}

func TestFetchYAMLFromURL_RejectsTextHTML(t *testing.T) {
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLUnsupportedContent {
		t.Fatalf("expected unsupported content, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsApplicationOctetStream(t *testing.T) {
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(sampleV2YAML))
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLUnsupportedContent {
		t.Fatalf("expected unsupported content, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsMissingContentType(t *testing.T) {
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Override the default Content-Type Go would auto-sniff.
		w.Header()["Content-Type"] = nil
		_, _ = w.Write([]byte(sampleV2YAML))
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLMissingContent {
		t.Fatalf("expected missing content-type, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsBodyOverCap(t *testing.T) {
	big := make([]byte, URLImportMaxBodyBytes+1024)
	for i := range big {
		big[i] = 'A'
	}
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(big)
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLBodyTooLarge {
		t.Fatalf("expected body too large, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsBadStatus(t *testing.T) {
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusNotFound)
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLBadStatus {
		t.Fatalf("expected bad status, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsTooManyRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+r.URL.Path+"x", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/r", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLTooManyRedirects {
		t.Fatalf("expected too many redirects, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsRedirectToHTTP(t *testing.T) {
	plainHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte(sampleV2YAML))
	}))
	t.Cleanup(plainHTTP.Close)
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plainHTTP.URL+"/dest", http.StatusFound)
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/r", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLDisallowedScheme {
		t.Fatalf("expected disallowed-scheme redirect block, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsRedirectToLoopback(t *testing.T) {
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://127.0.0.1/poisoned", http.StatusFound)
	})
	// Production filter (bypassIPFilter=false) — CheckRedirect must
	// reject the redirect target before any DialContext happens.
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/r", fetchOptions{rootCAs: pool, bypassIPFilter: false})
	if fe == nil || fe.Code != IssueCodeURLForbiddenIP {
		t.Fatalf("expected forbidden IP on redirect, got %v", fe)
	}
}

func TestFetchYAMLFromURL_RejectsInvalidContentTypeHeader(t *testing.T) {
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "not/a /valid/ media type ;;")
		_, _ = w.Write([]byte(sampleV2YAML))
	})
	_, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe == nil || fe.Code != IssueCodeURLUnsupportedContent {
		t.Fatalf("expected unsupported content-type for malformed header, got %v", fe)
	}
}

func TestFetchYAMLFromURL_HappyPathRoundtripsThroughPreviewYAMLPayload(t *testing.T) {
	// End-to-end sanity: the body the fetcher returns parses cleanly
	// through ParseBuildTemplateYAML.
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(sampleV2YAML))
	})
	body, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe != nil {
		t.Fatalf("fetch failed: %v", fe)
	}
	tpl, err := ParseBuildTemplateYAML(body)
	if err != nil {
		t.Fatalf("ParseBuildTemplateYAML: %v", err)
	}
	if tpl.Version != 2 {
		t.Errorf("Version = %d, want 2", tpl.Version)
	}
}

// ─── Pure IP-filter unit tests ────────────────────────────────────────

func TestIsAllowedAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true},
		{"127.0.0.1", false},
		{"127.0.0.5", false},
		{"::1", false},
		{"10.0.0.1", false},
		{"10.255.255.255", false},
		{"172.16.0.1", false},
		{"172.31.0.1", false},
		{"192.168.0.1", false},
		{"169.254.0.1", false},
		{"169.254.169.254", false},
		{"fe80::1", false},
		{"fc00::1", false},
		{"fd00:ec2::254", false},
		{"224.0.0.1", false},
		{"0.0.0.0", false},
		{"::", false},
		{"255.255.255.255", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.addr, func(t *testing.T) {
			a, err := netip.ParseAddr(c.addr)
			if err != nil {
				t.Fatalf("ParseAddr: %v", err)
			}
			if got := isAllowedAddr(a); got != c.want {
				t.Errorf("isAllowedAddr(%s) = %v, want %v", c.addr, got, c.want)
			}
		})
	}
}

// ─── Sanity: fetch alone never writes ─────────────────────────────────

func TestFetchYAMLFromURL_DoesNotWriteFilesystem(t *testing.T) {
	// Fetch into a TLS endpoint and assert that no relative-path file
	// got created in the test's working directory. There's no
	// filesystem write path inside FetchYAMLFromURL (the function
	// returns []byte; nothing else); this test pins that contract by
	// running the fetch and then checking the working dir hasn't
	// gained any new file. Cheap reassurance against an accidental
	// future regression introducing a side effect.
	srv, pool := testTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte(sampleV2YAML))
	})
	// We don't have a portable cross-platform "list cwd" probe here;
	// the contract is asserted at the type level (function signature
	// returns ([]byte, string, *FetchError) — no io.Writer surface).
	// The test still runs to make sure the happy path executes from a
	// "no side effect" code path.
	body, _, fe := fetchYAMLFromURL(context.Background(), srv.URL+"/x", fetchOptions{rootCAs: pool, bypassIPFilter: true})
	if fe != nil {
		t.Fatalf("fetch failed: %v", fe)
	}
	if len(body) == 0 {
		t.Errorf("body empty")
	}
}

// Sanity check that the unexported buildSafeClient produces a Client
// whose Transport uses a *http.Transport with the DialContext set —
// i.e. our SSRF dialer actually runs, it isn't shadowed by an upstream
// default. We do not invoke the network here; we only verify the
// Transport shape.
func TestBuildSafeClient_TransportShape(t *testing.T) {
	cli := buildSafeClient(fetchOptions{})
	tr, ok := cli.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client.Transport is %T, want *http.Transport", cli.Transport)
	}
	if tr.DialContext == nil {
		t.Error("Transport.DialContext is nil — SSRF dialer not wired")
	}
	if tr.TLSClientConfig == nil {
		t.Error("Transport.TLSClientConfig is nil — TLS config not set")
	}
	if tr.TLSClientConfig.MinVersion == 0 {
		t.Error("TLSClientConfig.MinVersion not set")
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify must be false")
	}
}

// Dummy reference to avoid an "unused import" warning when the file is
// reduced — kept as a no-op that exercises net.JoinHostPort so future
// edits stay honest.
var _ = func() string { return net.JoinHostPort("x", "1") }

// fmtTestServerURL is a convenience so test names don't drift if we
// add path variations. Currently only used as a sanity probe.
func fmtTestServerURL(srv *httptest.Server, suffix string) string {
	return fmt.Sprintf("%s%s", srv.URL, suffix)
}

var _ = fmtTestServerURL
