# proc-status 命令使用指南

## 功能说明

`proc-status` 命令用于查看进程的 Linux **Capabilities** 和 **Signals** 信息，自动将十六进制 bitmask 解码为可读的权限和信号名称。

### 支持的使用场景

1. **本地进程查看**：直接查看本机进程的 capabilities 和 signals
2. **Pod 容器内进程查看**：通过 `kubectl exec` 查看 Kubernetes Pod 容器内进程的状态

## 基本用法

### 1. 查看本地进程

```bash
# 查看进程 PID 1 的所有信息（Capabilities + Signals）
k8s-toolkit proc-status --pid 1

# 只查看 Capabilities
k8s-toolkit proc-status --pid 1 --capabilities

# 只查看 Signals
k8s-toolkit proc-status --pid 1 --signals

# 详细模式
k8s-toolkit proc-status --pid 1234 -v
```

### 2. 查看 Pod 容器内的进程

```bash
# 查看 default 命名空间中 my-pod 的进程 1
k8s-toolkit proc-status -p my-pod --pid 1

# 指定命名空间
k8s-toolkit proc-status -p my-pod -n kube-system --pid 1

# 查看第二个容器的进程
k8s-toolkit proc-status -p my-pod -n default -c 1 --pid 1

# 只查看 Capabilities
k8s-toolkit proc-status -p my-pod -n default --pid 1 --capabilities

# 只查看 Signals
k8s-toolkit proc-status -p my-pod -n default --pid 1 --signals
```

## 参数说明

### 必需参数

- `--pid <PID>`: 要查看的进程 PID（必需）

### Pod 相关参数（用于查看容器内进程）

- `-p, --pod <名称>`: Pod 名称
- `-n, --namespace <命名空间>`: Kubernetes 命名空间（默认: `default`）
- `-c, --container <索引>`: 容器索引（默认: 0，即第一个容器）

### 过滤选项

- `--capabilities`: 只显示 Capabilities 信息
- `--signals`: 只显示 Signals 信息
- 如果两个都不指定，则显示所有信息

### 全局选项

- `-v, --verbose`: 详细输出模式（显示调试信息）

## 输出示例

### Capabilities 输出

```
Process: 1 (systemd)
State:   S (sleeping)

========== Capabilities ==========
CapInh: 0x0000000000000000 -> <none>
CapPrm: 0x000001ffffffffff -> CAP_CHOWN, CAP_DAC_OVERRIDE, CAP_DAC_READ_SEARCH, CAP_FOWNER, CAP_FSETID, CAP_KILL, CAP_SETGID, CAP_SETUID, CAP_SETPCAP, CAP_LINUX_IMMUTABLE, CAP_NET_BIND_SERVICE, CAP_NET_BROADCAST, CAP_NET_ADMIN, CAP_NET_RAW, CAP_IPC_LOCK, CAP_IPC_OWNER, CAP_SYS_MODULE, CAP_SYS_RAWIO, CAP_SYS_CHROOT, CAP_SYS_PTRACE, CAP_SYS_PACCT, CAP_SYS_ADMIN, CAP_SYS_BOOT, CAP_SYS_NICE, CAP_SYS_RESOURCE, CAP_SYS_TIME, CAP_SYS_TTY_CONFIG, CAP_MKNOD, CAP_LEASE, CAP_AUDIT_WRITE, CAP_AUDIT_CONTROL, CAP_SETFCAP, CAP_MAC_OVERRIDE, CAP_MAC_ADMIN, CAP_SYSLOG, CAP_WAKE_ALARM, CAP_BLOCK_SUSPEND, CAP_AUDIT_READ, CAP_PERFMON, CAP_BPF, CAP_CHECKPOINT_RESTORE
CapEff: 0x000001ffffffffff -> CAP_CHOWN, CAP_DAC_OVERRIDE, ...
CapBnd: 0x000001ffffffffff -> CAP_CHOWN, CAP_DAC_OVERRIDE, ...
CapAmb: 0x0000000000000000 -> <none>
```

### Signals 输出

```
Process: 1234 (nginx)
State:   S (sleeping)

========== Signals ==========
SigQ:   0/58791
SigPnd: 0x0000000000000000 -> <none>
ShdPnd: 0x0000000000000000 -> <none>
SigBlk: 0x0000000000000000 -> <none>
SigIgn: 0x0000000000001000 -> SIGPIPE
SigCgt: 0x00000001a0016623 -> SIGHUP, SIGINT, SIGQUIT, SIGTERM, SIGCHLD, SIGUSR1, SIGUSR2, SIGWINCH
```

## Capabilities 说明

**Capabilities 字段含义**：

