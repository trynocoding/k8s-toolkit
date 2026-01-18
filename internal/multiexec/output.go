package multiexec

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// OutputWriter 输出写入器
type OutputWriter struct {
	writer  io.Writer
	mode    OutputMode
	verbose bool
	mu      sync.Mutex
}

// NewOutputWriter 创建输出写入器
func NewOutputWriter(w io.Writer, mode OutputMode, verbose bool) *OutputWriter {
	return &OutputWriter{
		writer:  w,
		mode:    mode,
		verbose: verbose,
	}
}

// WriteHeader 写入执行头部信息
func (o *OutputWriter) WriteHeader(command string, nodes []string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	fmt.Fprintf(o.writer, "执行命令: %s\n", command)
	fmt.Fprintf(o.writer, "目标节点: %s\n", strings.Join(nodes, ", "))
	fmt.Fprintln(o.writer)
}

// WriteEvent 写入节点事件（用于流式输出）
func (o *OutputWriter) WriteEvent(event NodeEvent) {
	if o.mode != OutputModeStream {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	switch event.Type {
	case EventConnecting:
		fmt.Fprintf(o.writer, "[%s] 连接中...\n", event.Node)
	case EventConnected:
		fmt.Fprintf(o.writer, "[%s] ✓ 已连接\n", event.Node)
	case EventExecuting:
		if o.verbose {
			fmt.Fprintf(o.writer, "[%s] 执行: %s\n", event.Node, event.Message)
		}
	case EventOutput:
		// 输出命令结果的每一行
		lines := strings.Split(event.Message, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Fprintf(o.writer, "[%s] %s\n", event.Node, line)
			}
		}
	case EventCompleted:
		if event.Result != nil {
			fmt.Fprintf(o.writer, "[%s] ✓ 完成 (exit: %d, %v)\n",
				event.Node, event.Result.ExitCode, event.Result.Duration.Round(time.Millisecond))
		}
	case EventFailed:
		if event.Result != nil && event.Result.Error != nil {
			fmt.Fprintf(o.writer, "[%s] ✗ 失败: %v\n", event.Node, event.Result.Error)
		} else if event.Result != nil {
			fmt.Fprintf(o.writer, "[%s] ✗ 失败 (exit: %d)\n", event.Node, event.Result.ExitCode)
		}
	}
}

// WriteProgress 写入进度信息（用于分组输出模式）
func (o *OutputWriter) WriteProgress(node string, success bool, duration time.Duration, err error) {
	if o.mode != OutputModeGrouped {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if success {
		fmt.Fprintf(o.writer, "[✓] %s 完成 (%v)\n", node, duration.Round(time.Millisecond))
	} else if err != nil {
		fmt.Fprintf(o.writer, "[✗] %s 失败: %v\n", node, err)
	} else {
		fmt.Fprintf(o.writer, "[✗] %s 失败\n", node)
	}
}

// WriteGroupedResults 写入分组结果
func (o *OutputWriter) WriteGroupedResults(result *ExecResult, nodes []string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	fmt.Fprintln(o.writer)

	// 按原始顺序输出每个节点的结果
	for _, node := range nodes {
		nodeResult, ok := result.NodesResults[node]
		if !ok {
			continue
		}

		fmt.Fprintf(o.writer, "========== %s ==========\n", node)

		if nodeResult.Error != nil {
			fmt.Fprintf(o.writer, "错误: %v\n", nodeResult.Error)
		} else {
			// 输出 stdout
			if nodeResult.Stdout != "" {
				fmt.Fprint(o.writer, nodeResult.Stdout)
				if !strings.HasSuffix(nodeResult.Stdout, "\n") {
					fmt.Fprintln(o.writer)
				}
			}
			// 输出 stderr（如果有且不同于 stdout）
			if nodeResult.Stderr != "" && nodeResult.Stderr != nodeResult.Stdout {
				if o.verbose {
					fmt.Fprintf(o.writer, "[stderr] %s", nodeResult.Stderr)
					if !strings.HasSuffix(nodeResult.Stderr, "\n") {
						fmt.Fprintln(o.writer)
					}
				}
			}
		}
		fmt.Fprintln(o.writer)
	}
}

// WriteSummary 写入执行摘要
func (o *OutputWriter) WriteSummary(result *ExecResult) {
	o.mu.Lock()
	defer o.mu.Unlock()

	successful, failed, timeout := result.Summary()
	total := len(result.NodesResults)

	fmt.Fprintln(o.writer, "========== Summary ==========")

	if successful == total {
		fmt.Fprintf(o.writer, "✅ Successful: %d/%d\n", successful, total)
	} else {
		if successful > 0 {
			fmt.Fprintf(o.writer, "✅ Successful: %d/%d\n", successful, total)
		}
		if failed > 0 {
			fmt.Fprintf(o.writer, "❌ Failed: %d/%d\n", failed, total)
		}
		if timeout > 0 {
			fmt.Fprintf(o.writer, "⏱️  Timeout: %d/%d\n", timeout, total)
		}
	}

	fmt.Fprintf(o.writer, "⏱️  Total time: %v\n", result.TotalTime.Round(time.Millisecond))
}

// FormatDuration 格式化时间
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
