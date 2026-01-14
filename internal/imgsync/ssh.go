package imgsync

import (
	"context"
	"os/exec"
)

// newSSHCommand 创建 SSH 命令
func newSSHCommand(ctx context.Context, host string, remoteCmd ...string) *exec.Cmd {
	// 构建完整的远程命令
	args := []string{host}
	args = append(args, remoteCmd...)

	return exec.CommandContext(ctx, "ssh", args...)
}

// NewSCPCommand 创建 SCP 命令（备用方案）
func NewSCPCommand(ctx context.Context, localPath, remotePath string) *exec.Cmd {
	return exec.CommandContext(ctx, "scp", localPath, remotePath)
}
