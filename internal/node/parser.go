package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func ParseYamlFile(filePath string, stats *Stats) ([]Node, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 YAML 文件失败: %w", err)
	}

	var raw struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 YAML 文件失败: %w", err)
	}
	if len(raw.Proxies) == 0 {
		return nil, fmt.Errorf("文件中没有找到 proxies 节点")
	}

	nodes := make([]Node, 0, len(raw.Proxies))
	for _, item := range raw.Proxies {
		node, ok := parseProxyMap(item)
		if !ok {
			continue
		}
		increaseStats(stats, node.Type)
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func LoadSubscriptionUrls(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取订阅文件失败: %w", err)
	}

	var urls []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "http://") || strings.HasPrefix(strings.ToLower(line), "https://") {
			urls = append(urls, line)
		}
	}
	return urls, nil
}

func FetchAndParseSubscription(url string, stats *Stats) ([]Node, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求订阅失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取订阅响应失败: %w", err)
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		return nil, fmt.Errorf("订阅响应为空")
	}
	if strings.Contains(content, "proxy-groups") {
		return nil, nil
	}

	decoded, err := decodeBase64String(content)
	if err != nil {
		return nil, fmt.Errorf("解码订阅内容失败: %w", err)
	}

	return parseNodes(decoded, stats), nil
}

func parseNodes(rawData string, stats *Stats) []Node {
	lines := strings.Split(strings.ReplaceAll(rawData, "\r\n", "\n"), "\n")
	nodes := make([]Node, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var (
			n  Node
			ok bool
		)

		switch {
		case strings.HasPrefix(line, "vmess://"):
			n, ok = parseVmess(line[len("vmess://"):])
		case strings.HasPrefix(line, "ss://"):
			n, ok = parseShadowsocks(line[len("ss://"):])
		case strings.HasPrefix(line, "trojan://"):
			n, ok = parseTrojan(line[len("trojan://"):])
		case strings.HasPrefix(line, "hysteria2://"):
			n, ok = parseHysteria2(line)
		case strings.HasPrefix(line, "hy2://"):
			n, ok = parseHysteria2(line)
		}

		if ok {
			increaseStats(stats, n.Type)
			nodes = append(nodes, n)
		}
	}

	return nodes
}

func parseProxyMap(item map[string]any) (Node, bool) {
	dict := make(map[string]any, len(item))
	for key, value := range item {
		dict[strings.ToLower(key)] = value
	}

	typ := strings.ToLower(toString(dict["type"]))
	switch typ {
	case "ss", "shadowsocks":
		return Node{
			Name:     defaultName(toString(dict["name"]), "未知SS"),
			Type:     "ss",
			Server:   toString(dict["server"]),
			Port:     toInt(dict["port"], 8388),
			Cipher:   defaultName(toString(dict["cipher"]), "aes-256-gcm"),
			Password: toString(dict["password"]),
		}, true
	case "vmess":
		return Node{
			Name:    defaultName(toString(dict["name"]), "未知Vmess"),
			Type:    "vmess",
			Server:  toString(dict["server"]),
			Port:    toInt(dict["port"], 443),
			UUID:    toString(dict["uuid"]),
			Cipher:  defaultName(toString(dict["cipher"]), "auto"),
			AlterID: toInt(dict["alterid"], 0),
		}, true
	case "trojan":
		return Node{
			Name:     defaultName(toString(dict["name"]), "未知Trojan"),
			Type:     "trojan",
			Server:   toString(dict["server"]),
			Port:     toInt(dict["port"], 443),
			Password: toString(dict["password"]),
			SNI:      firstNonEmpty(toString(dict["sni"]), toString(dict["servername"])),
		}, true
	case "hysteria2", "hy2":
		return Node{
			Name:           defaultName(toString(dict["name"]), "未知Hysteria2"),
			Type:           "hysteria2",
			Server:         toString(dict["server"]),
			Port:           toInt(dict["port"], 443),
			Password:       toString(dict["password"]),
			SNI:            firstNonEmpty(toString(dict["sni"]), toString(dict["servername"])),
			SkipCertVerify: toBool(dict["skip-cert-verify"]),
		}, true
	default:
		return Node{}, false
	}
}

func parseVmess(encoded string) (Node, bool) {
	data, err := decodeBase64String(encoded)
	if err != nil {
		return Node{}, false
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return Node{}, false
	}

	return Node{
		Name:    defaultName(toString(payload["ps"]), "xxxx"),
		Type:    "vmess",
		Server:  toString(payload["add"]),
		Port:    toInt(payload["port"], 443),
		UUID:    toString(payload["id"]),
		Cipher:  defaultName(toString(payload["cipher"]), "auto"),
		AlterID: 0,
	}, true
}

func parseShadowsocks(raw string) (Node, bool) {
	name := "xxxx"
	if idx := strings.Index(raw, "#"); idx >= 0 {
		name = decodeURLString(raw[idx+1:])
		raw = raw[:idx]
	}

	if idx := strings.Index(raw, "?"); idx >= 0 {
		raw = raw[:idx]
	}

	var decoded string
	if strings.Contains(raw, "@") {
		parts := strings.SplitN(raw, "@", 2)
		userInfo, err := decodeBase64String(parts[0])
		if err != nil {
			return Node{}, false
		}
		decoded = userInfo + "@" + parts[1]
	} else {
		all, err := decodeBase64String(raw)
		if err != nil {
			return Node{}, false
		}
		decoded = all
	}

	parts := strings.SplitN(decoded, "@", 2)
	if len(parts) != 2 {
		return Node{}, false
	}

	methodPass := strings.SplitN(parts[0], ":", 2)
	host, port, ok := splitHostPort(parts[1])
	if len(methodPass) != 2 || !ok {
		return Node{}, false
	}

	return Node{
		Name:     defaultName(name, "xxxx"),
		Type:     "ss",
		Server:   host,
		Port:     port,
		Cipher:   methodPass[0],
		Password: methodPass[1],
	}, true
}

