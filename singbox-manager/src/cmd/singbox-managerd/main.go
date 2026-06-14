package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
	"github.com/openwrt-singbox/singbox-manager/internal/firewall"
	"github.com/openwrt-singbox/singbox-manager/internal/health"
	"github.com/openwrt-singbox/singbox-manager/internal/pac"
	"github.com/openwrt-singbox/singbox-manager/internal/render"
	"github.com/openwrt-singbox/singbox-manager/internal/ruleset"
	"github.com/openwrt-singbox/singbox-manager/internal/runtime"
	"github.com/openwrt-singbox/singbox-manager/internal/subscription"
)

const (
	defaultConfigPath = "/etc/config/singbox-manager"
	defaultSocketPath = managerconfig.DefaultSocketPath
)

type ManagerConfig struct {
	Enabled       bool   `json:"enabled"`
	ActiveGroup   string `json:"active_group"`
	RuntimeMode   string `json:"runtime_mode"`
	SelectedNode  string `json:"selected_node"`
	Strategy      string `json:"strategy"`
	Health        string `json:"health"`
	LatencyMS     int    `json:"latency_ms"`
	SocketPath    string `json:"socket_path"`
	SingBoxBinary string `json:"sing_box_bin"`
	PACListen     string `json:"pac_listen"`
	TProxyEnabled bool   `json:"tproxy_enabled"`
	DNSHijack     bool   `json:"dns_hijack"`
	KillSwitch    bool   `json:"kill_switch"`
	TUNEnabled    bool   `json:"tun_enabled"`
}

type Status struct {
	Daemon             bool   `json:"daemon"`
	ManagerEnabled     bool   `json:"manager_enabled"`
	Running            bool   `json:"running"`
	SingBoxPID         int    `json:"sing_box_pid"`
	ActiveGroup        string `json:"active_group"`
	SelectedProfile    string `json:"selected_profile"`
	SelectedOutbound   string `json:"selected_outbound"`
	RuntimeMode        string `json:"runtime_mode"`
	Strategy           string `json:"strategy"`
	Health             string `json:"health"`
	ActiveSubscription string `json:"active_subscription"`
	LatencyMS          int    `json:"latency_ms"`
	MemoryKB           uint64 `json:"memory_kb"`
	CPUPercent         string `json:"cpu_percent"`
	Connections        int    `json:"connections"`
	RxBytes            uint64 `json:"rx_bytes"`
	TxBytes            uint64 `json:"tx_bytes"`
	TProxyEnabled      bool   `json:"tproxy_enabled"`
	DNSHijack          bool   `json:"dns_hijack"`
	KillSwitch         bool   `json:"kill_switch"`
	NftablesInclude    string `json:"nftables_include"`
	TUNEnabled         bool   `json:"tun_enabled"`
}

type RPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type RPCError struct {
	Error string `json:"error"`
}

type IDParams struct {
	ID string `json:"id"`
}

type EnabledParams struct {
	Enabled bool `json:"enabled"`
}

type LatencyParams struct {
	URL string `json:"url"`
}

type NodeTestParams struct {
	ID  string `json:"id"`
	URL string `json:"url,omitempty"`
}

type DNSTestParams struct {
	Server string `json:"server"`
	Domain string `json:"domain"`
}

type ImportPayload struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Input          string `json:"input"`
	Format         string `json:"format"`
	UpdateInterval string `json:"update_interval"`
}

type SubscriptionPayload struct {
	ID             string `json:"id"`
	Enabled        *bool  `json:"enabled,omitempty"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Format         string `json:"format"`
	UpdateInterval string `json:"update_interval"`
	Group          string `json:"group"`
}

type NodePayload struct {
	ID           string `json:"id"`
	Enabled      *bool  `json:"enabled,omitempty"`
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
}

type DNSProfilePayload struct {
	ID      string   `json:"id"`
	Enabled *bool    `json:"enabled,omitempty"`
	Name    string   `json:"name"`
	Mode    string   `json:"mode"`
	Servers []string `json:"servers"`
	Hijack  bool     `json:"hijack"`
}

type DNSServerPayload struct {
	ID      string `json:"id"`
	Enabled *bool  `json:"enabled,omitempty"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
	Detour  string `json:"detour"`
}

type RoutingProfilePayload struct {
	ID       string   `json:"id"`
	Enabled  *bool    `json:"enabled,omitempty"`
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	RuleSets []string `json:"rulesets"`
	Final    string   `json:"final"`
}

type RuleSetPayload struct {
	ID             string `json:"id"`
	Enabled        *bool  `json:"enabled,omitempty"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Format         string `json:"format"`
	URL            string `json:"url"`
	Path           string `json:"path"`
	UpdateInterval string `json:"update_interval"`
}

type SourceRulePayload struct {
	ID       string   `json:"id"`
	Enabled  *bool    `json:"enabled,omitempty"`
	Name     string   `json:"name"`
	Profile  string   `json:"profile"`
	Sources  []string `json:"sources"`
	Outbound string   `json:"outbound"`
}

type PACSettingsPayload struct {
	Enabled        *bool    `json:"enabled,omitempty"`
	Source         string   `json:"source"`
	SelectedCustom string   `json:"selected_custom"`
	LocalBypass    *bool    `json:"local_bypass,omitempty"`
	CustomRules    []string `json:"custom_rules"`
	Whitelist      []string `json:"whitelist"`
	Blacklist      []string `json:"blacklist"`
}

type CustomPACPayload struct {
	ID      string `json:"id"`
	Enabled *bool  `json:"enabled,omitempty"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type LogsParams struct {
	Lines int `json:"lines"`
}

type RuntimeStats struct {
	Connections int    `json:"connections"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
}

type Device struct {
	IP     string `json:"ip"`
	MAC    string `json:"mac"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

func main() {
	if filepath.Base(os.Args[0]) == "singbox.manager" {
		runRPCD(os.Args[1:])
		return
	}

	if len(os.Args) < 2 {
		runServe(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "cleanup":
		runCleanup(os.Args[2:])
	case "rpcd":
		runRPCD(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "UCI config path")
	socketPath := fs.String("socket", "", "Unix socket path")
	_ = fs.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Printf("failed to load config: %v", err)
		cfg = defaultConfig()
	}
	if *socketPath != "" {
		cfg.SocketPath = *socketPath
	}

	startPACServer(*configPath, cfg.PACListen)

	if err := os.MkdirAll(filepath.Dir(cfg.SocketPath), 0755); err != nil {
		log.Fatalf("failed to create socket directory: %v", err)
	}
	_ = os.Remove(cfg.SocketPath)

	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.SocketPath, err)
	}
	defer listener.Close()
	if err := os.Chmod(cfg.SocketPath, 0660); err != nil {
		log.Printf("failed to chmod socket: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		handleRPC(w, r, *configPath)
	})

	startHealthScheduler(*configPath)
	startSubscriptionScheduler(*configPath)
	startRuleSetScheduler(*configPath)
	startRuntimeSupervisor(*configPath)
	startSignalCleanup(*configPath)

	log.Printf("singbox-managerd listening on %s", cfg.SocketPath)
	if err := http.Serve(listener, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}

func runCleanup(args []string) {
	fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "UCI config path")
	_ = fs.Parse(args)

	if err := cleanupRuntime(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func startSignalCleanup(configPath string) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signals
		if err := cleanupRuntime(configPath); err != nil {
			log.Printf("runtime cleanup after %s failed: %v", sig, err)
		}
		os.Exit(0)
	}()
}

func cleanupRuntime(configPath string) error {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		defaultCfg := managerconfig.DefaultConfig()
		cfg = &defaultCfg
	}
	_, err = runtime.Control(*cfg, runtime.ActionTeardown, runtime.DefaultPaths, render.Render)
	return err
}

