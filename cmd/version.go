package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long:  `显示k8s-toolkit的版本、构建日期和Git提交信息。`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("k8s-toolkit version %s\n", Version)
		fmt.Printf("Build Date: %s\n", BuildDate)
		fmt.Printf("Git Commit: %s\n", GitCommit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
