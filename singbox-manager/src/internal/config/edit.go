package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type HealthState struct {
	Health    string
	LatencyMS int
	LastCheck string
}

func UpsertSubscription(path string, subscription Subscription, groupID string) error {
	if subscription.ID == "" {
		return fmt.Errorf("subscription id is required")
	}
	if subscription.Name == "" {
		subscription.Name = subscription.ID
	}
	if subscription.Format == "" {
		subscription.Format = "auto"
	}
	if subscription.UpdateInterval == "" {
		subscription.UpdateInterval = "24h"
	}
	if subscription.Health == "" {
		subscription.Health = "unknown"
	}

	return editConfig(path, func(sections []section) ([]section, error) {
		replacement := subscriptionSection(subscription)
		found := false
		for i, sec := range sections {
			if sec.typ != "subscription" || sec.name != subscription.ID {
				continue
			}
			if subscription.LastUpdate == "" {
				replacement.options["last_update"] = sec.options["last_update"]
			}
			if subscription.LastCheck == "" {
				replacement.options["last_check"] = sec.options["last_check"]
			}
			if subscription.LastError == "" {
				replacement.options["last_error"] = sec.options["last_error"]
			}
			if subscription.LatencyMS == 0 {
				if latency := sec.options["latency_ms"]; latency != "" {
					replacement.options["latency_ms"] = latency
				}
			}
			sections[i] = replacement
			found = true
			break
		}
		if !found {
			sections = append(sections, replacement)
		}
		if groupID != "" {
			if err := attachSubscriptionToGroup(sections, groupID, subscription.ID); err != nil {
				return nil, err
			}
		}
		return sections, nil
	})
}

func ReplaceSubscriptionNodes(path string, subscriptionID string, nodes []Node) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		next := make([]section, 0, len(sections)+len(nodes))
		for _, sec := range sections {
			if sec.typ == "node" && sec.options["subscription"] == subscriptionID {
				continue
			}
			if sec.typ == "subscription" && sec.name == subscriptionID {
				sec.options["last_update"] = time.Now().UTC().Format(time.RFC3339)
				sec.options["health"] = "ok"
				delete(sec.options, "last_error")
			}
			next = append(next, sec)
		}
		for _, node := range nodes {
			if node.Subscription == "" {
				node.Subscription = subscriptionID
			}
			next = append(next, nodeSection(node))
		}
		return next, nil
	})
}

func UpdateHealth(path string, nodes map[string]HealthState, groups map[string]HealthState, subscriptions map[string]HealthState) error {
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			switch sections[i].typ {
			case "node":
				state, ok := nodes[sections[i].name]
				if !ok {
					continue
				}
				applyHealthOptions(&sections[i], state, checkedAt)
			case "group":
				state, ok := groups[sections[i].name]
				if !ok {
					continue
				}
				applyHealthOptions(&sections[i], state, checkedAt)
			case "subscription":
				state, ok := subscriptions[sections[i].name]
				if !ok {
					continue
				}
				applyHealthOptions(&sections[i], state, checkedAt)
			}
		}
		return sections, nil
	})
}

func MarkSubscriptionError(path string, subscriptionID string, message string) error {
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			if sections[i].typ == "subscription" && sections[i].name == subscriptionID {
				sections[i].options["last_error"] = firstLine(message)
			}
		}
		return sections, nil
	})
}

func MarkRuleSetUpdated(path string, rulesetID string) error {
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			if isRuleSetSection(sections[i], rulesetID) {
				sections[i].options["last_update"] = time.Now().UTC().Format(time.RFC3339)
				delete(sections[i].options, "last_error")
			}
		}
		return sections, nil
	})
}

func MarkRuleSetError(path string, rulesetID string, message string) error {
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			if isRuleSetSection(sections[i], rulesetID) {
				sections[i].options["last_error"] = firstLine(message)
			}
		}
		return sections, nil
	})
}

func isRuleSetSection(sec section, rulesetID string) bool {
	return sec.typ == "ruleset" && (sec.name == rulesetID || sec.options["id"] == rulesetID)
}

