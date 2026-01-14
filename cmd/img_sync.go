package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var imgSyncCmd = &cobra.Command{
	Use:   "img-sync -i IMAGE [OPTIONS]",
	Short: "Docker镜像同步和分发工具",
	Long: `拉取Docker镜像，导出为tar，导入到containerd，并可选地分发到远程节点。

这个工具自动化了镜像迁移流程:
1. 使用docker拉取镜像
2. 导出为tar文件
3. 导入到本地containerd (k8s.io namespace)
4. (可选) 通过SSH分发到远程节点

示例:
  # 拉取并同步nginx镜像
  k8s-toolkit img-sync -i nginx:latest

  # 同步并分发到远程节点
  k8s-toolkit img-sync -i redis:alpine -n node1,node2,node3

  # 指定输出目录并在完成后清理
  k8s-toolkit img-sync -i mysql:8.0 -d /tmp/images -c

  # 详细模式查看执行过程
  k8s-toolkit img-sync -i nginx:latest -v`,
	RunE: runImgSync,
}

var (
	imageName  string
	nodes      string
	outputDir  string
	cleanup    bool
)

func init() {
	rootCmd.AddCommand(imgSyncCmd)

	imgSyncCmd.Flags().StringVarP(&imageName, "image", "i", "", 
		"要处理的镜像名称 (必需)")
	imgSyncCmd.MarkFlagRequired("image")

	imgSyncCmd.Flags().StringVarP(&nodes, "nodes", "n", "", 
		"远程节点列表，逗号分隔 (例如: node1,node2)")
	imgSyncCmd.Flags().StringVarP(&outputDir, "output-dir", "d", "./images", 
		"输出目录")
	imgSyncCmd.Flags().BoolVarP(&cleanup, "cleanup", "c", false, 
		"处理完成后清理临时文件")
}

func runImgSync(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	// 验证镜像名称
	if imageName == "" {
		return fmt.Errorf("必须指定镜像名称 (使用 -i 或 --image)")
	}

	// 创建临时脚本文件
	tmpDir, err := ioutil.TempDir("", "k8s-toolkit-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "img_tool.sh")
	if err := ioutil.WriteFile(scriptPath, []byte(imgToolScript), 0755); err != nil {
		return fmt.Errorf("写入脚本文件失败: %w", err)
	}

	// 构建bash命令参数
	bashArgs := []string{scriptPath, "-i", imageName}

	if nodes != "" {
		bashArgs = append(bashArgs, "-n", nodes)
	}
	if outputDir != "./images" {
		bashArgs = append(bashArgs, "-d", outputDir)
	}
	if cleanup {
		bashArgs = append(bashArgs, "-c")
	}

	// 执行bash脚本
	bashCmd := exec.Command("bash", bashArgs...)
	bashCmd.Stdin = os.Stdin
	bashCmd.Stdout = os.Stdout
	bashCmd.Stderr = os.Stderr

	// 如果verbose模式，让bash脚本也verbose (它使用set -x)
	if verbose {
		fmt.Printf("[DEBUG] 执行命令: bash %s\n", strings.Join(bashArgs, " "))
		bashCmd.Env = append(os.Environ(), "VERBOSE=1")
	}

	if err := bashCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("执行脚本失败: %w", err)
	}

	return nil
}
