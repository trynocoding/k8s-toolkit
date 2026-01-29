package procinfo

// ProcessStatus 表示进程状态信息
type ProcessStatus struct {
	PID          int
	Name         string
	State        string
	Capabilities *CapabilitiesInfo
	Signals      *SignalsInfo
}

// CapabilitiesInfo 表示进程的 Linux Capabilities 信息
type CapabilitiesInfo struct {
	Inheritable uint64 // CapInh
	Permitted   uint64 // CapPrm
	Effective   uint64 // CapEff
	Bounding    uint64 // CapBnd
	Ambient     uint64 // CapAmb
}

// SignalsInfo 表示进程的信号信息
type SignalsInfo struct {
	Queued        string // SigQ (格式: "current/max")
	Pending       uint64 // SigPnd (线程级待处理信号)
	SharedPending uint64 // ShdPnd (进程级待处理信号)
	Blocked       uint64 // SigBlk (被阻塞的信号)
	Ignored       uint64 // SigIgn (被忽略的信号)
	Caught        uint64 // SigCgt (被捕获的信号)
}

// ParseOptions 解析选项
type ParseOptions struct {
	ParseCapabilities bool
	ParseSignals      bool
	Verbose           bool
}
