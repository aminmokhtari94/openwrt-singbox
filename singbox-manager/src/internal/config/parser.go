package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type section struct {
	typ     string
	name    string
	line    int
	options map[string]string
	lists   map[string][]string
}

func Load(path string) (*Config, error) {
	cfg, err := LoadUnvalidated(path)
	if err != nil {
		return nil, err
	}
	if err := Validate(*cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadUnvalidated(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	sections, err := parseUCI(string(data))
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := applySections(&cfg, sections); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func parseUCI(data string) ([]section, error) {
	var sections []section
	var current *section

	for i, raw := range strings.Split(data, "\n") {
		lineNo := i + 1
		tokens, err := tokenizeUCI(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if len(tokens) == 0 {
			continue
		}

		switch tokens[0] {
		case "config":
			if len(tokens) != 2 && len(tokens) != 3 {
				return nil, fmt.Errorf("line %d: config expects type and optional name", lineNo)
			}
			name := ""
			if len(tokens) == 3 {
				name = tokens[2]
			}
			sections = append(sections, section{
				typ:     tokens[1],
				name:    name,
				line:    lineNo,
				options: map[string]string{},
				lists:   map[string][]string{},
			})
			current = &sections[len(sections)-1]
		case "option":
			if current == nil {
				return nil, fmt.Errorf("line %d: option without config section", lineNo)
			}
			if len(tokens) != 3 {
				return nil, fmt.Errorf("line %d: option expects key and value", lineNo)
			}
			if _, exists := current.options[tokens[1]]; exists {
				return nil, fmt.Errorf("line %d: duplicate option %q in %s %q", lineNo, tokens[1], current.typ, current.name)
			}
			current.options[tokens[1]] = tokens[2]
		case "list":
			if current == nil {
				return nil, fmt.Errorf("line %d: list without config section", lineNo)
			}
			if len(tokens) != 3 {
				return nil, fmt.Errorf("line %d: list expects key and value", lineNo)
			}
			current.lists[tokens[1]] = append(current.lists[tokens[1]], tokens[2])
		default:
			return nil, fmt.Errorf("line %d: unsupported UCI directive %q", lineNo, tokens[0])
		}
	}

	return sections, nil
}

func tokenizeUCI(raw string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	var quote rune
	escaped := false
	inToken := false

	for _, ch := range raw {
		if escaped {
			current.WriteRune(ch)
			inToken = true
			escaped = false
			continue
		}

		if quote != 0 {
			if quote == '"' && ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
				continue
			}
			current.WriteRune(ch)
			inToken = true
			continue
		}

		switch {
		case ch == '#':
			if inToken {
				current.WriteRune(ch)
			}
			goto finish
		case ch == '\'' || ch == '"':
			quote = ch
			inToken = true
		case ch == ' ' || ch == '\t' || ch == '\r':
			if inToken {
				tokens = append(tokens, current.String())
				current.Reset()
				inToken = false
			}
		default:
			current.WriteRune(ch)
			inToken = true
		}
	}

finish:
	if escaped {
		return nil, fmt.Errorf("dangling escape")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if inToken {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

func applySections(cfg *Config, sections []section) error {
	seenManager := false

	for _, sec := range sections {
		switch sec.typ {
		case "manager":
			if sec.name != "main" {
				return fmt.Errorf("line %d: unsupported manager section %q", sec.line, sec.name)
			}
			if seenManager {
				return fmt.Errorf("line %d: duplicate manager main section", sec.line)
			}
			seenManager = true
			if err := applyManager(cfg, sec); err != nil {
				return err
			}
		case "group":
			if sec.name == "" {
				return fmt.Errorf("line %d: group section requires a name", sec.line)
			}
			if _, exists := cfg.Groups[sec.name]; exists {
				return fmt.Errorf("line %d: duplicate group %q", sec.line, sec.name)
			}
			group, err := readGroup(sec)
			if err != nil {
				return err
			}
			cfg.Groups[group.ID] = group
		case "subscription":
			subscription, err := readSubscription(sec)
			if err != nil {
				return err
			}
			cfg.Subscriptions[subscription.ID] = subscription
		case "node":
			node, err := readNode(sec)
			if err != nil {
				return err
			}
			cfg.Nodes[node.ID] = node
		case "dns_server":
			server, err := readDNSServer(sec)
			if err != nil {
				return err
			}
			cfg.DNSServers[server.ID] = server
		case "dns_rule":
			rule, err := readDNSRule(sec)
			if err != nil {
				return err
			}
			cfg.DNSRules[rule.ID] = rule
		case "route_rule":
			rule, err := readRouteRule(sec)
			if err != nil {
				return err
			}
			cfg.RouteRules[rule.ID] = rule
		case "ruleset":
			ruleset, err := readRuleSet(sec)
			if err != nil {
				return err
			}
			cfg.RuleSets[ruleset.ID] = ruleset
		case "transparent":
			if sec.name != "main" && sec.name != "transparent" {
				return fmt.Errorf("line %d: unsupported transparent section %q", sec.line, sec.name)
			}
			devices := cfg.Transparent.Devices
			transparent, err := readTransparent(sec)
			if err != nil {
				return err
			}
			transparent.Devices = devices
			cfg.Transparent = transparent
		case "proxy_device":
			if sec.name == "" {
				return fmt.Errorf("line %d: proxy_device section requires a name", sec.line)
			}
			device, err := readProxyDevice(sec)
			if err != nil {
				return err
			}
			cfg.Transparent.Devices = append(cfg.Transparent.Devices, device)
		case "tun":
			if sec.name != "main" && sec.name != "tun" {
				return fmt.Errorf("line %d: unsupported tun section %q", sec.line, sec.name)
			}
			tun, err := readTUN(sec)
			if err != nil {
				return err
			}
			cfg.TUN = tun
		default:
			return fmt.Errorf("line %d: unsupported section type %q", sec.line, sec.typ)
		}
	}

	return nil
}

func applyManager(cfg *Config, sec section) error {
	allowed := map[string]bool{
		"enabled": true, "log_level": true, "active_group": true, "runtime_mode": true,
		"sing_box_bin": true, "socket_path": true, "api_listen": true,
		"mixed_listen": true, "mixed_port": true, "tproxy_port": true, "dns_port": true,
		"update_interval": true,
	}
	if err := rejectUnknownOptions(sec, allowed); err != nil {
		return err
	}
	if err := rejectUnknownLists(sec, nil); err != nil {
		return err
	}

	var err error
	for key, value := range sec.options {
		switch key {
		case "enabled":
			cfg.Manager.Enabled, err = parseBool(sec, key, value)
		case "log_level":
			cfg.Manager.LogLevel = value
		case "active_group":
			cfg.Manager.ActiveGroup = value
		case "runtime_mode":
			cfg.Manager.RuntimeMode = value
		case "sing_box_bin":
			cfg.Manager.SingBoxBinary = value
		case "socket_path":
			cfg.Manager.SocketPath = value
		case "api_listen":
			cfg.Manager.APIListen = value
		case "mixed_listen":
			cfg.Manager.MixedListen = value
		case "mixed_port":
			cfg.Manager.MixedPort, err = parsePort(sec, key, value)
		case "tproxy_port":
			cfg.Manager.TProxyPort, err = parsePort(sec, key, value)
		case "dns_port":
			cfg.Manager.DNSPort, err = parsePort(sec, key, value)
		case "update_interval":
			cfg.Manager.UpdateInterval = value
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func readGroup(sec section) (Group, error) {
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "name": true, "route_final": true, "dns_final": true,
		"strategy": true, "selected_node": true,
		"health": true, "latency_ms": true, "last_check": true,
	}); err != nil {
		return Group{}, err
	}
	if err := rejectUnknownLists(sec, map[string]bool{"subscription": true}); err != nil {
		return Group{}, err
	}

	group := Group{
		ID:            sec.name,
		Enabled:       true,
		Name:          sec.name,
		Subscriptions: cleanList(sec.lists["subscription"]),
		Strategy:      valueOrDefault(sec.options["strategy"], "manual"),
		SelectedNode:  sec.options["selected_node"],
		RouteFinal:    valueOrDefault(sec.options["route_final"], "proxy"),
		DNSFinal:      sec.options["dns_final"],
		Health:        valueOrDefault(sec.options["health"], "unknown"),
		LastCheck:     sec.options["last_check"],
	}
	if value, ok := sec.options["latency_ms"]; ok {
		latency, err := parseNonNegativeInt(sec, "latency_ms", value)
		if err != nil {
			return Group{}, err
		}
		group.LatencyMS = latency
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return Group{}, err
		}
		group.Enabled = enabled
	}
	if name := sec.options["name"]; name != "" {
		group.Name = name
	}
	return group, nil
}

func readSubscription(sec section) (Subscription, error) {
	if sec.name == "" {
		return Subscription{}, fmt.Errorf("line %d: subscription section requires a name", sec.line)
	}
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "name": true, "url": true, "format": true, "update_interval": true,
		"last_update": true, "last_error": true, "health": true, "latency_ms": true, "last_check": true,
	}); err != nil {
		return Subscription{}, err
	}
	if err := rejectUnknownLists(sec, nil); err != nil {
		return Subscription{}, err
	}

	subscription := Subscription{
		ID:             sec.name,
		Enabled:        false,
		Name:           sec.name,
		URL:            sec.options["url"],
		Format:         valueOrDefault(sec.options["format"], "auto"),
		UpdateInterval: valueOrDefault(sec.options["update_interval"], "24h"),
		LastUpdate:     sec.options["last_update"],
		LastError:      sec.options["last_error"],
		Health:         valueOrDefault(sec.options["health"], "unknown"),
		LastCheck:      sec.options["last_check"],
	}
	if value, ok := sec.options["latency_ms"]; ok {
		latency, err := parseNonNegativeInt(sec, "latency_ms", value)
		if err != nil {
			return Subscription{}, err
		}
		subscription.LatencyMS = latency
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return Subscription{}, err
		}
		subscription.Enabled = enabled
	}
	if name := sec.options["name"]; name != "" {
		subscription.Name = name
	}
	return subscription, nil
}

func readNode(sec section) (Node, error) {
	if sec.name == "" {
		return Node{}, fmt.Errorf("line %d: node section requires a name", sec.line)
	}
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "name": true, "type": true, "address": true, "server": true,
		"port": true, "uuid": true, "password": true, "method": true, "security": true,
		"tls": true, "flow": true, "transport": true, "host": true, "path": true, "sni": true,
		"alpn": true, "insecure": true, "congestion": true, "udp_relay_mode": true,
		"tag": true, "subscription": true, "health": true, "latency_ms": true, "last_check": true,
	}); err != nil {
		return Node{}, err
	}
	if err := rejectUnknownLists(sec, nil); err != nil {
		return Node{}, err
	}

	node := Node{
		ID:           sec.name,
		Enabled:      true,
		Name:         valueOrDefault(sec.options["name"], sec.name),
		Type:         sec.options["type"],
		Address:      sec.options["address"],
		Server:       sec.options["server"],
		UUID:         sec.options["uuid"],
		Password:     sec.options["password"],
		Method:       sec.options["method"],
		Security:     sec.options["security"],
		TLS:          false,
		Flow:         sec.options["flow"],
		Transport:    sec.options["transport"],
		Host:         sec.options["host"],
		Path:         sec.options["path"],
		SNI:          sec.options["sni"],
		ALPN:         sec.options["alpn"],
		Congestion:   sec.options["congestion"],
		UDPRelayMode: sec.options["udp_relay_mode"],
		Tag:          valueOrDefault(sec.options["tag"], sec.name),
		Subscription: sec.options["subscription"],
		Health:       valueOrDefault(sec.options["health"], "unknown"),
		LastCheck:    sec.options["last_check"],
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return Node{}, err
		}
		node.Enabled = enabled
	}
	if value, ok := sec.options["port"]; ok {
		port, err := parsePort(sec, "port", value)
		if err != nil {
			return Node{}, err
		}
		node.Port = port
	}
	if value, ok := sec.options["tls"]; ok {
		tls, err := parseBool(sec, "tls", value)
		if err != nil {
			return Node{}, err
		}
		node.TLS = tls
	}
	if value, ok := sec.options["insecure"]; ok {
		insecure, err := parseBool(sec, "insecure", value)
		if err != nil {
			return Node{}, err
		}
		node.Insecure = insecure
	}
	if value, ok := sec.options["latency_ms"]; ok {
		latency, err := parseNonNegativeInt(sec, "latency_ms", value)
		if err != nil {
			return Node{}, err
		}
		node.LatencyMS = latency
	}
	return node, nil
}

