package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

var errUnsupportedTransport = errors.New("unsupported transport")

type singBoxConfig struct {
	Log          logConfig           `json:"log"`
	DNS          *dnsConfig          `json:"dns,omitempty"`
	Inbounds     []map[string]any    `json:"inbounds"`
	Outbounds    []map[string]any    `json:"outbounds"`
	Route        routeConfig         `json:"route"`
	Experimental *experimentalConfig `json:"experimental,omitempty"`
}

// experimentalConfig enables sing-box's on-disk cache. The cache stores
// downloaded rule-sets so that, once fetched, they survive restarts and the
// daemon does not depend on reaching the network at every startup.
type experimentalConfig struct {
	CacheFile cacheFileConfig `json:"cache_file"`
}

type cacheFileConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path,omitempty"`
}

// CacheFilePath is where sing-box persists its cache (downloaded rule-sets,
// FakeIP mappings, etc.).
const CacheFilePath = "/etc/singbox-manager/cache.db"

type logConfig struct {
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp"`
}

type dnsConfig struct {
	Servers []map[string]any `json:"servers,omitempty"`
	Rules   []map[string]any `json:"rules,omitempty"`
	Final   string           `json:"final,omitempty"`
}

type routeConfig struct {
	Rules                 []map[string]any `json:"rules,omitempty"`
	RuleSet               []map[string]any `json:"rule_set,omitempty"`
	Final                 string           `json:"final"`
	AutoDetectInterface   bool             `json:"auto_detect_interface"`
	DefaultDomainResolver string           `json:"default_domain_resolver,omitempty"`
}