func startRuntimeSupervisor(configPath string) {
	if managedRuntimePID(runtime.DefaultPaths) == 0 {
		return
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		log.Printf("runtime supervision skipped: %v", err)
		return
	}
	runtime.Supervise(*cfg, runtime.DefaultPaths, render.Render)
	log.Printf("runtime supervision attached to existing sing-box process")
}

func startSubscriptionScheduler(configPath string) {
	go func() {
		delay := 15 * time.Second
		for {
			time.Sleep(delay)
			delay = time.Minute
			cfg, err := managerconfig.Load(configPath)
			if err != nil {
				log.Printf("scheduled subscription update skipped: %v", err)
				continue
			}
			if !cfg.Manager.Enabled {
				continue
			}
			now := time.Now().UTC()
			for _, source := range cfg.Subscriptions {
				if !subscription.Due(source, now) {
					continue
				}
				imported, err := refreshSubscriptionByID(configPath, source.ID)
				if err != nil {
					log.Printf("scheduled subscription %s update failed: %v", source.ID, err)
					continue
				}
				log.Printf("scheduled subscription %s updated: %d nodes", source.ID, imported)
			}
		}
	}()
}

func startPACServer(configPath string, listen string) {
	if listen == "" {
		return
	}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/proxy.pac", func(w http.ResponseWriter, r *http.Request) {
			cfg, err := managerconfig.Load(configPath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !cfg.PAC.Enabled {
				http.Error(w, "PAC is disabled", http.StatusNotFound)
				return
			}
			data, err := pac.RenderActiveForProxyHost(*cfg, r.Host)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(data)
		})
		log.Printf("PAC server listening on %s", listen)
		if err := http.ListenAndServe(listen, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("PAC server stopped: %v", err)
		}
	}()
}

func startHealthScheduler(configPath string) {
	go func() {
		delay := 5 * time.Second
		for {
			time.Sleep(delay)
			cfg, err := managerconfig.Load(configPath)
			if err != nil {
				log.Printf("scheduled health check skipped: %v", err)
				delay = time.Hour
				continue
			}
			delay = healthInterval(*cfg)
			if !cfg.Manager.Enabled {
				continue
			}
			result := health.Check(context.Background(), *cfg)
			nodes, groups, subscriptions := health.ToHealthStates(result)
			if err := managerconfig.UpdateHealth(configPath, nodes, groups, subscriptions); err != nil {
				log.Printf("scheduled health check failed: %v", err)
			}
		}
	}()
}

func healthInterval(cfg managerconfig.Config) time.Duration {
	interval, err := time.ParseDuration(cfg.Manager.UpdateInterval)
	if err != nil || interval <= 0 {
		return 24 * time.Hour
	}
	if interval < time.Minute {
		return time.Minute
	}
	return interval
}

func startRuleSetScheduler(configPath string) {
	go func() {
		delay := 10 * time.Second
		for {
			time.Sleep(delay)
			delay = time.Hour
			cfg, err := managerconfig.Load(configPath)
			if err != nil {
				log.Printf("scheduled ruleset update skipped: %v", err)
				continue
			}
			if !cfg.Manager.Enabled {
				continue
			}
			for _, entry := range cfg.RuleSets {
				if !ruleset.Due(entry, time.Now().UTC()) {
					continue
				}
				result, err := ruleset.Download(context.Background(), entry)
				if err != nil {
					_ = managerconfig.MarkRuleSetError(configPath, entry.ID, err.Error())
					log.Printf("scheduled ruleset %s update failed: %v", entry.ID, err)
					continue
				}
				_ = managerconfig.MarkRuleSetUpdated(configPath, entry.ID)
				log.Printf("scheduled ruleset %s updated: %d bytes at %s", entry.ID, result.Bytes, result.Path)
			}
		}
	}()
}

func runRPCD(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: singbox-managerd rpcd <list|call>")
		os.Exit(2)
	}

	switch args[0] {
	case "list":
		writeJSON(map[string]map[string]any{
			"status":                 {},
			"start":                  {},
			"stop":                   {},
			"restart":                {},
			"reload":                 {},
			"validate":               {},
			"manager_set_enabled":    {"enabled": "boolean"},
			"subscriptions":          {},
			"subscription_set":       {"subscription": "object"},
			"subscription_delete":    {"id": "string"},
			"subscription_import":    {"request": "object"},
			"refresh_subscription":   {"id": "string"},
			"refresh_subscriptions":  {},
			"nodes":                  {},
			"node_set":               {"node": "object"},
			"node_delete":            {"id": "string"},
			"node_select":            {"id": "string"},
			"node_ping_test":         {"id": "string"},
			"node_latency_test":      {"id": "string", "url": "string"},
			"health_check":           {},
			"latency_test":           {"url": "string"},
			"dns":                    {},
			"dns_profile_set":        {"profile": "object"},
			"dns_profile_delete":     {"id": "string"},
			"dns_server_set":         {"server": "object"},
			"dns_server_delete":      {"id": "string"},
			"dns_test":               {"server": "string", "domain": "string"},
			"rulesets":               {},
			"routing_profile_set":    {"profile": "object"},
			"routing_profile_delete": {"id": "string"},
			"ruleset_set":            {"ruleset": "object"},
			"ruleset_delete":         {"id": "string"},
			"refresh_ruleset":        {"id": "string"},
			"source_rule_set":        {"rule": "object"},
			"source_rule_delete":     {"id": "string"},
			"pac":                    {},
			"pac_set":                {"pac": "object"},
			"pac_custom_set":         {"pac": "object"},
			"pac_custom_delete":      {"id": "string"},
			"pac_custom_save":        {"pac": "object"},
			"runtime_stats":          {},
			"logs":                   {"lines": "number"},
			"devices":                {},
			"tproxy":                 {},
			"tun":                    {},
		})
	case "call":
		if len(args) < 2 {
			writeJSON(RPCError{Error: "missing method"})
			os.Exit(1)
		}
		callDaemon(args[1], os.Stdin)
	default:
		writeJSON(RPCError{Error: "unsupported rpcd command"})
		os.Exit(1)
	}
}

