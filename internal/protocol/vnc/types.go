package vnc

import (
	"fmt"
	"runtime"
)

// ToolType VNC工具类型
type ToolType string

const (
	ToolRemmina   ToolType = "remmina"   // Linux Remmina
	ToolTigerVNC  ToolType = "tigervnc"   // TigerVNC (vncviewer)
	ToolMacScreen ToolType = "macscreen" // macOS屏幕共享
)

// ToolCapability VNC工具能力
type ToolCapability struct {
	Name        string // 工具名称
	Description string // 工具描述
	Platform    string // 支持平台: linux, darwin, windows, all
	HasPassword bool   // 是否支持密码参数
	HasViewOnly bool   // 是否支持只读模式
}

// 已知工具能力
var toolCapabilities = map[ToolType]ToolCapability{
	ToolRemmina: {
		Name:        "Remmina",
		Description: "Linux远程桌面客户端",
		Platform:    "linux",
		HasPassword: true,
		HasViewOnly: true,
	},
	ToolTigerVNC: {
		Name:        "TigerVNC",
		Description: "跨平台VNC客户端",
		Platform:    "all",
		HasPassword: true,
		HasViewOnly: true,
	},
	ToolMacScreen: {
		Name:        "Screen Sharing",
		Description: "macOS内置屏幕共享",
		Platform:    "darwin",
		HasPassword: true,
		HasViewOnly: false,
	},
}

// GetToolCapability 获取工具能力
func GetToolCapability(toolType ToolType) *ToolCapability {
	if cap, ok := toolCapabilities[toolType]; ok {
		return &cap
	}
	return nil
}

// ToolInfo 检测到的工具信息
type ToolInfo struct {
	Type       ToolType       // 工具类型
	Path       string         // 工具路径
	Executable string         // 可执行文件名
	Capability *ToolCapability
}

// GetPreferredPlatformTool 根据平台获取首选工具
func GetPreferredPlatformTool() ToolType {
	switch runtime.GOOS {
	case "linux":
		return ToolRemmina
	case "darwin":
		return ToolMacScreen
	default:
		return ToolTigerVNC
	}
}

// String 实现fmt.Stringer接口
func (t ToolType) String() string {
	switch t {
	case ToolRemmina:
		return "Remmina"
	case ToolTigerVNC:
		return "TigerVNC"
	case ToolMacScreen:
		return "Screen Sharing (macOS)"
	default:
		return string(t)
	}
}

// IsPlatformSupported 检查工具是否支持当前平台
func (t ToolType) IsPlatformSupported() bool {
	cap := GetToolCapability(t)
	if cap == nil {
		return false
	}
	return cap.Platform == "all" || cap.Platform == runtime.GOOS
}

// DetectOrder 检测顺序（优先级）
var DetectOrder = []ToolType{
	ToolRemmina,
	ToolTigerVNC,
	ToolMacScreen,
}

// GetDetectOrderForPlatform 获取当前平台的检测顺序
func GetDetectOrderForPlatform() []ToolType {
	var order []ToolType
	for _, tool := range DetectOrder {
		if tool.IsPlatformSupported() {
			order = append(order, tool)
		}
	}
	if len(order) == 0 {
		// 至少添加TigerVNC作为后备
		order = append(order, ToolTigerVNC)
	}
	return order
}

// VNCSupportedPlatforms 当前支持的平台
var VNCSupportedPlatforms = []string{
	"linux",
	"darwin",
}

// IsPlatformSupported 检查当前平台是否支持VNC
func IsPlatformSupported() bool {
	for _, p := range VNCSupportedPlatforms {
		if p == runtime.GOOS {
			return true
		}
	}
	return false
}

// GetPlatformName 获取当前平台名称
func GetPlatformName() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

// ErrorToolNotFound 未找到工具错误
type ErrorToolNotFound struct {
	ToolType ToolType
}

func (e *ErrorToolNotFound) Error() string {
	return fmt.Sprintf("VNC工具未找到: %s", e.ToolType)
}

// ErrorPlatformUnsupported 平台不支持错误
type ErrorPlatformUnsupported struct {
	Platform string
}

func (e *ErrorPlatformUnsupported) Error() string {
	return fmt.Sprintf("当前平台 %s 不支持VNC连接", e.Platform)
}
