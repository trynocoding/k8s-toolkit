package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trynocoding/k8s-toolkit/internal/imgsync"
)

var multiarchCmd = &cobra.Command{
	Use:   "img-multiarch -i IMAGE -a ARCH1,ARCH2 [OPTIONS]",
	Short: "多架构镜像拉取与打包工具",
	Long: `拉取多个架构的镜像并打包为 tar.gz 归档文件。

适用场景:
  - 离线环境部署准备
  - 多架构集群批量迁移
  - 镜像版本归档管理

工作流程:
  1. 并发拉取指定架构的镜像 (使用 Docker SDK)
  2. 为每个架构创建临时标签
  3. 保存每个架构为独立 tar 文件
  4. 打包所有 tar 为单一 tar.gz 归档
  5. (可选) 清理临时文件和标签

示例:
  # 拉取 golang 的 amd64 和 arm64 镜像
  k8s-toolkit img-multiarch -i golang:1.25.5 -a amd64,arm64

  # 指定输出目录
  k8s-toolkit img-multiarch -i nginx:latest -a amd64,arm64,arm -o /data/images

  # 完成后清理临时标签和 tar 文件
  k8s-toolkit img-multiarch -i redis:7 -a amd64,arm64 -c

  # 详细模式查看每个步骤
  k8s-toolkit img-multiarch -i alpine:3.18 -a amd64,arm64 -v`,
	RunE: runMultiarch,
}

var (
	multiarchImage   string
	architectures    string
	multiarchOutput  string
	multiarchCleanup bool
)

func init() {
	rootCmd.AddCommand(multiarchCmd)

	multiarchCmd.Flags().StringVarP(&multiarchImage, "image", "i", "",
		"镜像名称 (必需)")
	multiarchCmd.MarkFlagRequired("image")

	multiarchCmd.Flags().StringVarP(&architectures, "arch", "a", "",
		"架构列表，逗号分隔 (例: amd64,arm64,arm)")
	multiarchCmd.MarkFlagRequired("arch")

	multiarchCmd.Flags().StringVarP(&multiarchOutput, "output", "o", "./images",
		"输出目录")
	multiarchCmd.Flags().BoolVarP(&multiarchCleanup, "cleanup", "c", false,
		"完成后清理临时镜像标签和 tar 文件")

	// 注册补全函数
	registerMultiarchCompletions()
}

// registerMultiarchCompletions 注册参数补全
func registerMultiarchCompletions() {
	// 架构列表补全
	multiarchCmd.RegisterFlagCompletionFunc("arch",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"amd64\t常见 x86-64 架构",
				"arm64\t常见 ARM64 架构",
				"arm\tARM 32位架构",
				"ppc64le\tPowerPC 64位小端",
				"s390x\tIBM Z 系列",
			}, cobra.ShellCompDirectiveNoFileComp
		})
}

func runMultiarch(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	ctx := context.Background()

	// 验证镜像名称
	if multiarchImage == "" {
		return fmt.Errorf("必须指定镜像名称 (使用 -i 或 --image)")
	}

	// 解析架构列表
	if architectures == "" {
		return fmt.Errorf("必须指定架构列表 (使用 -a 或 --arch)")
	}

	archList := strings.Split(architectures, ",")
	for i, a := range archList {
		archList[i] = strings.TrimSpace(a)
	}

	if len(archList) == 0 {
		return fmt.Errorf("架构列表不能为空")
	}

	// 打印任务信息
	fmt.Printf("========== 多架构镜像拉取 ==========\n")
	fmt.Printf("镜像: %s\n", multiarchImage)
	fmt.Printf("架构: %s\n", strings.Join(archList, ", "))
	fmt.Printf("输出: %s\n", multiarchOutput)
	fmt.Println()

	// 创建同步选项
	opts := imgsync.MultiArchOptions{
		ImageName:     multiarchImage,
		Architectures: archList,
		OutputDir:     multiarchOutput,
		Cleanup:       multiarchCleanup,
		Verbose:       verbose,
		ProgressCb: func(stage string, progress float64, message string) {
			if verbose {
				fmt.Printf("[%s] %s\n", stage, message)
			} else {
				// 简洁模式：只显示关键阶段
				switch stage {
				case "初始化", "拉取", "保存", "打包", "清理", "完成":
					fmt.Printf("[%s] %s\n", stage, message)
				}
			}
		},
	}

	// 执行同步
	result, err := imgsync.PullAndPackMultiArch(ctx, opts)
	if err != nil {
		// 显示错误详情
		if result != nil && len(result.Errors) > 0 {
			fmt.Println("\n❌ 失败的架构:")
			for arch, archErr := range result.Errors {
				fmt.Printf("  ❌ %s: %v\n", arch, archErr)
			}

			// 如果有部分成功的架构
			if len(result.ArchTars) > 0 {
				fmt.Println("\n⚠️  部分架构处理成功:")
				for arch, tarPath := range result.ArchTars {
					fmt.Printf("  ✅ %s: %s\n", arch, tarPath)
				}
			}
		}
		return fmt.Errorf("同步失败: %w", err)
	}

	// 输出结果
	fmt.Println("\n========== 处理结果 ==========")
	fmt.Printf("镜像: %s\n", result.ImageName)
	fmt.Printf("归档: %s\n", result.ArchivePath)
	fmt.Printf("耗时: %v\n", result.Duration)

	if len(result.Errors) > 0 {
		fmt.Println("\n⚠️  部分架构失败:")
		for arch, archErr := range result.Errors {
			fmt.Printf("  ❌ %s: %v\n", arch, archErr)
		}
	} else {
		fmt.Println("\n✅ 所有架构处理成功!")
	}

	fmt.Println("\n包含文件:")
	for _, tarPath := range result.ArchTars {
		stat, _ := os.Stat(tarPath)
		if stat != nil {
			fmt.Printf("  - %s (%d MB)\n", tarPath, stat.Size()/1024/1024)
		} else {
			fmt.Printf("  - %s (已清理)\n", tarPath)
		}
	}

	// 如果有失败的架构，退出码为 1
	if len(result.Errors) > 0 {
		os.Exit(1)
	}

	return nil
}