func DeleteSubscription(path string, subscriptionID string) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		next := make([]section, 0, len(sections))
		found := false
		for _, sec := range sections {
			switch {
			case sec.typ == "subscription" && sec.name == subscriptionID:
				found = true
				continue
			case sec.typ == "node" && sec.options["subscription"] == subscriptionID:
				continue
			case sec.typ == "group":
				removeListValue(&sec, "subscription", subscriptionID)
			}
			next = append(next, sec)
		}
		if !found {
			return nil, fmt.Errorf("subscription %q not found", subscriptionID)
		}
		return next, nil
	})
}

func UpsertManualNode(path string, node Node) error {
	if node.ID == "" {
		return fmt.Errorf("node id is required")
	}
	if node.Subscription != "" {
		return fmt.Errorf("manual node cannot include subscription metadata")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		replacement := nodeSection(node)
		for i, sec := range sections {
			if sec.typ == "node" && sec.name == node.ID {
				if sec.options["subscription"] != "" {
					return nil, fmt.Errorf("node %q belongs to subscription %q", node.ID, sec.options["subscription"])
				}
				sections[i] = replacement
				return sections, nil
			}
		}
		return append(sections, replacement), nil
	})
}

func DeleteManualNode(path string, nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("node id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		next := make([]section, 0, len(sections))
		found := false
		for _, sec := range sections {
			if sec.typ == "node" && sec.name == nodeID {
				if sec.options["subscription"] != "" {
					return nil, fmt.Errorf("node %q belongs to subscription %q", nodeID, sec.options["subscription"])
				}
				found = true
				continue
			}
			next = append(next, sec)
		}
		if !found {
			return nil, fmt.Errorf("node %q not found", nodeID)
		}
		return next, nil
	})
}

func SetManagerEnabled(path string, enabled bool) error {
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			if sections[i].typ != "manager" || sections[i].name != "main" {
				continue
			}
			if sections[i].options == nil {
				sections[i].options = map[string]string{}
			}
			sections[i].options["enabled"] = boolString(enabled)
			return sections, nil
		}
		return nil, fmt.Errorf("manager main section not found")
	})
}

func SetManagerRuntimeMode(path string, mode string) error {
	if !inSet(mode, "direct", "rule", "global") {
		return fmt.Errorf("runtime mode must be direct, rule, or global, got %q", mode)
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			if sections[i].typ != "manager" || sections[i].name != "main" {
				continue
			}
			if sections[i].options == nil {
				sections[i].options = map[string]string{}
			}
			sections[i].options["runtime_mode"] = mode
			return sections, nil
		}
		return nil, fmt.Errorf("manager main section not found")
	})
}

func SelectNode(path string, groupID string, nodeID string) error {
	if groupID == "" {
		return fmt.Errorf("group id is required")
	}
	if nodeID == "" {
		return fmt.Errorf("node id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			if sections[i].typ != "group" || sections[i].name != groupID {
				continue
			}
			if sections[i].options == nil {
				sections[i].options = map[string]string{}
			}
			sections[i].options["selected_node"] = nodeID
			if sections[i].options["strategy"] != "selector" {
				sections[i].options["strategy"] = "manual"
			}
			return sections, nil
		}
		return nil, fmt.Errorf("group %q not found", groupID)
	})
}

func UpsertDNSRule(path string, rule DNSRule) error {
	if rule.ID == "" {
		return fmt.Errorf("dns rule id is required")
	}
	if rule.Name == "" {
		rule.Name = rule.ID
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		replacement := dnsRuleSection(rule)
		for i, sec := range sections {
			if sec.typ == "dns_rule" && sec.name == rule.ID {
				sections[i] = replacement
				return sections, nil
			}
		}
		return append(sections, replacement), nil
	})
}

func DeleteDNSRule(path string, ruleID string) error {
	if ruleID == "" {
		return fmt.Errorf("dns rule id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		next := make([]section, 0, len(sections))
		found := false
		for _, sec := range sections {
			if sec.typ == "dns_rule" && sec.name == ruleID {
				found = true
				continue
			}
			next = append(next, sec)
		}
		if !found {
			return nil, fmt.Errorf("dns rule %q not found", ruleID)
		}
		return next, nil
	})
}

