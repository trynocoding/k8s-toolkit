package multiexec

import (
	"time"
)

// OutputMode 输出模式
type OutputMode string

const (
	// OutputModeGrouped 分组输出 - 执行完成后按节点分组显示
	OutputModeGrouped OutputMode = "grouped"
	// OutputModeStream 流式输出 - 实时显示每个节点的输出
	OutputModeStream OutputMode = "stream"
)

// ExecOptions 命令执行选项
type ExecOptions struct {
	Command  string        // 要执行的命令
	Nodes    []string      // 目标节点列表
	User     string        // SSH 用户名
	Password string        // SSH 密码（可选）
	Identity string        // SSH 私钥路径（可选）
	Port     string        // SSH 端口（默认 22）
	Timeout  time.Duration // 命令超时时间
	Sudo     bool          // 是否使用 sudo
	Verbose  bool          // 详细输出
	Output   OutputMode    // 输出模式
}

// NodeExecResult 单个节点的执行结果
type NodeExecResult struct {
	Node     string        // 节点名称
	Stdout   string        // 标准输出
	Stderr   string        // 标准错误
	ExitCode int           // 退出码
	Duration time.Duration // 执行耗时
	Error    error         // 执行错误（连接失败、超时等）
}

// IsSuccess 判断执行是否成功
func (r *NodeExecResult) IsSuccess() bool {
	return r.Error == nil && r.ExitCode == 0
}

// IsTimeout 判断是否超时
func (r *NodeExecResult) IsTimeout() bool {
	if r.Error != nil {
		return r.Error.Error() == "命令超时"
	}
	return false
}

// ExecResult 总执行结果
type ExecResult struct {
	Command      string                     // 执行的命令
	NodesResults map[string]*NodeExecResult // 节点名称 -> 执行结果
	TotalTime    time.Duration              // 总耗时
}

// Summary 获取执行摘要
func (r *ExecResult) Summary() (successful, failed, timeout int) {
	for _, result := range r.NodesResults {
		if result.IsSuccess() {
			successful++
		} else if result.IsTimeout() {
			timeout++
		} else {
			failed++
		}
	}
	return
}

// NodeCallback 节点事件回调
type NodeCallback func(event NodeEvent)

// NodeEventType 节点事件类型
type NodeEventType int

const (
	EventConnecting NodeEventType = iota // 正在连接
	EventConnected                       // 已连接
	EventExecuting                       // 正在执行
	EventOutput                          // 输出数据
	EventCompleted                       // 执行完成
	EventFailed                          // 执行失败
)

// NodeEvent 节点事件
type NodeEvent struct {
	Type    NodeEventType
	Node    string
	Message string
	Result  *NodeExecResult // 仅在 EventCompleted 或 EventFailed 时有值
}