func callDaemon(method string, input io.Reader) {
	cfg, err := loadConfig(defaultConfigPath)
	if err != nil {
		cfg = defaultConfig()
	}

	var params json.RawMessage
	body, _ := io.ReadAll(input)
	if len(strings.TrimSpace(string(body))) > 0 {
		params = body
	}

	reqBody, _ := json.Marshal(RPCRequest{Method: method, Params: params})
	client := http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				dialer := net.Dialer{}
				return dialer.DialContext(ctx, "unix", cfg.SocketPath)
			},
		},
	}

	resp, err := client.Post("http://unix/rpc", "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		if method == "status" {
			writeJSON(statusUnavailable(cfg, err))
			return
		}
		writeJSON(RPCError{Error: err.Error()})
		os.Exit(1)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		writeJSON(RPCError{Error: err.Error()})
		os.Exit(1)
	}
}

func handleRPC(w http.ResponseWriter, r *http.Request, configPath string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPJSON(w, http.StatusBadRequest, RPCError{Error: err.Error()})
		return
	}

	switch req.Method {
	case "status":
		cfg, err := loadConfig(configPath)
		if err != nil {
			cfg = defaultConfig()
		}
		writeHTTPJSON(w, http.StatusOK, collectStatus(cfg))
	case "validate":
		writeHTTPJSON(w, http.StatusOK, validateRuntimeConfig(configPath))
	case "manager_set_enabled":
		writeHTTPJSON(w, http.StatusOK, setManagerEnabled(configPath, req.Params))
	case "subscriptions":
		writeHTTPJSON(w, http.StatusOK, listSubscriptions(configPath))
	case "subscription_set":
		writeHTTPJSON(w, http.StatusOK, setSubscription(configPath, req.Params))
	case "subscription_delete":
		writeHTTPJSON(w, http.StatusOK, deleteSubscription(configPath, req.Params))
	case "subscription_import":
		writeHTTPJSON(w, http.StatusOK, importSubscription(configPath, req.Params))
	case "refresh_subscription":
		writeHTTPJSON(w, http.StatusOK, refreshSubscription(configPath, req.Params))
	case "refresh_subscriptions":
		writeHTTPJSON(w, http.StatusOK, refreshSubscriptions(configPath))
	case "nodes":
		writeHTTPJSON(w, http.StatusOK, listNodes(configPath))
	case "node_set":
		writeHTTPJSON(w, http.StatusOK, setNode(configPath, req.Params))
	case "node_delete":
		writeHTTPJSON(w, http.StatusOK, deleteNode(configPath, req.Params))
	case "node_select":
		writeHTTPJSON(w, http.StatusOK, selectNode(configPath, req.Params))
	case "node_ping_test":
		writeHTTPJSON(w, http.StatusOK, nodePingTest(configPath, req.Params))
	case "node_latency_test":
		writeHTTPJSON(w, http.StatusOK, nodeLatencyTest(configPath, req.Params))
	case "health_check":
		writeHTTPJSON(w, http.StatusOK, healthCheck(configPath))
	case "latency_test":
		writeHTTPJSON(w, http.StatusOK, latencyTest(req.Params))
	case "dns":
		writeHTTPJSON(w, http.StatusOK, listDNS(configPath))
	case "dns_profile_set":
		writeHTTPJSON(w, http.StatusOK, setDNSProfile(configPath, req.Params))
	case "dns_profile_delete":
		writeHTTPJSON(w, http.StatusOK, deleteDNSProfile(configPath, req.Params))
	case "dns_server_set":
		writeHTTPJSON(w, http.StatusOK, setDNSServer(configPath, req.Params))
	case "dns_server_delete":
		writeHTTPJSON(w, http.StatusOK, deleteDNSServer(configPath, req.Params))
	case "dns_test":
		writeHTTPJSON(w, http.StatusOK, dnsTest(configPath, req.Params))
	case "rulesets":
		writeHTTPJSON(w, http.StatusOK, listRuleSets(configPath))
	case "routing_profile_set":
		writeHTTPJSON(w, http.StatusOK, setRoutingProfile(configPath, req.Params))
	case "routing_profile_delete":
		writeHTTPJSON(w, http.StatusOK, deleteRoutingProfile(configPath, req.Params))
	case "ruleset_set":
		writeHTTPJSON(w, http.StatusOK, setRuleSet(configPath, req.Params))
	case "ruleset_delete":
		writeHTTPJSON(w, http.StatusOK, deleteRuleSet(configPath, req.Params))
	case "refresh_ruleset":
		writeHTTPJSON(w, http.StatusOK, refreshRuleSet(configPath, req.Params))
	case "source_rule_set":
		writeHTTPJSON(w, http.StatusOK, setSourceRule(configPath, req.Params))
	case "source_rule_delete":
		writeHTTPJSON(w, http.StatusOK, deleteSourceRule(configPath, req.Params))
	case "pac":
		writeHTTPJSON(w, http.StatusOK, pacStatus(configPath))
	case "pac_set":
		writeHTTPJSON(w, http.StatusOK, setPAC(configPath, req.Params))
	case "pac_custom_set":
		writeHTTPJSON(w, http.StatusOK, setCustomPAC(configPath, req.Params))
	case "pac_custom_delete":
		writeHTTPJSON(w, http.StatusOK, deleteCustomPAC(configPath, req.Params))
	case "pac_custom_save":
		writeHTTPJSON(w, http.StatusOK, saveRenderedPAC(configPath, req.Params))
	case "runtime_stats":
		writeHTTPJSON(w, http.StatusOK, runtimeStats())
	case "logs":
		writeHTTPJSON(w, http.StatusOK, logs(req.Params))
	case "devices":
		writeHTTPJSON(w, http.StatusOK, devices())
	case "tproxy":
		writeHTTPJSON(w, http.StatusOK, tproxyStatus(configPath))
	case "tun":
		writeHTTPJSON(w, http.StatusOK, tunStatus(configPath))
	case "start":
		writeHTTPJSON(w, http.StatusOK, controlRuntime(configPath, runtime.ActionStart))
	case "stop":
		writeHTTPJSON(w, http.StatusOK, controlRuntime(configPath, runtime.ActionStop))
	case "restart":
		writeHTTPJSON(w, http.StatusOK, controlRuntime(configPath, runtime.ActionRestart))
	case "reload":
		writeHTTPJSON(w, http.StatusOK, controlRuntime(configPath, runtime.ActionReload))
	default:
		writeHTTPJSON(w, http.StatusNotFound, RPCError{Error: "unknown method"})
	}
}

func listSubscriptions(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	subscriptions := make([]managerconfig.Subscription, 0, len(cfg.Subscriptions))
	for _, source := range cfg.Subscriptions {
		subscriptions = append(subscriptions, source)
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].ID < subscriptions[j].ID
	})
	return map[string]any{"ok": true, "subscriptions": subscriptions}
}