func UpsertDNSServer(path string, server DNSServer) error {
	if server.ID == "" {
		return fmt.Errorf("dns server id is required")
	}
	if server.Name == "" {
		server.Name = server.ID
	}
	if server.Type == "" {
		server.Type = "udp"
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		replacement := dnsServerSection(server)
		for i, sec := range sections {
			if sec.typ == "dns_server" && sec.name == server.ID {
				sections[i] = replacement
				return sections, nil
			}
		}
		return append(sections, replacement), nil
	})
}

func DeleteDNSServer(path string, serverID string) error {
	if serverID == "" {
		return fmt.Errorf("dns server id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		next := make([]section, 0, len(sections))
		found := false
		for _, sec := range sections {
			switch {
			case sec.typ == "dns_server" && sec.name == serverID:
				found = true
				continue
			case sec.typ == "dns_rule" && sec.options["server"] == serverID:
				continue
			case sec.typ == "group" && sec.options["dns_final"] == serverID:
				delete(sec.options, "dns_final")
			}
			next = append(next, sec)
		}
		if !found {
			return nil, fmt.Errorf("dns server %q not found", serverID)
		}
		return next, nil
	})
}

func UpsertRouteRule(path string, rule RouteRule) error {
	if rule.ID == "" {
		return fmt.Errorf("route rule id is required")
	}
	if rule.Name == "" {
		rule.Name = rule.ID
	}
	if rule.Outbound == "" {
		rule.Outbound = "proxy"
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		replacement := routeRuleSection(rule)
		for i, sec := range sections {
			if sec.typ == "route_rule" && sec.name == rule.ID {
				sections[i] = replacement
				return sections, nil
			}
		}
		return append(sections, replacement), nil
	})
}

func DeleteRouteRule(path string, ruleID string) error {
	if ruleID == "" {
		return fmt.Errorf("route rule id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		next := make([]section, 0, len(sections))
		found := false
		for _, sec := range sections {
			if sec.typ == "route_rule" && sec.name == ruleID {
				found = true
				continue
			}
			next = append(next, sec)
		}
		if !found {
			return nil, fmt.Errorf("route rule %q not found", ruleID)
		}
		return next, nil
	})
}

// SetGroupSettings updates the user-editable fields of a group section
// (name, strategy, route/DNS defaults, selected node, subscriptions) while
// preserving health metadata managed by the daemon.
func SetGroupSettings(path string, group Group) error {
	if group.ID == "" {
		return fmt.Errorf("group id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		for i := range sections {
			if sections[i].typ != "group" || sections[i].name != group.ID {
				continue
			}
			if sections[i].options == nil {
				sections[i].options = map[string]string{}
			}
			opts := sections[i].options
			opts["name"] = valueOrDefault(group.Name, group.ID)
			opts["strategy"] = valueOrDefault(group.Strategy, "manual")
			opts["route_final"] = valueOrDefault(group.RouteFinal, "proxy")
			setOrDelete(opts, "dns_final", group.DNSFinal)
			setOrDelete(opts, "selected_node", group.SelectedNode)
			if sections[i].lists == nil {
				sections[i].lists = map[string][]string{}
			}
			if subs := cleanList(group.Subscriptions); len(subs) > 0 {
				sections[i].lists["subscription"] = subs
			} else {
				delete(sections[i].lists, "subscription")
			}
			return sections, nil
		}
		return nil, fmt.Errorf("group %q not found", group.ID)
	})
}

