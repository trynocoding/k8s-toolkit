package imgsync

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// SyncOptions 同步选项
type SyncOptions struct {
	OutputDir  string   // 输出目录（仅用于传统模式）
	Nodes      []string // 远程节点列表
	Cleanup    bool     // 是否清理临时文件
	Verbose    bool     // 详细模式
	ProgressCb func(stage string, progress float64, message string)
}

// SyncResult 同步结果
type SyncResult struct {
	ImageName     string
	LocalImported bool
	RemoteNodes   map[string]error // 节点 -> 错误（nil 表示成功）
	Duration      time.Duration
}

// SyncImage 流式同步镜像：Docker → Containerd
func SyncImage(ctx context.Context, imageName string, opts SyncOptions) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		ImageName:   imageName,
		RemoteNodes: make(map[string]error),
	}

	// 进度回调封装
	progress := func(stage string, pct float64, msg string) {
		if opts.ProgressCb != nil {
			opts.ProgressCb(stage, pct, msg)
		}
	}

	// 1. 创建 Docker 客户端
	progress("初始化", 0, "创建 Docker 客户端...")
	docker, err := NewDockerClient()
	if err != nil {
		return nil, fmt.Errorf("创建 Docker 客户端失败: %w", err)
	}
	defer docker.Close()

	// 2. 拉取镜像
	progress("拉取", 0.1, fmt.Sprintf("正在拉取镜像 %s...", imageName))
	pullCb := func(p PullProgress) {
		if opts.Verbose && p.Status != "" {
			msg := p.Status
			if p.Progress != "" {
				msg += " " + p.Progress
			}
			progress("拉取", 0.1, msg)
		}
	}
	if err := docker.Pull(ctx, imageName, pullCb); err != nil {
		return nil, fmt.Errorf("拉取镜像失败: %w", err)
	}
	progress("拉取", 0.4, "镜像拉取完成")

	// 3. 创建 Containerd 客户端
	progress("初始化", 0.4, "创建 Containerd 客户端...")
	ctrd, err := NewContainerdClient(DefaultContainerdOptions())
	if err != nil {
		return nil, fmt.Errorf("创建 Containerd 客户端失败: %w", err)
	}
	defer ctrd.Close()

	// 4. 流式传输：Docker → Containerd（核心优化点）
	progress("同步", 0.5, "正在流式传输镜像到 Containerd...")
	reader, err := docker.SaveToStream(ctx, imageName)
	if err != nil {
		return nil, fmt.Errorf("获取镜像流失败: %w", err)
	}
	defer reader.Close()

	// 直接导入到 containerd（无临时文件）
	imported, err := ctrd.ImportFromStream(ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("导入到 Containerd 失败: %w", err)
	}
	result.LocalImported = true
	progress("同步", 0.8, fmt.Sprintf("本地导入完成: %v", imported))

	// 5. 远程节点分发（如果有）
	if len(opts.Nodes) > 0 {
		progress("分发", 0.8, fmt.Sprintf("正在分发到 %d 个远程节点...", len(opts.Nodes)))
		nodeErrors := DistributeToNodes(ctx, docker, imageName, opts.Nodes, opts.Verbose)
		result.RemoteNodes = nodeErrors

		successCount := 0
		for _, err := range nodeErrors {
			if err == nil {
				successCount++
			}
		}
		progress("分发", 1.0, fmt.Sprintf("分发完成: %d/%d 成功", successCount, len(opts.Nodes)))
	}

	result.Duration = time.Since(startTime)
	progress("完成", 1.0, fmt.Sprintf("总耗时: %v", result.Duration))

	return result, nil
}

// StreamSync 执行流式同步（高级 API，支持自定义 Reader/Writer）
func StreamSync(ctx context.Context, reader io.Reader, ctrd *ContainerdClient) ([]string, error) {
	return ctrd.ImportFromStream(ctx, reader)
}

// ParallelSync 并行同步到多个目标
func ParallelSync(ctx context.Context, imageName string, targets []SyncTarget) map[string]error {
	var wg sync.WaitGroup
	results := make(map[string]error)
	var mu sync.Mutex

	for _, target := range targets {
		wg.Add(1)
		go func(t SyncTarget) {
			defer wg.Done()
			err := t.Sync(ctx, imageName)
			mu.Lock()
			results[t.Name()] = err
			mu.Unlock()
		}(target)
	}

	wg.Wait()
	return results
}

// SyncTarget 同步目标接口
type SyncTarget interface {
	Name() string
	Sync(ctx context.Context, imageName string) error
}