func setManagerEnabled(configPath string, raw json.RawMessage) map[string]any {
	var params EnabledParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return validationResult(false, runtime.Result{}, err)
		}
	}
	if err := managerconfig.SetManagerEnabled(configPath, params.Enabled); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "enabled": params.Enabled}
}

func setSubscription(configPath string, raw json.RawMessage) map[string]any {
	subscription, groupID, err := decodeSubscription(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertSubscription(configPath, subscription, groupID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "subscription": subscription.ID}
}

func deleteSubscription(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteSubscription(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "subscription": params.ID}
}

func importSubscription(configPath string, raw json.RawMessage) map[string]any {
	payload, err := decodeImportPayload(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	input := strings.TrimSpace(payload.Input)
	if input == "" {
		return validationResult(false, runtime.Result{}, fmt.Errorf("subscription or config link is required"))
	}
	if payload.Format == "" {
		payload.Format = "auto"
	}
	if payload.UpdateInterval == "" {
		payload.UpdateInterval = "24h"
	}

	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	id := payload.ID
	if id == "" {
		id = generatedImportID(input)
	}
	name := firstNonEmpty(payload.Name, id)

	if isRemoteSubscriptionInput(input) {
		entry := managerconfig.Subscription{
			ID:             id,
			Enabled:        true,
			Name:           name,
			URL:            input,
			Format:         payload.Format,
			UpdateInterval: payload.UpdateInterval,
			Health:         "unknown",
		}
		if err := managerconfig.UpsertSubscription(configPath, entry, cfg.Manager.ActiveGroup); err != nil {
			return validationResult(false, runtime.Result{}, err)
		}
		result := refreshSubscription(configPath, mustMarshal(IDParams{ID: id}))
		result["subscription"] = id
		result["saved"] = true
		result["remote"] = true
		return result
	}

	nodes, err := subscription.Parse([]byte(input), payload.Format)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	entry := managerconfig.Subscription{
		ID:             id,
		Enabled:        false,
		Name:           name,
		Format:         "plain",
		UpdateInterval: payload.UpdateInterval,
		Health:         "ok",
	}
	if err := managerconfig.UpsertSubscription(configPath, entry, cfg.Manager.ActiveGroup); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	nodes = subscription.AssignSource(nodes, id)
	if err := managerconfig.ReplaceSubscriptionNodes(configPath, id, nodes); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "subscription": id, "saved": true, "remote": false, "imported": len(nodes)}
}

func refreshSubscription(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	imported, err := refreshSubscriptionByID(configPath, params.ID)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "subscription": params.ID, "imported": imported}
}

func refreshSubscriptions(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	refreshed := 0
	imported := 0
	failures := []map[string]string{}
	for _, source := range cfg.Subscriptions {
		if !source.Enabled {
			continue
		}
		count, err := refreshSubscriptionByID(configPath, source.ID)
		if err != nil {
			failures = append(failures, map[string]string{"id": source.ID, "error": err.Error()})
			continue
		}
		refreshed++
		imported += count
	}
	return map[string]any{
		"ok":        len(failures) == 0,
		"refreshed": refreshed,
		"imported":  imported,
		"failures":  failures,
	}
}

func refreshSubscriptionByID(configPath string, id string) (int, error) {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return 0, err
	}
	source, ok := cfg.Subscriptions[id]
	if !ok {
		return 0, fmt.Errorf("subscription %q not found", id)
	}
	if !source.Enabled {
		return 0, fmt.Errorf("subscription %q is disabled", id)
	}

	data, err := subscription.Fetch(context.Background(), source.URL)
	if err != nil {
		_ = managerconfig.MarkSubscriptionError(configPath, id, err.Error())
		return 0, err
	}
	nodes, err := subscription.Parse(data, source.Format)
	if err != nil {
		_ = managerconfig.MarkSubscriptionError(configPath, id, err.Error())
		return 0, err
	}
	nodes = subscription.AssignSource(nodes, id)
	if err := managerconfig.ReplaceSubscriptionNodes(configPath, id, nodes); err != nil {
		return 0, err
	}
	return len(nodes), nil
}

func listNodes(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	nodes := make([]managerconfig.Node, 0, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	subscriptions := make([]managerconfig.Subscription, 0, len(cfg.Subscriptions))
	for _, source := range cfg.Subscriptions {
		subscriptions = append(subscriptions, source)
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].ID < subscriptions[j].ID
	})
	selected := ""
	strategy := ""
	group := cfg.ActiveGroup()
	if group != nil {
		selected = group.SelectedNode
		strategy = group.Strategy
	}
	return map[string]any{
		"ok":            true,
		"active_group":  cfg.Manager.ActiveGroup,
		"selected_node": selected,
		"strategy":      strategy,
		"nodes":         nodes,
		"subscriptions": subscriptions,
	}
}

func listDNS(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	profiles := make([]managerconfig.DNSProfile, 0, len(cfg.DNSProfiles))
	for _, profile := range cfg.DNSProfiles {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].ID < profiles[j].ID
	})
	servers := make([]managerconfig.DNSServer, 0, len(cfg.DNSServers))
	for _, server := range cfg.DNSServers {
		servers = append(servers, server)
	}
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].ID < servers[j].ID
	})
	return map[string]any{"ok": true, "profiles": profiles, "servers": servers}
}

func setDNSProfile(configPath string, raw json.RawMessage) map[string]any {
	profile, err := decodeDNSProfile(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertDNSProfile(configPath, profile); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "profile": profile.ID}
}

func deleteDNSProfile(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteDNSProfile(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "profile": params.ID}
}

func setDNSServer(configPath string, raw json.RawMessage) map[string]any {
	server, err := decodeDNSServer(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertDNSServer(configPath, server); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "server": server.ID}
}

func deleteDNSServer(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteDNSServer(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "server": params.ID}
}

func listRuleSets(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	profiles := make([]managerconfig.RoutingProfile, 0, len(cfg.Routing))
	for _, profile := range cfg.Routing {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].ID < profiles[j].ID
	})
	sets := make([]managerconfig.RuleSet, 0, len(cfg.RuleSets))
	for _, set := range cfg.RuleSets {
		if set.Path == "" {
			set.Path = ruleset.Path(set)
		}
		sets = append(sets, set)
	}
	sort.Slice(sets, func(i, j int) bool {
		return sets[i].ID < sets[j].ID
	})
	sourceRules := make([]managerconfig.SourceRule, 0, len(cfg.SourceRules))
	for _, rule := range cfg.SourceRules {
		sourceRules = append(sourceRules, rule)
	}
	sort.Slice(sourceRules, func(i, j int) bool {
		return sourceRules[i].ID < sourceRules[j].ID
	})
	return map[string]any{"ok": true, "profiles": profiles, "rulesets": sets, "source_rules": sourceRules, "devices": discoverDevices()}
}