func readDNSServer(sec section) (DNSServer, error) {
	if sec.name == "" {
		return DNSServer{}, fmt.Errorf("line %d: dns_server section requires a name", sec.line)
	}
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "name": true, "type": true, "address": true, "detour": true,
	}); err != nil {
		return DNSServer{}, err
	}
	if err := rejectUnknownLists(sec, nil); err != nil {
		return DNSServer{}, err
	}

	server := DNSServer{
		ID:      sec.name,
		Enabled: true,
		Name:    valueOrDefault(sec.options["name"], sec.name),
		Type:    valueOrDefault(sec.options["type"], "udp"),
		Address: sec.options["address"],
		Detour:  sec.options["detour"],
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return DNSServer{}, err
		}
		server.Enabled = enabled
	}
	return server, nil
}

func readRuleSet(sec section) (RuleSet, error) {
	if sec.name == "" {
		return RuleSet{}, fmt.Errorf("line %d: ruleset section requires a name", sec.line)
	}
	if err := rejectUnknownOptions(sec, map[string]bool{
		"id": true, "enabled": true, "name": true, "type": true, "format": true, "url": true,
		"path": true, "update_interval": true,
		"last_update": true, "last_error": true,
	}); err != nil {
		return RuleSet{}, err
	}
	if err := rejectUnknownLists(sec, nil); err != nil {
		return RuleSet{}, err
	}

	ruleset := RuleSet{
		ID:             valueOrDefault(sec.options["id"], sec.name),
		Enabled:        true,
		Name:           valueOrDefault(sec.options["name"], sec.name),
		Type:           valueOrDefault(sec.options["type"], "local"),
		Format:         valueOrDefault(sec.options["format"], "srs"),
		URL:            sec.options["url"],
		Path:           sec.options["path"],
		UpdateInterval: valueOrDefault(sec.options["update_interval"], "168h"),
		LastUpdate:     sec.options["last_update"],
		LastError:      sec.options["last_error"],
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return RuleSet{}, err
		}
		ruleset.Enabled = enabled
	}
	return ruleset, nil
}