func UpsertRuleSet(path string, ruleset RuleSet) error {
	if ruleset.ID == "" {
		return fmt.Errorf("ruleset id is required")
	}
	if ruleset.Name == "" {
		ruleset.Name = ruleset.ID
	}
	if ruleset.Type == "" {
		ruleset.Type = "local"
	}
	if ruleset.Format == "" {
		ruleset.Format = "srs"
	}
	if ruleset.UpdateInterval == "" {
		ruleset.UpdateInterval = "168h"
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		found := false
		for i, sec := range sections {
			if !isRuleSetSection(sec, ruleset.ID) {
				continue
			}
			if ruleset.LastUpdate == "" {
				ruleset.LastUpdate = sec.options["last_update"]
			}
			if ruleset.LastError == "" {
				ruleset.LastError = sec.options["last_error"]
			}
			sections[i] = ruleSetSection(sec.name, ruleset)
			found = true
			break
		}
		if !found {
			sections = append(sections, ruleSetSection(sectionNameFromID(ruleset.ID), ruleset))
		}
		return sections, nil
	})
}

func DeleteRuleSet(path string, rulesetID string) error {
	if rulesetID == "" {
		return fmt.Errorf("ruleset id is required")
	}
	return editConfig(path, func(sections []section) ([]section, error) {
		next := make([]section, 0, len(sections))
		found := false
		for _, sec := range sections {
			if isRuleSetSection(sec, rulesetID) {
				found = true
				continue
			}
			if sec.typ == "route_rule" || sec.typ == "dns_rule" {
				removeListValue(&sec, "ruleset", rulesetID)
				// A rule that no longer matches anything (no source IPs and no
				// rule sets) is meaningless, so drop it instead of leaving an
				// invalid section behind.
				if len(sec.lists["source_ip"]) == 0 && len(sec.lists["ruleset"]) == 0 {
					continue
				}
			}
			next = append(next, sec)
		}
		if !found {
			return nil, fmt.Errorf("ruleset %q not found", rulesetID)
		}
		return next, nil
	})
}

// UpsertTProxy rewrites the transparent-proxy section. Cross-section rules
// (e.g. tproxy and tun cannot both be enabled) are enforced by editConfig's
// validation pass, so an invalid combination is rejected before it is written.
func UpsertTProxy(path string, tproxy TProxy) error {
	return editConfig(path, func(sections []section) ([]section, error) {
		replacement := tproxySection(tproxy)
		for i := range sections {
			if sections[i].typ == "tproxy" {
				replacement.name = sections[i].name
				sections[i] = replacement
				return sections, nil
			}
		}
		return append(sections, replacement), nil
	})
}

// UpsertTUN rewrites the TUN section.
func UpsertTUN(path string, tun TUN) error {
	return editConfig(path, func(sections []section) ([]section, error) {
		replacement := tunSection(tun)
		for i := range sections {
			if sections[i].typ == "tun" {
				replacement.name = sections[i].name
				sections[i] = replacement
				return sections, nil
			}
		}
		return append(sections, replacement), nil
	})
}

func editConfig(path string, mutate func([]section) ([]section, error)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sections, err := parseUCI(string(data))
	if err != nil {
		return err
	}
	sections, err = mutate(sections)
	if err != nil {
		return err
	}
	cfg := DefaultConfig()
	if err := applySections(&cfg, sections); err != nil {
		return err
	}
	if err := Validate(cfg); err != nil {
		return err
	}
	rendered := renderUCI(sections)
	if err := writeAtomic(path, []byte(rendered), 0644); err != nil {
		return err
	}
	return nil
}

func nodeSection(node Node) section {
	options := map[string]string{
		"enabled": boolString(node.Enabled),
		"name":    node.Name,
		"type":    node.Type,
	}
	set := func(key string, value string) {
		if value != "" {
			options[key] = value
		}
	}
	set("address", node.Address)
	set("server", node.Server)
	if node.Port > 0 {
		options["port"] = strconv.Itoa(node.Port)
	}
	set("uuid", node.UUID)
	set("password", node.Password)
	set("method", node.Method)
	set("security", node.Security)
	if node.TLS {
		options["tls"] = "1"
	}
	set("flow", node.Flow)
	set("transport", node.Transport)
	set("host", node.Host)
	set("path", node.Path)
	set("sni", node.SNI)
	set("alpn", node.ALPN)
	if node.Insecure {
		options["insecure"] = "1"
	}
	set("congestion", node.Congestion)
	set("udp_relay_mode", node.UDPRelayMode)
	set("tag", valueOrDefault(node.Tag, node.ID))
	set("subscription", node.Subscription)
	set("health", node.Health)
	if node.LatencyMS > 0 {
		options["latency_ms"] = strconv.Itoa(node.LatencyMS)
	}
	set("last_check", node.LastCheck)
	return section{
		typ:     "node",
		name:    node.ID,
		options: options,
		lists:   map[string][]string{},
	}
}