func setRoutingProfile(configPath string, raw json.RawMessage) map[string]any {
	profile, err := decodeRoutingProfile(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertRoutingProfile(configPath, profile); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "profile": profile.ID}
}

func deleteRoutingProfile(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteRoutingProfile(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "profile": params.ID}
}

func setRuleSet(configPath string, raw json.RawMessage) map[string]any {
	entry, err := decodeRuleSet(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertRuleSet(configPath, entry); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "ruleset": entry.ID}
}

func deleteRuleSet(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteRuleSet(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "ruleset": params.ID}
}

func refreshRuleSet(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	entry, ok := cfg.RuleSets[params.ID]
	if !ok {
		return validationResult(false, runtime.Result{}, fmt.Errorf("ruleset %q not found", params.ID))
	}
	result, err := ruleset.Download(context.Background(), entry)
	if err != nil {
		_ = managerconfig.MarkRuleSetError(configPath, params.ID, err.Error())
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.MarkRuleSetUpdated(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "id": result.ID, "path": result.Path, "bytes": result.Bytes}
}

func setSourceRule(configPath string, raw json.RawMessage) map[string]any {
	rule, err := decodeSourceRule(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertSourceRule(configPath, rule); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "rule": rule.ID}
}

func deleteSourceRule(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteSourceRule(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "rule": params.ID}
}

func pacStatus(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	generated, err := pac.Render(*cfg)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	active, err := pac.RenderActive(*cfg)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	customs := make([]managerconfig.CustomPAC, 0, len(cfg.CustomPACs))
	for _, entry := range cfg.CustomPACs {
		customs = append(customs, entry)
	}
	sort.Slice(customs, func(i, j int) bool {
		return customs[i].ID < customs[j].ID
	})
	return map[string]any{
		"ok":                true,
		"enabled":           cfg.PAC.Enabled,
		"listen":            cfg.Manager.PACListen,
		"url":               "http://" + cfg.Manager.PACListen + "/proxy.pac",
		"source":            cfg.PAC.Source,
		"selected_custom":   cfg.PAC.SelectedCustom,
		"local_bypass":      cfg.PAC.LocalBypass,
		"custom_rules":      cfg.PAC.CustomRules,
		"whitelist":         cfg.PAC.Whitelist,
		"blacklist":         cfg.PAC.Blacklist,
		"custom_pacs":       customs,
		"preview":           string(active),
		"generated_preview": string(generated),
	}
}

func setPAC(configPath string, raw json.RawMessage) map[string]any {
	settings, err := decodePACSettings(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpdatePAC(configPath, settings); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true}
}

func setCustomPAC(configPath string, raw json.RawMessage) map[string]any {
	entry, err := decodeCustomPAC(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertCustomPAC(configPath, entry); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "pac": entry.ID}
}

func deleteCustomPAC(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteCustomPAC(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "pac": params.ID}
}

func saveRenderedPAC(configPath string, raw json.RawMessage) map[string]any {
	entry, err := decodeCustomPAC(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	data, err := pac.Render(*cfg)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	entry.Content = string(data)
	if err := managerconfig.UpsertCustomPAC(configPath, entry); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "pac": entry.ID}
}

func runtimeStats() map[string]any {
	stats := collectRuntimeStats()
	return map[string]any{
		"ok":          true,
		"connections": stats.Connections,
		"rx_bytes":    stats.RxBytes,
		"tx_bytes":    stats.TxBytes,
	}
}

func logs(raw json.RawMessage) map[string]any {
	params := LogsParams{Lines: 200}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &params)
	}
	if params.Lines <= 0 {
		params.Lines = 200
	}
	if params.Lines > 2000 {
		params.Lines = 2000
	}
	text := readManagerLogs(params.Lines)
	return map[string]any{"ok": true, "lines": params.Lines, "text": text}
}

func devices() map[string]any {
	return map[string]any{"ok": true, "devices": discoverDevices()}
}

func tproxyStatus(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	preview := ""
	if cfg.TProxy.Enabled {
		data, err := firewall.Render(*cfg)
		if err != nil {
			return validationResult(false, runtime.Result{}, err)
		}
		preview = string(data)
	}
	return map[string]any{
		"ok":                 true,
		"enabled":            cfg.TProxy.Enabled,
		"lan_ifnames":        cfg.TProxy.LANIfnames,
		"include_subnet":     cfg.TProxy.IncludeSubnet,
		"exclude_subnet":     cfg.TProxy.ExcludeSubnet,
		"include_mac":        cfg.TProxy.IncludeMAC,
		"dns_hijack":         cfg.TProxy.DNSHijack,
		"kill_switch":        cfg.TProxy.KillSwitch,
		"tproxy_port":        cfg.Manager.TProxyPort,
		"dns_port":           cfg.Manager.DNSPort,
		"nftables_include":   runtime.DefaultPaths.NftablesInclude,
		"nftables_present":   fileExists(runtime.DefaultPaths.NftablesInclude),
		"nftables_preview":   preview,
		"tproxy_inbound_tag": "tproxy-in",
	}
}

func tunStatus(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{
		"ok":            true,
		"enabled":       cfg.TUN.Enabled,
		"auto_route":    cfg.TUN.AutoRoute,
		"auto_redirect": cfg.TUN.AutoRedirect,
		"inet4_address": cfg.TUN.Inet4Address,
		"inet6_address": cfg.TUN.Inet6Address,
		"interface":     "singbox0",
	}
}

func setNode(configPath string, raw json.RawMessage) map[string]any {
	node, err := decodeNode(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.UpsertManualNode(configPath, node); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "node": node.ID}
}

func deleteNode(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	if err := managerconfig.DeleteManualNode(configPath, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "node": params.ID}
}

func selectNode(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeIDParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	group := cfg.ActiveGroup()
	if group == nil {
		return validationResult(false, runtime.Result{}, fmt.Errorf("active group %q not found", cfg.Manager.ActiveGroup))
	}
	node, ok := cfg.Nodes[params.ID]
	if !ok {
		return validationResult(false, runtime.Result{}, fmt.Errorf("node %q not found", params.ID))
	}
	if !node.Enabled {
		return validationResult(false, runtime.Result{}, fmt.Errorf("node %q is disabled", params.ID))
	}
	if !nodeAvailableForGroup(node, *group) {
		return validationResult(false, runtime.Result{}, fmt.Errorf("node %q is not available in active group %q", params.ID, cfg.Manager.ActiveGroup))
	}
	if err := managerconfig.SelectNode(configPath, cfg.Manager.ActiveGroup, params.ID); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{"ok": true, "active_group": cfg.Manager.ActiveGroup, "node": params.ID}
}

func nodeAvailableForGroup(node managerconfig.Node, group managerconfig.Group) bool {
	if len(group.Subscriptions) == 0 || node.Subscription == "" {
		return true
	}
	for _, subscription := range group.Subscriptions {
		if subscription == node.Subscription {
			return true
		}
	}
	return false
}

