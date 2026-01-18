package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trynocoding/k8s-toolkit/internal/filecopy"
)

var fcpCmd = &cobra.Command{
	Use:   "fcp -f FILE -n NODES -d DEST [OPTIONS]",
	Short: "文件分发工具 - 批量复制文件到多个节点",
	Long: `使用 SSH/SCP 协议将文件并行分发到多个远程节点。

支持免密登录（SSH key）和密码认证两种方式。

示例:
  # 使用 SSH key 免密登录（推荐）
  k8s-toolkit fcp -f /path/to/file.tar.gz -n node1,node2,node3 -d /opt/data/

  # 使用密码登录（所有节点密码相同）
  k8s-toolkit fcp -f /path/to/file.tar.gz -n node1,node2,node3 -d /opt/data/ -p yourpassword

  # 指定用户名和端口
  k8s-toolkit fcp -f /path/to/file.tar.gz -n node1,node2,node3 -d /opt/data/ -u root --port 2222

  # 使用指定私钥
  k8s-toolkit fcp -f /path/to/file.tar.gz -n node1,node2,node3 -d /opt/data/ -i ~/.ssh/my_key

  # 详细模式查看进度
  k8s-toolkit fcp -f /path/to/file.tar.gz -n node1,node2,node3 -d /opt/data/ -v`,
	RunE: runFcp,
}

var (
	fcpSourceFile string
	fcpNodes      string
	fcpDestDir    string
	fcpUser       string
	fcpPassword   string
	fcpIdentity   string
	fcpPort       string
)

func init() {
	rootCmd.AddCommand(fcpCmd)

	fcpCmd.Flags().StringVarP(&fcpSourceFile, "file", "f", "",
		"源文件路径 (必需)")
	fcpCmd.MarkFlagRequired("file")

	fcpCmd.Flags().StringVarP(&fcpNodes, "nodes", "n", "",
		"目标节点列表，逗号分隔 (必需，例如: node1,node2,node3)")
	fcpCmd.MarkFlagRequired("nodes")

	fcpCmd.Flags().StringVarP(&fcpDestDir, "dest", "d", "",
		"目标目录 (必需)")
	fcpCmd.MarkFlagRequired("dest")

	fcpCmd.Flags().StringVarP(&fcpUser, "user", "u", "",
		"SSH 用户名 (默认: 当前用户)")

	fcpCmd.Flags().StringVarP(&fcpPassword, "password", "p", "",
		"SSH 密码 (可选，适用于所有节点密码相同的情况)")

	fcpCmd.Flags().StringVarP(&fcpIdentity, "identity", "i", "",
		"SSH 私钥路径 (可选，默认使用 ~/.ssh/id_rsa 等)")

	fcpCmd.Flags().StringVar(&fcpPort, "port", "22",
		"SSH 端口 (默认: 22)")
}

func runFcp(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	ctx := context.Background()

	// 验证参数
	if fcpSourceFile == "" {
		return fmt.Errorf("必须指定源文件 (使用 -f 或 --file)")
	}
	if fcpNodes == "" {
		return fmt.Errorf("必须指定目标节点 (使用 -n 或 --nodes)")
	}
	if fcpDestDir == "" {
		return fmt.Errorf("必须指定目标目录 (使用 -d 或 --dest)")
	}

	// 解析节点列表
	nodeList := strings.Split(fcpNodes, ",")
	for i, n := range nodeList {
		nodeList[i] = strings.TrimSpace(n)
	}

	if len(nodeList) == 0 {
		return fmt.Errorf("节点列表为空")
	}

	// 创建复制选项
	opts := filecopy.CopyOptions{
		SourceFile: fcpSourceFile,
		DestDir:    fcpDestDir,
		Nodes:      nodeList,
		User:       fcpUser,
		Password:   fcpPassword,
		Identity:   fcpIdentity,
		Port:       fcpPort,
		Verbose:    verbose,
	}

	// 显示任务信息
	fmt.Printf("========== 文件分发任务 ==========\n")
	fmt.Printf("源文件: %s\n", fcpSourceFile)
	fmt.Printf("目标目录: %s\n", fcpDestDir)
	fmt.Printf("目标节点: %s\n", strings.Join(nodeList, ", "))
	if fcpUser != "" {
		fmt.Printf("SSH 用户: %s\n", fcpUser)
	}
	if fcpPassword != "" {
		fmt.Printf("认证方式: 密码\n")
	} else if fcpIdentity != "" {
		fmt.Printf("认证方式: 私钥 (%s)\n", fcpIdentity)
	} else {
		fmt.Printf("认证方式: 默认 (ssh-agent 或 ~/.ssh/id_*)\n")
	}
	fmt.Printf("端口: %s\n", fcpPort)
	fmt.Println("==================================")
	fmt.Println()

	// 执行复制
	result, err := filecopy.CopyToNodes(ctx, opts, func(node string, bytesWritten, totalBytes int64, percent float64) {
		if verbose {
			fmt.Printf("[%s] 进度: %.1f%% (%s / %s)\n",
				node, percent,
				formatBytes(bytesWritten),
				formatBytes(totalBytes))
		}
	})

	if err != nil {
		return fmt.Errorf("文件分发失败: %w", err)
	}

	// 输出结果
	fmt.Println("\n========== 分发结果 ==========")
	fmt.Printf("源文件: %s\n", result.SourceFile)
	fmt.Printf("耗时: %v\n", result.Duration)
	fmt.Println("\n节点状态:")

	hasError := false
	for _, node := range nodeList {
		nodeErr := result.NodesStatus[node]
		if nodeErr != nil {
			fmt.Printf("  ❌ %s: %v\n", node, nodeErr)
			hasError = true
		} else {
			fmt.Printf("  ✅ %s: 成功\n", node)
		}
	}

	if hasError {
		os.Exit(1)
	}

	return nil
}

// formatBytes 格式化字节数为人类可读形式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
