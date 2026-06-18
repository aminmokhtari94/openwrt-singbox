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
		if !inSet(group.Strategy, "manual", "selector", "urltest", "load-balance") {
			errors = append(errors, fmt.Sprintf("group.%s.strategy is unsupported: %q", id, group.Strategy))
		}
		if !inSet(group.RouteFinal, "direct", "proxy", "block") {
			errors = append(errors, fmt.Sprintf("group.%s.route_final must be direct, proxy, or block, got %q", id, group.RouteFinal))
		}
		if group.DNSFinal != "" {
			if _, ok := cfg.DNSServers[group.DNSFinal]; !ok {
				errors = append(errors, fmt.Sprintf("group.%s.dns_final references missing dns_server %q", id, group.DNSFinal))
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

	for id, rule := range cfg.DNSRules {
		validateRuleGroup(&errors, "dns_rule", id, rule.Group, cfg)
		if rule.Server == "" {
			errors = append(errors, fmt.Sprintf("dns_rule.%s.server is required", id))
		} else if _, ok := cfg.DNSServers[rule.Server]; !ok {
			errors = append(errors, fmt.Sprintf("dns_rule.%s.server references missing dns_server %q", id, rule.Server))
		}
		if len(rule.Sources) == 0 && len(rule.RuleSets) == 0 {
			errors = append(errors, fmt.Sprintf("dns_rule.%s needs at least one source_ip or ruleset", id))
		}
		validateSourceCIDRs(&errors, "dns_rule", id, rule.Sources)
		validateRuleSetRefs(&errors, "dns_rule", id, rule.RuleSets, cfg)
	}

	for id, rule := range cfg.RouteRules {
		validateRuleGroup(&errors, "route_rule", id, rule.Group, cfg)
		if !inSet(rule.Outbound, "direct", "proxy", "block") {
			errors = append(errors, fmt.Sprintf("route_rule.%s.outbound must be direct, proxy, or block, got %q", id, rule.Outbound))
		}
		if len(rule.Sources) == 0 && len(rule.RuleSets) == 0 {
			errors = append(errors, fmt.Sprintf("route_rule.%s needs at least one source_ip or ruleset", id))
		}
		validateSourceCIDRs(&errors, "route_rule", id, rule.Sources)
		validateRuleSetRefs(&errors, "route_rule", id, rule.RuleSets, cfg)
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

func validateRuleGroup(errors *[]string, kind string, id string, groupID string, cfg Config) {
	if groupID == "" {
		*errors = append(*errors, fmt.Sprintf("%s.%s.group is required", kind, id))
		return
	}
	if _, ok := cfg.Groups[groupID]; !ok {
		*errors = append(*errors, fmt.Sprintf("%s.%s.group references missing group %q", kind, id, groupID))
	}
}

func validateSourceCIDRs(errors *[]string, kind string, id string, sources []string) {
	for _, source := range sources {
		if _, err := netip.ParseAddr(source); err == nil {
			continue
		}
		if _, err := netip.ParsePrefix(source); err != nil {
			*errors = append(*errors, fmt.Sprintf("%s.%s.source_ip is invalid: %q", kind, id, source))
		}
	}
}

func validateRuleSetRefs(errors *[]string, kind string, id string, rulesets []string, cfg Config) {
	for _, rulesetID := range rulesets {
		if _, ok := cfg.RuleSets[rulesetID]; !ok {
			*errors = append(*errors, fmt.Sprintf("%s.%s.ruleset references missing ruleset %q", kind, id, rulesetID))
		}
	}
}

func inSet(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
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