func Render(cfg managerconfig.Config) ([]byte, error) {
	outbounds, proxyTag, err := renderOutbounds(cfg)
	if err != nil {
		return nil, err
	}
	resolvers := domainResolversForActiveGroup(cfg, proxyTag)
	applyOutboundDomainResolvers(outbounds, resolvers)

	document := singBoxConfig{
		Log: logConfig{
			Level:     cfg.Manager.LogLevel,
			Timestamp: true,
		},
		DNS:       renderDNS(cfg),
		Inbounds:  renderInbounds(cfg),
		Outbounds: outbounds,
		Route:     renderRoute(cfg, proxyTag, resolvers),
		Experimental: &experimentalConfig{
			CacheFile: cacheFileConfig{Enabled: true, Path: CacheFilePath},
		},
	}

	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func renderInbounds(cfg managerconfig.Config) []map[string]any {
	inbounds := []map[string]any{
		{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      cfg.Manager.MixedListen,
			"listen_port": cfg.Manager.MixedPort,
			"sniff":       true,
		},
	}

	if cfg.UsesTProxy() {
		inbounds = append(inbounds, map[string]any{
			"type":        "tproxy",
			"tag":         "tproxy-in",
			"listen":      "::",
			"listen_port": cfg.Manager.TProxyPort,
			"sniff":       true,
		})
	}

	if cfg.TUN.Enabled {
		addresses := []string{}
		if cfg.TUN.Inet4Address != "" {
			addresses = append(addresses, cfg.TUN.Inet4Address)
		}
		if cfg.TUN.Inet6Address != "" {
			addresses = append(addresses, cfg.TUN.Inet6Address)
		}
		inbounds = append(inbounds, map[string]any{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": "singbox0",
			"auto_route":     cfg.TUN.AutoRoute,
			"auto_redirect":  cfg.TUN.AutoRedirect,
			"sniff":          true,
			"address":        addresses,
		})
	}

	return inbounds
}

func renderOutbounds(cfg managerconfig.Config) ([]map[string]any, string, error) {
	outbounds := []map[string]any{
		{"type": "direct", "tag": "direct"},
		{"type": "block", "tag": "block"},
	}

	group := cfg.ActiveGroup()
	if group == nil || !group.Enabled {
		outbounds = append(outbounds, fallbackSelector())
		return outbounds, "proxy", nil
	}

	switch group.Strategy {
	case "selector":
		strategyOutbounds, err := renderStrategyNodes(cfg, *group)
		if err != nil {
			return nil, "", err
		}
		outbounds = append(outbounds, strategyOutbounds.nodes...)
		outbounds = append(outbounds, renderSelector(strategyOutbounds.tags, selectedTag(cfg, *group, strategyOutbounds.tags)))
	case "urltest", "load-balance":
		strategyOutbounds, err := renderStrategyNodes(cfg, *group)
		if err != nil {
			return nil, "", err
		}
		outbounds = append(outbounds, strategyOutbounds.nodes...)
		outbounds = append(outbounds, renderURLTest(strategyOutbounds.tags))
	default:
		node := cfg.ActiveNode()
		if node == nil || !node.Enabled {
			outbounds = append(outbounds, fallbackSelector())
			return outbounds, "proxy", nil
		}
		nodeOutbound, err := renderNodeOutbound(*node)
		if err != nil {
			// A selected node whose transport sing-box cannot render (e.g.
			// xhttp/splithttp) must not abort the whole render: that would block
			// every reload, including independent firewall/transparent changes.
			// Fall back to a direct selector, mirroring renderStrategyNodes which
			// already skips unsupported nodes.
			if errors.Is(err, errUnsupportedTransport) {
				outbounds = append(outbounds, fallbackSelector())
				return outbounds, "proxy", nil
			}
			return nil, "", err
		}
		nodeTag := stringValue(nodeOutbound["tag"])
		outbounds = append(outbounds, nodeOutbound)
		outbounds = append(outbounds, renderSelector([]string{nodeTag}, nodeTag))
	}
	return outbounds, "proxy", nil
}

type strategyOutbounds struct {
	nodes []map[string]any
	tags  []string
}

func renderStrategyNodes(cfg managerconfig.Config, group managerconfig.Group) (strategyOutbounds, error) {
	candidates := groupNodes(cfg, group)
	if len(candidates) == 0 {
		return strategyOutbounds{}, nil
	}

	seen := map[string]bool{
		"direct": true,
		"block":  true,
		"proxy":  true,
	}
	result := strategyOutbounds{}
	for _, node := range candidates {
		outbound, err := renderNodeOutbound(node)
		if err != nil {
			if errors.Is(err, errUnsupportedTransport) {
				continue
			}
			return strategyOutbounds{}, err
		}
		tag := stringValue(outbound["tag"])
		if tag == "" {
			return strategyOutbounds{}, fmt.Errorf("node %q rendered without an outbound tag", node.ID)
		}
		if seen[tag] {
			return strategyOutbounds{}, fmt.Errorf("node %q uses duplicate outbound tag %q", node.ID, tag)
		}
		seen[tag] = true
		result.nodes = append(result.nodes, outbound)
		result.tags = append(result.tags, tag)
	}
	return result, nil
}

func groupNodes(cfg managerconfig.Config, group managerconfig.Group) []managerconfig.Node {
	subscriptions := map[string]bool{}
	for _, id := range group.Subscriptions {
		subscriptions[id] = true
	}

	nodes := make([]managerconfig.Node, 0, len(cfg.Nodes))
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

func sortNodes(nodes []managerconfig.Node) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j-1].ID > nodes[j].ID; j-- {
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
		}
	}
}

func selectedTag(cfg managerconfig.Config, group managerconfig.Group, tags []string) string {
	if group.SelectedNode != "" {
		if node, ok := cfg.Nodes[group.SelectedNode]; ok {
			tag := node.Tag
			if tag == "" {
				tag = node.ID
			}
			for _, candidate := range tags {
				if candidate == tag {
					return tag
				}
			}
		}
	}
	if len(tags) > 0 {
		return tags[0]
	}
	return "direct"
}

