package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunClash(configFilePath string) {
	configPath, err := filepath.Abs(configFilePath)
	if err != nil {
		fmt.Printf("[-] 解析配置路径失败: %v\n", err)
		return
	}

	mihomoPath, err := findMihomo()
	if err != nil {
		fmt.Printf("[-] 未找到 mihomo 可执行文件: %v\n", err)
		return
	}

	cmd := exec.Command(mihomoPath, "-f", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Println("[+] running...")
	if err := cmd.Run(); err != nil {
		fmt.Printf("[-] 启动 mihomo 失败: %v\n", err)
		return
	}
	fmt.Println("[-] stop...")
}

func findMihomo() (string, error) {
	candidates := make([]string, 0, 4)

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "mihomo.exe"),
			filepath.Join(exeDir, "mihomo"),
		)
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "mihomo.exe"),
			filepath.Join(wd, "mihomo"),
		)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("请将 `mihomo.exe` 放到程序目录或当前工作目录")
}
