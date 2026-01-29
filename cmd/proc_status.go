package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trynocoding/k8s-toolkit/internal/procinfo"
)

var procStatusCmd = &cobra.Command{
	Use:   "proc-status -p PID [OPTIONS]",
	Short: "查看进程的 Capabilities 和 Signals 信息",
	Long: `查看进程的 Linux Capabilities 和 Signals 信息，自动解码为可读格式。

可以直接查看本地进程，或通过 kubectl exec 查看 Pod 容器内的进程。

示例:
  # 查看本地进程 (PID 1234)
  k8s-toolkit proc-status --pid 1234

  # 只查看 Capabilities
  k8s-toolkit proc-status --pid 1234 --capabilities

  # 只查看 Signals
  k8s-toolkit proc-status --pid 1234 --signals

  # 查看 Pod 容器内的进程
  k8s-toolkit proc-status -p my-pod -n default --pid 1 --capabilities

  # 查看指定容器的进程
  k8s-toolkit proc-status -p my-pod -n kube-system -c 1 --pid 1

  # 详细模式
  k8s-toolkit proc-status --pid 1234 -v`,
	RunE: runProcStatus,
}

var (
	procPID              int
	procPodName          string
	procNamespace        string
	procContainerIndex   int
	procShowCapabilities bool
	procShowSignals      bool
)

func init() {
	rootCmd.AddCommand(procStatusCmd)

	// PID 参数 (必需)
	procStatusCmd.Flags().IntVar(&procPID, "pid", 0,
		"进程 PID (必需)")
	procStatusCmd.MarkFlagRequired("pid")

	// Pod 相关参数 (可选，用于查看 Pod 内进程)
	procStatusCmd.Flags().StringVarP(&procPodName, "pod", "p", "",
		"Pod 名称 (可选，用于查看 Pod 内进程)")
	procStatusCmd.Flags().StringVarP(&procNamespace, "namespace", "n", "default",
		"Kubernetes 命名空间 (默认: default)")
	procStatusCmd.Flags().IntVarP(&procContainerIndex, "container", "c", 0,
		"容器索引 (默认: 0，即第一个容器)")

	// 过滤选项
	procStatusCmd.Flags().BoolVar(&procShowCapabilities, "capabilities", false,
		"只显示 Capabilities 信息")
	procStatusCmd.Flags().BoolVar(&procShowSignals, "signals", false,
		"只显示 Signals 信息")
}

func runProcStatus(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	// 验证 PID
	if procPID <= 0 {
		return fmt.Errorf("invalid PID: %d", procPID)
	}

	// 确定要解析的内容
	parseOpts := procinfo.ParseOptions{
		ParseCapabilities: true,
		ParseSignals:      true,
		Verbose:           verbose,
	}

	// 如果指定了过滤选项，只解析对应的内容
	if procShowCapabilities && !procShowSignals {
		parseOpts.ParseSignals = false
	} else if procShowSignals && !procShowCapabilities {
		parseOpts.ParseCapabilities = false
	}

	var status *procinfo.ProcessStatus
	var err error

	// 判断是查看本地进程还是 Pod 内进程
	if procPodName == "" {
		// 查看本地进程
		status, err = procinfo.ParseProcStatus(procPID, parseOpts)
		if err != nil {
			return fmt.Errorf("failed to parse /proc/%d/status: %w", procPID, err)
		}
	} else {
		// 通过 kubectl exec 查看 Pod 内进程
		status, err = parseProcStatusInPod(procPodName, procNamespace, procContainerIndex, procPID, parseOpts, verbose)
		if err != nil {
			return fmt.Errorf("failed to parse process status in pod: %w", err)
		}
	}

	// 输出结果
	output := procinfo.FormatProcessStatus(status, parseOpts)
	fmt.Println(output)

	return nil
}

