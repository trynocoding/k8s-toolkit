# 🎯 下一步操作指南

恭喜！您的 k8s-toolkit 项目已成功构建。以下是接下来的操作步骤：

## 🚀 立即使用

### 1. 在当前Windows环境测试（有限）

```powershell
# 查看帮助
.\k8s-toolkit.exe --help

# 查看各命令帮助
.\k8s-toolkit.exe enter-ns --help
.\k8s-toolkit.exe img-sync --help
.\k8s-toolkit.exe version
```

**注意**: enter-ns和img-sync需要Linux环境和相应依赖才能实际执行。

### 2. 在Linux环境使用（完整功能）

#### 方法A: 交叉编译
```bash
# 在Windows上构建Linux版本
make build-linux

# 或手动编译
GOOS=linux GOARCH=amd64 go build -o k8s-toolkit-linux-amd64

# 传输到Linux服务器
scp k8s-toolkit-linux-amd64 user@linux-host:/usr/local/bin/k8s-toolkit
```

#### 方法B: 在Linux上直接构建
```bash
# 将项目复制到Linux机器
scp -r k8s-toolkit user@linux-host:~/

# SSH到Linux机器
ssh user@linux-host

# 构建
cd k8s-toolkit
go build -o k8s-toolkit

# 移动到PATH
sudo mv k8s-toolkit /usr/local/bin/
```

### 3. 实际使用示例

```bash
# 进入Pod网络命名空间（需要sudo）
sudo k8s-toolkit enter-ns my-pod

# 同步Docker镜像
k8s-toolkit img-sync -i nginx:latest

# 分发到多个节点
k8s-toolkit img-sync -i redis:alpine -n node1,node2,node3
```

## 📦 版本管理（可选）

### 初始化Git仓库

```bash
# 初始化Git
git init

# 添加所有文件
git add .

# 首次提交
git commit -m "feat: 初始化k8s-toolkit项目

- 实现enter-ns命令（Pod网络命名空间工具）
- 实现img-sync命令（Docker镜像同步工具）
- 基于Cobra框架的Go CLI
- 嵌入bash脚本，单一二进制分发
- 完整文档和使用指南"

# 添加远程仓库（替换为你的仓库地址）
git remote add origin https://github.com/yourname/k8s-toolkit.git

# 推送
git push -u origin main
```

### 标签版本

```bash
# 创建v0.1.0标签
git tag -a v0.1.0 -m "Release v0.1.0 - 初始版本"

# 推送标签
git push origin v0.1.0
```

## 🔧 配置CI/CD（可选）

### GitHub Actions示例

创建 `.github/workflows/build.yml`:

```yaml
name: Build

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Build
      run: |
        make build
        make build-linux
    
    - name: Upload artifacts
      uses: actions/upload-artifact@v3
      with:
        name: binaries
        path: k8s-toolkit*
```

## 📝 添加更多命令

### 示例：添加新的"logs"命令

1. 创建 `cmd/logs.go`:

```go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
    Use:   "logs POD_NAME",
    Short: "查看Pod日志",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        podName := args[0]
        fmt.Printf("获取 %s 的日志...\n", podName)
        // 实现逻辑
        return nil
    },
}

func init() {
    rootCmd.AddCommand(logsCmd)
    logsCmd.Flags().StringP("namespace", "n", "default", "命名空间")
    logsCmd.Flags().BoolP("follow", "f", false, "持续跟踪日志")
}
```

2. 重新构建:

```bash
make build
```

3. 测试:

```bash
./k8s-toolkit logs my-pod --help
```

## 🎓 学习资源

### Go相关
- [Go Tour](https://tour.golang.org) - Go语言入门
- [Effective Go](https://golang.org/doc/effective_go.html) - Go最佳实践

### Cobra框架
- [Cobra官方文档](https://cobra.dev)
- [Cobra GitHub](https://github.com/spf13/cobra)

### Kubernetes客户端
- [client-go](https://github.com/kubernetes/client-go) - Kubernetes官方Go客户端
- [client-go示例](https://github.com/kubernetes/client-go/tree/master/examples)

## 🐛 故障排查

### 问题1: "command not found"

```bash
# 确保文件有执行权限
chmod +x k8s-toolkit

# 或使用绝对路径
./k8s-toolkit --help
```

### 问题2: 构建失败

```bash
# 清理并重新构建
make clean
go mod tidy
make build
```

### 问题3: 在Linux上运行Windows构建的二进制

```bash
# 错误示例：运行 k8s-toolkit.exe 在Linux上

# 正确做法：构建Linux版本
GOOS=linux GOARCH=amd64 go build -o k8s-toolkit
```

### 问题4: embed路径错误

确保bash脚本在正确位置：
```bash
ls cmd/enter_pod_ns.sh
ls cmd/img_tool.sh
```

## 📊 性能优化（未来）

### 1. 减小二进制大小

```bash
# 使用strip移除调试信息
go build -ldflags="-s -w" -o k8s-toolkit

# 使用UPX压缩（可选）
upx --best --lzma k8s-toolkit
```

### 2. 静态链接（完全独立）

```bash
# CGO_ENABLED=0 构建纯静态二进制
CGO_ENABLED=0 go build -ldflags="-s -w" -o k8s-toolkit
```

## 🤝 分享和反馈

### 内部分享

1. 分发二进制文件给团队成员
2. 分享README.md和QUICKSTART.md
3. 收集使用反馈

### 公开分享（可选）

1. 创建GitHub仓库
2. 编写详细的README
3. 添加LICENSE文件
4. 发布Release

## ✅ 检查清单

在部署到生产环境前：

- [ ] 在实际Linux环境测试所有命令
- [ ] 验证enter-ns在真实K8s集群上工作
- [ ] 验证img-sync能成功同步镜像
- [ ] 测试所有参数组合
- [ ] 检查错误处理是否友好
- [ ] 确认帮助文档准确
- [ ] 添加版本信息到构建

## 🎉 庆祝

您已成功完成：

✅ Go项目从零到一  
✅ bash脚本现代化改造  
✅ CLI工具开发  
✅ 单一二进制分发  
✅ 完整文档编写  

**下一个里程碑**: 在生产环境使用，并根据反馈持续改进！

---

**有问题？** 查看：
- [README.md](README.md) - 完整功能说明
- [QUICKSTART.md](QUICKSTART.md) - 快速入门
- [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md) - 项目总结
- [VERIFICATION.md](VERIFICATION.md) - 验证报告
