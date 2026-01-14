# 快速入门指南

## 5分钟上手k8s-toolkit

### 第1步：构建项目

```bash
# 直接使用go build
go build -o k8s-toolkit

# 或使用make
make build
```

构建完成后，你会得到一个单一的可执行文件 `k8s-toolkit`（Windows上是`k8s-toolkit.exe`）。

### 第2步：查看可用命令

```bash
./k8s-toolkit --help
```

你会看到：
- `enter-ns` - 进入Pod网络命名空间
- `img-sync` - Docker镜像同步工具
- `version` - 版本信息

### 第3步：使用enter-ns命令

**前提条件:**
- 需要root权限
- 需要kubectl配置（能访问K8s集群）
- 安装了jq, ctr/docker, nsenter

**示例:**

```bash
# 查看帮助
./k8s-toolkit enter-ns --help

# 进入Pod网络命名空间（需要sudo）
sudo ./k8s-toolkit enter-ns -p my-pod

# 进入kube-system命名空间的Pod
sudo ./k8s-toolkit enter-ns -n kube-system -p coredns-xxx

# 进入第二个容器
sudo ./k8s-toolkit enter-ns -n default -p my-pod -c 1
```

进入namespace后，你可以使用各种网络调试工具：
```bash
# 查看网络接口
ip addr

# 查看路由
ip route

# 抓包
tcpdump -i eth0

# 查看连接
netstat -antp
```

### 第4步：使用img-sync命令

**前提条件:**
- 安装docker和containerd
- （可选）配置SSH以访问远程节点

**示例:**

```bash
# 查看帮助
./k8s-toolkit img-sync --help

# 同步nginx镜像到本地containerd
./k8s-toolkit img-sync -i nginx:latest

# 同步并分发到3个节点
./k8s-toolkit img-sync -i redis:alpine -n node1,node2,node3

# 指定输出目录并清理
./k8s-toolkit img-sync -i mysql:8.0 -d /tmp/images -c
```

### 第5步：分发到其他机器

由于所有功能都打包在单一二进制文件中，分发非常简单：

```bash
# 直接复制到其他机器
scp k8s-toolkit user@remote-host:/usr/local/bin/

# 在远程机器上使用
ssh user@remote-host
sudo k8s-toolkit enter-ns -p my-pod
```

## 常见问题

### Q: 为什么enter-ns需要sudo？
A: 因为进入network namespace需要特权操作（nsenter需要CAP_SYS_ADMIN权限）。

### Q: 如何在Linux上构建？
A: 如果在Windows/macOS开发，交叉编译到Linux：
```bash
GOOS=linux GOARCH=amd64 go build -o k8s-toolkit-linux-amd64
```

### Q: 脚本嵌入在哪里？
A: bash脚本通过Go的`//go:embed`指令嵌入到二进制文件中（见`cmd/scripts.go`）。运行时会临时写入到`/tmp`并执行。

### Q: 如何添加新命令？
A: 
1. 在`cmd/`目录创建新的`.go`文件
2. 定义`cobra.Command`
3. 在`init()`中注册到`rootCmd`
4. 重新构建

示例见README.md的"开发"章节。

### Q: 原始bash脚本还能独立使用吗？
A: 可以！原始脚本保存在`cmd/scripts/`目录，依然可以独立运行：
```bash
bash cmd/scripts/enter_pod_ns.sh my-pod
bash cmd/scripts/img_tool.sh -i nginx:latest
```

## 下一步

- 阅读 [README.md](README.md) 了解完整功能
- 查看 [Makefile](Makefile) 了解构建选项
- 探索 `cmd/` 目录学习如何添加新命令

## 获取帮助

对于任何命令，都可以使用 `--help` 获取详细说明：

```bash
k8s-toolkit --help                  # 主帮助
k8s-toolkit enter-ns --help         # enter-ns帮助
k8s-toolkit img-sync --help         # img-sync帮助
```
