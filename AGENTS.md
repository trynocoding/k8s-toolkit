# k8s-toolkit — PROJECT KNOWLEDGE BASE

**Generated:** 2026-01-29 02:53:09 UTC  
**Commit:** 581890f  
**Branch:** main

## OVERVIEW
Kubernetes运维工具集，单一Go二进制封装bash脚本，Cobra CLI框架，渐进式迁移到原生Go实现。

## STRUCTURE
```
k8s-toolkit/
├── cmd/                 # Cobra command definitions
│   ├── scripts/        # UNIQUE: Embedded bash scripts (via go:embed)
│   └── scripts.go      # Bridge: embeds .sh files into Go binary
├── internal/           # Private library code
│   ├── imgsync/       # Native Go: Docker→Containerd streaming
│   ├── multiexec/     # Native Go: parallel SSH execution
│   └── filecopy/      # Native Go: parallel SCP with verification
├── main.go            # Entry: delegates to cmd.Execute()
├── Makefile           # Build + ldflags metadata injection
└── go.mod             # Go 1.25.5, Cobra + Docker/Containerd SDKs
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Add new command | `cmd/newcmd.go` | Follow Cobra pattern, register in `init()` |
| CLI entry point | `main.go` → `cmd/root.go` | Standard delegation |
| Embedded scripts | `cmd/scripts/*.sh` | Source of truth for Phase 1 logic |
| Script bridge | `cmd/scripts.go` | `go:embed` bindings |
| Native implementations | `internal/*/` | Phase 2+ Go rewrites |
| Build config | `Makefile` | Version injection via ldflags |
| Verification tests | `VERIFICATION.md` | Manual test cases (no *_test.go yet) |

## CONVENTIONS

### Hybrid Progressive Migration Strategy
- **Phase 1 (Current)**: Go CLI wraps embedded bash scripts
- **Phase 2 (Planned)**: Incremental Go native replacements (using client-go, Docker/Containerd SDKs)
- **Phase 3 (Long-term)**: Full native implementation where appropriate

### Script Embedding Pattern
```go
//go:embed scripts/enter_pod_ns.sh
var enterPodNsScript string

// At runtime:
tmpDir, _ := ioutil.TempDir("", "k8s-toolkit-*")
defer os.RemoveAll(tmpDir)
scriptPath := filepath.Join(tmpDir, "script.sh")
ioutil.WriteFile(scriptPath, []byte(enterPodNsScript), 0755)
exec.Command("bash", scriptPath, args...).Run()
```

### CLI Conventions
- **Framework**: Cobra (github.com/spf13/cobra)
- **Command naming**: kebab-case (`enter-ns`, `img-sync`, `multi-exec`)
- **Flag shortcuts**:
  - `-p`: pod name
  - `-n`: namespace
  - `-v`: verbose (global persistent flag)
  - `-c`: container index OR cleanup (context-dependent)
  - `-i`: image name OR identity file
- **Global flags**: `--verbose` defined in `cmd/root.go`

### Build Conventions
- **Version injection**: Makefile uses `-ldflags` to inject `Version`, `BuildDate`, `GitCommit` into `cmd` package
- **Binary naming**: `k8s-toolkit`, platform-specific suffixes for cross-builds (`k8s-toolkit-linux-amd64`)
- **Cross-compilation**: `make build-linux` for Linux amd64 target

## ANTI-PATTERNS (THIS PROJECT)

### 🚫 Forbidden
1. **External runtime dependencies**: NEVER add dependencies requiring manual installation (violates "Single Binary" mission)
2. **Direct script calls**: DO NOT bypass Go CLI and run `.sh` files directly
3. **Unsafe shell**: AVOID shell scripts without `set -euo pipefail`
4. **Interactive prompts**: DO NOT use `y/n` prompts (breaks automation)
5. **Shadowing core logic**: NEVER modify bash scripts in Phase 1 without updating embedded version in `cmd/scripts.go`

### ✅ Mandatory
1. **Root privileges**: ALWAYS check `os.Geteuid() != 0` before `enter-ns` operations
2. **Cobra flag validation**: ALWAYS use `MarkFlagRequired()` for mandatory inputs
3. **Exit code preservation**: ALWAYS preserve bash script exit codes:
   ```go
   if exitErr, ok := err.(*exec.ExitError); ok {
       os.Exit(exitErr.ExitCode())
   }
   ```
4. **Hybrid migration adherence**: Keep bash scripts as source of truth in Phase 1

## COMMANDS

### Development
```bash
make build          # Build current platform binary
make build-linux    # Cross-compile for Linux amd64
make deps           # Update Go module dependencies
make run            # Direct execution (go run main.go)
make test           # Run tests (currently [no test files])
make clean          # Remove build artifacts
```

### Usage Examples
```bash
# Enter pod network namespace (requires sudo)
sudo k8s-toolkit enter-ns -p my-pod -n default

# Sync Docker image to containerd + remote nodes
k8s-toolkit img-sync -i nginx:latest -n node1,node2

# Parallel command execution
k8s-toolkit multi-exec -c "uptime" -n node1,node2,node3

# File copy with verification
k8s-toolkit fcp -f ./config.yaml -d /etc/app/ -n node1,node2 --verify
```

## NOTES

### Testing
- **Current state**: No automated tests (`*_test.go` files absent)
- **Verification**: Manual test cases documented in `VERIFICATION.md`
- **Future**: Add unit tests when implementing Phase 2 native rewrites

### Architecture Trade-offs
- **Bash scripts**: Proven field-tested logic, but harder to test and distribute
- **Go wrapper**: Provides single binary, validation, unified error handling
- **Native Go**: Ultimate goal for maintainability, but requires careful migration to preserve functionality

### Dependencies
- **Runtime**: kubectl, jq, ctr/docker (for specific commands)
- **Build**: Go 1.25.5+
- **Libraries**: Cobra CLI, Docker SDK, Containerd SDK, golang.org/x/crypto/ssh

### Special Cases
- `enter-ns`: Requires Linux, root privileges, nsenter
- `img-sync`: Uses streaming (no temp files) for Docker→Containerd transfer
- `multi-exec`: Parallel SSH with configurable output modes (grouped/stream)
