package main

import (
	"befree/internal/config"
	"befree/internal/node"
	"befree/internal/runner"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	inputFile  string
	localYamls string
	listenPort int
	speedUrl   string
	yamlFile   string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "befree",
		Short: "Befree - 代理节点转换工具",
		Long: `
  ____              ____
 |  _ \            / ___|  ___ _ ____   _____ _ __
 | |_) |  _____  _| |  _  / _ \ '_ \ \ / / _ \ '__|
 |  _ <  |_____| |_| |_| ||  __/ | | \ V /  __/ |
 |_| \_\            \____| \___|_| |_|\_/ \___|_|

Befree v1.0 (Go) - 迷人安全
代理节点转换工具，支持多协议节点解析和 Clash 配置生成`,
		Run: func(cmd *cobra.Command, args []string) {
			if yamlFile != "" {
				// 直接使用指定的 yaml 文件启动
				if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
					fmt.Printf("[-] 文件不存在: %s\n", yamlFile)
					return
				}
				fmt.Printf("[+] 检测到 %s 文件，程序正在启动\n", yamlFile)
				runner.RunClash(yamlFile)
				return
			}

			// 正常模式：解析节点并生成配置
			if inputFile == "" {
				cmd.Help()
				return
			}

			runMain()
		},
	}

	rootCmd.Flags().StringVarP(&inputFile, "file", "f", "", "指定订阅文件路径")
	rootCmd.Flags().StringVarP(&localYamls, "local", "l", "", "指定本地YAML文件(逗号分隔)")
	rootCmd.Flags().IntVarP(&listenPort, "port", "p", 59981, "指定监听端口")
	rootCmd.Flags().StringVarP(&speedUrl, "test", "t", "https://www.google.com", "指定测速URL")
	rootCmd.Flags().StringVarP(&yamlFile, "yaml", "y", "", "直接使用指定的Clash YAML文件")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runMain() {
	outputFile := "sectest.yaml"

	var allNodes []node.Node
	var stats node.Stats

	// 检查输入文件类型并加载
	if filepath.Ext(inputFile) == ".yaml" || filepath.Ext(inputFile) == ".yml" {
		fmt.Printf("[+] 检测到 YAML 配置文件，正在解析本地节点\n")
		nodes, err := node.ParseYamlFile(inputFile, &stats)
		if err != nil {
			fmt.Printf("[-] 解析 YAML 文件失败: %v\n", err)
			return
		}
		allNodes = append(allNodes, nodes...)
	} else {
		// 订阅地址文件
		urls, err := node.LoadSubscriptionUrls(inputFile)
		if err != nil {
			fmt.Printf("[-] 加载订阅文件失败: %v\n", err)
			return
		}
		fmt.Printf("[+] %s 文件中，发现 %d 个订阅地址\n", inputFile, len(urls))

		for _, url := range urls {
			nodes, err := node.FetchAndParseSubscription(url, &stats)
			if err != nil {
				fmt.Printf("[-] 处理订阅地址 %s 时出错: %v\n", url, err)
				continue
			}
			allNodes = append(allNodes, nodes...)
		}
	}

	// 解析本地 YAML 文件
	if localYamls != "" {
		files := splitFiles(localYamls)
		for _, file := range files {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				fmt.Printf("[-] 文件不存在: %s\n", file)
				continue
			}
			fmt.Printf("[+] 正在解析本地 YAML 文件: %s\n", file)
			nodes, err := node.ParseYamlFile(file, &stats)
			if err != nil {
				fmt.Printf("[-] 解析文件失败: %v\n", err)
				continue
			}
			allNodes = append(allNodes, nodes...)
		}
	}

	fmt.Printf("[+] 总共解析到 %d 个正常转换节点\n", len(allNodes))
	fmt.Printf("[+] 其中包含 vmess 节点数量为: %d\n", stats.VmessCount)
	fmt.Printf("[+] 其中包含 ss 节点数量为: %d\n", stats.SsCount)
	fmt.Printf("[+] 其中包含 trojan 节点数量为: %d\n", stats.TrojanCount)
	fmt.Printf("[+] 其中包含 hysteria2 节点数量为: %d\n", stats.Hysteria2Count)

	if len(allNodes) == 0 {
		fmt.Println("[-] 未获取到可用节点，无法启动 befree")
		return
	}

	// 生成 Clash 配置文件
	err := config.GenerateConfig(allNodes, outputFile, listenPort, speedUrl)
	if err != nil {
		fmt.Printf("[-] 生成配置失败: %v\n", err)
		return
	}

	// 运行 Clash
	runner.RunClash(outputFile)
}

func splitFiles(s string) []string {
	var result []string
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}
