package vnc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"
)

// ToolDetector VNC工具检测器
type ToolDetector struct {
	logger *zap.Logger
}

// NewToolDetector 创建新的工具检测器
func NewToolDetector() *ToolDetector {
	logger, _ := zap.NewDevelopment()
	return &ToolDetector{
		logger: logger,
	}
}

// 检测工具路径的可执行文件
var toolExecutables = map[ToolType][]string{
	ToolRemmina: {
		"remmina",
	},
	ToolTigerVNC: {
		"vncviewer",
		"xvnc4viewer",
		"vinagre",
	},
	ToolMacScreen: {
		// macOS使用系统命令，不需要检测
	},
}

// FindExecutable 查找可执行文件路径
func FindExecutable(names []string) string {
	for _, name := range names {
		// 首先尝试直接调用
		path, err := exec.LookPath(name)
		if err == nil {
			return path
		}

		// 尝试常见路径
		commonPaths := []string{
			"/usr/bin/" + name,
			"/usr/local/bin/" + name,
			"/opt/homebrew/bin/" + name,
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// DetectTool 检测指定类型的工具是否可用
func (d *ToolDetector) DetectTool(toolType ToolType) (*ToolInfo, error) {
	d.logger.Debug("检测VNC工具", zap.String("type", string(toolType)))

	// macOS屏幕共享使用系统命令，始终可用
	if toolType == ToolMacScreen {
		return &ToolInfo{
			Type:       ToolMacScreen,
			Path:       "/usr/bin/open",
			Executable: "open",
			Capability: GetToolCapability(ToolMacScreen),
		}, nil
	}

	executables, ok := toolExecutables[toolType]
	if !ok {
		return nil, fmt.Errorf("未知的工具类型: %s", toolType)
	}

	path := FindExecutable(executables)
	if path == "" {
		return nil, &ErrorToolNotFound{ToolType: toolType}
	}

	// 获取可执行文件名
	executable := filepath.Base(path)

	return &ToolInfo{
		Type:       toolType,
		Path:       path,
		Executable: executable,
		Capability: GetToolCapability(toolType),
	}, nil
}

// GetPreferredTool 获取首选的VNC工具
func (d *ToolDetector) GetPreferredTool() (*ToolInfo, error) {
	// 检查平台是否支持
	if !IsPlatformSupported() {
		return nil, &ErrorPlatformUnsupported{Platform: runtime.GOOS}
	}

	// 根据平台优先级检测
	order := GetDetectOrderForPlatform()
	d.logger.Debug("VNC工具检测顺序", zap.Any("order", order))

	var lastErr error
	for _, toolType := range order {
		toolInfo, err := d.DetectTool(toolType)
		if err == nil {
			d.logger.Info("检测到VNC工具",
				zap.String("name", toolInfo.Capability.Name),
				zap.String("path", toolInfo.Path))
			return toolInfo, nil
		}
		lastErr = err
		d.logger.Debug("工具不可用", zap.String("type", string(toolType)), zap.Error(err))
	}

	// 所有工具都不可用，返回错误和安装帮助
	if lastErr != nil {
		return nil, fmt.Errorf("%v\n\n%s", lastErr, NewInstallHelper().GetInstallHelp())
	}

	return nil, fmt.Errorf("未找到可用的VNC工具\n\n%s", NewInstallHelper().GetInstallHelp())
}

// DetectAllTools 检测所有可用的VNC工具
func (d *ToolDetector) DetectAllTools() []*ToolInfo {
	var tools []*ToolInfo
	order := GetDetectOrderForPlatform()

	for _, toolType := range order {
		toolInfo, err := d.DetectTool(toolType)
		if err == nil {
			tools = append(tools, toolInfo)
		}
	}

	return tools
}

// GetToolByName 根据名称获取工具信息
func (d *ToolDetector) GetToolByName(name string) (*ToolInfo, error) {
	// 尝试解析为ToolType
	var toolType ToolType
	switch strings.ToLower(name) {
	case "remmina":
		toolType = ToolRemmina
	case "tigervnc", "vncviewer", "vnc":
		toolType = ToolTigerVNC
	case "mac", "macos", "screen":
		toolType = ToolMacScreen
	default:
		return nil, fmt.Errorf("未知的VNC工具: %s", name)
	}

	return d.DetectTool(toolType)
}