func subscriptionSection(subscription Subscription) section {
	options := map[string]string{
		"enabled":         boolString(subscription.Enabled),
		"name":            subscription.Name,
		"format":          valueOrDefault(subscription.Format, "auto"),
		"update_interval": valueOrDefault(subscription.UpdateInterval, "24h"),
		"health":          valueOrDefault(subscription.Health, "unknown"),
	}
	set := func(key string, value string) {
		if value != "" {
			options[key] = value
		}
	}
	set("url", subscription.URL)
	set("last_update", subscription.LastUpdate)
	set("last_error", subscription.LastError)
	set("last_check", subscription.LastCheck)
	if subscription.LatencyMS > 0 {
		options["latency_ms"] = strconv.Itoa(subscription.LatencyMS)
	}
	return section{
		typ:     "subscription",
		name:    subscription.ID,
		options: options,
		lists:   map[string][]string{},
	}
}

func dnsRuleSection(rule DNSRule) section {
	options := map[string]string{
		"enabled": boolString(rule.Enabled),
		"name":    valueOrDefault(rule.Name, rule.ID),
	}
	setOrDelete(options, "group", rule.Group)
	setOrDelete(options, "server", rule.Server)
	lists := map[string][]string{}
	if sources := cleanList(rule.Sources); len(sources) > 0 {
		lists["source_ip"] = sources
	}
	if rulesets := cleanList(rule.RuleSets); len(rulesets) > 0 {
		lists["ruleset"] = rulesets
	}
	return section{
		typ:     "dns_rule",
		name:    rule.ID,
		options: options,
		lists:   lists,
	}
}

func dnsServerSection(server DNSServer) section {
	options := map[string]string{
		"enabled": boolString(server.Enabled),
		"name":    server.Name,
		"type":    valueOrDefault(server.Type, "udp"),
	}
	set := func(key string, value string) {
		if value != "" {
			options[key] = value
		}
	}
	set("address", server.Address)
	set("detour", server.Detour)
	return section{
		typ:     "dns_server",
		name:    server.ID,
		options: options,
		lists:   map[string][]string{},
	}
}

func routeRuleSection(rule RouteRule) section {
	options := map[string]string{
		"enabled":  boolString(rule.Enabled),
		"name":     valueOrDefault(rule.Name, rule.ID),
		"outbound": valueOrDefault(rule.Outbound, "proxy"),
	}
	setOrDelete(options, "group", rule.Group)
	lists := map[string][]string{}
	if sources := cleanList(rule.Sources); len(sources) > 0 {
		lists["source_ip"] = sources
	}
	if rulesets := cleanList(rule.RuleSets); len(rulesets) > 0 {
		lists["ruleset"] = rulesets
	}
	return section{
		typ:     "route_rule",
		name:    rule.ID,
		options: options,
		lists:   lists,
	}
}

func ruleSetSection(sectionName string, ruleset RuleSet) section {
	options := map[string]string{
		"id":              ruleset.ID,
		"enabled":         boolString(ruleset.Enabled),
		"name":            ruleset.Name,
		"type":            valueOrDefault(ruleset.Type, "local"),
		"format":          valueOrDefault(ruleset.Format, "srs"),
		"update_interval": valueOrDefault(ruleset.UpdateInterval, "168h"),
	}
	set := func(key string, value string) {
		if value != "" {
			options[key] = value
		}
	}
	set("url", ruleset.URL)
	set("path", ruleset.Path)
	set("last_update", ruleset.LastUpdate)
	set("last_error", ruleset.LastError)
	return section{
		typ:     "ruleset",
		name:    sectionName,
		options: options,
		lists:   map[string][]string{},
	}
}

