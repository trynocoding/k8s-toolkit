package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trynocoding/k8s-toolkit/internal/imgsync"
)

var imgSyncCmd = &cobra.Command{
	Use:   "img-sync -i IMAGE [OPTIONS]",
	Short: "Docker镜像同步和分发工具",
	Long: `拉取Docker镜像，流式导入到containerd，并可选地分发到远程节点。

这个工具自动化了镜像迁移流程（使用 Go 原生 SDK，无临时文件）:
1. 使用 Docker SDK 拉取镜像
2. 流式传输到 Containerd（无中间文件）
3. (可选) 通过 SSH 流式分发到远程节点

示例:
  # 拉取并同步nginx镜像
  k8s-toolkit img-sync -i nginx:latest

  # 同步并分发到远程节点
  k8s-toolkit img-sync -i redis:alpine -n node1,node2,node3

  # 详细模式查看执行过程
  k8s-toolkit img-sync -i nginx:latest -v`,
	RunE: runImgSync,
}

var (
	imageName string
	nodes     string
	outputDir string
	cleanup   bool
)

func init() {
	rootCmd.AddCommand(imgSyncCmd)

	imgSyncCmd.Flags().StringVarP(&imageName, "image", "i", "",
		"要处理的镜像名称 (必需)")
	imgSyncCmd.MarkFlagRequired("image")

	imgSyncCmd.Flags().StringVarP(&nodes, "nodes", "n", "",
		"远程节点列表，逗号分隔 (例如: node1,node2)")
	imgSyncCmd.Flags().StringVarP(&outputDir, "output-dir", "d", "./images",
		"输出目录（兼容模式，原生模式不使用）")
	imgSyncCmd.Flags().BoolVarP(&cleanup, "cleanup", "c", false,
		"处理完成后清理临时文件（兼容模式）")

	// 注册补全函数
	registerImgSyncCompletions()
}

// registerImgSyncCompletions 注册参数补全
func registerImgSyncCompletions() {
	// 节点列表补全（可扩展为动态获取）
	imgSyncCmd.RegisterFlagCompletionFunc("nodes",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// TODO: 可以从配置文件或 SSH 配置中读取节点列表
			return nil, cobra.ShellCompDirectiveNoFileComp
		})
}

func runImgSync(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	ctx := context.Background()

	// 验证镜像名称
	if imageName == "" {
		return fmt.Errorf("必须指定镜像名称 (使用 -i 或 --image)")
	}

	// 解析节点列表
	var nodeList []string
	if nodes != "" {
		nodeList = strings.Split(nodes, ",")
		for i, n := range nodeList {
			nodeList[i] = strings.TrimSpace(n)
		}
	}

	// 创建同步选项
	opts := imgsync.SyncOptions{
		OutputDir: outputDir,
		Nodes:     nodeList,
		Cleanup:   cleanup,
		Verbose:   verbose,
		ProgressCb: func(stage string, progress float64, message string) {
			if verbose {
				fmt.Printf("[%s] %s\n", stage, message)
			} else {
				// 简洁模式：只显示关键阶段
				switch stage {
				case "拉取", "同步", "分发", "完成":
					fmt.Printf("[%s] %s\n", stage, message)
				}
			}
		},
	}

	// 执行同步
	result, err := imgsync.SyncImage(ctx, imageName, opts)
	if err != nil {
		return fmt.Errorf("同步失败: %w", err)
	}

	// 输出结果
	fmt.Println("\n========== 同步结果 ==========")
	fmt.Printf("镜像: %s\n", result.ImageName)
	fmt.Printf("本地导入: %v\n", result.LocalImported)
	fmt.Printf("耗时: %v\n", result.Duration)

	if len(result.RemoteNodes) > 0 {
		fmt.Println("\n远程节点状态:")
		for node, nodeErr := range result.RemoteNodes {
			if nodeErr != nil {
				fmt.Printf("  ❌ %s: %v\n", node, nodeErr)
			} else {
				fmt.Printf("  ✅ %s: 成功\n", node)
			}
		}
	}

	// 检查是否有失败的节点
	for _, nodeErr := range result.RemoteNodes {
		if nodeErr != nil {
			os.Exit(1)
		}
	}

	return nil
}