- **CapInh** (Inheritable): 可以被子进程继承的权限
- **CapPrm** (Permitted): 进程允许使用的权限集合
- **CapEff** (Effective): 进程当前实际使用的权限
- **CapBnd** (Bounding Set): 进程可以获得的权限上限
- **CapAmb** (Ambient): 非特权程序也能保留的权限

**常见 Capabilities**：

- `CAP_NET_BIND_SERVICE`: 绑定小于 1024 的端口
- `CAP_SYS_ADMIN`: 系统管理权限
- `CAP_NET_ADMIN`: 网络管理权限
- `CAP_SYS_PTRACE`: 跟踪任意进程
- `CAP_DAC_OVERRIDE`: 绕过文件权限检查

## Signals 说明

**Signals 字段含义**：

- **SigQ**: 当前排队的信号数 / 最大排队数
- **SigPnd**: 线程级待处理信号（已发送但未处理）
- **ShdPnd**: 进程级待处理信号（共享的）
- **SigBlk**: 被阻塞的信号（不会被传递）
- **SigIgn**: 被忽略的信号
- **SigCgt**: 被捕获的信号（已注册信号处理函数）

**常见信号**：

- `SIGHUP (1)`: 终端断开
- `SIGINT (2)`: 中断（Ctrl+C）
- `SIGQUIT (3)`: 退出
- `SIGKILL (9)`: 强制终止（不能被捕获）
- `SIGTERM (15)`: 终止请求
- `SIGCHLD (17)`: 子进程状态改变

## 典型使用场景

### 场景1：调试 Kubernetes Pod 权限问题

```bash
# 检查 Pod 内的进程是否有 CAP_NET_BIND_SERVICE 权限
k8s-toolkit proc-status -p nginx-pod -n production --pid 1 --capabilities

# 检查是否因为权限不足导致操作失败
k8s-toolkit proc-status -p app-pod --pid 42 --capabilities | grep CAP_SYS_ADMIN
```

### 场景2：排查信号处理问题

```bash
# 查看进程忽略了哪些信号
k8s-toolkit proc-status -p app-pod --pid 1 --signals

# 确认 SIGTERM 是否被正确捕获
k8s-toolkit proc-status -p graceful-shutdown-pod --pid 1 --signals | grep SIGTERM
```

### 场景3：安全审计

```bash
# 检查容器是否运行在过高权限下
k8s-toolkit proc-status -p suspicious-pod --pid 1 --capabilities

# 查找所有具有 CAP_SYS_ADMIN 的进程（需要在主机上运行）
for pid in $(pgrep -x nginx); do
    k8s-toolkit proc-status --pid $pid --capabilities | grep -q CAP_SYS_ADMIN && echo "PID $pid has CAP_SYS_ADMIN"
done
```

## 与 Python 脚本的对比

本命令集成了类似社区 Python 脚本的功能：

| 功能 | Python 脚本 | k8s-toolkit proc-status |
|------|-------------|------------------------|
| Capabilities 解码 | ❌ 不支持 | ✅ 完整支持 |
| Signals 解码 | ✅ 支持 | ✅ 完整支持 |
| Pod 集成 | ❌ 需要手动 kubectl exec | ✅ 自动集成 |
| 多 PID 支持 | ✅ 支持 | ⚠️ 需要循环调用 |
| 单一二进制 | ❌ 需要 Python 环境 | ✅ 无需额外依赖 |

## 依赖要求

### 本地进程查看

- Linux 系统（需要 `/proc` 文件系统）

### Pod 容器查看

- `kubectl` 命令行工具
- 对目标 Pod/Namespace 的访问权限

## 故障排查

### 错误：invalid PID: 0

**原因**：未指定 `--pid` 参数或 PID 为 0

**解决**：

```bash
k8s-toolkit proc-status --pid 1
```

### 错误：process 1234 does not exist

**原因**：指定的进程不存在

**解决**：

```bash
# 检查进程是否存在
ps aux | grep 1234

# 或使用 pgrep 查找进程
pgrep nginx
```

### 错误：kubectl exec failed

**原因**：
- Pod 不存在或名称错误
- 无权限访问指定的 namespace
- 容器索引超出范围

**解决**：

```bash
# 检查 Pod 是否存在
kubectl get pods -n <namespace>

# 查看 Pod 中有多少个容器
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.containers[*].name}'

# 确认权限
kubectl auth can-i exec pods -n <namespace>
```

## 参考资料

- [Linux Capabilities Manual](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- [Linux Signals Manual](https://man7.org/linux/man-pages/man7/signal.7.html)
- [/proc/pid/status Documentation](https://man7.org/linux/man-pages/man5/proc.5.html)