func fallbackSelector() map[string]any {
	return renderSelector(nil, "direct")
}

func renderSelector(tags []string, selected string) map[string]any {
	outboundTags := append([]string{}, tags...)
	outboundTags = append(outboundTags, "direct")
	if selected == "" {
		selected = outboundTags[0]
	}
	return map[string]any{
		"type":      "selector",
		"tag":       "proxy",
		"outbounds": outboundTags,
		"default":   selected,
	}
}

func renderURLTest(tags []string) map[string]any {
	if len(tags) == 0 {
		return fallbackSelector()
	}
	return map[string]any{
		"type":      "urltest",
		"tag":       "proxy",
		"outbounds": tags,
		"url":       "https://www.gstatic.com/generate_204",
		"interval":  "3m",
		"tolerance": 50,
	}
}

func renderNodeOutbound(node managerconfig.Node) (map[string]any, error) {
	tag := node.Tag
	if tag == "" {
		tag = node.ID
	}
	server := node.Server
	if server == "" {
		server = node.Address
	}

	switch node.Type {
	case "direct":
		return map[string]any{"type": "direct", "tag": tag}, nil
	case "shadowsocks":
		return map[string]any{
			"type":        "shadowsocks",
			"tag":         tag,
			"server":      server,
			"server_port": node.Port,
			"method":      node.Method,
			"password":    node.Password,
		}, nil
	case "trojan":
		outbound := map[string]any{
			"type":        "trojan",
			"tag":         tag,
			"server":      server,
			"server_port": node.Port,
			"password":    node.Password,
		}
		addTLS(outbound, node)
		if err := addTransport(outbound, node); err != nil {
			return nil, err
		}
		return outbound, nil
	case "vmess":
		outbound := map[string]any{
			"type":        "vmess",
			"tag":         tag,
			"server":      server,
			"server_port": node.Port,
			"uuid":        node.UUID,
		}
		if node.Security != "" {
			outbound["security"] = node.Security
		}
		addTLS(outbound, node)
		if err := addTransport(outbound, node); err != nil {
			return nil, err
		}
		return outbound, nil
	case "vless":
		outbound := map[string]any{
			"type":        "vless",
			"tag":         tag,
			"server":      server,
			"server_port": node.Port,
			"uuid":        node.UUID,
		}
		if node.Flow != "" {
			outbound["flow"] = node.Flow
		}
		addTLS(outbound, node)
		if err := addTransport(outbound, node); err != nil {
			return nil, err
		}
		return outbound, nil
	case "hysteria2":
		outbound := map[string]any{
			"type":        "hysteria2",
			"tag":         tag,
			"server":      server,
			"server_port": node.Port,
			"password":    node.Password,
		}
		addTLS(outbound, node)
		return outbound, nil
	case "tuic":
		outbound := map[string]any{
			"type":        "tuic",
			"tag":         tag,
			"server":      server,
			"server_port": node.Port,
			"uuid":        node.UUID,
			"password":    node.Password,
		}
		if node.Congestion != "" {
			outbound["congestion_control"] = node.Congestion
		}
		if node.UDPRelayMode != "" {
			outbound["udp_relay_mode"] = node.UDPRelayMode
		}
		addTLS(outbound, node)
		return outbound, nil
	default:
		return nil, fmt.Errorf("node %q type %q is not renderable", node.ID, node.Type)
	}
}

func addTLS(outbound map[string]any, node managerconfig.Node) {
	if !node.TLS && node.Security != "tls" && node.Security != "reality" && node.ALPN == "" && !node.Insecure {
		return
	}
	tls := map[string]any{"enabled": true}
	if node.SNI != "" {
		tls["server_name"] = node.SNI
	}
	if node.ALPN != "" {
		tls["alpn"] = splitCSV(node.ALPN)
	}
	if node.Insecure {
		tls["insecure"] = true
	}
	outbound["tls"] = tls
}

