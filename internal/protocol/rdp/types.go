package rdp

import (
	"errors"
)

// ToolType RDP工具类型
type ToolType string

const (
	ToolRemmina ToolType = "remmina" // Remmina (Linux GUI)
	ToolFreeRDP ToolType = "freerdp" // FreeRDP (命令行，跨平台)
)

// ToolCapability 工具能力定义
type ToolCapability struct {
	Name        string   // 工具显示名称
	Type        ToolType // 工具类型
	CLI         bool     // 是否是命令行工具
	GUI         bool     // 是否有图形界面
	Platforms   []string // 支持的平台: linux, darwin
	Executables []string // 可执行文件名列表
}

// ToolInfo 工具信息
type ToolInfo struct {
	Capability ToolCapability // 工具能力
	Executable  string        // 可执行文件名
	Path        string        // 完整路径
	Available   bool          // 是否可用
}

// 所有可用工具的定义
var AvailableTools = []ToolCapability{
	{
		Name:        "Remmina",
		Type:        ToolRemmina,
		CLI:         false,
		GUI:         true,
		Platforms:   []string{"linux"},
		Executables: []string{"remmina"},
	},
	{
		Name:        "FreeRDP",
		Type:        ToolFreeRDP,
		CLI:         true,
		GUI:         false,
		Platforms:   []string{"linux", "darwin"},
		Executables: []string{"xfreerdp", "freerdp"},
	},
}

// Errors
var (
	ErrNoRDPToolAvailable = errors.New("未找到可用的RDP连接工具")
)