func readDNSRule(sec section) (DNSRule, error) {
	if sec.name == "" {
		return DNSRule{}, fmt.Errorf("line %d: dns_rule section requires a name", sec.line)
	}
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "name": true, "group": true, "server": true,
	}); err != nil {
		return DNSRule{}, err
	}
	if err := rejectUnknownLists(sec, map[string]bool{"source_ip": true, "ruleset": true}); err != nil {
		return DNSRule{}, err
	}

	rule := DNSRule{
		ID:       sec.name,
		Enabled:  true,
		Name:     valueOrDefault(sec.options["name"], sec.name),
		Group:    sec.options["group"],
		Sources:  cleanList(sec.lists["source_ip"]),
		RuleSets: cleanList(sec.lists["ruleset"]),
		Server:   sec.options["server"],
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return DNSRule{}, err
		}
		rule.Enabled = enabled
	}
	return rule, nil
}

func readRouteRule(sec section) (RouteRule, error) {
	if sec.name == "" {
		return RouteRule{}, fmt.Errorf("line %d: route_rule section requires a name", sec.line)
	}
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "name": true, "group": true, "outbound": true,
	}); err != nil {
		return RouteRule{}, err
	}
	if err := rejectUnknownLists(sec, map[string]bool{"source_ip": true, "ruleset": true}); err != nil {
		return RouteRule{}, err
	}

	rule := RouteRule{
		ID:       sec.name,
		Enabled:  true,
		Name:     valueOrDefault(sec.options["name"], sec.name),
		Group:    sec.options["group"],
		Sources:  cleanList(sec.lists["source_ip"]),
		RuleSets: cleanList(sec.lists["ruleset"]),
		Outbound: valueOrDefault(sec.options["outbound"], "proxy"),
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return RouteRule{}, err
		}
		rule.Enabled = enabled
	}
	return rule, nil
}