func addTransport(outbound map[string]any, node managerconfig.Node) error {
	if node.Transport == "" || node.Transport == "tcp" {
		return nil
	}
	transport := map[string]any{"type": node.Transport}
	switch node.Transport {
	case "httpupgrade":
		if node.Host != "" {
			transport["host"] = node.Host
		}
		if node.Path != "" {
			transport["path"] = node.Path
		}
	case "ws":
		if node.Path != "" {
			transport["path"] = node.Path
		}
		if node.Host != "" {
			transport["headers"] = map[string]any{"Host": node.Host}
		}
	case "grpc":
		if node.Path != "" {
			transport["service_name"] = strings.TrimPrefix(node.Path, "/")
		}
	case "http":
		if node.Host != "" {
			transport["host"] = []string{node.Host}
		}
		if node.Path != "" {
			transport["path"] = node.Path
		}
	default:
		// sing-box only implements ws, grpc, http, httpupgrade, and quic V2Ray
		// transports. xhttp/splithttp are Xray-only and have no sing-box
		// equivalent, so a node using them cannot be rendered.
		return fmt.Errorf("%w %q for node %q: sing-box supports ws, grpc, http, and httpupgrade; xhttp/splithttp are not supported", errUnsupportedTransport, node.Transport, node.ID)
	}
	outbound["transport"] = transport
	return nil
}

