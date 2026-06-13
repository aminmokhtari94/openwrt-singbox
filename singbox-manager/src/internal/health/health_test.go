package health

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestTestURLMeasuresSuccessfulHTTP(t *testing.T) {
	previous := httpClient
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Request:    req,
		}, nil
	})}
	defer func() {
		httpClient = previous
	}()

	result, err := TestURL(context.Background(), "https://example.com/generate_204")
	if err != nil {
		t.Fatalf("test url: %v", err)
	}
	if result.Health != "ok" {
		t.Fatalf("health = %q, want ok", result.Health)
	}
	if result.LatencyMS <= 0 {
		t.Fatalf("latency = %d, want positive", result.LatencyMS)
	}
}

func TestCheckAggregatesNodeSubscriptionAndGroupHealth(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:            "home",
		Enabled:       true,
		Name:          "Home",
		Strategy:      "urltest",
		Subscriptions: []string{"sub-a"},
	}
	cfg.Subscriptions["sub-a"] = managerconfig.Subscription{
		ID:      "sub-a",
		Enabled: true,
		Name:    "Sub A",
		Format:  "auto",
		URL:     "https://example.com/sub",
	}
	cfg.Nodes["node-a"] = managerconfig.Node{
		ID:           "node-a",
		Enabled:      true,
		Type:         "direct",
		Subscription: "sub-a",
	}

	result := Check(context.Background(), cfg)
	if got := findResult(result.Nodes, "node-a").Health; got != "ok" {
		t.Fatalf("node health = %q, want ok", got)
	}
	if got := findResult(result.Subscriptions, "sub-a").Health; got != "ok" {
		t.Fatalf("subscription health = %q, want ok", got)
	}
	if got := findResult(result.Groups, "home").Health; got != "ok" {
		t.Fatalf("group health = %q, want ok", got)
	}
}

func findResult(results []EndpointResult, id string) EndpointResult {
	for _, result := range results {
		if result.ID == id {
			return result
		}
	}
	return EndpointResult{}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
