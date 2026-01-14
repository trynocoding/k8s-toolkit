package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var enterNsCmd = &cobra.Command{
	Use:   "enter-ns POD_NAME [NAMESPACE]",
	Short: "进入Pod的网络命名空间",
	Long: `进入指定Kubernetes Pod的网络命名空间。

这个命令允许你进入Pod的网络命名空间，可以在其中执行网络调试命令
（如 ip、tcpdump、netstat 等）。

示例:
  # 进入default命名空间中的my-pod
  k8s-toolkit enter-ns my-pod

  # 进入kube-system命名空间中的pod
  k8s-toolkit enter-ns coredns-xxx kube-system

  # 进入第二个容器的网络命名空间
  k8s-toolkit enter-ns my-pod default -c 1

  # 详细模式
  k8s-toolkit enter-ns my-pod -v`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runEnterNs,
}

var (
	containerIndex int
	runtime        string
)

func init() {
	rootCmd.AddCommand(enterNsCmd)

	enterNsCmd.Flags().IntVarP(&containerIndex, "container", "c", 0, 
		"容器索引 (默认: 0，即第一个容器)")
	enterNsCmd.Flags().StringVarP(&runtime, "runtime", "r", "auto", 
		"容器运行时 (containerd|docker|auto)")
}

func runEnterNs(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	
	// 检查是否有root权限
	if os.Geteuid() != 0 {
		return fmt.Errorf("此命令需要root权限运行，请使用: sudo %s", os.Args[0])
	}

	// 创建临时脚本文件
	tmpDir, err := ioutil.TempDir("", "k8s-toolkit-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "enter_pod_ns.sh")
	if err := ioutil.WriteFile(scriptPath, []byte(enterPodNsScript), 0755); err != nil {
		return fmt.Errorf("写入脚本文件失败: %w", err)
	}

	// 构建bash命令参数
	bashArgs := []string{scriptPath}
	bashArgs = append(bashArgs, args...) // POD_NAME 和 NAMESPACE

	// 添加选项
	if containerIndex != 0 {
		bashArgs = append(bashArgs, "-c", fmt.Sprintf("%d", containerIndex))
	}
	if runtime != "auto" {
		bashArgs = append(bashArgs, "-r", runtime)
	}
	if verbose {
		bashArgs = append(bashArgs, "-v")
	}

	// 执行bash脚本
	bashCmd := exec.Command("bash", bashArgs...)
	bashCmd.Stdin = os.Stdin
	bashCmd.Stdout = os.Stdout
	bashCmd.Stderr = os.Stderr

	if verbose {
		fmt.Printf("[DEBUG] 执行命令: bash %v\n", bashArgs)
	}

	if err := bashCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 保留bash脚本的退出码
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("执行脚本失败: %w", err)
	}

	return nil
}