func splitCSV(value string) []string {
	var values []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func renderDNS(cfg managerconfig.Config) *dnsConfig {
	servers := enabledDNSServers(cfg)
	if len(servers) == 0 {
		return nil
	}

	dns := &dnsConfig{}
	for _, server := range servers {
		dns.Servers = append(dns.Servers, renderDNSServer(server))
	}

	if group := cfg.ActiveGroup(); group != nil {
		for _, rule := range cfg.DNSRulesForGroup(group.ID) {
			if server, ok := cfg.DNSServers[rule.Server]; !ok || !server.Enabled {
				continue
			}
			sources := normalizeSourceCIDRs(rule.Sources)
			ruleSets := enabledRuleSetIDs(cfg, rule.RuleSets)
			if len(sources) == 0 && len(ruleSets) == 0 {
				continue
			}
			entry := map[string]any{}
			if len(sources) > 0 {
				entry["source_ip_cidr"] = sources
			}
			if len(ruleSets) > 0 {
				entry["rule_set"] = ruleSets
			}
			entry["server"] = rule.Server
			dns.Rules = append(dns.Rules, entry)
		}
	}

	dns.Final = defaultResolver(cfg, servers)
	return dns
}

// enabledDNSServers returns every enabled DNS server, ordered by ID so the
// rendered server list and the default resolver are deterministic.
func enabledDNSServers(cfg managerconfig.Config) []managerconfig.DNSServer {
	servers := make([]managerconfig.DNSServer, 0, len(cfg.DNSServers))
	for _, server := range cfg.DNSServers {
		if server.Enabled {
			servers = append(servers, server)
		}
	}
	for i := 1; i < len(servers); i++ {
		for j := i; j > 0 && servers[j-1].ID > servers[j].ID; j-- {
			servers[j-1], servers[j] = servers[j], servers[j-1]
		}
	}
	return servers
}

// defaultResolver picks the group's preferred resolver, falling back to the
// first enabled server.
func defaultResolver(cfg managerconfig.Config, servers []managerconfig.DNSServer) string {
	if group := cfg.ActiveGroup(); group != nil && group.DNSFinal != "" {
		if server, ok := cfg.DNSServers[group.DNSFinal]; ok && server.Enabled {
			return group.DNSFinal
		}
	}
	if len(servers) > 0 {
		return servers[0].ID
	}
	return ""
}

type domainResolvers struct {
	defaultResolver string
	byOutbound      map[string]string
}

// domainResolversForActiveGroup maps each outbound to the DNS server that
// resolves domains for connections leaving via that outbound. A server's
// detour ("direct"/"proxy") determines which outbound it serves.
func domainResolversForActiveGroup(cfg managerconfig.Config, proxyTag string) domainResolvers {
	servers := enabledDNSServers(cfg)
	resolvers := domainResolvers{
		defaultResolver: defaultResolver(cfg, servers),
		byOutbound:      map[string]string{},
	}
	for _, server := range servers {
		outbound := server.Detour
		if outbound == "" {
			continue
		}
		if outbound == "proxy" {
			outbound = proxyTag
		}
		if _, exists := resolvers.byOutbound[outbound]; !exists {
			resolvers.byOutbound[outbound] = server.ID
		}
	}
	return resolvers
}

// enabledRuleSetIDs filters a list of rule-set references to those that exist
// and are enabled, preserving order.
func enabledRuleSetIDs(cfg managerconfig.Config, ids []string) []string {
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if ruleset, ok := cfg.RuleSets[id]; ok && ruleset.Enabled {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

func applyOutboundDomainResolvers(outbounds []map[string]any, resolvers domainResolvers) {
	for _, outbound := range outbounds {
		tag := stringValue(outbound["tag"])
		resolver := resolvers.byOutbound[tag]
		if resolver == "" || !supportsDomainResolver(outbound) {
			continue
		}
		outbound["domain_resolver"] = resolver
	}
}

func supportsDomainResolver(outbound map[string]any) bool {
	switch stringValue(outbound["type"]) {
	case "selector", "urltest", "block":
		return false
	default:
		return true
	}
}

func renderDNSServer(server managerconfig.DNSServer) map[string]any {
	dnsType := normalizeDNSType(server.Type)
	host, port, path := parseDNSAddress(server.Address, dnsType)
	entry := map[string]any{
		"type": dnsType,
		"tag":  server.ID,
	}
	if host != "" {
		entry["server"] = host
	}
	if port > 0 {
		entry["server_port"] = port
	}
	if path != "" && dnsType == "https" {
		entry["path"] = path
	}
	if dnsType == "https" || dnsType == "tls" || dnsType == "quic" {
		entry["tls"] = map[string]any{"enabled": true}
	}
	if server.Detour != "" {
		entry["detour"] = server.Detour
	}
	return entry
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

func parseDNSAddress(address string, dnsType string) (string, int, string) {
	if address == "" {
		return "", 0, ""
	}
	parsed, err := url.Parse(address)
	if err == nil && parsed.Scheme != "" {
		host := parsed.Hostname()
		port := parseOptionalPort(parsed.Port())
		if host == "" && parsed.Opaque != "" {
			host, port = splitDNSHostPort(parsed.Opaque)
		}
		return host, valueOrDefaultPort(port, defaultDNSPort(dnsType)), parsed.EscapedPath()
	}
	host, port := splitDNSHostPort(address)
	return host, valueOrDefaultPort(port, defaultDNSPort(dnsType)), ""
}

func splitDNSHostPort(address string) (string, int) {
	host, portText, err := net.SplitHostPort(address)
	if err == nil {
		return host, parseOptionalPort(portText)
	}
	if strings.Count(address, ":") == 1 {
		host, portText, err := net.SplitHostPort(address)
		if err == nil {
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

func renderRoute(cfg managerconfig.Config, proxyTag string, resolvers domainResolvers) routeConfig {
	route := routeConfig{
		Final:                 routeFinal(cfg, proxyTag),
		AutoDetectInterface:   true,
		DefaultDomainResolver: resolvers.defaultResolver,
	}

	if dnsHijackEnabled(cfg) {
		route.Rules = append(route.Rules, map[string]any{
			"protocol": "dns",
			"action":   "hijack-dns",
		})
	}

	group := cfg.ActiveGroup()
	if group == nil {
		return route
	}

	used := map[string]bool{}
	if cfg.Manager.RuntimeMode == "rule" {
		for _, rule := range cfg.RouteRulesForGroup(group.ID) {
			sources := normalizeSourceCIDRs(rule.Sources)
			ruleSets := enabledRuleSetIDs(cfg, rule.RuleSets)
			if len(sources) == 0 && len(ruleSets) == 0 {
				continue
			}
			entry := map[string]any{}
			if len(sources) > 0 {
				entry["source_ip_cidr"] = sources
			}
			if len(ruleSets) > 0 {
				entry["rule_set"] = ruleSets
				for _, id := range ruleSets {
					used[id] = true
				}
			}
			outbound := rule.Outbound
			if outbound == "proxy" || outbound == "" {
				outbound = proxyTag
			}
			entry["outbound"] = outbound
			route.Rules = append(route.Rules, entry)
		}
	}

	// DNS rules render regardless of route mode, so their rule sets must be
	// defined too. Collect every rule set referenced by the active group.
	for _, rule := range cfg.DNSRulesForGroup(group.ID) {
		for _, id := range enabledRuleSetIDs(cfg, rule.RuleSets) {
			used[id] = true
		}
	}

	for _, id := range sortedKeys(used) {
		ruleset, ok := cfg.RuleSets[id]
		if !ok || !ruleset.Enabled {
			continue
		}
		if entry := renderRuleSet(ruleset, proxyTag); entry != nil {
			route.RuleSet = append(route.RuleSet, entry)
		}
	}

	return route
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

func normalizeSourceCIDRs(sources []string) []string {
	cidrs := make([]string, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if strings.Contains(source, "/") {
			cidrs = append(cidrs, source)
			continue
		}
		if ip := net.ParseIP(source); ip != nil && ip.To4() != nil {
			cidrs = append(cidrs, source+"/32")
			continue
		}
		cidrs = append(cidrs, source+"/128")
	}
	return cidrs
}

func dnsHijackEnabled(cfg managerconfig.Config) bool {
	return cfg.Transparent.Active() && cfg.Transparent.DNSHijack
}

func routeFinal(cfg managerconfig.Config, proxyTag string) string {
	switch cfg.Manager.RuntimeMode {
	case "direct":
		return "direct"
	case "global":
		return proxyTag
	}

	group := cfg.ActiveGroup()
	if group == nil || group.RouteFinal == "" || group.RouteFinal == "proxy" {
		return proxyTag
	}
	return group.RouteFinal
}

func renderRuleSet(ruleset managerconfig.RuleSet, downloadDetour string) map[string]any {
	format := "binary"
	if ruleset.Format == "source" {
		format = "source"
	}

	if ruleset.Path != "" && fileExists(ruleset.Path) {
		return map[string]any{
			"type":   "local",
			"tag":    ruleset.ID,
			"format": format,
			"path":   ruleset.Path,
		}
	}

	if ruleset.Type == "remote" && ruleset.URL != "" {
		// Fetch remote rule-sets through the proxy. Their hosts (e.g.
		// raw.githubusercontent.com) are frequently censored on the same
		// networks where this manager runs, so a "direct" download reliably
		// fails and takes sing-box startup down with it. Combined with the
		// cache_file, the first successful fetch is reused on later boots.
		if downloadDetour == "" {
			downloadDetour = "proxy"
		}
		entry := map[string]any{
			"type":            "remote",
			"tag":             ruleset.ID,
			"format":          format,
			"url":             ruleset.URL,
			"download_detour": downloadDetour,
		}
		if ruleset.UpdateInterval != "" {
			entry["update_interval"] = ruleset.UpdateInterval
		}
		return entry
	}
	if ruleset.Type == "remote" {
		return nil
	}
	if ruleset.Path != "" {
		return map[string]any{
			"type":   "local",
			"tag":    ruleset.ID,
			"format": format,
			"path":   ruleset.Path,
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func stringValue(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}
