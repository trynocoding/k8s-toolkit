package imgsync

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// packToTarGz 将多个 tar 文件打包为 tar.gz
func packToTarGz(imageName string, archTars map[string]string, outputDir string) (string, error) {
	// 生成归档文件名: imageName_version_all.tar.gz
	archiveName := generateArchiveName(imageName)
	archivePath := filepath.Join(outputDir, archiveName)

	// 创建输出文件
	file, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("创建归档文件失败: %w", err)
	}
	defer file.Close()

	// 创建 gzip 写入器
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	// 创建 tar 写入器
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// 添加每个架构的 tar 文件
	for arch, tarPath := range archTars {
		if err := addFileToTar(tarWriter, tarPath, arch); err != nil {
			return "", fmt.Errorf("添加 %s tar 文件失败: %w", arch, err)
		}
	}

	return archivePath, nil
}

// addFileToTar 将文件添加到 tar 归档
func addFileToTar(tw *tar.Writer, filepath, arch string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 创建 tar header
	header := &tar.Header{
		Name:    filepath[len(filepath)-len(stat.Name()):], // 使用文件名
		Mode:    0644,
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("写入 tar header 失败: %w", err)
	}

	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("写入文件内容失败: %w", err)
	}

	return nil
}

// generateArchiveName 生成归档文件名
// 格式: imageName_tag_all.tar.gz
// 例如: golang:1.25.5 -> golang_1.25.5_all.tar.gz
func generateArchiveName(imageName string) string {
	// 移除注册中心前缀 (如 docker.io/)
	parts := strings.Split(imageName, "/")
	name := parts[len(parts)-1]

	// 替换 : 为 _
	name = strings.ReplaceAll(name, ":", "_")

	// 添加后缀
	return name + "_all.tar.gz"
}
