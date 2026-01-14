package imgsync

import (
	"context"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

// RemoteNode 表示远程节点
type RemoteNode struct {
	Host      string
	User      string
	SSHConfig *ssh.ClientConfig
	Namespace string // containerd namespace，默认 k8s.io
}

// DistributeToNodes 并行分发镜像到远程节点
func DistributeToNodes(ctx context.Context, docker *DockerClient, imageName string, nodes []string, verbose bool) map[string]error {
	var wg sync.WaitGroup
	results := make(map[string]error)
	var mu sync.Mutex

	for _, node := range nodes {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			err := distributeToNode(ctx, docker, imageName, n, verbose)
			mu.Lock()
			results[n] = err
			mu.Unlock()
		}(node)
	}

	wg.Wait()
	return results
}

// distributeToNode 分发镜像到单个远程节点
func distributeToNode(ctx context.Context, docker *DockerClient, imageName, node string, verbose bool) error {
	if verbose {
		fmt.Printf("[%s] 开始分发镜像 %s\n", node, imageName)
	}

	// 获取镜像流
	reader, err := docker.SaveToStream(ctx, imageName)
	if err != nil {
		return fmt.Errorf("获取镜像流失败: %w", err)
	}
	defer reader.Close()

	// 通过 SSH 流式传输到远程节点
	if err := streamToRemote(ctx, node, reader, verbose); err != nil {
		return fmt.Errorf("传输到 %s 失败: %w", node, err)
	}

	if verbose {
		fmt.Printf("[%s] 分发完成\n", node)
	}

	return nil
}

// streamToRemote 通过 SSH 流式传输镜像到远程节点
func streamToRemote(ctx context.Context, node string, reader io.Reader, verbose bool) error {
	// 使用 SSH 执行远程命令：ctr -n k8s.io images import -
	// 这里通过 stdin 传输镜像数据，避免临时文件

	// 为了简化实现，我们使用 os/exec 调用 ssh 命令
	// 在生产环境中，可以使用 golang.org/x/crypto/ssh 库

	return streamViaSSHCommand(ctx, node, reader, verbose)
}

// streamViaSSHCommand 通过 ssh 命令流式传输
func streamViaSSHCommand(ctx context.Context, node string, reader io.Reader, verbose bool) error {
	// 构建 SSH 命令
	// ssh node "ctr -n k8s.io images import -"

	cmd := newSSHCommand(ctx, node, "ctr", "-n", "k8s.io", "images", "import", "-")

	// 设置标准输入为镜像流
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取 stdin 管道失败: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 SSH 命令失败: %w", err)
	}

	// 流式传输镜像数据
	go func() {
		defer stdin.Close()
		io.Copy(stdin, reader)
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("SSH 命令执行失败: %w", err)
	}

	return nil
}