func readTransparent(sec section) (Transparent, error) {
	if err := rejectUnknownOptions(sec, map[string]bool{
		"default_mode": true, "dns_hijack": true, "kill_switch": true,
	}); err != nil {
		return Transparent{}, err
	}
	if err := rejectUnknownLists(sec, map[string]bool{
		"lan_ifname": true, "bypass_subnet": true,
	}); err != nil {
		return Transparent{}, err
	}

	transparent := Transparent{
		DefaultMode:  valueOrDefault(sec.options["default_mode"], "off"),
		LANIfnames:   cleanList(sec.lists["lan_ifname"]),
		BypassSubnet: cleanList(sec.lists["bypass_subnet"]),
	}
	var err error
	if value, ok := sec.options["dns_hijack"]; ok {
		transparent.DNSHijack, err = parseBool(sec, "dns_hijack", value)
		if err != nil {
			return Transparent{}, err
		}
	}
	if value, ok := sec.options["kill_switch"]; ok {
		transparent.KillSwitch, err = parseBool(sec, "kill_switch", value)
		if err != nil {
			return Transparent{}, err
		}
	}
	return transparent, nil
}

func readProxyDevice(sec section) (Device, error) {
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "name": true, "mac": true, "ipv4": true, "ipv6": true,
		"mode": true, "bypass_udp": true, "group": true,
	}); err != nil {
		return Device{}, err
	}
	if err := rejectUnknownLists(sec, nil); err != nil {
		return Device{}, err
	}

	device := Device{
		ID:      sec.name,
		Enabled: true,
		Name:    valueOrDefault(sec.options["name"], sec.name),
		MAC:     sec.options["mac"],
		IPv4:    sec.options["ipv4"],
		IPv6:    sec.options["ipv6"],
		Mode:    valueOrDefault(sec.options["mode"], "default"),
		Group:   sec.options["group"],
	}
	if value, ok := sec.options["enabled"]; ok {
		enabled, err := parseBool(sec, "enabled", value)
		if err != nil {
			return Device{}, err
		}
		device.Enabled = enabled
	}
	if value, ok := sec.options["bypass_udp"]; ok {
		bypassUDP, err := parseBool(sec, "bypass_udp", value)
		if err != nil {
			return Device{}, err
		}
		device.BypassUDP = bypassUDP
	}
	return device, nil
}

