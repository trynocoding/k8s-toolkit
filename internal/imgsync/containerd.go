package imgsync

import (
	"context"
	"fmt"
	"io"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images/archive"
	"github.com/containerd/containerd/v2/pkg/namespaces"
)

// ContainerdClient 封装 Containerd 客户端操作
type ContainerdClient struct {
	client    *containerd.Client
	namespace string
}

// ContainerdOptions 创建客户端的选项
type ContainerdOptions struct {
	Socket    string // 默认 /run/containerd/containerd.sock
	Namespace string // 默认 k8s.io
}

// DefaultContainerdOptions 返回默认选项
func DefaultContainerdOptions() ContainerdOptions {
	return ContainerdOptions{
		Socket:    "/run/containerd/containerd.sock",
		Namespace: "k8s.io",
	}
}

// NewContainerdClient 创建 Containerd 客户端
func NewContainerdClient(opts ContainerdOptions) (*ContainerdClient, error) {
	if opts.Socket == "" {
		opts.Socket = "/run/containerd/containerd.sock"
	}
	if opts.Namespace == "" {
		opts.Namespace = "k8s.io"
	}

	client, err := containerd.New(opts.Socket)
	if err != nil {
		return nil, fmt.Errorf("创建 Containerd 客户端失败: %w", err)
	}

	return &ContainerdClient{
		client:    client,
		namespace: opts.Namespace,
	}, nil
}

// Close 关闭客户端连接
func (c *ContainerdClient) Close() error {
	return c.client.Close()
}

// ImportFromStream 从 tar 流导入镜像到 containerd
func (c *ContainerdClient) ImportFromStream(ctx context.Context, reader io.Reader) ([]string, error) {
	// 设置 namespace 上下文
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	// 导入镜像
	imgs, err := c.client.Import(ctx, reader, containerd.WithImportCompression())
	if err != nil {
		return nil, fmt.Errorf("导入镜像失败: %w", err)
	}

	// 收集导入的镜像名称
	var imported []string
	for _, img := range imgs {
		imported = append(imported, img.Name)
	}

	return imported, nil
}

// ListImages 列出所有镜像
func (c *ContainerdClient) ListImages(ctx context.Context) ([]string, error) {
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	images, err := c.client.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出镜像失败: %w", err)
	}

	var names []string
	for _, img := range images {
		names = append(names, img.Name())
	}
	return names, nil
}

// ImageExists 检查镜像是否存在
func (c *ContainerdClient) ImageExists(ctx context.Context, imageName string) (bool, error) {
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	_, err := c.client.GetImage(ctx, imageName)
	if err != nil {
		// containerd 没有明确的 NotFound 错误类型，检查错误消息
		return false, nil
	}
	return true, nil
}

// ExportToStream 将镜像导出为 tar 流
func (c *ContainerdClient) ExportToStream(ctx context.Context, imageName string) (io.ReadCloser, error) {
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	img, err := c.client.GetImage(ctx, imageName)
	if err != nil {
		return nil, fmt.Errorf("获取镜像失败: %w", err)
	}

	pr, pw := io.Pipe()

	go func() {
		err := c.client.Export(ctx, pw, archive.WithImage(c.client.ImageService(), img.Name()))
		pw.CloseWithError(err)
	}()

	return pr, nil
}