func tproxySection(tproxy TProxy) section {
	options := map[string]string{
		"enabled":     boolString(tproxy.Enabled),
		"dns_hijack":  boolString(tproxy.DNSHijack),
		"kill_switch": boolString(tproxy.KillSwitch),
	}
	lists := map[string][]string{}
	if v := cleanList(tproxy.LANIfnames); len(v) > 0 {
		lists["lan_ifname"] = v
	}
	if v := cleanList(tproxy.IncludeSubnet); len(v) > 0 {
		lists["include_subnet"] = v
	}
	if v := cleanList(tproxy.ExcludeSubnet); len(v) > 0 {
		lists["exclude_subnet"] = v
	}
	if v := cleanList(tproxy.IncludeMAC); len(v) > 0 {
		lists["include_mac"] = v
	}
	return section{typ: "tproxy", name: "tproxy", options: options, lists: lists}
}

func tunSection(tun TUN) section {
	options := map[string]string{
		"enabled":       boolString(tun.Enabled),
		"auto_route":    boolString(tun.AutoRoute),
		"auto_redirect": boolString(tun.AutoRedirect),
	}
	setOrDelete(options, "inet4_address", tun.Inet4Address)
	setOrDelete(options, "inet6_address", tun.Inet6Address)
	return section{typ: "tun", name: "tun", options: options, lists: map[string][]string{}}
}

func attachSubscriptionToGroup(sections []section, groupID string, subscriptionID string) error {
	for i := range sections {
		if sections[i].typ != "group" || sections[i].name != groupID {
			continue
		}
		if sections[i].lists == nil {
			sections[i].lists = map[string][]string{}
		}
		for _, existing := range sections[i].lists["subscription"] {
			if existing == subscriptionID {
				return nil
			}
		}
		sections[i].lists["subscription"] = append(sections[i].lists["subscription"], subscriptionID)
		return nil
	}
	return fmt.Errorf("group %q not found", groupID)
}

func setOrDelete(options map[string]string, key string, value string) {
	if value == "" {
		delete(options, key)
		return
	}
	options[key] = value
}

func removeListValue(sec *section, key string, value string) {
	values := sec.lists[key]
	next := make([]string, 0, len(values))
	for _, item := range values {
		if item != value {
			next = append(next, item)
		}
	}
	if len(next) == 0 {
		delete(sec.lists, key)
		return
	}
	sec.lists[key] = next
}

func sectionNameFromID(id string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, id)
	name = strings.Trim(name, "_")
	if name == "" {
		return "entry"
	}
	return name
}

func applyHealthOptions(sec *section, state HealthState, checkedAt string) {
	if state.Health != "" {
		sec.options["health"] = state.Health
	}
	if state.LatencyMS > 0 {
		sec.options["latency_ms"] = strconv.Itoa(state.LatencyMS)
	} else {
		delete(sec.options, "latency_ms")
	}
	if state.LastCheck != "" {
		sec.options["last_check"] = state.LastCheck
	} else {
		sec.options["last_check"] = checkedAt
	}
}

func renderUCI(sections []section) string {
	var builder strings.Builder
	for i, sec := range sections {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("config ")
		builder.WriteString(quoteUCI(sec.typ))
		if sec.name != "" {
			builder.WriteByte(' ')
			builder.WriteString(quoteUCI(sec.name))
		}
		builder.WriteString("\n")

		for _, key := range sortedKeys(sec.options) {
			builder.WriteString("\toption ")
			builder.WriteString(quoteUCI(key))
			builder.WriteByte(' ')
			builder.WriteString(quoteUCI(sec.options[key]))
			builder.WriteString("\n")
		}
		for _, key := range sortedKeys(sec.lists) {
			for _, value := range sec.lists[key] {
				builder.WriteString("\tlist ")
				builder.WriteString(quoteUCI(key))
				builder.WriteByte(' ')
				builder.WriteString(quoteUCI(value))
				builder.WriteString("\n")
			}
		}
	}
	return builder.String()
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quoteUCI(value string) string {
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value)
	return "\"" + escaped + "\""
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		value = value[:idx]
	}
	if value == "" {
		return "error"
	}
	return value
}
