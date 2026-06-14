package config

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

type ValidationError struct {
	Errors []string
}

func (err ValidationError) Error() string {
	return strings.Join(err.Errors, "; ")
}

func ErrorStrings(err error) []string {
	if err == nil {
		return nil
	}
	if validation, ok := err.(ValidationError); ok {
		return validation.Errors
	}
	return []string{err.Error()}
}

func Validate(cfg Config) error {
	var errors []string

	if cfg.Manager.ActiveGroup == "" {
		errors = append(errors, "manager.main.active_group is required")
	} else if _, ok := cfg.Groups[cfg.Manager.ActiveGroup]; !ok {
		errors = append(errors, fmt.Sprintf("manager.main.active_group references missing group %q", cfg.Manager.ActiveGroup))
	}
	if cfg.Manager.SingBoxBinary == "" {
		errors = append(errors, "manager.main.sing_box_bin is required")
	}
	if cfg.Manager.SocketPath == "" {
		errors = append(errors, "manager.main.socket_path is required")
	}
	if cfg.PAC.Enabled && cfg.Manager.PACListen == "" {
		errors = append(errors, "manager.main.pac_listen is required when pac is enabled")
	}
	if !inSet(cfg.Manager.RuntimeMode, "direct", "rule", "global") {
		errors = append(errors, fmt.Sprintf("manager.main.runtime_mode must be direct, rule, or global, got %q", cfg.Manager.RuntimeMode))
	}
	if !inSet(cfg.Manager.LogLevel, "trace", "debug", "info", "warn", "error", "fatal", "panic") {
		errors = append(errors, fmt.Sprintf("manager.main.log_level is unsupported: %q", cfg.Manager.LogLevel))
	}

	for id, group := range cfg.Groups {
		if group.Name == "" {
			errors = append(errors, fmt.Sprintf("group.%s.name is required", id))
		}
		if group.RoutingProfile != "" {
			if _, ok := cfg.Routing[group.RoutingProfile]; !ok {
				errors = append(errors, fmt.Sprintf("group.%s.routing_profile references missing profile %q", id, group.RoutingProfile))
			}
		}
		if group.DNSProfile != "" {
			if _, ok := cfg.DNSProfiles[group.DNSProfile]; !ok {
				errors = append(errors, fmt.Sprintf("group.%s.dns_profile references missing profile %q", id, group.DNSProfile))
			}
		}
		if group.SelectedNode != "" {
			if _, ok := cfg.Nodes[group.SelectedNode]; !ok {
				errors = append(errors, fmt.Sprintf("group.%s.selected_node references missing node %q", id, group.SelectedNode))
			}
		}
		for _, subscriptionID := range group.Subscriptions {
			if _, ok := cfg.Subscriptions[subscriptionID]; !ok {
				errors = append(errors, fmt.Sprintf("group.%s.subscription references missing subscription %q", id, subscriptionID))
			}
		}
		if !inSet(group.Strategy, "manual", "selector", "urltest", "load-balance") {
			errors = append(errors, fmt.Sprintf("group.%s.strategy is unsupported: %q", id, group.Strategy))
		}
	}

	for id, subscription := range cfg.Subscriptions {
		if subscription.Enabled && subscription.URL == "" {
			errors = append(errors, fmt.Sprintf("subscription.%s.url is required when enabled", id))
		}
		if !inSet(subscription.Format, "auto", "base64", "plain") {
			errors = append(errors, fmt.Sprintf("subscription.%s.format is unsupported: %q", id, subscription.Format))
		}
	}

	for id, node := range cfg.Nodes {
		if !node.Enabled {
			continue
		}
		if node.Type == "" {
			errors = append(errors, fmt.Sprintf("node.%s.type is required", id))
			continue
		}
		switch node.Type {
		case "direct":
		case "shadowsocks":
			if firstNonEmpty(node.Server, node.Address) == "" || node.Port == 0 || node.Method == "" || node.Password == "" {
				errors = append(errors, fmt.Sprintf("node.%s shadowsocks requires server/address, port, method, and password", id))
			}
		case "trojan":
			if firstNonEmpty(node.Server, node.Address) == "" || node.Port == 0 || node.Password == "" {
				errors = append(errors, fmt.Sprintf("node.%s trojan requires server/address, port, and password", id))
			}
		case "vmess", "vless":
			if firstNonEmpty(node.Server, node.Address) == "" || node.Port == 0 || node.UUID == "" {
				errors = append(errors, fmt.Sprintf("node.%s %s requires server/address, port, and uuid", id, node.Type))
			}
		case "hysteria2":
			if firstNonEmpty(node.Server, node.Address) == "" || node.Port == 0 || node.Password == "" {
				errors = append(errors, fmt.Sprintf("node.%s hysteria2 requires server/address, port, and password", id))
			}
		case "tuic":
			if firstNonEmpty(node.Server, node.Address) == "" || node.Port == 0 || node.UUID == "" || node.Password == "" {
				errors = append(errors, fmt.Sprintf("node.%s tuic requires server/address, port, uuid, and password", id))
			}
		default:
			errors = append(errors, fmt.Sprintf("node.%s.type is unsupported: %q", id, node.Type))
		}
	}

	for id, profile := range cfg.Routing {
		if !inSet(profile.Mode, "direct", "rule", "global") {
			errors = append(errors, fmt.Sprintf("routing_profile.%s.mode is unsupported: %q", id, profile.Mode))
		}
		if !inSet(profile.Final, "direct", "proxy", "block") {
			errors = append(errors, fmt.Sprintf("routing_profile.%s.final is unsupported: %q", id, profile.Final))
		}
		for _, rulesetID := range profile.RuleSets {
			if _, ok := cfg.RuleSets[rulesetID]; !ok {
				errors = append(errors, fmt.Sprintf("routing_profile.%s.ruleset references missing ruleset %q", id, rulesetID))
			}
		}
	}

	for id, rule := range cfg.SourceRules {
		if rule.Profile == "" {
			errors = append(errors, fmt.Sprintf("source_rule.%s.profile is required", id))
		} else if _, ok := cfg.Routing[rule.Profile]; !ok {
			errors = append(errors, fmt.Sprintf("source_rule.%s.profile references missing profile %q", id, rule.Profile))
		}
		if len(rule.Sources) == 0 {
			errors = append(errors, fmt.Sprintf("source_rule.%s.source_ip is required", id))
		}
		for _, source := range rule.Sources {
			if _, err := netip.ParseAddr(source); err == nil {
				continue
			}
			if _, err := netip.ParsePrefix(source); err != nil {
				errors = append(errors, fmt.Sprintf("source_rule.%s.source_ip is invalid: %q", id, source))
			}
		}
		if !inSet(rule.Outbound, "direct", "proxy", "block", "dns") {
			errors = append(errors, fmt.Sprintf("source_rule.%s.outbound must be direct, proxy, block, or dns, got %q", id, rule.Outbound))
		}
	}

	for id, profile := range cfg.DNSProfiles {
		if !inSet(profile.Mode, "direct", "proxy", "split") {
			errors = append(errors, fmt.Sprintf("dns_profile.%s.mode is unsupported: %q", id, profile.Mode))
		}
		for _, serverID := range profile.Servers {
			if _, ok := cfg.DNSServers[serverID]; !ok {
				errors = append(errors, fmt.Sprintf("dns_profile.%s.server references missing dns_server %q", id, serverID))
			}
		}
	}

	for id, server := range cfg.DNSServers {
		if server.Enabled && server.Address == "" {
			errors = append(errors, fmt.Sprintf("dns_server.%s.address is required when enabled", id))
		}
		if !inSet(server.Type, "udp", "tcp", "tls", "dot", "doh", "doq", "https", "quic") {
			errors = append(errors, fmt.Sprintf("dns_server.%s.type is unsupported: %q", id, server.Type))
		}
		if server.Detour != "" && !inSet(server.Detour, "direct", "proxy") {
			errors = append(errors, fmt.Sprintf("dns_server.%s.detour must be direct or proxy, got %q", id, server.Detour))
		}
	}

	for id, ruleset := range cfg.RuleSets {
		if !inSet(ruleset.Type, "local", "remote") {
			errors = append(errors, fmt.Sprintf("ruleset.%s.type is unsupported: %q", id, ruleset.Type))
		}
		if !inSet(ruleset.Format, "srs", "binary", "source") {
			errors = append(errors, fmt.Sprintf("ruleset.%s.format is unsupported: %q", id, ruleset.Format))
		}
		if ruleset.Enabled && ruleset.Type == "local" && ruleset.Path == "" {
			errors = append(errors, fmt.Sprintf("ruleset.%s.path is required for local rulesets", id))
		}
		if ruleset.Enabled && ruleset.Type == "remote" && ruleset.URL == "" && ruleset.Path == "" {
			errors = append(errors, fmt.Sprintf("ruleset.%s needs url or path when enabled", id))
		}
	}

	for _, rule := range cfg.PAC.CustomRules {
		if parsePACRulePattern(rule) == "" {
			errors = append(errors, fmt.Sprintf("pac.main.custom_rule is invalid: %q", rule))
		}
	}
	if !inSet(cfg.PAC.Source, "generated", "custom") {
		errors = append(errors, fmt.Sprintf("pac.main.source must be generated or custom, got %q", cfg.PAC.Source))
	}
	if cfg.PAC.Source == "custom" {
		custom, ok := cfg.CustomPACs[cfg.PAC.SelectedCustom]
		if cfg.PAC.SelectedCustom == "" || !ok {
			errors = append(errors, fmt.Sprintf("pac.main.selected_custom references missing custom PAC %q", cfg.PAC.SelectedCustom))
		} else if !custom.Enabled {
			errors = append(errors, fmt.Sprintf("pac.main.selected_custom references disabled custom PAC %q", cfg.PAC.SelectedCustom))
		}
	}
	for id, pac := range cfg.CustomPACs {
		if pac.Content == "" {
			errors = append(errors, fmt.Sprintf("pac_custom.%s.content is required", id))
		}
	}

	if cfg.TProxy.Enabled && cfg.TUN.Enabled {
		errors = append(errors, "tproxy.main and tun.main cannot both be enabled")
	}
	if cfg.TProxy.Enabled && cfg.Manager.TProxyPort == 0 {
		errors = append(errors, "manager.main.tproxy_port is required when tproxy is enabled")
	}
	if cfg.TProxy.Enabled && len(cfg.TProxy.LANIfnames) == 0 {
		errors = append(errors, "tproxy.main.lan_ifname is required when tproxy is enabled")
	}
	if cfg.TProxy.Enabled && cfg.TProxy.DNSHijack && cfg.Manager.DNSPort == 0 {
		errors = append(errors, "manager.main.dns_port is required when DNS hijacking is enabled")
	}
	for _, subnet := range cfg.TProxy.IncludeSubnet {
		if _, err := netip.ParsePrefix(subnet); err != nil {
			errors = append(errors, fmt.Sprintf("tproxy.main.include_subnet is invalid: %q", subnet))
		}
	}
	for _, subnet := range cfg.TProxy.ExcludeSubnet {
		if _, err := netip.ParsePrefix(subnet); err != nil {
			errors = append(errors, fmt.Sprintf("tproxy.main.exclude_subnet is invalid: %q", subnet))
		}
	}
	for _, mac := range cfg.TProxy.IncludeMAC {
		if _, err := net.ParseMAC(mac); err != nil {
			errors = append(errors, fmt.Sprintf("tproxy.main.include_mac is invalid: %q", mac))
		}
	}
	if cfg.TUN.Inet4Address != "" {
		prefix, err := netip.ParsePrefix(cfg.TUN.Inet4Address)
		if err != nil || !prefix.Addr().Is4() {
			errors = append(errors, fmt.Sprintf("tun.main.inet4_address is invalid: %q", cfg.TUN.Inet4Address))
		}
	}
	if cfg.TUN.Inet6Address != "" {
		prefix, err := netip.ParsePrefix(cfg.TUN.Inet6Address)
		if err != nil || !prefix.Addr().Is6() {
			errors = append(errors, fmt.Sprintf("tun.main.inet6_address is invalid: %q", cfg.TUN.Inet6Address))
		}
	}

	if len(errors) > 0 {
		return ValidationError{Errors: errors}
	}
	return nil
}

func inSet(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func parsePACRulePattern(rule string) string {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return ""
	}
	if strings.Contains(rule, ",") {
		parts := strings.SplitN(rule, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	fields := strings.Fields(rule)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		return fields[0]
	}
	if inSet(strings.ToLower(fields[0]), "direct", "proxy", "block", "reject") {
		return strings.Join(fields[1:], " ")
	}
	return fields[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
