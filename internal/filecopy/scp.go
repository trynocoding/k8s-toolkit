package filecopy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// scpUpload 使用 SCP 协议上传文件
// 参考: https://web.archive.org/web/20170215184048/https://blogs.oracle.com/janp/entry/how_the_scp_protocol_works
func scpUpload(client *ssh.Client, reader io.Reader, destPath string, fileSize int64, progressCallback func(int64)) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	// 获取目标文件名
	fileName := filepath.Base(destPath)
	destDir := filepath.Dir(destPath)

	// 创建远程命令: scp -t <目标目录>
	// -t 表示目标端（接收文件）
	remoteCmd := fmt.Sprintf("scp -t %s", destDir)

	// 设置 stdin/stdout
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取 stdin 失败: %w", err)
	}
	defer stdin.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 stdout 失败: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("获取 stderr 失败: %w", err)
	}

	// 启动远程命令
	if err := session.Start(remoteCmd); err != nil {
		return fmt.Errorf("启动远程命令失败: %w", err)
	}

	// SCP 协议流程:
	// 1. 客户端等待服务器的初始响应 (0 byte)
	if err := checkSCPResponse(stdout); err != nil {
		return fmt.Errorf("SCP 初始响应失败: %w", err)
	}

	// 2. 客户端发送文件元数据: C<权限> <大小> <文件名>\n
	// C 表示文件（D 表示目录）
	// 0644 表示权限
	header := fmt.Sprintf("C0644 %d %s\n", fileSize, fileName)
	if _, err := stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("发送 SCP 头部失败: %w", err)
	}

	// 3. 等待服务器响应
	if err := checkSCPResponse(stdout); err != nil {
		return fmt.Errorf("SCP 头部响应失败: %w", err)
	}

	// 4. 传输文件内容（带进度回调）
	var totalWritten int64
	buf := make([]byte, 32*1024) // 32KB 缓冲区

	lastReportTime := time.Now()
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			written, writeErr := stdin.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("写入数据失败: %w", writeErr)
			}
			totalWritten += int64(written)

			// 限制回调频率（每 100ms 最多一次）
			if progressCallback != nil && time.Since(lastReportTime) > 100*time.Millisecond {
				progressCallback(totalWritten)
				lastReportTime = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取源文件失败: %w", err)
		}
	}

	// 最后一次进度回调
	if progressCallback != nil {
		progressCallback(totalWritten)
	}

	// 5. 发送结束标记 (0 byte)
	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("发送结束标记失败: %w", err)
	}

	// 6. 等待服务器最终响应
	if err := checkSCPResponse(stdout); err != nil {
		return fmt.Errorf("SCP 最终响应失败: %w", err)
	}

	// 关闭 stdin，触发远程命令完成
	stdin.Close()

	// 等待命令结束
	if err := session.Wait(); err != nil {
		// 读取 stderr 获取错误信息
		stderrData, _ := io.ReadAll(stderr)
		if len(stderrData) > 0 {
			return fmt.Errorf("远程命令失败: %s", string(stderrData))
		}
		return fmt.Errorf("远程命令失败: %w", err)
	}

	return nil
}

// checkSCPResponse 检查 SCP 协议响应
// SCP 协议的响应格式:
// - 0: 成功
// - 1: 错误
// - 2: 致命错误
func checkSCPResponse(reader io.Reader) error {
	buf := make([]byte, 1)
	n, err := reader.Read(buf)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("响应长度错误: %d", n)
	}

	responseCode := buf[0]
	if responseCode == 0 {
		// 成功
		return nil
	}

	// 读取错误消息（直到换行符）
	msgBuf := make([]byte, 1024)
	msgLen := 0
	for msgLen < len(msgBuf) {
		n, err := reader.Read(msgBuf[msgLen : msgLen+1])
		if err != nil {
			break
		}
		if n == 1 && msgBuf[msgLen] == '\n' {
			break
		}
		msgLen += n
	}

	errMsg := string(msgBuf[:msgLen])
	if responseCode == 1 {
		return fmt.Errorf("SCP 错误: %s", errMsg)
	}
	return fmt.Errorf("SCP 致命错误: %s", errMsg)
}

// SCPUploadFile 上传单个文件（便捷函数）
func SCPUploadFile(client *ssh.Client, localPath, remotePath string, verbose bool) error {
	// 打开本地文件
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer file.Close()

	// 获取文件大小
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}
	fileSize := info.Size()

	// 上传
	return scpUpload(client, file, remotePath, fileSize, func(written int64) {
		if verbose {
			percent := float64(written) / float64(fileSize) * 100
			fmt.Printf("进度: %.1f%% (%s / %s)\n", percent, formatBytes(written), formatBytes(fileSize))
		}
	})
}