func nodePingTest(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeNodeTestParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	node, ok := cfg.Nodes[params.ID]
	if !ok {
		return validationResult(false, runtime.Result{}, fmt.Errorf("node %q not found", params.ID))
	}

	result := health.PingNode(context.Background(), node)
	nodes := map[string]managerconfig.HealthState{
		params.ID: {Health: result.Health, LatencyMS: result.LatencyMS},
	}
	if err := managerconfig.UpdateHealth(configPath, nodes, nil, nil); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}

	response := map[string]any{
		"ok":         result.Error == "",
		"node":       result.ID,
		"health":     result.Health,
		"latency_ms": result.LatencyMS,
		"method":     result.Method,
	}
	if result.Error != "" {
		response["errors"] = []string{result.Error}
	} else {
		response["errors"] = []string{}
	}
	return response
}

func nodeLatencyTest(configPath string, raw json.RawMessage) map[string]any {
	params, err := decodeNodeTestParams(raw)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	node, ok := cfg.Nodes[params.ID]
	if !ok {
		return validationResult(false, runtime.Result{}, fmt.Errorf("node %q not found", params.ID))
	}

	result, err := health.TestNodeURL(context.Background(), node, params.URL)
	nodes := map[string]managerconfig.HealthState{
		params.ID: {Health: result.Health, LatencyMS: result.LatencyMS},
	}
	if updateErr := managerconfig.UpdateHealth(configPath, nodes, nil, nil); updateErr != nil {
		return validationResult(false, runtime.Result{}, updateErr)
	}

	response := map[string]any{
		"ok":         err == nil,
		"node":       result.ID,
		"url":        firstNonEmpty(params.URL, health.DefaultTestURL),
		"health":     result.Health,
		"latency_ms": result.LatencyMS,
		"method":     result.Method,
	}
	if err != nil {
		response["errors"] = []string{err.Error()}
	} else {
		response["errors"] = []string{}
	}
	return response
}

func healthCheck(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	result := health.Check(context.Background(), *cfg)
	nodes, groups, subscriptions := health.ToHealthStates(result)
	if err := managerconfig.UpdateHealth(configPath, nodes, groups, subscriptions); err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	return map[string]any{
		"ok":            true,
		"nodes":         result.Nodes,
		"groups":        result.Groups,
		"subscriptions": result.Subscriptions,
	}
}

func latencyTest(raw json.RawMessage) map[string]any {
	var params LatencyParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return validationResult(false, runtime.Result{}, err)
		}
	}
	result, err := health.TestURL(context.Background(), params.URL)
	response := map[string]any{
		"ok":         err == nil,
		"url":        result.ID,
		"health":     result.Health,
		"latency_ms": result.LatencyMS,
	}
	if err != nil {
		response["errors"] = []string{err.Error()}
	} else {
		response["errors"] = []string{}
	}
	return response
}

func dnsTest(configPath string, raw json.RawMessage) map[string]any {
	var params DNSTestParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return validationResult(false, runtime.Result{}, err)
		}
	}
	if params.Server == "" {
		return validationResult(false, runtime.Result{}, fmt.Errorf("server is required"))
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}
	server, ok := cfg.DNSServers[params.Server]
	if !ok {
		return validationResult(false, runtime.Result{}, fmt.Errorf("dns server %q not found", params.Server))
	}
	result, err := health.TestDNS(context.Background(), server, params.Domain)
	response := map[string]any{
		"ok":         err == nil,
		"server":     result.ID,
		"domain":     params.Domain,
		"health":     result.Health,
		"latency_ms": result.LatencyMS,
	}
	if err != nil {
		response["errors"] = []string{err.Error()}
	} else {
		response["errors"] = []string{}
	}
	return response
}

func validateRuntimeConfig(configPath string) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}

	result, err := runtime.Validate(*cfg, runtime.DefaultPaths, render.Render)
	return validationResult(err == nil, result, err)
}

func controlRuntime(configPath string, action runtime.Action) map[string]any {
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		return validationResult(false, runtime.Result{}, err)
	}

	result, err := runtime.Control(*cfg, action, runtime.DefaultPaths, render.Render)
	response := validationResult(err == nil, result, err)
	if result.Message != "" {
		response["message"] = result.Message
	}
	if result.PID > 0 {
		response["pid"] = result.PID
	}
	return response
}

func validationResult(ok bool, result runtime.Result, err error) map[string]any {
	errors := []string{}
	if err != nil {
		errors = append(errors, managerconfig.ErrorStrings(err)...)
	}

	response := map[string]any{
		"ok":             ok,
		"errors":         errors,
		"generated_path": result.GeneratedPath,
		"runtime_path":   result.RuntimePath,
		"nftables_path":  result.NftablesPath,
		"check_output":   strings.TrimSpace(result.CheckOutput),
	}
	return response
}

func loadConfig(path string) (ManagerConfig, error) {
	cfg, err := managerconfig.Load(path)
	if err != nil {
		return defaultConfig(), err
	}
	return compactConfig(*cfg), nil
}

func compactConfig(cfg managerconfig.Config) ManagerConfig {
	group := cfg.ActiveGroup()
	selected := ""
	strategy := "manual"
	healthState := "unknown"
	latency := 0
	if group != nil {
		selected = group.SelectedNode
		strategy = group.Strategy
		healthState = group.Health
		latency = group.LatencyMS
		if selected != "" {
			if node, ok := cfg.Nodes[selected]; ok {
				if node.Health != "" {
					healthState = node.Health
				}
				if node.LatencyMS > 0 {
					latency = node.LatencyMS
				}
			}
		}
	}
	return ManagerConfig{
		Enabled:       cfg.Manager.Enabled,
		ActiveGroup:   cfg.Manager.ActiveGroup,
		RuntimeMode:   cfg.Manager.RuntimeMode,
		SelectedNode:  selected,
		Strategy:      strategy,
		Health:        healthState,
		LatencyMS:     latency,
		SocketPath:    cfg.Manager.SocketPath,
		SingBoxBinary: cfg.Manager.SingBoxBinary,
		PACListen:     cfg.Manager.PACListen,
		TProxyEnabled: cfg.TProxy.Enabled,
		DNSHijack:     cfg.TProxy.DNSHijack,
		KillSwitch:    cfg.TProxy.KillSwitch,
		TUNEnabled:    cfg.TUN.Enabled,
	}
}

func defaultConfig() ManagerConfig {
	cfg := managerconfig.DefaultConfig()
	return compactConfig(cfg)
}

