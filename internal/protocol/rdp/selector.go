package rdp

import (
	"runtime"
	"sort"
)

// PlatformSelector 平台工具选择器
type PlatformSelector struct {
	os       string
	priority map[ToolType]int
}

// NewPlatformSelector 创建平台工具选择器
func NewPlatformSelector() *PlatformSelector {
	osName := runtime.GOOS
	selector := &PlatformSelector{
		os:       osName,
		priority: make(map[ToolType]int),
	}

	// 根据平台设置优先级
	switch osName {
	case "linux":
		// Linux: 优先Remmina (GUI体验更好)
		selector.priority[ToolRemmina] = 1
		selector.priority[ToolFreeRDP] = 2

	case "darwin":
		// macOS: 只用freerdp
		selector.priority[ToolFreeRDP] = 1

	default:
		// 其他平台: 使用freerdp
		selector.priority[ToolFreeRDP] = 1
	}

	return selector
}

// GetRecommendedTools 获取推荐工具列表（按优先级排序）
func (s *PlatformSelector) GetRecommendedTools() []ToolCapability {
	var result []ToolCapability

	for _, tool := range AvailableTools {
		if s.supportsPlatform(tool) {
			result = append(result, tool)
		}
	}

	// 按优先级排序
	sort.Slice(result, func(i, j int) bool {
		return s.priority[result[i].Type] < s.priority[result[j].Type]
	})

	return result
}

// supportsPlatform 检查工具是否支持当前平台
func (s *PlatformSelector) supportsPlatform(tool ToolCapability) bool {
	for _, platform := range tool.Platforms {
		if platform == s.os {
			return true
		}
	}
	return false
}
