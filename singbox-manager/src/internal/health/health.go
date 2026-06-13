package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

const DefaultTestURL = "https://www.gstatic.com/generate_204"

var httpClient = http.DefaultClient

type EndpointResult struct {
	ID        string `json:"id"`
	Health    string `json:"health"`
	LatencyMS int    `json:"latency_ms"`
	Method    string `json:"method,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Result struct {
	Nodes         []EndpointResult `json:"nodes"`
	Subscriptions []EndpointResult `json:"subscriptions"`
	Groups        []EndpointResult `json:"groups"`
}

func TestURL(ctx context.Context, url string) (EndpointResult, error) {
	if url == "" {
		url = DefaultTestURL
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return EndpointResult{ID: url, Health: "error", Error: err.Error()}, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return EndpointResult{ID: url, Health: "error", Error: err.Error()}, err
	}
	defer resp.Body.Close()

	latency := latencyMS(started)
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return EndpointResult{ID: url, Health: "ok", LatencyMS: latency}, nil
	}
	err = fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	return EndpointResult{ID: url, Health: "error", LatencyMS: latency, Error: err.Error()}, err
}

func TestDNS(ctx context.Context, server managerconfig.DNSServer, domain string) (EndpointResult, error) {
	if domain == "" {
		domain = "example.com"
	}
	dnsType := normalizeDNSType(server.Type)
	host, port := parseDNSAddress(server.Address, dnsType)
	if host == "" {
		err := fmt.Errorf("dns server address is required")
		return EndpointResult{ID: server.ID, Health: "error", Error: err.Error()}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	started := time.Now()

	if dnsType == "udp" || dnsType == "tcp" {
		resolver := net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network string, address string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, dnsType, net.JoinHostPort(host, strconv.Itoa(port)))
			},
		}
		if _, err := resolver.LookupHost(ctx, domain); err != nil {
			return EndpointResult{ID: server.ID, Health: "down", LatencyMS: latencyMS(started), Error: err.Error()}, err
		}
		return EndpointResult{ID: server.ID, Health: "ok", LatencyMS: latencyMS(started)}, nil
	}

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return EndpointResult{ID: server.ID, Health: "down", LatencyMS: latencyMS(started), Error: err.Error()}, err
	}
	_ = conn.Close()
	return EndpointResult{ID: server.ID, Health: "ok", LatencyMS: latencyMS(started)}, nil
}

func Check(ctx context.Context, cfg managerconfig.Config) Result {
	nodeResults := checkNodes(ctx, cfg)
	return Result{
		Nodes:         sortedResults(nodeResults),
		Subscriptions: checkSubscriptions(cfg, nodeResults),
		Groups:        checkGroups(cfg, nodeResults),
	}
}

func ToHealthStates(result Result) (map[string]managerconfig.HealthState, map[string]managerconfig.HealthState, map[string]managerconfig.HealthState) {
	nodes := map[string]managerconfig.HealthState{}
	groups := map[string]managerconfig.HealthState{}
	subscriptions := map[string]managerconfig.HealthState{}

	for _, item := range result.Nodes {
		nodes[item.ID] = healthState(item)
	}
	for _, item := range result.Groups {
		groups[item.ID] = healthState(item)
	}
	for _, item := range result.Subscriptions {
		subscriptions[item.ID] = healthState(item)
	}
	return nodes, groups, subscriptions
}

func checkNodes(ctx context.Context, cfg managerconfig.Config) map[string]EndpointResult {
	results := map[string]EndpointResult{}
	for id, node := range cfg.Nodes {
		results[id] = CheckNode(ctx, node)
	}
	return results
}

func CheckNode(ctx context.Context, node managerconfig.Node) EndpointResult {
	result := EndpointResult{ID: node.ID}
	if !node.Enabled {
		result.Health = "disabled"
		return result
	}
	if node.Type == "direct" {
		result.Health = "ok"
		return result
	}

	server := node.Server
	if server == "" {
		server = node.Address
	}
	if server == "" || node.Port == 0 {
		result.Health = "error"
		result.Error = "missing server or port"
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var dialer net.Dialer
	started := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(server, fmt.Sprintf("%d", node.Port)))
	result.LatencyMS = latencyMS(started)
	if err != nil {
		result.Health = "down"
		result.Error = err.Error()
		return result
	}
	_ = conn.Close()
	result.Health = "ok"
	return result
}

func PingNode(ctx context.Context, node managerconfig.Node) EndpointResult {
	result := EndpointResult{ID: node.ID}
	if !node.Enabled {
		result.Health = "disabled"
		return result
	}
	if node.Type == "direct" {
		result.Health = "ok"
		result.Method = "direct"
		return result
	}

	host := firstNonEmpty(node.Server, node.Address)
	if host == "" {
		result.Health = "error"
		result.Error = "missing server or address"
		return result
	}

	if pingPath, err := exec.LookPath("ping"); err == nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		started := time.Now()
		if err := exec.CommandContext(ctx, pingPath, "-c", "1", "-W", "3", host).Run(); err == nil {
			result.Health = "ok"
			result.LatencyMS = latencyMS(started)
			result.Method = "icmp"
			return result
		}
	}

	result = CheckNode(ctx, node)
	if result.Method == "" {
		result.Method = "tcp"
	}
	return result
}

func TestNodeURL(ctx context.Context, node managerconfig.Node, rawURL string) (EndpointResult, error) {
	if rawURL == "" {
		rawURL = DefaultTestURL
	}
	result := CheckNode(ctx, node)
	result.Method = "tcp"
	if result.Error != "" || result.Health != "ok" {
		if result.Error == "" {
			result.Error = fmt.Sprintf("node health is %s", result.Health)
		}
		return result, fmt.Errorf("%s", result.Error)
	}

	urlResult, err := TestURL(ctx, rawURL)
	urlResult.ID = node.ID
	urlResult.Method = "url"
	return urlResult, err
}

func checkSubscriptions(cfg managerconfig.Config, nodeResults map[string]EndpointResult) []EndpointResult {
	results := make([]EndpointResult, 0, len(cfg.Subscriptions))
	for id, subscription := range cfg.Subscriptions {
		result := EndpointResult{ID: id}
		if !subscription.Enabled {
			result.Health = "disabled"
			results = append(results, result)
			continue
		}
		result.Health, result.LatencyMS = aggregateHealth(subscriptionNodes(cfg, id), nodeResults)
		results = append(results, result)
	}
	sortEndpointResults(results)
	return results
}

func checkGroups(cfg managerconfig.Config, nodeResults map[string]EndpointResult) []EndpointResult {
	results := make([]EndpointResult, 0, len(cfg.Groups))
	for id, group := range cfg.Groups {
		result := EndpointResult{ID: id}
		if !group.Enabled {
			result.Health = "disabled"
			results = append(results, result)
			continue
		}
		result.Health, result.LatencyMS = aggregateHealth(groupNodes(cfg, group), nodeResults)
		results = append(results, result)
	}
	sortEndpointResults(results)
	return results
}

func subscriptionNodes(cfg managerconfig.Config, subscriptionID string) []managerconfig.Node {
	nodes := []managerconfig.Node{}
	for _, node := range cfg.Nodes {
		if node.Subscription == subscriptionID {
			nodes = append(nodes, node)
		}
	}
	sortNodes(nodes)
	return nodes
}

func groupNodes(cfg managerconfig.Config, group managerconfig.Group) []managerconfig.Node {
	subscriptions := map[string]bool{}
	for _, id := range group.Subscriptions {
		subscriptions[id] = true
	}

	nodes := []managerconfig.Node{}
	for _, node := range cfg.Nodes {
		if !node.Enabled {
			continue
		}
		if node.Subscription != "" && len(subscriptions) > 0 && !subscriptions[node.Subscription] {
			continue
		}
		nodes = append(nodes, node)
	}
	sortNodes(nodes)
	return nodes
}

func aggregateHealth(nodes []managerconfig.Node, nodeResults map[string]EndpointResult) (string, int) {
	if len(nodes) == 0 {
		return "empty", 0
	}
	best := 0
	down := 0
	disabled := 0
	for _, node := range nodes {
		result := nodeResults[node.ID]
		switch result.Health {
		case "ok":
			if best == 0 || (result.LatencyMS > 0 && result.LatencyMS < best) {
				best = result.LatencyMS
			}
		case "disabled":
			disabled++
		default:
			down++
		}
	}
	if best > 0 || hasDirectOK(nodes, nodeResults) {
		if down > 0 {
			return "degraded", best
		}
		return "ok", best
	}
	if disabled == len(nodes) {
		return "disabled", 0
	}
	return "down", 0
}

func hasDirectOK(nodes []managerconfig.Node, nodeResults map[string]EndpointResult) bool {
	for _, node := range nodes {
		if node.Type == "direct" && nodeResults[node.ID].Health == "ok" {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func sortedResults(results map[string]EndpointResult) []EndpointResult {
	items := make([]EndpointResult, 0, len(results))
	for _, result := range results {
		items = append(items, result)
	}
	sortEndpointResults(items)
	return items
}

func sortEndpointResults(items []EndpointResult) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
}

func sortNodes(nodes []managerconfig.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
}

func healthState(result EndpointResult) managerconfig.HealthState {
	return managerconfig.HealthState{
		Health:    result.Health,
		LatencyMS: result.LatencyMS,
	}
}

func latencyMS(started time.Time) int {
	elapsed := time.Since(started)
	ms := int(elapsed / time.Millisecond)
	if ms < 1 {
		return 1
	}
	return ms
}

func normalizeDNSType(value string) string {
	switch value {
	case "doh", "https":
		return "https"
	case "dot", "tls":
		return "tls"
	case "doq", "quic":
		return "quic"
	case "tcp":
		return "tcp"
	default:
		return "udp"
	}
}

func parseDNSAddress(address string, dnsType string) (string, int) {
	if address == "" {
		return "", defaultDNSPort(dnsType)
	}
	parsed, err := url.Parse(address)
	if err == nil && parsed.Scheme != "" {
		host := parsed.Hostname()
		port := parseOptionalPort(parsed.Port())
		if host == "" && parsed.Opaque != "" {
			host, port = splitDNSHostPort(parsed.Opaque)
		}
		return host, valueOrDefaultPort(port, defaultDNSPort(dnsType))
	}
	host, port := splitDNSHostPort(address)
	return host, valueOrDefaultPort(port, defaultDNSPort(dnsType))
}

func splitDNSHostPort(address string) (string, int) {
	host, portText, err := net.SplitHostPort(address)
	if err == nil {
		return host, parseOptionalPort(portText)
	}
	if strings.Count(address, ":") == 1 {
		host, portText, found := strings.Cut(address, ":")
		if found {
			return host, parseOptionalPort(portText)
		}
	}
	return address, 0
}

func parseOptionalPort(value string) int {
	if value == "" {
		return 0
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return port
}

func valueOrDefaultPort(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func defaultDNSPort(dnsType string) int {
	switch dnsType {
	case "https":
		return 443
	case "tls", "quic":
		return 853
	default:
		return 53
	}
}
