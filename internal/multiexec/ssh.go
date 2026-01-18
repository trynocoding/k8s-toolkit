package multiexec

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHConfig SSH连接配置
type SSHConfig struct {
	User     string
	Password string
	Identity string
	Port     string
	Timeout  time.Duration
}

// BuildSSHClientConfig 构建SSH客户端配置
func BuildSSHClientConfig(opts ExecOptions) (*ssh.ClientConfig, error) {
	authMethods := []ssh.AuthMethod{}

	// 1. 如果提供了密码，使用密码认证
	if opts.Password != "" {
		authMethods = append(authMethods, ssh.Password(opts.Password))
	}

	// 2. 如果提供了私钥路径，使用私钥认证
	if opts.Identity != "" {
		signer := loadPrivateKey(opts.Identity)
		if signer == nil {
			return nil, fmt.Errorf("加载私钥失败: %s", opts.Identity)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// 3. 如果没有提供密码和私钥，尝试默认认证方式
	if len(authMethods) == 0 {
		// 尝试 ssh-agent
		if agentConn := getSSHAgent(); agentConn != nil {
			agentClient := agent.NewClient(agentConn)
			authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))
		}

		// 尝试默认私钥
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
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未找到可用的 SSH 认证方式（请提供密码、私钥路径，或确保 ssh-agent 运行）")
	}

	// 设置用户名
	username := opts.User
	if username == "" {
		username = os.Getenv("USER")
		if username == "" {
			username = os.Getenv("USERNAME") // Windows
			if username == "" {
				username = "root" // 默认使用 root
			}
		}
	}

	// 获取 known_hosts 回调
	hostKeyCallback := ssh.InsecureIgnoreHostKey() // 默认不验证
	homeDir, _ := os.UserHomeDir()
	knownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")
	if cb, err := knownhosts.New(knownHostsFile); err == nil {
		hostKeyCallback = cb
	}

	// 设置超时
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}, nil
}

// ExecuteOnNode 在单个节点上执行命令
func ExecuteOnNode(ctx context.Context, node string, opts ExecOptions, sshConfig *ssh.ClientConfig, callback NodeCallback) *NodeExecResult {
	startTime := time.Now()
	result := &NodeExecResult{
		Node: node,
	}

	// 发送连接事件
	if callback != nil {
		callback(NodeEvent{Type: EventConnecting, Node: node, Message: "正在连接..."})
	}

	// 解析主机和端口
	host, port := parseHostPort(node, opts.Port)
	addr := net.JoinHostPort(host, port)

	// 建立 SSH 连接
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		result.Error = fmt.Errorf("SSH 连接失败: %w", err)
		result.Duration = time.Since(startTime)
		if callback != nil {
			callback(NodeEvent{Type: EventFailed, Node: node, Message: result.Error.Error(), Result: result})
		}
		return result
	}
	defer client.Close()

	// 发送已连接事件
	if callback != nil {
		callback(NodeEvent{Type: EventConnected, Node: node, Message: "已连接"})
	}

	// 创建 session
	session, err := client.NewSession()
	if err != nil {
		result.Error = fmt.Errorf("创建 SSH session 失败: %w", err)
		result.Duration = time.Since(startTime)
		if callback != nil {
			callback(NodeEvent{Type: EventFailed, Node: node, Message: result.Error.Error(), Result: result})
		}
		return result
	}
	defer session.Close()

	// 构造命令
	cmd := opts.Command
	if opts.Sudo {
		cmd = "sudo " + cmd
	}

	// 发送执行事件
	if callback != nil {
		callback(NodeEvent{Type: EventExecuting, Node: node, Message: cmd})
	}

	// 捕获输出
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// 使用 goroutine 执行命令，支持超时
	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	// 等待命令完成或超时
	select {
	case err := <-done:
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
		result.Duration = time.Since(startTime)

		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				result.ExitCode = exitErr.ExitStatus()
			} else {
				result.Error = err
			}
		} else {
			result.ExitCode = 0
		}

		// 发送完成事件
		if callback != nil {
			if result.IsSuccess() {
				callback(NodeEvent{Type: EventCompleted, Node: node, Message: "执行完成", Result: result})
			} else {
				callback(NodeEvent{Type: EventFailed, Node: node, Message: "执行失败", Result: result})
			}
		}

	case <-ctx.Done():
		result.Error = fmt.Errorf("命令超时")
		result.Duration = time.Since(startTime)
		// 尝试终止命令
		session.Signal(ssh.SIGKILL)
		if callback != nil {
			callback(NodeEvent{Type: EventFailed, Node: node, Message: "命令超时", Result: result})
		}
	}

	return result
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
func parseHostPort(node, defaultPort string) (host, port string) {
	host, port, err := net.SplitHostPort(node)
	if err != nil {
		// 没有端口，使用默认端口
		if defaultPort == "" {
			defaultPort = "22"
		}
		return node, defaultPort
	}
	return host, port
}
