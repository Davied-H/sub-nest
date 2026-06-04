package aggregator

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"sub-nest/internal/domain"
)

type ParseResult struct {
	Format string
	Nodes  []domain.Node
}

type clashConfig struct {
	Proxies []map[string]interface{} `yaml:"proxies"`
}

func ParseSubscription(body []byte, sourceID string, sourceName string) (ParseResult, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ParseResult{}, fmt.Errorf("empty subscription")
	}

	if result, ok := parseClashYAML(text, sourceID, sourceName); ok {
		return result, nil
	}

	decoded := decodeMaybeBase64(text)
	if decoded != text {
		if result, ok := parseClashYAML(decoded, sourceID, sourceName); ok {
			result.Format = "base64-clash"
			return result, nil
		}
		nodes := parseShareLinks(decoded, sourceID, sourceName)
		if len(nodes) > 0 {
			return ParseResult{Format: "base64-links", Nodes: nodes}, nil
		}
	}

	nodes := parseShareLinks(text, sourceID, sourceName)
	if len(nodes) > 0 {
		return ParseResult{Format: "links", Nodes: nodes}, nil
	}

	return ParseResult{}, fmt.Errorf("unsupported subscription format")
}

func parseClashYAML(text string, sourceID string, sourceName string) (ParseResult, bool) {
	var cfg clashConfig
	if err := yaml.Unmarshal([]byte(text), &cfg); err != nil || len(cfg.Proxies) == 0 {
		return ParseResult{}, false
	}

	nodes := make([]domain.Node, 0, len(cfg.Proxies))
	for _, proxy := range cfg.Proxies {
		name, _ := proxy["name"].(string)
		typ, _ := proxy["type"].(string)
		server, _ := proxy["server"].(string)
		port := numberToInt(proxy["port"])
		if name == "" || typ == "" || server == "" || port == 0 {
			continue
		}
		extra := map[string]interface{}{}
		for key, value := range proxy {
			if key == "name" || key == "type" || key == "server" || key == "port" {
				continue
			}
			extra[key] = value
		}
		extra["_original_name"] = name
		node := domain.Node{
			Name:     name,
			Type:     typ,
			Server:   server,
			Port:     port,
			SourceID: sourceID,
			Source:   sourceName,
			Extra:    extra,
		}
		enrichNodeRegion(&node)
		nodes = append(nodes, node)
	}
	return ParseResult{Format: "clash", Nodes: nodes}, len(nodes) > 0
}

