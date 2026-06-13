package ruleset

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestDownloadWritesRuleset(t *testing.T) {
	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("srs-data")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() {
		http.DefaultClient.Transport = originalTransport
	})

	path := filepath.Join(t.TempDir(), "geoip-test.srs")
	result, err := Download(context.Background(), managerconfig.RuleSet{
		ID:      "geoip-test",
		Enabled: true,
		Type:    "remote",
		Format:  "srs",
		URL:     "https://example.com/geoip-test.srs",
		Path:    path,
	})
	if err != nil {
		t.Fatalf("download ruleset: %v", err)
	}
	if result.Path != path || result.Bytes != int64(len("srs-data")) {
		t.Fatalf("result = %#v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded ruleset: %v", err)
	}
	if string(data) != "srs-data" {
		t.Fatalf("downloaded data = %q", data)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestDueUsesLastUpdateAndInterval(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	entry := managerconfig.RuleSet{
		ID:             "geoip-test",
		Enabled:        true,
		Type:           "remote",
		URL:            "https://example.com/geoip-test.srs",
		UpdateInterval: "24h",
		LastUpdate:     now.Add(-25 * time.Hour).Format(time.RFC3339),
	}
	if !Due(entry, now) {
		t.Fatal("expected ruleset to be due")
	}
	entry.LastUpdate = now.Add(-23 * time.Hour).Format(time.RFC3339)
	if Due(entry, now) {
		t.Fatal("expected ruleset to not be due")
	}
}
