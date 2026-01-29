package procinfo

import (
	"fmt"
	"strings"
)

// Linux Capabilities 名称映射 (基于 include/uapi/linux/capability.h)
var capabilityNames = map[int]string{
	0:  "CAP_CHOWN",
	1:  "CAP_DAC_OVERRIDE",
	2:  "CAP_DAC_READ_SEARCH",
	3:  "CAP_FOWNER",
	4:  "CAP_FSETID",
	5:  "CAP_KILL",
	6:  "CAP_SETGID",
	7:  "CAP_SETUID",
	8:  "CAP_SETPCAP",
	9:  "CAP_LINUX_IMMUTABLE",
	10: "CAP_NET_BIND_SERVICE",
	11: "CAP_NET_BROADCAST",
	12: "CAP_NET_ADMIN",
	13: "CAP_NET_RAW",
	14: "CAP_IPC_LOCK",
	15: "CAP_IPC_OWNER",
	16: "CAP_SYS_MODULE",
	17: "CAP_SYS_RAWIO",
	18: "CAP_SYS_CHROOT",
	19: "CAP_SYS_PTRACE",
	20: "CAP_SYS_PACCT",
	21: "CAP_SYS_ADMIN",
	22: "CAP_SYS_BOOT",
	23: "CAP_SYS_NICE",
	24: "CAP_SYS_RESOURCE",
	25: "CAP_SYS_TIME",
	26: "CAP_SYS_TTY_CONFIG",
	27: "CAP_MKNOD",
	28: "CAP_LEASE",
	29: "CAP_AUDIT_WRITE",
	30: "CAP_AUDIT_CONTROL",
	31: "CAP_SETFCAP",
	32: "CAP_MAC_OVERRIDE",
	33: "CAP_MAC_ADMIN",
	34: "CAP_SYSLOG",
	35: "CAP_WAKE_ALARM",
	36: "CAP_BLOCK_SUSPEND",
	37: "CAP_AUDIT_READ",
	38: "CAP_PERFMON",
	39: "CAP_BPF",
	40: "CAP_CHECKPOINT_RESTORE",
}

// DecodeCapabilityMask 解码 capabilities bitmask 为可读名称列表
func DecodeCapabilityMask(mask uint64) []string {
	if mask == 0 {
		return nil
	}

	var caps []string
	for bit := 0; bit < 64; bit++ {
		if mask&(1<<uint(bit)) != 0 {
			if name, ok := capabilityNames[bit]; ok {
				caps = append(caps, name)
			} else {
				caps = append(caps, fmt.Sprintf("CAP_UNKNOWN_%d", bit))
			}
		}
	}
	return caps
}

// FormatCapabilityMask 格式化 capability mask 为可读字符串
func FormatCapabilityMask(mask uint64) string {
	if mask == 0 {
		return "<none>"
	}

	caps := DecodeCapabilityMask(mask)
	return strings.Join(caps, ", ")
}

// FormatCapabilitiesInfo 格式化完整的 Capabilities 信息
func FormatCapabilitiesInfo(caps *CapabilitiesInfo) string {
	if caps == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CapInh: 0x%016x -> %s\n", caps.Inheritable, FormatCapabilityMask(caps.Inheritable)))
	sb.WriteString(fmt.Sprintf("CapPrm: 0x%016x -> %s\n", caps.Permitted, FormatCapabilityMask(caps.Permitted)))
	sb.WriteString(fmt.Sprintf("CapEff: 0x%016x -> %s\n", caps.Effective, FormatCapabilityMask(caps.Effective)))
	sb.WriteString(fmt.Sprintf("CapBnd: 0x%016x -> %s\n", caps.Bounding, FormatCapabilityMask(caps.Bounding)))
	sb.WriteString(fmt.Sprintf("CapAmb: 0x%016x -> %s", caps.Ambient, FormatCapabilityMask(caps.Ambient)))
	return sb.String()
}