func collectStatus(cfg ManagerConfig) Status {
	pid := managedRuntimePID(runtime.DefaultPaths)
	memoryKB := uint64(0)
	if pid > 0 {
		memoryKB = readRSS(pid)
	}
	stats := collectRuntimeStats()

	selected := cfg.SelectedNode
	if selected == "" {
		selected = "auto"
	}

	return Status{
		Daemon:             true,
		ManagerEnabled:     cfg.Enabled,
		Running:            pid > 0,
		SingBoxPID:         pid,
		ActiveGroup:        cfg.ActiveGroup,
		SelectedProfile:    cfg.ActiveGroup,
		SelectedOutbound:   selected,
		RuntimeMode:        cfg.RuntimeMode,
		Strategy:           cfg.Strategy,
		Health:             cfg.Health,
		ActiveSubscription: "",
		LatencyMS:          cfg.LatencyMS,
		MemoryKB:           memoryKB,
		CPUPercent:         "0",
		Connections:        stats.Connections,
		RxBytes:            stats.RxBytes,
		TxBytes:            stats.TxBytes,
		TProxyEnabled:      cfg.TProxyEnabled,
		DNSHijack:          cfg.DNSHijack,
		KillSwitch:         cfg.KillSwitch,
		NftablesInclude:    runtime.DefaultPaths.NftablesInclude,
		TUNEnabled:         cfg.TUNEnabled,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func statusUnavailable(cfg ManagerConfig, err error) Status {
	status := collectStatus(cfg)
	status.Daemon = false
	return status
}

func managedRuntimePID(paths runtime.Paths) int {
	pid := runtime.RunningPID(paths)
	if pid == 0 || !processHasArg(pid, paths.RuntimeConfig) {
		return 0
	}
	return pid
}

func processHasArg(pid int, arg string) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	return cmdlineHasArg(data, arg)
}

func cmdlineHasArg(data []byte, arg string) bool {
	for _, part := range strings.Split(strings.TrimRight(string(data), "\x00"), "\x00") {
		if part == arg {
			return true
		}
	}
	return false
}

func findProcess(name string) int {
	out, err := exec.Command("pidof", name).Output()
	if err != nil {
		return 0
	}
	for _, part := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(part)
		if err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

func readRSS(pid int) uint64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return value
	}
	return 0
}

func collectRuntimeStats() RuntimeStats {
	rx, tx := readTrafficCounters()
	return RuntimeStats{
		Connections: readConnectionCount(),
		RxBytes:     rx,
		TxBytes:     tx,
	}
}

func readTrafficCounters() (uint64, uint64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	var rx, tx uint64
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if strings.TrimSpace(parts[0]) == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		in, _ := strconv.ParseUint(fields[0], 10, 64)
		out, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += in
		tx += out
	}
	return rx, tx
}

func readConnectionCount() int {
	return countEstablishedTCP("/proc/net/tcp") + countEstablishedTCP("/proc/net/tcp6")
}

func countEstablishedTCP(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[3] == "01" {
			count++
		}
	}
	return count
}

func readManagerLogs(lines int) string {
	out, err := exec.Command("logread", "-e", "singbox-manager", "-l", strconv.Itoa(lines)).Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return tailLines(string(out), lines)
	}
	return tailFilteredFile("/var/log/messages", "singbox-manager", lines)
}

func tailFilteredFile(path string, needle string, lines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	matched := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, needle) {
			matched = append(matched, line)
		}
	}
	return strings.Join(lastStrings(matched, lines), "\n")
}

func tailLines(text string, lines int) string {
	return strings.Join(lastStrings(strings.Split(strings.TrimRight(text, "\n"), "\n"), lines), "\n")
}

func lastStrings(values []string, n int) []string {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func discoverDevices() []Device {
	byIP := map[string]Device{}
	for _, device := range parseDHCPLeases("/tmp/dhcp.leases") {
		byIP[device.IP] = device
	}
	for _, device := range parseARP("/proc/net/arp") {
		if existing, ok := byIP[device.IP]; ok {
			if existing.MAC == "" {
				existing.MAC = device.MAC
				byIP[device.IP] = existing
			}
			continue
		}
		byIP[device.IP] = device
	}
	devices := make([]Device, 0, len(byIP))
	for _, device := range byIP {
		devices = append(devices, device)
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].IP < devices[j].IP
	})
	return devices
}

func parseDHCPLeases(path string) []Device {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	devices := []Device{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := ""
		if len(fields) >= 4 && fields[3] != "*" {
			name = fields[3]
		}
		devices = append(devices, Device{
			IP:     fields[2],
			MAC:    fields[1],
			Name:   name,
			Source: "dhcp",
		})
	}
	return devices
}

func parseARP(path string) []Device {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	devices := []Device{}
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] == "00:00:00:00:00:00" {
			continue
		}
		devices = append(devices, Device{
			IP:     fields[0],
			MAC:    fields[3],
			Source: "arp",
		})
	}
	return devices
}

func decodeIDParams(raw json.RawMessage) (IDParams, error) {
	var params IDParams
	if len(raw) == 0 {
		return params, fmt.Errorf("id is required")
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return params, err
	}
	if params.ID == "" {
		return params, fmt.Errorf("id is required")
	}
	return params, nil
}

func decodeNodeTestParams(raw json.RawMessage) (NodeTestParams, error) {
	var params NodeTestParams
	if len(raw) == 0 {
		return params, fmt.Errorf("id is required")
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return params, err
	}
	if params.ID == "" {
		return params, fmt.Errorf("id is required")
	}
	return params, nil
}

func decodeImportPayload(raw json.RawMessage) (ImportPayload, error) {
	var envelope struct {
		Request *ImportPayload `json:"request"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Request != nil {
		return *envelope.Request, nil
	}

	var payload ImportPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ImportPayload{}, err
	}
	return payload, nil
}

func decodeSubscription(raw json.RawMessage) (managerconfig.Subscription, string, error) {
	var envelope struct {
		Subscription *SubscriptionPayload `json:"subscription"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Subscription != nil {
		subscription := subscriptionFromPayload(*envelope.Subscription)
		return subscription, envelope.Subscription.Group, nil
	}

	var payload SubscriptionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.Subscription{}, "", err
	}
	return subscriptionFromPayload(payload), payload.Group, nil
}

func decodeNode(raw json.RawMessage) (managerconfig.Node, error) {
	var envelope struct {
		Node *NodePayload `json:"node"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Node != nil {
		return nodeFromPayload(*envelope.Node), nil
	}

	var payload NodePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.Node{}, err
	}
	return nodeFromPayload(payload), nil
}

func decodeDNSProfile(raw json.RawMessage) (managerconfig.DNSProfile, error) {
	var envelope struct {
		Profile *DNSProfilePayload `json:"profile"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Profile != nil {
		return dnsProfileFromPayload(*envelope.Profile), nil
	}

	var payload DNSProfilePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.DNSProfile{}, err
	}
	return dnsProfileFromPayload(payload), nil
}

