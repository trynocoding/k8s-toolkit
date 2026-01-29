package procinfo

import (
	"fmt"
	"strings"
)

// FormatProcessStatus 格式化进程状态为可读字符串
func FormatProcessStatus(status *ProcessStatus, opts ParseOptions) string {
	var sb strings.Builder

	// 基本信息
	sb.WriteString(fmt.Sprintf("Process: %d (%s)\n", status.PID, status.Name))
	if status.State != "" {
		sb.WriteString(fmt.Sprintf("State:   %s\n", status.State))
	}

	// Capabilities 信息
	if status.Capabilities != nil {
		sb.WriteString("\n========== Capabilities ==========\n")
		sb.WriteString(FormatCapabilitiesInfo(status.Capabilities))
	}

	// Signals 信息
	if status.Signals != nil {
		sb.WriteString("\n\n========== Signals ==========\n")
		sb.WriteString(FormatSignalsInfo(status.Signals))
	}

	return sb.String()
}

// FormatProcessStatusCompact 格式化进程状态为紧凑格式
func FormatProcessStatusCompact(status *ProcessStatus, maxPIDLen int) string {
	var sb strings.Builder

	pidFormat := fmt.Sprintf("%%0%dd", maxPIDLen)

	// Capabilities 信息
	if status.Capabilities != nil {
		sb.WriteString(fmt.Sprintf("[%s] Capabilities:\n", fmt.Sprintf(pidFormat, status.PID)))
		sb.WriteString(fmt.Sprintf("  Effective:   %s\n", FormatCapabilityMask(status.Capabilities.Effective)))
		sb.WriteString(fmt.Sprintf("  Permitted:   %s\n", FormatCapabilityMask(status.Capabilities.Permitted)))
	}

	// Signals 信息
	if status.Signals != nil {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("[%s] Signals:\n", fmt.Sprintf(pidFormat, status.PID)))
		sb.WriteString(fmt.Sprintf("  Blocked:  %s\n", FormatSignalMask(status.Signals.Blocked)))
		sb.WriteString(fmt.Sprintf("  Ignored:  %s\n", FormatSignalMask(status.Signals.Ignored)))
		sb.WriteString(fmt.Sprintf("  Caught:   %s\n", FormatSignalMask(status.Signals.Caught)))
	}

	return sb.String()
}