func parseShareLinks(text string, sourceID string, sourceName string) []domain.Node {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	nodes := make([]domain.Node, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if node, ok := parseShareLink(field, sourceID, sourceName); ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func parseShareLink(raw string, sourceID string, sourceName string) (domain.Node, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return domain.Node{}, false
	}
	switch strings.ToLower(u.Scheme) {
	case "ss":
		return parseSS(raw, u, sourceID, sourceName)
	case "vmess":
		return parseVMess(raw, strings.TrimPrefix(raw, "vmess://"), sourceID, sourceName)
	case "vless", "trojan", "hysteria", "hysteria2", "hy2":
		return parseGenericLink(raw, u, strings.ToLower(u.Scheme), sourceID, sourceName), true
	default:
		return domain.Node{}, false
	}
}

func parseGenericLink(raw string, u *url.URL, typ string, sourceID string, sourceName string) domain.Node {
	name, _ := url.QueryUnescape(u.Fragment)
	if name == "" {
		name = u.Hostname()
	}
	port, _ := strconv.Atoi(u.Port())
	extra := map[string]interface{}{}
	if u.User != nil {
		extra["user"] = u.User.String()
	}
	for key, values := range u.Query() {
		if len(values) > 0 {
			extra[key] = values[0]
		}
	}
	extra["_original_name"] = name
	node := domain.Node{
		Name:     name,
		Type:     typ,
		Server:   u.Hostname(),
		Port:     port,
		Raw:      raw,
		SourceID: sourceID,
		Source:   sourceName,
		Extra:    extra,
	}
	enrichNodeRegion(&node)
	return node
}

func parseSS(raw string, u *url.URL, sourceID string, sourceName string) (domain.Node, bool) {
	name, _ := url.QueryUnescape(u.Fragment)
	server := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	user := ""

	if u.User != nil && server != "" && port > 0 {
		user = u.User.String()
	} else {
		payload := strings.TrimPrefix(raw, "ss://")
		if hash := strings.Index(payload, "#"); hash >= 0 {
			if name == "" {
				name, _ = url.QueryUnescape(payload[hash+1:])
			}
			payload = payload[:hash]
		}
		if q := strings.Index(payload, "?"); q >= 0 {
			payload = payload[:q]
		}
		decoded := decodeMaybeBase64(payload)
		if strings.Contains(decoded, "@") {
			parts := strings.SplitN(decoded, "@", 2)
			user = parts[0]
			hostPort := parts[1]
			if host, p, err := netSplitHostPort(hostPort); err == nil {
				server = host
				port = p
			}
		}
	}
	if name == "" {
		name = server
	}
	if server == "" || port == 0 {
		return domain.Node{}, false
	}
	extra := map[string]interface{}{"user": user, "_original_name": name}
	if method, password, ok := strings.Cut(user, ":"); ok {
		extra["cipher"] = method
		extra["password"] = password
	}
	node := domain.Node{
		Name:     name,
		Type:     "ss",
		Server:   server,
		Port:     port,
		Raw:      raw,
		SourceID: sourceID,
		Source:   sourceName,
		Extra:    extra,
	}
	enrichNodeRegion(&node)
	return node, true
}

func parseVMess(raw string, payload string, sourceID string, sourceName string) (domain.Node, bool) {
	decoded := decodeMaybeBase64(payload)
	var vm map[string]interface{}
	if err := yaml.Unmarshal([]byte(decoded), &vm); err != nil {
		return domain.Node{}, false
	}
	name, _ := vm["ps"].(string)
	server, _ := vm["add"].(string)
	port := numberToInt(vm["port"])
	if name == "" {
		name = server
	}
	if server == "" || port == 0 {
		return domain.Node{}, false
	}
	node := domain.Node{
		Name:     name,
		Type:     "vmess",
		Server:   server,
		Port:     port,
		Raw:      raw,
		SourceID: sourceID,
		Source:   sourceName,
		Extra:    withOriginalName(vm, name),
	}
	enrichNodeRegion(&node)
	return node, true
}

func withOriginalName(extra map[string]interface{}, name string) map[string]interface{} {
	if extra == nil {
		extra = map[string]interface{}{}
	}
	extra["_original_name"] = name
	return extra
}

func netSplitHostPort(value string) (string, int, error) {
	if strings.Count(value, ":") > 1 && !strings.HasPrefix(value, "[") {
		lastColon := strings.LastIndex(value, ":")
		port, err := strconv.Atoi(value[lastColon+1:])
		if err != nil {
			return "", 0, err
		}
		return strings.Trim(value[:lastColon], "[]"), port, nil
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		if strings.Contains(value, ":") {
			lastColon := strings.LastIndex(value, ":")
			port, convErr := strconv.Atoi(value[lastColon+1:])
			if convErr != nil {
				return "", 0, err
			}
			return strings.Trim(value[:lastColon], "[]"), port, nil
		}
		return "", 0, err
	}
	port, err := strconv.Atoi(portText)
	return strings.Trim(host, "[]"), port, err
}

func decodeMaybeBase64(text string) string {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	} {
		if decoded, err := enc.DecodeString(cleaned); err == nil {
			out := strings.TrimSpace(string(decoded))
			if out != "" {
				return out
			}
		}
	}
	return text
}

func numberToInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}
