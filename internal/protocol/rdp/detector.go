package rdp

import (
	"fmt"
	"os/exec"
	"strings"
)

// ToolDetector 工具检测器
type ToolDetector struct {
	selector *PlatformSelector
}

// NewToolDetector 创建工具检测器
func NewToolDetector() *ToolDetector {
	return &ToolDetector{
		selector: NewPlatformSelector(),
	}
}

// Detect 检测所有可用工具
func (d *ToolDetector) Detect() []*ToolInfo {
	var found []*ToolInfo

	for _, tool := range d.selector.GetRecommendedTools() {
		for _, execName := range tool.Executables {
			if path, err := exec.LookPath(execName); err == nil {
				info := &ToolInfo{
					Capability: tool,
					Executable:  execName,
					Path:        path,
					Available:    true,
				}
				found = append(found, info)
				break // 找到一个就停止
			}
		}
	}

	return found
}

// GetPreferredTool 获取首选工具
func (d *ToolDetector) GetPreferredTool() (*ToolInfo, error) {
	available := d.Detect()

	if len(available) == 0 {
		return nil, ErrNoRDPToolAvailable
	}

	return available[0], nil
}

// GetToolByName 根据类型获取工具
func (d *ToolDetector) GetToolByName(toolType ToolType) (*ToolInfo, error) {
	available := d.Detect()

	for _, info := range available {
		if info.Capability.Type == toolType {
			return info, nil
		}
	}

	return nil, fmt.Errorf("工具 %s 不可用", toolType)
}

// GetAvailableToolNames 获取可用工具名称列表（用于提示）
func (d *ToolDetector) GetAvailableToolNames() string {
	available := d.Detect()
	if len(available) == 0 {
		return "无"
	}

	names := make([]string, len(available))
	for i, info := range available {
		names[i] = info.Capability.Name
	}

	return strings.Join(names, ", ")
}
