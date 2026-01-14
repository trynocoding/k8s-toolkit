package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information
	Version   = "0.1.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "k8s-toolkit",
	Short: "Kubernetes工具集 - 集成常用K8s运维脚本",
	Long: `k8s-toolkit 是一个用Go编写的Kubernetes运维工具集。
	
它整合了多个常用的bash脚本，提供统一的命令行接口：
- enter-ns: 进入Pod的网络命名空间
- img-sync: Docker镜像同步和分发工具

所有功能都打包在单一二进制文件中，无需额外依赖。`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// 全局标志
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "详细输出模式")
}
