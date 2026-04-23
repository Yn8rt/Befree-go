package config

import (
	"fmt"
	"os"
	"path/filepath"

	"befree/internal/node"

	"gopkg.in/yaml.v3"
)

type clashConfig struct {
	AllowLan    bool             `yaml:"allow-lan"`
	MixedPort   int              `yaml:"mixed-port"`
	Rules       []string         `yaml:"rules"`
	ProxyGroups []proxyGroup     `yaml:"proxy-groups"`
	Proxies     []map[string]any `yaml:"proxies"`
}

type proxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url"`
	Interval int      `yaml:"interval"`
	Strategy string   `yaml:"strategy"`
}

func GenerateConfig(nodes []node.Node, outputFile string, listenPort int, speedURL string) error {
	if len(nodes) == 0 {
		return fmt.Errorf("没有可写入的节点")
	}

	normalized := make([]node.Node, len(nodes))
	copy(normalized, nodes)
	resolveDuplicateNames(normalized)

	proxies := make([]map[string]any, 0, len(normalized))
	proxyNames := make([]string, 0, len(normalized))
	for _, item := range normalized {
		proxy := item.ClashProxy()
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, item.Name)
	}

	cfg := clashConfig{
		AllowLan:  false,
		MixedPort: listenPort,
		Rules: []string{
			"MATCH,proxy_pool",
		},
		ProxyGroups: []proxyGroup{
			{
				Name:     "proxy_pool",
				Type:     "load-balance",
				Proxies:  proxyNames,
				URL:      speedURL,
				Interval: 5,
				Strategy: "round-robin",
			},
		},
		Proxies: proxies,
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("生成 YAML 失败: %w", err)
	}

	outputPath := outputFile
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(".", outputPath)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	fmt.Printf("[+] http & socks 监听端口: %d\n", listenPort)
	return nil
}

func resolveDuplicateNames(nodes []node.Node) {
	nameCount := make(map[string]int, len(nodes))
	for i := range nodes {
		name := nodes[i].Name
		if name == "" {
			name = "xxxx"
		}
		nameCount[name]++
		if nameCount[name] > 1 {
			nodes[i].Name = fmt.Sprintf("%s__%d", name, nameCount[name])
			continue
		}
		nodes[i].Name = name
	}
}
