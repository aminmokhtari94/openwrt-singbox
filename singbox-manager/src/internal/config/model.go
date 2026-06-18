package config

const (
	DefaultSocketPath    = "/var/run/singbox-manager/singbox-managerd.sock"
	DefaultSingBoxBinary = "/usr/bin/sing-box"
)

type Config struct {
	Manager       Manager
	Groups        map[string]Group
	Subscriptions map[string]Subscription
	Nodes         map[string]Node
	DNSServers    map[string]DNSServer
	DNSRules      map[string]DNSRule
	RouteRules    map[string]RouteRule
	RuleSets      map[string]RuleSet
	TProxy        TProxy
	TUN           TUN
}

type Manager struct {
	Enabled        bool
	LogLevel       string
	ActiveGroup    string
	RuntimeMode    string
	SingBoxBinary  string
	SocketPath     string
	APIListen      string
	MixedListen    string
	MixedPort      int
	TProxyPort     int
	DNSPort        int
	UpdateInterval string
}

// Group bundles a set of subscriptions, a proxy selection strategy, and the
// routing/DNS defaults that apply while the group is active. Routing and DNS
// rules reference a group by ID; there is no longer a separate profile layer.
type Group struct {
	ID            string   `json:"id"`
	Enabled       bool     `json:"enabled"`
	Name          string   `json:"name"`
	Subscriptions []string `json:"subscriptions"`
	Strategy      string   `json:"strategy"`
	SelectedNode  string   `json:"selected_node"`
	RouteFinal    string   `json:"route_final"`
	DNSFinal      string   `json:"dns_final"`
	Health        string   `json:"health"`
	LatencyMS     int      `json:"latency_ms"`
	LastCheck     string   `json:"last_check"`
}

type Subscription struct {
	ID             string `json:"id"`
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Format         string `json:"format"`
	UpdateInterval string `json:"update_interval"`
	LastUpdate     string `json:"last_update"`
	LastError      string `json:"last_error"`
	Health         string `json:"health"`
	LatencyMS      int    `json:"latency_ms"`
	LastCheck      string `json:"last_check"`
}

type Node struct {
	ID           string `json:"id"`
	Enabled      bool   `json:"enabled"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Address      string `json:"address"`
	Server       string `json:"server"`
	Port         int    `json:"port"`
	UUID         string `json:"uuid"`
	Password     string `json:"password"`
	Method       string `json:"method"`
	Security     string `json:"security"`
	TLS          bool   `json:"tls"`
	Flow         string `json:"flow"`
	Transport    string `json:"transport"`
	Host         string `json:"host"`
	Path         string `json:"path"`
	SNI          string `json:"sni"`
	ALPN         string `json:"alpn"`
	Insecure     bool   `json:"insecure"`
	Congestion   string `json:"congestion"`
	UDPRelayMode string `json:"udp_relay_mode"`
	Tag          string `json:"tag"`
	Subscription string `json:"subscription"`
	Health       string `json:"health"`
	LatencyMS    int    `json:"latency_ms"`
	LastCheck    string `json:"last_check"`
}

// DNSServer is an upstream resolver definition shared across groups.
type DNSServer struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
	Detour  string `json:"detour"`
}

// DNSRule selects a DNS server for traffic that matches a set of source IPs
// and/or rule sets. It renders to a sing-box dns.rules entry.
type DNSRule struct {
	ID       string   `json:"id"`
	Enabled  bool     `json:"enabled"`
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Sources  []string `json:"sources"`
	RuleSets []string `json:"rulesets"`
	Server   string   `json:"server"`
}

// RouteRule sends matching traffic to an outbound. A rule may match on source
// IPs, rule sets, or both (an AND match), which renders to a single sing-box
// route.rules entry.
type RouteRule struct {
	ID       string   `json:"id"`
	Enabled  bool     `json:"enabled"`
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Sources  []string `json:"sources"`
	RuleSets []string `json:"rulesets"`
	Outbound string   `json:"outbound"`
}

type RuleSet struct {
	ID             string `json:"id"`
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Format         string `json:"format"`
	URL            string `json:"url"`
	Path           string `json:"path"`
	UpdateInterval string `json:"update_interval"`
	LastUpdate     string `json:"last_update"`
	LastError      string `json:"last_error"`
}

type TProxy struct {
	Enabled       bool
	LANIfnames    []string
	IncludeSubnet []string
	ExcludeSubnet []string
	IncludeMAC    []string
	DNSHijack     bool
	KillSwitch    bool
}

type TUN struct {
	Enabled      bool
	AutoRoute    bool
	AutoRedirect bool
	Inet4Address string
	Inet6Address string
}

func DefaultConfig() Config {
	return Config{
		Manager: Manager{
			Enabled:        false,
			LogLevel:       "info",
			ActiveGroup:    "home",
			RuntimeMode:    "rule",
			SingBoxBinary:  DefaultSingBoxBinary,
			SocketPath:     DefaultSocketPath,
			APIListen:      "127.0.0.1:9090",
			MixedListen:    "0.0.0.0",
			MixedPort:      2080,
			TProxyPort:     7893,
			DNSPort:        1053,
			UpdateInterval: "24h",
		},
		Groups:        map[string]Group{},
		Subscriptions: map[string]Subscription{},
		Nodes:         map[string]Node{},
		DNSServers:    map[string]DNSServer{},
		DNSRules:      map[string]DNSRule{},
		RouteRules:    map[string]RouteRule{},
		RuleSets:      map[string]RuleSet{},
		TProxy: TProxy{
			Enabled: false,
		},
		TUN: TUN{
			Enabled:      false,
			AutoRoute:    true,
			AutoRedirect: true,
			Inet4Address: "172.19.0.1/30",
			Inet6Address: "fdfe:dcba:9876::1/126",
		},
	}
}

func (cfg Config) ActiveGroup() *Group {
	group, ok := cfg.Groups[cfg.Manager.ActiveGroup]
	if !ok {
		return nil
	}
	return &group
}

func (cfg Config) ActiveNode() *Node {
	group := cfg.ActiveGroup()
	if group == nil || group.SelectedNode == "" {
		return nil
	}
	node, ok := cfg.Nodes[group.SelectedNode]
	if !ok {
		return nil
	}
	return &node
}

// DNSRulesForGroup returns the enabled DNS rules bound to the given group,
// ordered by ID so first-match precedence is deterministic.
func (cfg Config) DNSRulesForGroup(groupID string) []DNSRule {
	rules := make([]DNSRule, 0, len(cfg.DNSRules))
	for _, rule := range cfg.DNSRules {
		if rule.Enabled && rule.Group == groupID {
			rules = append(rules, rule)
		}
	}
	sortByID(rules, func(r DNSRule) string { return r.ID })
	return rules
}

// RouteRulesForGroup returns the enabled route rules bound to the given group,
// ordered by ID so first-match precedence is deterministic.
func (cfg Config) RouteRulesForGroup(groupID string) []RouteRule {
	rules := make([]RouteRule, 0, len(cfg.RouteRules))
	for _, rule := range cfg.RouteRules {
		if rule.Enabled && rule.Group == groupID {
			rules = append(rules, rule)
		}
	}
	sortByID(rules, func(r RouteRule) string { return r.ID })
	return rules
}

func sortByID[T any](items []T, id func(T) string) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && id(items[j-1]) > id(items[j]); j-- {
			items[j-1], items[j] = items[j], items[j-1]
		}
	}
}
