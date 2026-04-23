package node

import "strings"

// Node 表示一个可写入 Clash 配置的代理节点。
type Node struct {
	Name           string
	Type           string
	Server         string
	Port           int
	Password       string
	Cipher         string
	UUID           string
	AlterID        int
	SNI            string
	SkipCertVerify bool
}

type Stats struct {
	VmessCount     int
	SsCount        int
	TrojanCount    int
	Hysteria2Count int
}

func (n Node) ClashProxy() map[string]any {
	proxy := map[string]any{
		"name":   n.Name,
		"type":   n.Type,
		"server": n.Server,
		"port":   n.Port,
	}

	switch strings.ToLower(n.Type) {
	case "vmess":
		proxy["uuid"] = n.UUID
		proxy["cipher"] = valueOrDefault(n.Cipher, "auto")
		proxy["alterId"] = n.AlterID
	case "ss":
		proxy["cipher"] = n.Cipher
		proxy["password"] = n.Password
	case "trojan":
		proxy["password"] = n.Password
		if n.SNI != "" {
			proxy["sni"] = n.SNI
		}
		proxy["skip-cert-verify"] = true
	case "hysteria2":
		if n.Password != "" {
			proxy["password"] = n.Password
		}
		if n.SNI != "" {
			proxy["sni"] = n.SNI
		}
		proxy["skip-cert-verify"] = n.SkipCertVerify
	}

	return proxy
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
