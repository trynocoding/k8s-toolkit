package imgsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// DockerClient 封装 Docker 客户端操作
type DockerClient struct {
	cli *client.Client
}

// NewDockerClient 创建 Docker 客户端
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("创建 Docker 客户端失败: %w", err)
	}
	return &DockerClient{cli: cli}, nil
}

// Close 关闭客户端连接
func (d *DockerClient) Close() error {
	return d.cli.Close()
}

// PullProgress 表示拉取进度信息
type PullProgress struct {
	Status         string `json:"status"`
	Progress       string `json:"progress,omitempty"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail,omitempty"`
	ID string `json:"id,omitempty"`
}

// Pull 拉取镜像，返回进度信息
func (d *DockerClient) Pull(ctx context.Context, imageName string, progressCb func(PullProgress)) error {
	out, err := d.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}
	defer out.Close()

	// 解析进度信息
	decoder := json.NewDecoder(out)
	for {
		var progress PullProgress
		if err := decoder.Decode(&progress); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("解析进度信息失败: %w", err)
		}
		if progressCb != nil {
			progressCb(progress)
		}
	}

	return nil
}

// SaveToStream 将镜像导出为 tar 流（无需临时文件）
func (d *DockerClient) SaveToStream(ctx context.Context, imageName string) (io.ReadCloser, error) {
	reader, err := d.cli.ImageSave(ctx, []string{imageName})
	if err != nil {
		return nil, fmt.Errorf("导出镜像流失败: %w", err)
	}
	return reader, nil
}

// ImageExists 检查镜像是否存在
func (d *DockerClient) ImageExists(ctx context.Context, imageName string) (bool, error) {
	_, _, err := d.cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetImageSize 获取镜像大小（字节）
func (d *DockerClient) GetImageSize(ctx context.Context, imageName string) (int64, error) {
	inspect, _, err := d.cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return 0, fmt.Errorf("获取镜像信息失败: %w", err)
	}
	return inspect.Size, nil
}

// PullPlatform 拉取指定平台的镜像
func (d *DockerClient) PullPlatform(ctx context.Context, imageName, arch string) error {
	platform := fmt.Sprintf("linux/%s", arch)

	out, err := d.cli.ImagePull(ctx, imageName, image.PullOptions{
		Platform: platform,
	})
	if err != nil {
		return fmt.Errorf("拉取 %s 平台镜像失败: %w", platform, err)
	}
	defer out.Close()

	// 消费输出流（避免阻塞）
	_, err = io.Copy(io.Discard, out)
	return err
}

// Tag 为镜像创建新标签
func (d *DockerClient) Tag(ctx context.Context, source, target string) error {
	if err := d.cli.ImageTag(ctx, source, target); err != nil {
		return fmt.Errorf("创建镜像标签失败: %w", err)
	}
	return nil
}

// SaveToFile 保存镜像到文件
func (d *DockerClient) SaveToFile(ctx context.Context, imageName, filepath string) error {
	reader, err := d.SaveToStream(ctx, imageName)
	if err != nil {
		return err
	}
	defer reader.Close()

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("保存镜像到文件失败: %w", err)
	}

	return nil
}

// ImageRemove 删除镜像
func (d *DockerClient) ImageRemove(ctx context.Context, imageName string) error {
	if _, err := d.cli.ImageRemove(ctx, imageName, image.RemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("删除镜像失败: %w", err)
	}
	return nil
}
