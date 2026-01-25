package imgsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MultiArchOptions 多架构同步选项
type MultiArchOptions struct {
	ImageName     string   // 镜像名称
	Architectures []string // 架构列表 ["amd64", "arm64"]
	OutputDir     string   // 输出目录
	Cleanup       bool     // 是否清理临时文件
	Verbose       bool     // 详细模式
	ProgressCb    func(stage string, progress float64, message string)
}

// MultiArchResult 多架构同步结果
type MultiArchResult struct {
	ImageName   string            // 镜像名称
	ArchivePath string            // 最终 tar.gz 路径
	ArchTars    map[string]string // arch -> tar 路径
	Errors      map[string]error  // arch -> 错误
	Duration    time.Duration     // 总耗时
}

// PullAndPackMultiArch 拉取多架构镜像并打包
func PullAndPackMultiArch(ctx context.Context, opts MultiArchOptions) (*MultiArchResult, error) {
	startTime := time.Now()

	result := &MultiArchResult{
		ImageName: opts.ImageName,
		ArchTars:  make(map[string]string),
		Errors:    make(map[string]error),
	}

	// 进度回调包装
	progress := func(stage string, pct float64, msg string) {
		if opts.ProgressCb != nil {
			opts.ProgressCb(stage, pct, msg)
		}
	}

	// 确保输出目录存在
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 1. 创建 Docker 客户端
	progress("初始化", 0, "创建 Docker 客户端...")
	docker, err := NewDockerClient()
	if err != nil {
		return nil, fmt.Errorf("创建 Docker 客户端失败: %w", err)
	}
	defer docker.Close()

	// 2. 并发拉取多架构镜像
	progress("拉取", 0.1, fmt.Sprintf("开始拉取 %d 个架构的镜像...", len(opts.Architectures)))

	var wg sync.WaitGroup
	var mu sync.Mutex
	tempTags := []string{} // 记录临时标签用于清理

	for i, arch := range opts.Architectures {
		wg.Add(1)
		go func(a string, index int) {
			defer wg.Done()

			archProgress := float64(index) / float64(len(opts.Architectures))

			// 2.1 拉取指定平台镜像
			progress("拉取", 0.1+archProgress*0.4, fmt.Sprintf("正在拉取 %s 架构...", a))
			if err := docker.PullPlatform(ctx, opts.ImageName, a); err != nil {
				mu.Lock()
				result.Errors[a] = fmt.Errorf("拉取失败: %w", err)
				mu.Unlock()
				return
			}

			// 2.2 创建临时标签 (格式: image:tag-arch)
			archTag := generateArchTag(opts.ImageName, a)
			if err := docker.Tag(ctx, opts.ImageName, archTag); err != nil {
				mu.Lock()
				result.Errors[a] = fmt.Errorf("创建标签失败: %w", err)
				mu.Unlock()
				return
			}

			mu.Lock()
			tempTags = append(tempTags, archTag)
			mu.Unlock()

			// 2.3 保存为 tar 文件
			tarName := generateTarName(opts.ImageName, a)
			tarPath := filepath.Join(opts.OutputDir, tarName)

			progress("保存", 0.5+archProgress*0.3, fmt.Sprintf("正在保存 %s 镜像到 tar...", a))
			if err := docker.SaveToFile(ctx, archTag, tarPath); err != nil {
				mu.Lock()
				result.Errors[a] = fmt.Errorf("保存失败: %w", err)
				mu.Unlock()
				return
			}

			mu.Lock()
			result.ArchTars[a] = tarPath
			mu.Unlock()

			if opts.Verbose {
				progress("保存", 0.5+archProgress*0.3, fmt.Sprintf("✓ %s 架构保存完成: %s", a, tarPath))
			}
		}(arch, i)
	}

	wg.Wait()

	// 检查是否有错误
	if len(result.Errors) > 0 {
		// 部分失败，仍然继续打包成功的架构
		if len(result.Errors) == len(opts.Architectures) {
			return result, fmt.Errorf("所有架构拉取失败")
		}
		progress("警告", 0.8, fmt.Sprintf("%d/%d 架构拉取成功", len(result.ArchTars), len(opts.Architectures)))
	}

	// 3. 打包所有 tar 为 tar.gz
	if len(result.ArchTars) > 0 {
		progress("打包", 0.8, "正在创建 tar.gz 归档...")
		archivePath, err := packToTarGz(opts.ImageName, result.ArchTars, opts.OutputDir)
		if err != nil {
			return result, fmt.Errorf("打包失败: %w", err)
		}
		result.ArchivePath = archivePath
		progress("打包", 0.9, fmt.Sprintf("归档创建完成: %s", archivePath))
	}

	// 4. 清理临时文件
	if opts.Cleanup {
		progress("清理", 0.95, "正在清理临时文件...")

		// 清理临时镜像标签
		for _, tag := range tempTags {
			docker.ImageRemove(ctx, tag)
		}

		// 清理临时 tar 文件
		for _, tarPath := range result.ArchTars {
			os.Remove(tarPath)
		}

		progress("清理", 1.0, "临时文件清理完成")
	}

	result.Duration = time.Since(startTime)
	progress("完成", 1.0, fmt.Sprintf("总耗时: %v", result.Duration))

	return result, nil
}

// generateArchTag 生成架构专用标签
// 例如: golang:1.25.5 + amd64 -> golang:1.25.5-amd64
func generateArchTag(imageName, arch string) string {
	return fmt.Sprintf("%s-%s", imageName, arch)
}

// generateTarName 生成 tar 文件名
// 例如: golang:1.25.5 + amd64 -> golang_1.25.5_amd64.tar
func generateTarName(imageName, arch string) string {
	// 移除注册中心前缀
	parts := strings.Split(imageName, "/")
	name := parts[len(parts)-1]

	// 替换 : 为 _
	name = strings.ReplaceAll(name, ":", "_")

	return fmt.Sprintf("%s_%s.tar", name, arch)
}