func decodeDNSServer(raw json.RawMessage) (managerconfig.DNSServer, error) {
	var envelope struct {
		Server *DNSServerPayload `json:"server"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Server != nil {
		return dnsServerFromPayload(*envelope.Server), nil
	}

	var payload DNSServerPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.DNSServer{}, err
	}
	return dnsServerFromPayload(payload), nil
}

func decodeRoutingProfile(raw json.RawMessage) (managerconfig.RoutingProfile, error) {
	var envelope struct {
		Profile *RoutingProfilePayload `json:"profile"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Profile != nil {
		return routingProfileFromPayload(*envelope.Profile), nil
	}

	var payload RoutingProfilePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.RoutingProfile{}, err
	}
	return routingProfileFromPayload(payload), nil
}

func decodeRuleSet(raw json.RawMessage) (managerconfig.RuleSet, error) {
	var envelope struct {
		RuleSet *RuleSetPayload `json:"ruleset"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.RuleSet != nil {
		return ruleSetFromPayload(*envelope.RuleSet), nil
	}

	var payload RuleSetPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.RuleSet{}, err
	}
	return ruleSetFromPayload(payload), nil
}

func decodeSourceRule(raw json.RawMessage) (managerconfig.SourceRule, error) {
	var envelope struct {
		Rule *SourceRulePayload `json:"rule"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Rule != nil {
		return sourceRuleFromPayload(*envelope.Rule), nil
	}

	var payload SourceRulePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.SourceRule{}, err
	}
	return sourceRuleFromPayload(payload), nil
}

func decodePACSettings(raw json.RawMessage) (managerconfig.PAC, error) {
	var envelope struct {
		PAC *PACSettingsPayload `json:"pac"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.PAC != nil {
		return pacFromPayload(*envelope.PAC), nil
	}

	var payload PACSettingsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.PAC{}, err
	}
	return pacFromPayload(payload), nil
}

func decodeCustomPAC(raw json.RawMessage) (managerconfig.CustomPAC, error) {
	var envelope struct {
		PAC *CustomPACPayload `json:"pac"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.PAC != nil {
		return customPACFromPayload(*envelope.PAC), nil
	}

	var payload CustomPACPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return managerconfig.CustomPAC{}, err
	}
	return customPACFromPayload(payload), nil
}

func isRemoteSubscriptionInput(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func generatedImportID(seed string) string {
	hash := sha1.Sum([]byte(seed))
	suffix := hex.EncodeToString(hash[:])[:8]
	candidate := strings.ToLower(seed)
	if isRemoteSubscriptionInput(candidate) {
		candidate = strings.TrimPrefix(candidate, "https://")
		candidate = strings.TrimPrefix(candidate, "http://")
	}
	if idx := strings.IndexAny(candidate, "/?#"); idx >= 0 {
		candidate = candidate[:idx]
	}
	candidate = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, candidate)
	candidate = strings.Trim(candidate, "_")
	if candidate == "" || len(candidate) > 32 {
		candidate = "import"
	}
	return fmt.Sprintf("%s_%s", candidate, suffix)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func mustMarshal(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func subscriptionFromPayload(payload SubscriptionPayload) managerconfig.Subscription {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	return managerconfig.Subscription{
		ID:             payload.ID,
		Enabled:        enabled,
		Name:           name,
		URL:            payload.URL,
		Format:         payload.Format,
		UpdateInterval: payload.UpdateInterval,
		Health:         "unknown",
	}
}

func nodeFromPayload(payload NodePayload) managerconfig.Node {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	tag := payload.Tag
	if tag == "" {
		tag = payload.ID
	}
	return managerconfig.Node{
		ID:           payload.ID,
		Enabled:      enabled,
		Name:         name,
		Type:         payload.Type,
		Address:      payload.Address,
		Server:       payload.Server,
		Port:         payload.Port,
		UUID:         payload.UUID,
		Password:     payload.Password,
		Method:       payload.Method,
		Security:     payload.Security,
		TLS:          payload.TLS,
		Flow:         payload.Flow,
		Transport:    payload.Transport,
		Host:         payload.Host,
		Path:         payload.Path,
		SNI:          payload.SNI,
		ALPN:         payload.ALPN,
		Insecure:     payload.Insecure,
		Congestion:   payload.Congestion,
		UDPRelayMode: payload.UDPRelayMode,
		Tag:          tag,
		Subscription: payload.Subscription,
	}
}

func dnsProfileFromPayload(payload DNSProfilePayload) managerconfig.DNSProfile {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	return managerconfig.DNSProfile{
		ID:      payload.ID,
		Enabled: enabled,
		Name:    name,
		Mode:    payload.Mode,
		Servers: payload.Servers,
		Hijack:  payload.Hijack,
	}
}

func dnsServerFromPayload(payload DNSServerPayload) managerconfig.DNSServer {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	return managerconfig.DNSServer{
		ID:      payload.ID,
		Enabled: enabled,
		Name:    name,
		Type:    payload.Type,
		Address: payload.Address,
		Detour:  payload.Detour,
	}
}

func routingProfileFromPayload(payload RoutingProfilePayload) managerconfig.RoutingProfile {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	return managerconfig.RoutingProfile{
		ID:       payload.ID,
		Enabled:  enabled,
		Name:     name,
		Mode:     payload.Mode,
		RuleSets: payload.RuleSets,
		Final:    payload.Final,
	}
}

func ruleSetFromPayload(payload RuleSetPayload) managerconfig.RuleSet {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	return managerconfig.RuleSet{
		ID:             payload.ID,
		Enabled:        enabled,
		Name:           name,
		Type:           payload.Type,
		Format:         payload.Format,
		URL:            payload.URL,
		Path:           payload.Path,
		UpdateInterval: payload.UpdateInterval,
	}
}

func sourceRuleFromPayload(payload SourceRulePayload) managerconfig.SourceRule {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	return managerconfig.SourceRule{
		ID:       payload.ID,
		Enabled:  enabled,
		Name:     name,
		Profile:  payload.Profile,
		Sources:  payload.Sources,
		Outbound: payload.Outbound,
	}
}

func pacFromPayload(payload PACSettingsPayload) managerconfig.PAC {
	enabled := false
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	localBypass := true
	if payload.LocalBypass != nil {
		localBypass = *payload.LocalBypass
	}
	return managerconfig.PAC{
		Enabled:        enabled,
		Source:         payload.Source,
		SelectedCustom: payload.SelectedCustom,
		LocalBypass:    localBypass,
		CustomRules:    payload.CustomRules,
		Whitelist:      payload.Whitelist,
		Blacklist:      payload.Blacklist,
	}
}

func customPACFromPayload(payload CustomPACPayload) managerconfig.CustomPAC {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	name := payload.Name
	if name == "" {
		name = payload.ID
	}
	return managerconfig.CustomPAC{
		ID:      payload.ID,
		Enabled: enabled,
		Name:    name,
		Content: payload.Content,
	}
}

func writeHTTPJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSON(value any) {
	_ = json.NewEncoder(os.Stdout).Encode(value)
}
