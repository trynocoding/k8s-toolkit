package procinfo

import (
	"fmt"
	"strings"
	"syscall"
)

// Linux 信号名称映射 (基于 signal(7))
var signalNames = map[int]string{
	1:  "SIGHUP",
	2:  "SIGINT",
	3:  "SIGQUIT",
	4:  "SIGILL",
	5:  "SIGTRAP",
	6:  "SIGABRT",
	7:  "SIGBUS",
	8:  "SIGFPE",
	9:  "SIGKILL",
	10: "SIGUSR1",
	11: "SIGSEGV",
	12: "SIGUSR2",
	13: "SIGPIPE",
	14: "SIGALRM",
	15: "SIGTERM",
	16: "SIGSTKFLT",
	17: "SIGCHLD",
	18: "SIGCONT",
	19: "SIGSTOP",
	20: "SIGTSTP",
	21: "SIGTTIN",
	22: "SIGTTOU",
	23: "SIGURG",
	24: "SIGXCPU",
	25: "SIGXFSZ",
	26: "SIGVTALRM",
	27: "SIGPROF",
	28: "SIGWINCH",
	29: "SIGIO",
	30: "SIGPWR",
	31: "SIGSYS",
	34: "SIGRTMIN",
	35: "SIGRTMIN+1",
	36: "SIGRTMIN+2",
	37: "SIGRTMIN+3",
	38: "SIGRTMIN+4",
	39: "SIGRTMIN+5",
	40: "SIGRTMIN+6",
	41: "SIGRTMIN+7",
	42: "SIGRTMIN+8",
	43: "SIGRTMIN+9",
	44: "SIGRTMIN+10",
	45: "SIGRTMIN+11",
	46: "SIGRTMIN+12",
	47: "SIGRTMIN+13",
	48: "SIGRTMIN+14",
	49: "SIGRTMIN+15",
	50: "SIGRTMAX-14",
	51: "SIGRTMAX-13",
	52: "SIGRTMAX-12",
	53: "SIGRTMAX-11",
	54: "SIGRTMAX-10",
	55: "SIGRTMAX-9",
	56: "SIGRTMAX-8",
	57: "SIGRTMAX-7",
	58: "SIGRTMAX-6",
	59: "SIGRTMAX-5",
	60: "SIGRTMAX-4",
	61: "SIGRTMAX-3",
	62: "SIGRTMAX-2",
	63: "SIGRTMAX-1",
	64: "SIGRTMAX",
}

// DecodeSignalMask 解码信号 bitmask 为可读名称列表
func DecodeSignalMask(mask uint64) []string {
	if mask == 0 {
		return nil
	}

	var signals []string
	// Linux 信号从 1 到 64
	for signum := 1; signum <= 64; signum++ {
		if mask&(1<<uint(signum-1)) != 0 {
			if name, ok := signalNames[signum]; ok {
				signals = append(signals, name)
			} else {
				// 对于未知信号，使用 syscall 包尝试获取名称
				sig := syscall.Signal(signum)
				signals = append(signals, sig.String())
			}
		}
	}
	return signals
}

// FormatSignalMask 格式化信号 mask 为可读字符串
func FormatSignalMask(mask uint64) string {
	if mask == 0 {
		return "<none>"
	}

	signals := DecodeSignalMask(mask)
	return strings.Join(signals, ", ")
}

// FormatSignalsInfo 格式化完整的 Signals 信息
func FormatSignalsInfo(sigs *SignalsInfo) string {
	if sigs == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("SigQ:   %s\n", sigs.Queued))
	sb.WriteString(fmt.Sprintf("SigPnd: 0x%016x -> %s\n", sigs.Pending, FormatSignalMask(sigs.Pending)))
	sb.WriteString(fmt.Sprintf("ShdPnd: 0x%016x -> %s\n", sigs.SharedPending, FormatSignalMask(sigs.SharedPending)))
	sb.WriteString(fmt.Sprintf("SigBlk: 0x%016x -> %s\n", sigs.Blocked, FormatSignalMask(sigs.Blocked)))
	sb.WriteString(fmt.Sprintf("SigIgn: 0x%016x -> %s\n", sigs.Ignored, FormatSignalMask(sigs.Ignored)))
	sb.WriteString(fmt.Sprintf("SigCgt: 0x%016x -> %s", sigs.Caught, FormatSignalMask(sigs.Caught)))
	return sb.String()
}
