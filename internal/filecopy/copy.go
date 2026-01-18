package filecopy

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// CopyOptions 文件复制选项
type CopyOptions struct {
	SourceFile string   // 源文件路径
	DestDir    string   // 目标目录
	Nodes      []string // 目标节点列表
	User       string   // SSH 用户名
	Password   string   // SSH 密码（可选）
	Identity   string   // SSH 私钥路径（可选）
	Port       string   // SSH 端口（默认 22）
	Verify     bool     // 是否校验文件完整性
	Verbose    bool     // 详细输出
}

// NodeResult 单个节点的复制结果
type NodeResult struct {
	Error    error           // 复制错误（nil 表示成功）
	Checksum *ChecksumResult // 校验结果（如果启用校验）
}

// CopyResult 复制结果
type CopyResult struct {
	SourceFile     string
	LocalChecksum  string                 // 本地文件校验和
	NodesStatus    map[string]*NodeResult // 节点名称 -> 节点结果
	Duration       time.Duration
	VerifyDuration time.Duration // 校验耗时
}

// ProgressCallback 进度回调
type ProgressCallback func(node string, bytesWritten int64, totalBytes int64, percent float64)

// CopyToNodes 并行复制文件到多个节点
func CopyToNodes(ctx context.Context, opts CopyOptions, progressCb ProgressCallback) (*CopyResult, error) {
	startTime := time.Now()

	// 验证源文件
	fileInfo, err := os.Stat(opts.SourceFile)
	if err != nil {
		return nil, fmt.Errorf("源文件不存在: %w", err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("源路径是目录，当前仅支持单文件复制")
	}

	// 获取文件大小
	fileSize := fileInfo.Size()

	// 如果启用校验，先计算本地文件校验和
	var localChecksum string
	if opts.Verify {
		if opts.Verbose {
			fmt.Printf("计算本地文件校验和 (%s)...\n", GetChecksumAlgorithm())
		}
		localChecksum, err = CalculateFileChecksum(opts.SourceFile)
		if err != nil {
			return nil, fmt.Errorf("计算本地校验和失败: %w", err)
		}
		if opts.Verbose {
			fmt.Printf("本地校验和: %s\n\n", localChecksum)
		}
	}

	// 构建 SSH 配置
	sshConfig, err := buildSSHConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("构建 SSH 配置失败: %w", err)
	}

	// 并行复制到各节点
	var wg sync.WaitGroup
	results := make(map[string]*NodeResult)
	var mu sync.Mutex

	for _, node := range opts.Nodes {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()

			if opts.Verbose {
				fmt.Printf("[%s] 开始复制文件 %s\n", n, opts.SourceFile)
			}

			nodeResult := &NodeResult{}
			nodeResult.Error = copyToNode(ctx, opts, n, sshConfig, fileSize, progressCb)

			if nodeResult.Error != nil {
				if opts.Verbose {
					fmt.Printf("[%s] 复制失败: %v\n", n, nodeResult.Error)
				}
			} else {
				if opts.Verbose {
					fmt.Printf("[%s] 复制成功 ✓\n", n)
				}
			}

			mu.Lock()
			results[n] = nodeResult
			mu.Unlock()
		}(node)
	}

	wg.Wait()

	copyDuration := time.Since(startTime)

	// 如果启用校验，验证远程文件
	var verifyDuration time.Duration
	if opts.Verify && localChecksum != "" {
		verifyStart := time.Now()
		if opts.Verbose {
			fmt.Println("\n========== 开始文件校验 ==========")
		}

		var verifyWg sync.WaitGroup
		for _, node := range opts.Nodes {
			// 只验证复制成功的节点
			if results[node].Error != nil {
				continue
			}

			verifyWg.Add(1)
			go func(n string) {
				defer verifyWg.Done()

				if opts.Verbose {
					fmt.Printf("[%s] 验证远程文件...\n", n)
				}

				// 重新建立连接进行校验
				host, port := parseHostPort(n, opts.Port)
				addr := net.JoinHostPort(host, port)
				client, err := ssh.Dial("tcp", addr, sshConfig)
				if err != nil {
					mu.Lock()
					results[n].Checksum = &ChecksumResult{
						LocalChecksum: localChecksum,
						Error:         fmt.Errorf("SSH 连接失败: %w", err),
					}
					mu.Unlock()
					return
				}
				defer client.Close()

				fileName := filepath.Base(opts.SourceFile)
				destPath := filepath.Join(opts.DestDir, fileName)

				checksumResult, err := VerifyRemoteChecksum(client, destPath, localChecksum)
				if err != nil && opts.Verbose {
					fmt.Printf("[%s] 校验失败: %v\n", n, err)
				}

				mu.Lock()
				results[n].Checksum = checksumResult
				mu.Unlock()

				if opts.Verbose {
					if checksumResult.Verified {
						fmt.Printf("[%s] 校验通过 ✓ (远程: %s)\n", n, checksumResult.RemoteChecksum)
					} else {
						fmt.Printf("[%s] 校验失败 ✗ (期望: %s, 实际: %s)\n", n, localChecksum, checksumResult.RemoteChecksum)
					}
				}
			}(node)
		}

		verifyWg.Wait()
		verifyDuration = time.Since(verifyStart)
	}

	return &CopyResult{
		SourceFile:     opts.SourceFile,
		LocalChecksum:  localChecksum,
		NodesStatus:    results,
		Duration:       copyDuration,
		VerifyDuration: verifyDuration,
	}, nil
}

// copyToNode 复制文件到单个节点
func copyToNode(ctx context.Context, opts CopyOptions, node string, sshConfig *ssh.ClientConfig, fileSize int64, progressCb ProgressCallback) error {
	// 解析主机和端口
	host, port := parseHostPort(node, opts.Port)
	addr := net.JoinHostPort(host, port)

	if opts.Verbose {
		fmt.Printf("[%s] 连接到 %s\n", node, addr)
	}

	// 建立 SSH 连接
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()

	// 打开源文件
	srcFile, err := os.Open(opts.SourceFile)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	// 获取文件名
	fileName := filepath.Base(opts.SourceFile)
	destPath := filepath.Join(opts.DestDir, fileName)

	// 使用 SCP 协议传输文件
	err = scpUpload(client, srcFile, destPath, fileSize, func(written int64) {
		if progressCb != nil {
			percent := float64(written) / float64(fileSize) * 100
			progressCb(node, written, fileSize, percent)
		}
	})

	if err != nil {
		return fmt.Errorf("SCP 传输失败: %w", err)
	}

	return nil
}

// buildSSHConfig 构建 SSH 客户端配置
func buildSSHConfig(opts CopyOptions) (*ssh.ClientConfig, error) {
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
			username = "root" // 默认使用 root
		}
	}

	// 获取 known_hosts 回调
	hostKeyCallback := ssh.InsecureIgnoreHostKey() // 默认不验证（开发用）
	homeDir, _ := os.UserHomeDir()
	knownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")
	if cb, err := knownhosts.New(knownHostsFile); err == nil {
		hostKeyCallback = cb
	}

	return &ssh.ClientConfig{
		User:            username,
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

// formatBytes 格式化字节数为人类可读形式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