func parseTrojan(raw string) (Node, bool) {
	name := "xxxx"
	if idx := strings.Index(raw, "#"); idx >= 0 {
		name = decodeURLString(raw[idx+1:])
		raw = raw[:idx]
	}

	query := ""
	if idx := strings.Index(raw, "?"); idx >= 0 {
		query = raw[idx+1:]
		raw = raw[:idx]
	}

	parts := strings.SplitN(raw, "@", 2)
	if len(parts) != 2 {
		return Node{}, false
	}
	host, port, ok := splitHostPort(parts[1])
	if !ok {
		return Node{}, false
	}

	sni := ""
	if query != "" {
		values, err := neturl.ParseQuery(query)
		if err == nil {
			sni = firstNonEmpty(values.Get("sni"), values.Get("peer"))
		}
		if sni == "" && !strings.Contains(query, "=") {
			sni = decodeURLString(query)
		}
	}

	return Node{
		Name:     defaultName(name, host),
		Type:     "trojan",
		Server:   host,
		Port:     port,
		Password: parts[0],
		SNI:      sni,
	}, true
}

func parseHysteria2(raw string) (Node, bool) {
	name := "xxxx"
	if idx := strings.Index(raw, "#"); idx >= 0 {
		name = decodeURLString(raw[idx+1:])
		raw = raw[:idx]
	}

	query := ""
	if idx := strings.Index(raw, "?"); idx >= 0 {
		query = raw[idx+1:]
		raw = raw[:idx]
	}

	raw = strings.TrimPrefix(raw, "hysteria2://")
	raw = strings.TrimPrefix(raw, "hy2://")

	password := ""
	hostPort := raw
	if idx := strings.Index(raw, "@"); idx >= 0 {
		password = raw[:idx]
		hostPort = raw[idx+1:]
	}

	host, port, ok := splitHostPortDefault(hostPort, 443)
	if !ok {
		return Node{}, false
	}

	sni := ""
	skipVerify := false
	if query != "" {
		values, err := neturl.ParseQuery(query)
		if err == nil {
			sni = firstNonEmpty(values.Get("sni"), values.Get("peer"))
			skipVerify = parseBoolString(values.Get("insecure"))
		}
	}

	return Node{
		Name:           defaultName(name, host),
		Type:           "hysteria2",
		Server:         host,
		Port:           port,
		Password:       password,
		SNI:            sni,
		SkipCertVerify: skipVerify,
	}, true
}

func splitHostPort(value string) (string, int, bool) {
	return splitHostPortDefault(value, 0)
}

func splitHostPortDefault(value string, defaultPort int) (string, int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0, false
	}

	if host, portStr, err := net.SplitHostPort(value); err == nil {
		port, err := strconv.Atoi(portStr)
		return host, port, err == nil
	}

	lastColon := strings.LastIndex(value, ":")
	if lastColon <= 0 || strings.Count(value, ":") > 1 && !strings.HasPrefix(value, "[") {
		if defaultPort == 0 {
			return "", 0, false
		}
		return value, defaultPort, true
	}

	port, err := strconv.Atoi(strings.TrimSpace(value[lastColon+1:]))
	if err != nil {
		if defaultPort == 0 {
			return "", 0, false
		}
		return strings.TrimSpace(value), defaultPort, true
	}
	return strings.TrimSpace(value[:lastColon]), port, true
}

func decodeBase64String(value string) (string, error) {
	cleaned := cleanBase64String(value)
	data, err := base64.StdEncoding.DecodeString(cleaned)
	if err == nil {
		return string(data), nil
	}

	data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(cleaned, "="))
	if err == nil {
		return string(data), nil
	}

	data, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.NewReplacer("+", "-", "/", "_").Replace(cleaned), "="))
	if err == nil {
		return string(data), nil
	}

	return "", err
}

func cleanBase64String(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "+")
	value = strings.ReplaceAll(value, "_", "/")
	if mod := len(value) % 4; mod != 0 {
		value += strings.Repeat("=", 4-mod)
	}
	return value
}

func increaseStats(stats *Stats, nodeType string) {
	if stats == nil {
		return
	}
	switch strings.ToLower(nodeType) {
	case "vmess":
		stats.VmessCount++
	case "ss":
		stats.SsCount++
	case "trojan":
		stats.TrojanCount++
	case "hysteria2":
		stats.Hysteria2Count++
	}
}

func defaultName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func toInt(value any, fallback int) int {
	switch v := value.(type) {
	case nil:
		return fallback
	case int:
		return v
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return fallback
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return parseBoolString(v)
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func parseBoolString(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes"
}

func decodeURLString(value string) string {
	if decoded, err := neturl.QueryUnescape(value); err == nil {
		return strings.TrimSpace(decoded)
	}
	return strings.TrimSpace(value)
}