func readTUN(sec section) (TUN, error) {
	if err := rejectUnknownOptions(sec, map[string]bool{
		"enabled": true, "auto_route": true, "auto_redirect": true,
		"inet4_address": true, "inet6_address": true,
	}); err != nil {
		return TUN{}, err
	}
	if err := rejectUnknownLists(sec, nil); err != nil {
		return TUN{}, err
	}

	tun := TUN{
		AutoRoute:    true,
		AutoRedirect: true,
		Inet4Address: valueOrDefault(sec.options["inet4_address"], "172.19.0.1/30"),
		Inet6Address: valueOrDefault(sec.options["inet6_address"], "fdfe:dcba:9876::1/126"),
	}
	var err error
	if value, ok := sec.options["enabled"]; ok {
		tun.Enabled, err = parseBool(sec, "enabled", value)
		if err != nil {
			return TUN{}, err
		}
	}
	if value, ok := sec.options["auto_route"]; ok {
		tun.AutoRoute, err = parseBool(sec, "auto_route", value)
		if err != nil {
			return TUN{}, err
		}
	}
	if value, ok := sec.options["auto_redirect"]; ok {
		tun.AutoRedirect, err = parseBool(sec, "auto_redirect", value)
		if err != nil {
			return TUN{}, err
		}
	}
	return tun, nil
}

func rejectUnknownOptions(sec section, allowed map[string]bool) error {
	for key := range sec.options {
		if allowed == nil || !allowed[key] {
			return fmt.Errorf("line %d: unsupported option %q in %s %q", sec.line, key, sec.typ, sec.name)
		}
	}
	return nil
}

func rejectUnknownLists(sec section, allowed map[string]bool) error {
	for key := range sec.lists {
		if allowed == nil || !allowed[key] {
			return fmt.Errorf("line %d: unsupported list %q in %s %q", sec.line, key, sec.typ, sec.name)
		}
	}
	return nil
}

func parseBool(sec section, key string, value string) (bool, error) {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on", "enabled":
		return true, nil
	case "0", "false", "no", "off", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("line %d: option %s must be boolean, got %q", sec.line, key, value)
	}
}

func parsePort(sec section, key string, value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("line %d: option %s must be a TCP/UDP port, got %q", sec.line, key, value)
	}
	return port, nil
}

func parseNonNegativeInt(sec section, key string, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("line %d: option %s must be a non-negative integer, got %q", sec.line, key, value)
	}
	return parsed, nil
}

func cleanList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func valueOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
