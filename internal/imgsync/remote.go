package imgsync

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// RemoteNode 表示远程节点
type RemoteNode struct {
	Host      string
	Port      string // 默认 22
	User      string
	Namespace string // containerd namespace，默认 k8s.io
}

// ProgressCallback 进度回调函数
type ProgressCallback func(node string, bytesWritten int64, totalBytes int64, percent float64)

// DistributeOptions 分发选项
type DistributeOptions struct {
	Verbose    bool
	ProgressCb ProgressCallback
	SSHConfig  *ssh.ClientConfig
}

// DistributeToNodes 并行分发镜像到远程节点
func DistributeToNodes(ctx context.Context, docker *DockerClient, imageName string, nodes []string, verbose bool) map[string]error {
	opts := DistributeOptions{
		Verbose: verbose,
		ProgressCb: func(node string, written, total int64, pct float64) {
			if verbose {
				fmt.Printf("[%s] 进度: %.1f%% (%d / %d bytes)\n", node, pct, written, total)
			}
		},
	}
	return DistributeToNodesWithOptions(ctx, docker, imageName, nodes, opts)
}

// DistributeToNodesWithOptions 带选项的并行分发
func DistributeToNodesWithOptions(ctx context.Context, docker *DockerClient, imageName string, nodes []string, opts DistributeOptions) map[string]error {
	var wg sync.WaitGroup
	results := make(map[string]error)
	var mu sync.Mutex

	for _, node := range nodes {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			err := distributeToNodeWithSSH(ctx, docker, imageName, n, opts)
			mu.Lock()
			results[n] = err
			mu.Unlock()
		}(node)
	}

	wg.Wait()
	return results
}

// distributeToNodeWithSSH 使用纯 Go SSH 库分发镜像
func distributeToNodeWithSSH(ctx context.Context, docker *DockerClient, imageName, node string, opts DistributeOptions) error {
	if opts.Verbose {
		fmt.Printf("[%s] 开始分发镜像 %s\n", node, imageName)
	}

	// 1. 获取镜像流
	reader, err := docker.SaveToStream(ctx, imageName)
	if err != nil {
		return fmt.Errorf("获取镜像流失败: %w", err)
	}
	defer reader.Close()

	// 2. 建立 SSH 连接
	sshConfig := opts.SSHConfig
	if sshConfig == nil {
		sshConfig, err = getDefaultSSHConfig()
		if err != nil {
			return fmt.Errorf("获取 SSH 配置失败: %w", err)
		}
	}

	host, port := parseHostPort(node)
	addr := net.JoinHostPort(host, port)

	if opts.Verbose {
		fmt.Printf("[%s] 连接到 %s\n", node, addr)
	}

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()

	// 3. 创建 session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	// 4. 设置 stdin 管道
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取 stdin 管道失败: %w", err)
	}

	// 5. 启动远程命令
	remoteCmd := "ctr -n k8s.io images import -"
	if err := session.Start(remoteCmd); err != nil {
		return fmt.Errorf("启动远程命令失败: %w", err)
	}

	// 6. 带进度的流式传输
	var copyErr error
	var bytesWritten int64
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer stdin.Close()

		// 使用 ProgressWriter 包装
		pw := &progressWriter{
			writer: stdin,
			node:   node,
			cb:     opts.ProgressCb,
		}

		bytesWritten, copyErr = io.Copy(pw, reader)

		if opts.Verbose {
			fmt.Printf("[%s] 传输完成: %d bytes\n", node, bytesWritten)
		}
	}()

	// 7. 等待传输完成
	<-done

	// 8. 检查传输错误
	if copyErr != nil {
		return fmt.Errorf("流式传输失败: %w", copyErr)
	}

	// 9. 等待远程命令完成
	if err := session.Wait(); err != nil {
		return fmt.Errorf("远程命令执行失败: %w", err)
	}

	if opts.Verbose {
		fmt.Printf("[%s] 分发完成 ✓\n", node)
	}

	return nil
}

// progressWriter 带进度回调的 Writer
type progressWriter struct {
	writer      io.Writer
	node        string
	cb          ProgressCallback
	written     int64
	lastReport  time.Time
	reportEvery time.Duration
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		atomic.AddInt64(&pw.written, int64(n))

		// 限制回调频率（每 100ms 最多一次）
		now := time.Now()
		if pw.cb != nil && now.Sub(pw.lastReport) > 100*time.Millisecond {
			pw.lastReport = now
			// 注意：这里无法获取总大小，显示已写入字节数
			pw.cb(pw.node, pw.written, 0, 0)
		}
	}
	return n, err
}

// getDefaultSSHConfig 获取默认 SSH 配置（使用 ssh-agent）
func getDefaultSSHConfig() (*ssh.ClientConfig, error) {
	// 尝试连接 ssh-agent
	authMethods := []ssh.AuthMethod{}

	// 1. 尝试 ssh-agent
	if agentConn := getSSHAgent(); agentConn != nil {
		agentClient := agent.NewClient(agentConn)
		authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))
	}

	// 2. 尝试默认私钥
	homeDir, _ := os.UserHomeDir()
	keyFiles := []string{
		filepath.Join(homeDir, ".ssh", "id_rsa"),
		filepath.Join(homeDir, ".ssh", "id_ed25519"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa"),
	}

	for _, keyFile := range keyFiles {
		if signer := loadPrivateKey(keyFile); signer != nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未找到可用的 SSH 认证方式（请确保 ssh-agent 运行或 ~/.ssh/ 中有密钥）")
	}

	// 获取 known_hosts 回调
	hostKeyCallback := ssh.InsecureIgnoreHostKey() // 默认不验证（开发用）
	knownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")
	if cb, err := knownhosts.New(knownHostsFile); err == nil {
		hostKeyCallback = cb
	}

	return &ssh.ClientConfig{
		User:            os.Getenv("USER"),
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}, nil
}

// getSSHAgent 获取 ssh-agent 连接
func getSSHAgent() net.Conn {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil
	}
	return conn
}

// loadPrivateKey 加载私钥文件
func loadPrivateKey(path string) ssh.Signer {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil
	}
	return signer
}

// parseHostPort 解析主机和端口
func parseHostPort(node string) (host, port string) {
	host, port, err := net.SplitHostPort(node)
	if err != nil {
		// 没有端口，使用默认 22
		return node, "22"
	}
	return host, port
}
