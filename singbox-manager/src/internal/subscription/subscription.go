package subscription

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

const maxSubscriptionSize = 10 << 20

var unsafeIDChars = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("subscription URL must use http or https")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "singbox-manager/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription fetch failed: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxSubscriptionSize {
		return nil, fmt.Errorf("subscription response exceeds %d bytes", maxSubscriptionSize)
	}
	return body, nil
}

func Parse(data []byte, format string) ([]managerconfig.Node, error) {
	payload, err := decodePayload(strings.TrimSpace(string(data)), format)
	if err != nil {
		return nil, err
	}

	// Many providers serve a full sing-box JSON config (or a bare outbounds
	// array) instead of a list of share URIs. Detect and parse that here so we
	// don't treat each JSON token as an unsupported URI.
	if looksLikeJSONConfig(payload) {
		return parseSingBoxConfig([]byte(strings.TrimSpace(payload)))
	}

	var nodes []managerconfig.Node
	var parseErrors []string
	for _, line := range strings.FieldsFunc(payload, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		node, err := ParseURI(line)
		if err != nil {
			parseErrors = append(parseErrors, err.Error())
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 && len(parseErrors) > 0 {
		return nil, errors.New(strings.Join(parseErrors, "; "))
	}
	return nodes, nil
}

func AssignSource(nodes []managerconfig.Node, subscriptionID string) []managerconfig.Node {
	assigned := make([]managerconfig.Node, 0, len(nodes))
	seen := map[string]int{}
	for _, node := range nodes {
		node.Subscription = subscriptionID
		node.Enabled = true
		node.ID = stableNodeID(subscriptionID, node)
		if seen[node.ID] > 0 {
			seen[node.ID]++
			node.ID = fmt.Sprintf("%s_%d", node.ID, seen[node.ID])
		} else {
			seen[node.ID] = 1
		}
		node.Tag = node.ID
		if node.Name == "" {
			node.Name = node.ID
		}
		assigned = append(assigned, node)
	}
	return assigned
}

func Due(source managerconfig.Subscription, now time.Time) bool {
	if !source.Enabled || source.URL == "" {
		return false
	}
	if source.LastUpdate == "" {
		return true
	}
	updated, err := time.Parse(time.RFC3339, source.LastUpdate)
	if err != nil {
		return true
	}
	interval, err := time.ParseDuration(source.UpdateInterval)
	if err != nil || interval <= 0 {
		interval = 24 * time.Hour
	}
	if interval < time.Minute {
		interval = time.Minute
	}
	return !updated.Add(interval).After(now)
}

func ParseURI(raw string) (managerconfig.Node, error) {
	switch {
	case strings.HasPrefix(raw, "vmess://"):
		return parseVMess(raw)
	case strings.HasPrefix(raw, "vless://"):
		return parseStandardURI(raw, "vless")
	case strings.HasPrefix(raw, "trojan://"):
		return parseStandardURI(raw, "trojan")
	case strings.HasPrefix(raw, "ss://"):
		return parseShadowsocks(raw)
	case strings.HasPrefix(raw, "hysteria2://"):
		return parseStandardURI(raw, "hysteria2")
	case strings.HasPrefix(raw, "hy2://"):
		return parseStandardURI("hysteria2://"+strings.TrimPrefix(raw, "hy2://"), "hysteria2")
	case strings.HasPrefix(raw, "tuic://"):
		return parseStandardURI(raw, "tuic")
	default:
		return managerconfig.Node{}, fmt.Errorf("unsupported subscription URI %q", raw)
	}
}

// supportedOutboundTypes lists the sing-box outbound types that map to a proxy
// node. Group/built-in outbounds (selector, urltest, direct, block, dns) are
// skipped during import.
var supportedOutboundTypes = map[string]bool{
	"shadowsocks": true,
	"vmess":       true,
	"vless":       true,
	"trojan":      true,
	"hysteria2":   true,
	"tuic":        true,
}

// parseSingBoxConfig extracts proxy nodes from a full sing-box JSON config or a
// bare array of outbound objects.
func parseSingBoxConfig(data []byte) ([]managerconfig.Node, error) {
	var outbounds []any

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err == nil {
		raw, ok := doc["outbounds"].([]any)
		if !ok {
			return nil, fmt.Errorf("sing-box config has no outbounds array")
		}
		outbounds = raw
	} else {
		if err2 := json.Unmarshal(data, &outbounds); err2 != nil {
			return nil, fmt.Errorf("invalid sing-box JSON config: %w", err)
		}
	}

	var nodes []managerconfig.Node
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		node, ok := outboundToNode(outbound)
		if !ok {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no proxy nodes found in sing-box config")
	}
	return nodes, nil
}

func outboundToNode(outbound map[string]any) (managerconfig.Node, bool) {
	typ := stringField(outbound, "type")
	if !supportedOutboundTypes[typ] {
		return managerconfig.Node{}, false
	}
	server := stringField(outbound, "server")
	if server == "" {
		return managerconfig.Node{}, false
	}
	node := managerconfig.Node{
		Enabled:      true,
		Type:         typ,
		Name:         stringField(outbound, "tag"),
		Server:       server,
		Port:         intField(outbound, "server_port"),
		UUID:         stringField(outbound, "uuid"),
		Password:     stringField(outbound, "password"),
		Method:       stringField(outbound, "method"),
		Security:     stringField(outbound, "security"),
		Flow:         stringField(outbound, "flow"),
		Congestion:   stringField(outbound, "congestion_control"),
		UDPRelayMode: stringField(outbound, "udp_relay_mode"),
	}
	applyJSONTLS(&node, outbound["tls"])
	applyJSONTransport(&node, outbound["transport"])
	if node.Name == "" {
		node.Name = node.Server
	}
	return node, true
}

func applyJSONTLS(node *managerconfig.Node, raw any) {
	tls, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if enabled, ok := tls["enabled"].(bool); ok && enabled {
		node.TLS = true
		if node.Security == "" {
			node.Security = "tls"
		}
	}
	if _, ok := tls["reality"].(map[string]any); ok {
		node.Security = "reality"
		node.TLS = true
	}
	if sni := stringField(tls, "server_name"); sni != "" {
		node.SNI = sni
	}
	if alpn := joinStringList(tls["alpn"]); alpn != "" {
		node.ALPN = alpn
	}
	if insecure, ok := tls["insecure"].(bool); ok {
		node.Insecure = insecure
	}
}

func applyJSONTransport(node *managerconfig.Node, raw any) {
	transport, ok := raw.(map[string]any)
	if !ok {
		return
	}
	node.Transport = normalizeTransport(stringField(transport, "type"))
	node.Path = stringField(transport, "path")
	switch node.Transport {
	case "ws":
		node.Host = headerHost(transport["headers"])
		if node.Host == "" {
			node.Host = stringField(transport, "host")
		}
	case "httpupgrade":
		node.Host = stringField(transport, "host")
		if node.Host == "" {
			node.Host = headerHost(transport["headers"])
		}
	case "grpc":
		if name := stringField(transport, "service_name"); name != "" {
			node.Path = name
		}
	}
}

// headerHost extracts a Host header value from a transport headers object. The
// key is matched case-insensitively and the value may be a string or a list of
// strings (sing-box accepts both forms).
func headerHost(raw any) string {
	headers, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	for key, value := range headers {
		if strings.EqualFold(key, "host") {
			return firstStringValue(value)
		}
	}
	return ""
}

func firstStringValue(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		for _, item := range value {
			if text, ok := item.(string); ok && text != "" {
				return text
			}
		}
	}
	return ""
}

func joinStringList(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		var parts []string
		for _, item := range value {
			if text, ok := item.(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	}
	return ""
}

func decodePayload(payload string, format string) (string, error) {
	switch format {
	case "", "auto":
		if looksPlain(payload) || looksLikeJSONConfig(payload) {
			return payload, nil
		}
		if decoded, ok := decodeBase64(payload); ok && (looksPlain(decoded) || looksLikeJSONConfig(decoded)) {
			return decoded, nil
		}
		return payload, nil
	case "plain":
		return payload, nil
	case "base64":
		decoded, ok := decodeBase64(payload)
		if !ok {
			return "", fmt.Errorf("subscription payload is not valid base64")
		}
		return decoded, nil
	default:
		return "", fmt.Errorf("unsupported subscription format %q", format)
	}
}

func looksPlain(payload string) bool {
	return strings.Contains(payload, "vmess://") ||
		strings.Contains(payload, "vless://") ||
		strings.Contains(payload, "trojan://") ||
		strings.Contains(payload, "ss://") ||
		strings.Contains(payload, "hysteria2://") ||
		strings.Contains(payload, "hy2://") ||
		strings.Contains(payload, "tuic://")
}

func looksLikeJSONConfig(payload string) bool {
	trimmed := strings.TrimSpace(payload)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func decodeBase64(value string) (string, bool) {
	compact := strings.NewReplacer("\n", "", "\r", "", " ", "", "\t", "").Replace(value)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		data, err := encoding.DecodeString(compact)
		if err == nil {
			return string(data), true
		}
	}
	if remainder := len(compact) % 4; remainder != 0 {
		padded := compact + strings.Repeat("=", 4-remainder)
		data, err := base64.StdEncoding.DecodeString(padded)
		if err == nil {
			return string(data), true
		}
		data, err = base64.URLEncoding.DecodeString(padded)
		if err == nil {
			return string(data), true
		}
	}
	return "", false
}

func parseVMess(raw string) (managerconfig.Node, error) {
	decoded, ok := decodeBase64(strings.TrimPrefix(raw, "vmess://"))
	if !ok {
		return managerconfig.Node{}, fmt.Errorf("invalid vmess payload")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
		return managerconfig.Node{}, err
	}
	node := managerconfig.Node{
		Enabled:   true,
		Type:      "vmess",
		Name:      stringField(payload, "ps"),
		Server:    stringField(payload, "add"),
		Port:      intField(payload, "port"),
		UUID:      stringField(payload, "id"),
		Security:  stringField(payload, "scy"),
		TLS:       stringField(payload, "tls") == "tls",
		Transport: normalizeTransport(stringField(payload, "net")),
		Host:      stringField(payload, "host"),
		Path:      stringField(payload, "path"),
		SNI:       stringField(payload, "sni"),
	}
	if node.Name == "" {
		node.Name = node.Server
	}
	return node, nil
}

func parseStandardURI(raw string, typ string) (managerconfig.Node, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return managerconfig.Node{}, err
	}
	name, _ := url.QueryUnescape(parsed.Fragment)
	host := parsed.Hostname()
	port, _ := strconv.Atoi(parsed.Port())
	query := parsed.Query()

	node := managerconfig.Node{
		Enabled:      true,
		Type:         typ,
		Name:         firstNonEmpty(name, host),
		Server:       host,
		Port:         port,
		Security:     firstNonEmpty(query.Get("security"), query.Get("tls")),
		TLS:          inSet(firstNonEmpty(query.Get("security"), query.Get("tls")), "tls", "reality"),
		Transport:    normalizeTransport(firstNonEmpty(query.Get("type"), query.Get("transport"))),
		Host:         query.Get("host"),
		Path:         query.Get("path"),
		SNI:          firstNonEmpty(query.Get("sni"), query.Get("peer"), query.Get("server_name")),
		ALPN:         query.Get("alpn"),
		Insecure:     isTruthy(firstNonEmpty(query.Get("insecure"), query.Get("allowInsecure"))),
		Flow:         query.Get("flow"),
		Congestion:   query.Get("congestion_control"),
		UDPRelayMode: query.Get("udp_relay_mode"),
	}

	switch typ {
	case "vless":
		node.UUID = parsed.User.Username()
	case "trojan", "hysteria2":
		password, _ := url.QueryUnescape(parsed.User.Username())
		node.Password = password
		if node.Security == "" && (node.SNI != "" || strings.EqualFold(query.Get("security"), "tls")) {
			node.Security = "tls"
		}
		node.TLS = node.TLS || node.Security == "tls" || node.SNI != ""
	case "tuic":
		node.UUID = parsed.User.Username()
		password, _ := parsed.User.Password()
		node.Password, _ = url.QueryUnescape(password)
		if node.Security == "" {
			node.Security = "tls"
		}
		node.TLS = true
	}
	if node.Security == "1" || node.Security == "true" {
		node.Security = "tls"
		node.TLS = true
	}
	return node, nil
}

func parseShadowsocks(raw string) (managerconfig.Node, error) {
	body := strings.TrimPrefix(raw, "ss://")
	fragment := ""
	if idx := strings.Index(body, "#"); idx >= 0 {
		fragment, _ = url.QueryUnescape(body[idx+1:])
		body = body[:idx]
	}

	if strings.Contains(body, "@") {
		parsed, err := url.Parse("ss://" + body)
		if err != nil {
			return managerconfig.Node{}, err
		}
		user := parsed.User.Username()
		if decoded, ok := decodeBase64(user); ok {
			user = decoded
		}
		method, password := splitPair(user, ":")
		return managerconfig.Node{
			Enabled:  true,
			Type:     "shadowsocks",
			Name:     firstNonEmpty(fragment, parsed.Hostname()),
			Server:   parsed.Hostname(),
			Port:     mustPort(parsed.Port()),
			Method:   method,
			Password: passwordFromURL(password),
		}, nil
	}

	queryStart := strings.Index(body, "?")
	if queryStart >= 0 {
		body = body[:queryStart]
	}
	decoded, ok := decodeBase64(body)
	if !ok {
		return managerconfig.Node{}, fmt.Errorf("invalid shadowsocks payload")
	}
	methodPassword, serverPort := splitPair(decoded, "@")
	method, password := splitPair(methodPassword, ":")
	server, portText := splitLast(serverPort, ":")
	return managerconfig.Node{
		Enabled:  true,
		Type:     "shadowsocks",
		Name:     firstNonEmpty(fragment, server),
		Server:   server,
		Port:     mustPort(portText),
		Method:   method,
		Password: passwordFromURL(password),
	}, nil
}

func stableNodeID(subscriptionID string, node managerconfig.Node) string {
	prefix := sanitizeID(subscriptionID)
	sum := sha1.Sum([]byte(strings.Join([]string{
		node.Type, node.Name, node.Server, strconv.Itoa(node.Port), node.UUID, node.Password, node.Method,
	}, "|")))
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(sum[:])[:10])
}

func sanitizeID(value string) string {
	value = strings.Trim(unsafeIDChars.ReplaceAllString(value, "_"), "_")
	if value == "" {
		return "node"
	}
	return strings.ToLower(value)
}

func stringField(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func intField(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case float64:
		return int(value)
	case string:
		port, _ := strconv.Atoi(value)
		return port
	default:
		return 0
	}
}

func normalizeTransport(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "websocket" {
		return "ws"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func inSet(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func splitPair(value string, sep string) (string, string) {
	left, right, _ := strings.Cut(value, sep)
	return left, right
}

func splitLast(value string, sep string) (string, string) {
	index := strings.LastIndex(value, sep)
	if index < 0 {
		return value, ""
	}
	return value[:index], value[index+len(sep):]
}

func mustPort(value string) int {
	port, _ := strconv.Atoi(value)
	return port
}

func passwordFromURL(value string) string {
	password, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return password
}

func isTruthy(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
