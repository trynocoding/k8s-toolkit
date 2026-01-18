package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trynocoding/k8s-toolkit/internal/multiexec"
)

var multiExecCmd = &cobra.Command{
	Use:   "multi-exec -c COMMAND -n NODES [OPTIONS]",
	Short: "多节点命令执行工具 - 在多个节点上并行执行相同命令",
	Long: `在多个远程节点上并行执行相同的命令，实时显示执行结果。

这个工具解决了需要在多个服务器上执行相同命令的痛点，无需再手动切换多个 SSH 会话。

支持两种输出模式:
- grouped: 执行完成后按节点分组显示结果（默认）
- stream:  实时显示每个节点的输出

示例:
  # 在多个节点上执行命令
  k8s-toolkit multi-exec -c "uptime" -n node1,node2,node3

  # 使用 sudo 执行
  k8s-toolkit multi-exec -c "systemctl status kubelet" -n node1,node2 --sudo

  # 指定用户和超时
  k8s-toolkit multi-exec -c "docker ps" -n node1,node2 -u root -t 60s

  # 使用密码认证
  k8s-toolkit multi-exec -c "df -h" -n node1,node2 -p yourpassword

  # 使用私钥认证
  k8s-toolkit multi-exec -c "free -m" -n node1,node2 -i ~/.ssh/my_key

  # 流式输出模式（实时显示）
  k8s-toolkit multi-exec -c "tail -n 10 /var/log/syslog" -n node1,node2 -o stream

  # 详细模式
  k8s-toolkit multi-exec -c "ls -la" -n node1,node2 -v`,
	RunE: runMultiExec,
}

var (
	execCommand  string
	execNodes    string
	execUser     string
	execPassword string
	execIdentity string
	execPort     string
	execTimeout  string
	execSudo     bool
	execOutput   string
)

func init() {
	rootCmd.AddCommand(multiExecCmd)

	multiExecCmd.Flags().StringVarP(&execCommand, "command", "c", "",
		"要执行的命令 (必需)")
	multiExecCmd.MarkFlagRequired("command")

	multiExecCmd.Flags().StringVarP(&execNodes, "nodes", "n", "",
		"目标节点列表，逗号分隔 (必需，例如: node1,node2,node3)")
	multiExecCmd.MarkFlagRequired("nodes")

	multiExecCmd.Flags().StringVarP(&execUser, "user", "u", "",
		"SSH 用户名 (默认: 当前用户)")

	multiExecCmd.Flags().StringVarP(&execPassword, "password", "p", "",
		"SSH 密码 (可选，适用于所有节点密码相同的情况)")

	multiExecCmd.Flags().StringVarP(&execIdentity, "identity", "i", "",
		"SSH 私钥路径 (可选，默认使用 ~/.ssh/id_rsa 等)")

	multiExecCmd.Flags().StringVar(&execPort, "port", "22",
		"SSH 端口 (默认: 22)")

	multiExecCmd.Flags().StringVarP(&execTimeout, "timeout", "t", "30s",
		"命令执行超时时间 (默认: 30s，支持: 10s, 1m, 2m30s)")

	multiExecCmd.Flags().BoolVar(&execSudo, "sudo", false,
		"使用 sudo 执行命令")

	multiExecCmd.Flags().StringVarP(&execOutput, "output", "o", "grouped",
		"输出模式: grouped(分组显示) 或 stream(实时流式)")

	// 注册补全函数
	registerMultiExecCompletions()
}

// registerMultiExecCompletions 注册参数补全
func registerMultiExecCompletions() {
	// 输出模式补全
	multiExecCmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"grouped", "stream"}, cobra.ShellCompDirectiveNoFileComp
		})

	// 节点列表补全（可扩展为动态获取）
	multiExecCmd.RegisterFlagCompletionFunc("nodes",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// TODO: 可以从配置文件中读取节点列表
			return nil, cobra.ShellCompDirectiveNoFileComp
		})
}

func runMultiExec(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	ctx := context.Background()

	// 验证参数
	if execCommand == "" {
		return fmt.Errorf("必须指定要执行的命令 (使用 -c 或 --command)")
	}
	if execNodes == "" {
		return fmt.Errorf("必须指定目标节点 (使用 -n 或 --nodes)")
	}

	// 解析节点列表
	nodeList := strings.Split(execNodes, ",")
	for i, n := range nodeList {
		nodeList[i] = strings.TrimSpace(n)
	}

	if len(nodeList) == 0 {
		return fmt.Errorf("节点列表为空")
	}

	// 解析超时时间
	timeout, err := time.ParseDuration(execTimeout)
	if err != nil {
		return fmt.Errorf("无效的超时时间格式: %s (示例: 30s, 1m, 2m30s)", execTimeout)
	}

	// 解析输出模式
	var outputMode multiexec.OutputMode
	switch strings.ToLower(execOutput) {
	case "grouped":
		outputMode = multiexec.OutputModeGrouped
	case "stream":
		outputMode = multiexec.OutputModeStream
	default:
		return fmt.Errorf("无效的输出模式: %s (可选: grouped, stream)", execOutput)
	}

	// 创建执行选项
	opts := multiexec.ExecOptions{
		Command:  execCommand,
		Nodes:    nodeList,
		User:     execUser,
		Password: execPassword,
		Identity: execIdentity,
		Port:     execPort,
		Timeout:  timeout,
		Sudo:     execSudo,
		Verbose:  verbose,
		Output:   outputMode,
	}

	// 创建输出写入器
	output := multiexec.NewOutputWriter(os.Stdout, outputMode, verbose)

	// 显示任务信息
	output.WriteHeader(execCommand, nodeList)

	// 创建事件回调
	var callback multiexec.NodeCallback
	if outputMode == multiexec.OutputModeStream {
		callback = func(event multiexec.NodeEvent) {
			output.WriteEvent(event)
		}
	} else {
		// 分组模式：显示进度
		callback = func(event multiexec.NodeEvent) {
			if event.Type == multiexec.EventCompleted || event.Type == multiexec.EventFailed {
				if event.Result != nil {
					output.WriteProgress(event.Node, event.Result.IsSuccess(),
						event.Result.Duration, event.Result.Error)
				}
			}
		}
	}

	// 执行命令
	result, err := multiexec.ExecuteOnNodes(ctx, opts, callback)
	if err != nil {
		return fmt.Errorf("执行失败: %w", err)
	}

	// 输出分组结果（仅分组模式）
	if outputMode == multiexec.OutputModeGrouped {
		output.WriteGroupedResults(result, nodeList)
	}

	// 输出摘要
	output.WriteSummary(result)

	// 检查是否有失败的节点
	successful, failed, timedOut := result.Summary()
	if failed > 0 || timedOut > 0 {
		// 有失败，返回非零退出码
		os.Exit(1)
	}

	_ = successful // 避免未使用警告

	return nil
}
