# k8s-toolkit

一个用Go编写的Kubernetes运维工具集，整合常用的bash脚本，提供统一的命令行接口。

## ✨ 特性

- 🚀 **单一二进制** - 所有功能打包在一个可执行文件中
- 📦 **无需依赖** - bash脚本已嵌入，无需单独分发
- 🔧 **易于扩展** - 基于Cobra框架，轻松添加新命令
- 🎯 **保留原有功能** - 完全兼容原始bash脚本的所有功能
- 🌈 **友好的CLI** - 完善的帮助文档和参数验证

## 📦 安装

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/trynocoding/k8s-toolkit.git
cd k8s-toolkit

# 构建
make build

# 或使用go build
go build -o k8s-toolkit
```

### 交叉编译

```bash
# 构建Linux版本（在macOS/Windows上）
make build-linux

# 手动指定目标平台
GOOS=linux GOARCH=amd64 go build -o k8s-toolkit-linux-amd64
```

## 🚀 使用方法

### 查看帮助

```bash
k8s-toolkit --help
```

### 命令列表

#### 1. `enter-ns` - 进入Pod网络命名空间

进入指定Kubernetes Pod的网络命名空间，用于网络调试。

**基本用法:**
```bash
# 进入default命名空间中的Pod
sudo k8s-toolkit enter-ns -p my-pod

# 进入指定命名空间的Pod
sudo k8s-toolkit enter-ns -n kube-system -p coredns-xxx
```

**高级选项:**
```bash
# 进入第二个容器的网络命名空间
sudo k8s-toolkit enter-ns -n default -p my-pod -c 1

# 指定容器运行时
sudo k8s-toolkit enter-ns -p my-pod -r containerd

# 详细输出模式
sudo k8s-toolkit enter-ns -p my-pod -v
```

**参数说明:**
- `-p, --pod` - Pod名称（必需）
- `-n, --namespace` - Kubernetes命名空间（默认: default）
- `-c, --container` - 容器索引（默认: 0）
- `-r, --runtime` - 容器运行时（auto|containerd|docker，默认: auto）
- `-v, --verbose` - 详细输出模式

**依赖要求:**
- kubectl
- jq
- containerd (ctr) 或 docker
- nsenter
- root权限

#### 2. `img-sync` - Docker镜像同步和分发

自动化Docker镜像的拉取、导出、导入到containerd，并可选地分发到远程节点。

**基本用法:**
```bash
# 拉取并同步nginx镜像
k8s-toolkit img-sync -i nginx:latest

# 同步并分发到远程节点
k8s-toolkit img-sync -i redis:alpine -n node1,node2,node3
```

**高级选项:**
```bash
# 指定输出目录
k8s-toolkit img-sync -i mysql:8.0 -d /tmp/images

# 完成后清理临时文件
k8s-toolkit img-sync -i nginx:latest -c

# 详细模式
k8s-toolkit img-sync -i nginx:latest -v
```

**参数说明:**
- `-i, --image` - 镜像名称（必需）
- `-n, --nodes` - 远程节点列表，逗号分隔（可选）
- `-d, --output-dir` - 输出目录（默认: ./images）
- `-c, --cleanup` - 完成后清理临时文件
- `-v, --verbose` - 详细输出模式

**依赖要求:**
- docker
- ctr (containerd)
- ssh/scp（如果需要远程分发）

#### 3. `version` - 显示版本信息

```bash
k8s-toolkit version
```

## 🏗️ 项目结构

```
k8s-toolkit/
├── cmd/                    # Cobra命令定义
│   ├── root.go            # 根命令
│   ├── enter_ns.go        # enter-ns子命令
│   ├── img_sync.go        # img-sync子命令
│   ├── version.go         # version命令
│   ├── scripts.go         # 嵌入的bash脚本
│   └── scripts/           # bash脚本源文件
│       ├── enter_pod_ns.sh
│       └── img_tool.sh
├── main.go                # 程序入口
├── go.mod                 # Go模块定义
├── Makefile              # 构建脚本
└── README.md             # 本文档
```

## 🔧 开发

### 添加新命令

1. 在`cmd/`目录下创建新文件，例如`cmd/newcmd.go`
2. 定义Cobra命令结构
3. 在`init()`函数中注册到rootCmd

```go
package cmd

import (
    "github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
    Use:   "new",
    Short: "新命令描述",
    RunE:  runNew,
}

func init() {
    rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
    // 命令实现
    return nil
}
```

### 构建命令

```bash
# 查看所有make目标
make help

# 构建
make build

# 清理
make clean

# 更新依赖
make deps
```

## 📝 设计理念

这个项目采用**混合渐进式迁移策略**:

1. **阶段1（当前）**: 使用Go CLI框架封装现有bash脚本
   - ✅ 立即获得单一二进制文件的优势
   - ✅ 更好的参数验证和帮助文档
   - ✅ 为后续扩展建立框架

2. **阶段2（未来）**: 逐步用Go原生实现替换bash脚本
   - 优先迁移简单的`img-sync`功能
   - 使用client-go替换kubectl调用
   - 使用Docker/Containerd SDK替换CLI调用

3. **阶段3（可选）**: 完全原生化
   - 根据实际需求决定是否完全重写
   - 某些功能（如nsenter）保留subprocess调用是合理的

## 🤝 贡献

欢迎贡献！请随时提交Issue或Pull Request。

## 📄 许可证

[MIT License](LICENSE)

## 🙏 致谢

- [Cobra](https://github.com/spf13/cobra) - 强大的Go CLI框架
- Kubernetes社区 - 提供丰富的工具和最佳实践