// parseProcStatusInPod 通过 kubectl exec 解析 Pod 内进程的状态
func parseProcStatusInPod(podName, namespace string, containerIndex, pid int, opts procinfo.ParseOptions, verbose bool) (*procinfo.ProcessStatus, error) {
	if verbose {
		fmt.Printf("[DEBUG] Fetching /proc/%d/status from pod %s/%s (container index: %d)\n",
			pid, namespace, podName, containerIndex)
	}

	// 构建 kubectl exec 命令
	args := []string{
		"exec",
		podName,
		"-n", namespace,
		"--container", fmt.Sprintf("$(%s)", getContainerNameCommand(containerIndex)),
		"--",
		"cat", fmt.Sprintf("/proc/%d/status", pid),
	}

	// 如果容器索引为 0，简化命令（不指定容器）
	if containerIndex == 0 {
		args = []string{
			"exec",
			podName,
			"-n", namespace,
			"--",
			"cat", fmt.Sprintf("/proc/%d/status", pid),
		}
	} else {
		// 需要先获取容器名称
		containerName, err := getContainerName(podName, namespace, containerIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to get container name: %w", err)
		}
		args = []string{
			"exec",
			podName,
			"-n", namespace,
			"--container", containerName,
			"--",
			"cat", fmt.Sprintf("/proc/%d/status", pid),
		}
	}

	// 执行 kubectl exec
	kubectlCmd := exec.Command("kubectl", args...)
	var stdout, stderr bytes.Buffer
	kubectlCmd.Stdout = &stdout
	kubectlCmd.Stderr = &stderr

	if verbose {
		fmt.Printf("[DEBUG] Executing: kubectl %s\n", strings.Join(args, " "))
	}

	if err := kubectlCmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl exec failed: %w\nStderr: %s", err, stderr.String())
	}

	// 解析输出
	status, err := parseProcStatusFromContent(stdout.Bytes(), pid, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proc status: %w", err)
	}

	return status, nil
}

// getContainerName 获取 Pod 中指定索引的容器名称
func getContainerName(podName, namespace string, containerIndex int) (string, error) {
	args := []string{
		"get", "pod", podName,
		"-n", namespace,
		"-o", fmt.Sprintf("jsonpath={.spec.containers[%d].name}", containerIndex),
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get container name: %w", err)
	}

	containerName := strings.TrimSpace(string(output))
	if containerName == "" {
		return "", fmt.Errorf("container index %d not found", containerIndex)
	}

	return containerName, nil
}

// getContainerNameCommand 返回获取容器名称的命令
func getContainerNameCommand(containerIndex int) string {
	return fmt.Sprintf("kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.spec.containers[%d].name}'", containerIndex)
}

// parseProcStatusFromContent 从内容中解析进程状态
func parseProcStatusFromContent(content []byte, pid int, opts procinfo.ParseOptions) (*procinfo.ProcessStatus, error) {
	status := &procinfo.ProcessStatus{
		PID: pid,
	}

	if opts.ParseCapabilities {
		status.Capabilities = &procinfo.CapabilitiesInfo{}
	}
	if opts.ParseSignals {
		status.Signals = &procinfo.SignalsInfo{}
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}

		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "Name":
			status.Name = value
		case "State":
			status.State = value

		// Capabilities
		case "CapInh":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Inheritable = mask
				}
			}
		case "CapPrm":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Permitted = mask
				}
			}
		case "CapEff":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Effective = mask
				}
			}
		case "CapBnd":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Bounding = mask
				}
			}
		case "CapAmb":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Ambient = mask
				}
			}

		// Signals
		case "SigQ":
			if status.Signals != nil {
				status.Signals.Queued = value
			}
		case "SigPnd":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Pending = mask
				}
			}
		case "ShdPnd":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.SharedPending = mask
				}
			}
		case "SigBlk":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Blocked = mask
				}
			}
		case "SigIgn":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Ignored = mask
				}
			}
		case "SigCgt":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Caught = mask
				}
			}
		}
	}

	return status, nil
}

// parseHexUint64 解析十六进制字符串为 uint64
func parseHexUint64(s string) (uint64, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimSpace(s)
	return strconv.ParseUint(s, 16, 64)
}
