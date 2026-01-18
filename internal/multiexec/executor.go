package multiexec

import (
	"context"
	"sync"
	"time"
)

// ExecuteOnNodes 并行在多个节点上执行命令
func ExecuteOnNodes(ctx context.Context, opts ExecOptions, callback NodeCallback) (*ExecResult, error) {
	startTime := time.Now()

	// 构建 SSH 配置
	sshConfig, err := BuildSSHClientConfig(opts)
	if err != nil {
		return nil, err
	}

	// 设置命令超时
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 创建带超时的 context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 并行执行
	var wg sync.WaitGroup
	results := make(map[string]*NodeExecResult)
	var mu sync.Mutex

	for _, node := range opts.Nodes {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()

			result := ExecuteOnNode(execCtx, n, opts, sshConfig, callback)

			mu.Lock()
			results[n] = result
			mu.Unlock()
		}(node)
	}

	wg.Wait()

	return &ExecResult{
		Command:      opts.Command,
		NodesResults: results,
		TotalTime:    time.Since(startTime),
	}, nil
}

// ExecuteOnNodesSequential 顺序在多个节点上执行命令（用于需要顺序执行的场景）
func ExecuteOnNodesSequential(ctx context.Context, opts ExecOptions, callback NodeCallback) (*ExecResult, error) {
	startTime := time.Now()

	// 构建 SSH 配置
	sshConfig, err := BuildSSHClientConfig(opts)
	if err != nil {
		return nil, err
	}

	// 设置命令超时
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	results := make(map[string]*NodeExecResult)

	for _, node := range opts.Nodes {
		// 为每个节点创建独立的超时 context
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		result := ExecuteOnNode(execCtx, node, opts, sshConfig, callback)
		cancel()

		results[node] = result

		// 检查是否被取消
		select {
		case <-ctx.Done():
			return &ExecResult{
				Command:      opts.Command,
				NodesResults: results,
				TotalTime:    time.Since(startTime),
			}, ctx.Err()
		default:
		}
	}

	return &ExecResult{
		Command:      opts.Command,
		NodesResults: results,
		TotalTime:    time.Since(startTime),
	}, nil
}
