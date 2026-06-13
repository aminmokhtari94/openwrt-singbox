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
	Routing       map[string]RoutingProfile
	DNSProfiles   map[string]DNSProfile
	DNSServers    map[string]DNSServer
	RuleSets      map[string]RuleSet
	SourceRules   map[string]SourceRule
	PAC           PAC
	CustomPACs    map[string]CustomPAC
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
	PACListen      string
	MixedListen    string
	MixedPort      int
	TProxyPort     int
	DNSPort        int
	UpdateInterval string
}

type Group struct {
	ID             string
	Enabled        bool
	Name           string
	Subscriptions  []string
	RoutingProfile string
	DNSProfile     string
	Strategy       string
	SelectedNode   string
	Health         string `json:"health"`
	LatencyMS      int    `json:"latency_ms"`
	LastCheck      string `json:"last_check"`
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

type RoutingProfile struct {
	ID       string   `json:"id"`
	Enabled  bool     `json:"enabled"`
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	RuleSets []string `json:"rulesets"`
	Final    string   `json:"final"`
}

type SourceRule struct {
	ID       string   `json:"id"`
	Enabled  bool     `json:"enabled"`
	Name     string   `json:"name"`
	Profile  string   `json:"profile"`
	Sources  []string `json:"sources"`
	Outbound string   `json:"outbound"`
}

type DNSProfile struct {
	ID      string   `json:"id"`
	Enabled bool     `json:"enabled"`
	Name    string   `json:"name"`
	Mode    string   `json:"mode"`
	Servers []string `json:"servers"`
	Hijack  bool     `json:"hijack"`
}

type DNSServer struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
	Detour  string `json:"detour"`
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

type PAC struct {
	Enabled        bool     `json:"enabled"`
	Source         string   `json:"source"`
	SelectedCustom string   `json:"selected_custom"`
	LocalBypass    bool     `json:"local_bypass"`
	CustomRules    []string `json:"custom_rules"`
	Whitelist      []string `json:"whitelist"`
	Blacklist      []string `json:"blacklist"`
}

type CustomPAC struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type TProxy struct {
	Enabled       bool
	LANIfnames    []string
	IncludeSubnet []string
	ExcludeSubnet []string
	IncludeMAC    []string
	DNSHijack     bool
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
			PACListen:      "0.0.0.0:1088",
			MixedListen:    "0.0.0.0",
			MixedPort:      2080,
			TProxyPort:     7893,
			DNSPort:        1053,
			UpdateInterval: "24h",
		},
		Groups:        map[string]Group{},
		Subscriptions: map[string]Subscription{},
		Nodes:         map[string]Node{},
		Routing:       map[string]RoutingProfile{},
		DNSProfiles:   map[string]DNSProfile{},
		DNSServers:    map[string]DNSServer{},
		RuleSets:      map[string]RuleSet{},
		SourceRules:   map[string]SourceRule{},
		CustomPACs:    map[string]CustomPAC{},
		PAC: PAC{
			Enabled:     false,
			Source:      "generated",
			LocalBypass: true,
		},
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

func (cfg Config) ActiveRoutingProfile() *RoutingProfile {
	group := cfg.ActiveGroup()
	if group == nil || group.RoutingProfile == "" {
		return nil
	}
	profile, ok := cfg.Routing[group.RoutingProfile]
	if !ok {
		return nil
	}
	return &profile
}

func (cfg Config) ActiveDNSProfile() *DNSProfile {
	group := cfg.ActiveGroup()
	if group == nil || group.DNSProfile == "" {
		return nil
	}
	profile, ok := cfg.DNSProfiles[group.DNSProfile]
	if !ok {
		return nil
	}
	return &profile
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
