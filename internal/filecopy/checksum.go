package filecopy

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/crypto/ssh"
)

// ChecksumResult 校验结果
type ChecksumResult struct {
	LocalChecksum  string // 本地文件校验和
	RemoteChecksum string // 远程文件校验和
	Verified       bool   // 是否校验通过
	Error          error  // 校验错误
}

// CalculateFileChecksum 计算本地文件的 xxHash64 校验和
func CalculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	hash := xxhash.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算校验和失败: %w", err)
	}

	// 返回16进制字符串
	return fmt.Sprintf("%016x", hash.Sum64()), nil
}

// VerifyRemoteChecksum 验证远程文件的校验和
// 使用 xxhsum (如果可用) 或回退到自定义实现
func VerifyRemoteChecksum(client *ssh.Client, remotePath string, expectedChecksum string) (*ChecksumResult, error) {
	result := &ChecksumResult{
		LocalChecksum: expectedChecksum,
	}

	// 尝试使用远程的 xxhsum 命令（如果安装了）
	remoteChecksum, err := calculateRemoteChecksumWithXXHsum(client, remotePath)
	if err == nil {
		result.RemoteChecksum = remoteChecksum
		result.Verified = (remoteChecksum == expectedChecksum)
		return result, nil
	}

	// 如果 xxhsum 不可用，使用 SSH 传输文件内容到本地计算
	// 这种方式会增加网络传输，但保证了一致性
	remoteChecksum, err = calculateRemoteChecksumViaTransfer(client, remotePath)
	if err != nil {
		result.Error = fmt.Errorf("远程校验失败: %w", err)
		return result, err
	}

	result.RemoteChecksum = remoteChecksum
	result.Verified = (remoteChecksum == expectedChecksum)
	return result, nil
}

// calculateRemoteChecksumWithXXHsum 使用远程的 xxhsum 命令计算校验和
func calculateRemoteChecksumWithXXHsum(client *ssh.Client, remotePath string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	// 尝试执行 xxhsum (xxHash 命令行工具)
	// 输出格式: <hash> <filename>
	cmd := fmt.Sprintf("xxhsum %s 2>/dev/null || xxh64sum %s 2>/dev/null", remotePath, remotePath)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("xxhsum 不可用: %w", err)
	}

	// 解析输出: "hash  filename"
	parts := strings.Fields(string(output))
	if len(parts) < 1 {
		return "", fmt.Errorf("解析 xxhsum 输出失败")
	}

	// 返回校验和（去除可能的前缀）
	checksum := strings.TrimSpace(parts[0])
	return checksum, nil
}

// calculateRemoteChecksumViaTransfer 通过 SSH 传输文件内容并在本地计算校验和
func calculateRemoteChecksumViaTransfer(client *ssh.Client, remotePath string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	// 使用 cat 读取远程文件内容
	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("获取 stdout 失败: %w", err)
	}

	cmd := fmt.Sprintf("cat %s", remotePath)
	if err := session.Start(cmd); err != nil {
		return "", fmt.Errorf("启动远程命令失败: %w", err)
	}

	// 计算校验和
	hash := xxhash.New()
	if _, err := io.Copy(hash, stdout); err != nil {
		return "", fmt.Errorf("读取远程文件失败: %w", err)
	}

	if err := session.Wait(); err != nil {
		return "", fmt.Errorf("远程命令执行失败: %w", err)
	}

	return fmt.Sprintf("%016x", hash.Sum64()), nil
}

// GetChecksumAlgorithm 返回使用的校验算法名称
func GetChecksumAlgorithm() string {
	return "xxHash64"
}
