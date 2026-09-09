package helpers

import (
	"context"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type whiteboardDownloadTransport func(*http.Request) (*http.Response, error)

func (f whiteboardDownloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCrossPlatformCoverageWhiteboardDownloadTargets(t *testing.T) {
	for _, raw := range []string{"http://example.com/a", "https://user:pass@example.com/a", "https://example.com:8443/a", "https://127.0.0.1/a", "https://169.254.169.254/a", "https://10.0.0.1/a", "https://[::1]/a", "https://[::ffff:127.0.0.1]/a", "https://[fe80::1]/a", "file:///tmp/a", "https://"} {
		if err := validateWhiteboardDownloadURL(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if err := validateWhiteboardDownloadURL("https://storage.example.com:443/a"); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"127.0.0.1:443", "10.0.0.1:443", "169.254.169.254:443", "[::1]:443", "[fc00::1]:443", "100.64.0.1:443", "example.com:443", "8.8.8.8:80", "invalid"} {
		if err := whiteboardDownloadControl("tcp", host, nil); err == nil {
			t.Errorf("accepted dial %s", host)
		}
	}
	if err := whiteboardDownloadControl("tcp", "8.8.8.8:443", nil); err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"0.0.0.0", "224.0.0.1", "240.0.0.1", "2002:7f00:1::", "64:ff9b::7f00:1", "2001::1"} {
		if whiteboardPublicIP(netip.MustParseAddr(ip)) {
			t.Errorf("accepted %s", ip)
		}
	}
}

func TestCrossPlatformCoverageWhiteboardDownloadRedirect(t *testing.T) {
	calls := 0
	client := &http.Client{CheckRedirect: whiteboardDownloadRedirect, Transport: whiteboardDownloadTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://127.0.0.1/private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	err := downloadWhiteboardExportLimited(context.Background(), client, "https://example.com/a", filepath.Join(t.TempDir(), "out"), 16)
	if err == nil || calls != 1 {
		t.Fatalf("redirect err=%v calls=%d", err, calls)
	}
	u, _ := url.Parse("https://other.example/a")
	req := &http.Request{URL: u, Header: http.Header{"Authorization": []string{"secret"}}}
	if err := whiteboardDownloadRedirect(req, nil); err != nil || len(req.Header) != 0 {
		t.Fatalf("redirect hygiene: %v", err)
	}
	if err := whiteboardDownloadRedirect(req, make([]*http.Request, 5)); err == nil {
		t.Fatal("unbounded redirects")
	}
}

func TestCrossPlatformCoverageWhiteboardDownloadSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int64
		body   string
		fail   bool
	}{
		{"declared-too-large", 17, "", true}, {"stream-too-large", -1, strings.Repeat("a", 17), true}, {"exact", 16, strings.Repeat("a", 16), false}, {"unknown-small", -1, "small", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tmp")
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: whiteboardDownloadTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, ContentLength: tc.length, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header), Request: r}, nil
			})}
			err := downloadWhiteboardExportLimited(context.Background(), client, "https://example.com/a", path, 16)
			if (err != nil) != tc.fail {
				t.Fatalf("err=%v", err)
			}
			info, _ := os.Stat(path)
			if info.Size() > 17 {
				t.Fatalf("unbounded write: %d", info.Size())
			}
		})
	}
}

type whiteboardFailReader struct{}

func (whiteboardFailReader) Read([]byte) (int, error) { return 0, context.Canceled }
func (whiteboardFailReader) Close() error             { return nil }
func TestCrossPlatformCoverageWhiteboardDownloadFailures(t *testing.T) {
	if err := downloadWhiteboardExportHTTP(context.Background(), "https://127.0.0.1/a", nil, "unused"); err == nil {
		t.Fatal("unsafe URL accepted")
	}
	for _, tc := range []string{"bad-method-url", "status", "open", "read", "close"} {
		t.Run(tc, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tmp")
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
			raw := "https://example.test/a"
			if tc == "bad-method-url" {
				raw = "https://example.test/a"
			}
			client := &http.Client{Transport: whiteboardDownloadTransport(func(r *http.Request) (*http.Response, error) {
				body := io.ReadCloser(io.NopCloser(strings.NewReader("body")))
				status := 200
				if tc == "status" {
					status = 500
				}
				if tc == "read" {
					body = whiteboardFailReader{}
				}
				return &http.Response{StatusCode: status, Body: body, ContentLength: -1, Header: make(http.Header), Request: r}, nil
			})}
			if tc == "open" {
				path = filepath.Join(path, "missing")
			}
			if tc == "close" {
				testseam.Swap(t, &whiteboardDownloadClose, func(f *os.File) error { f.Close(); return context.Canceled })
			}
			ctx := context.Background()
			if tc == "bad-method-url" {
				ctx = nil
			}
			if err := downloadWhiteboardExportLimited(ctx, client, raw, path, 16); err == nil {
				t.Fatal("failure lost")
			}
		})
	}
}
