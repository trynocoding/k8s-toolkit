package cmd

import (
	_ "embed"
)

// 嵌入bash脚本
// 注意: embed路径使用相对于当前包的路径

//go:embed enter_pod_ns.sh
var enterPodNsScript string

//go:embed img_tool.sh
var imgToolScript string
