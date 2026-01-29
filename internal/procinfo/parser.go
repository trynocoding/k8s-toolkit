package procinfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ParseProcStatus 解析 /proc/pid/status 文件
func ParseProcStatus(pid int, opts ParseOptions) (*ProcessStatus, error) {
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	file, err := os.Open(statusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("process %d does not exist", pid)
		}
		return nil, fmt.Errorf("failed to open %s: %w", statusPath, err)
	}
	defer file.Close()

	status := &ProcessStatus{
		PID: pid,
	}

	if opts.ParseCapabilities {
		status.Capabilities = &CapabilitiesInfo{}
	}
	if opts.ParseSignals {
		status.Signals = &SignalsInfo{}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}

		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "Name":
			status.Name = value
		case "State":
			status.State = value

		// Capabilities (需要启用 ParseCapabilities)
		case "CapInh":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Inheritable = mask
				}
			}
		case "CapPrm":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Permitted = mask
				}
			}
		case "CapEff":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Effective = mask
				}
			}
		case "CapBnd":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Bounding = mask
				}
			}
		case "CapAmb":
			if status.Capabilities != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Capabilities.Ambient = mask
				}
			}

		// Signals (需要启用 ParseSignals)
		case "SigQ":
			if status.Signals != nil {
				status.Signals.Queued = value
			}
		case "SigPnd":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Pending = mask
				}
			}
		case "ShdPnd":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.SharedPending = mask
				}
			}
		case "SigBlk":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Blocked = mask
				}
			}
		case "SigIgn":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Ignored = mask
				}
			}
		case "SigCgt":
			if status.Signals != nil {
				if mask, err := parseHexUint64(value); err == nil {
					status.Signals.Caught = mask
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan %s: %w", statusPath, err)
	}

	return status, nil
}

// parseHexUint64 解析十六进制字符串为 uint64
func parseHexUint64(s string) (uint64, error) {
	// 移除可能的 "0x" 前缀
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimSpace(s)

	return strconv.ParseUint(s, 16, 64)
}
