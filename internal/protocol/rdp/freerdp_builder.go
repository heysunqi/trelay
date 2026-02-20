package rdp

import (
	"fmt"
	"os/exec"
	"remote-desktop-manager/pkg/models"
)

// FreeRDPBuilder freerdp命令构建器
type FreeRDPBuilder struct {
	toolInfo *ToolInfo
}

// NewFreeRDPBuilder 创建FreeRDP构建器
func NewFreeRDPBuilder(toolInfo *ToolInfo) *FreeRDPBuilder {
	return &FreeRDPBuilder{toolInfo: toolInfo}
}

// BuildCommand 构建freerdp命令
func (b *FreeRDPBuilder) BuildCommand(host *models.Host) (*exec.Cmd, error) {
	args := []string{}

	// 连接目标
	if host.Port > 0 {
		args = append(args, fmt.Sprintf("/v:%s:%d", host.Host, host.Port))
	} else {
		args = append(args, fmt.Sprintf("/v:%s", host.Host))
	}

	// 认证信息
	if host.Username != "" {
		args = append(args, fmt.Sprintf("/u:%s", host.Username))
	}
	if host.Password != "" {
		args = append(args, fmt.Sprintf("/p:%s", host.Password))
	}
	if host.Domain != "" {
		args = append(args, fmt.Sprintf("/d:%s", host.Domain))
	}

	// 显示设置 - 使用动态分辨率
	// 注意：smart-sizing 会让窗口大小自动适应，因此不需要固定 size
	// 用户可以在配置中设置 ScreenSize 来指定固定分辨率
	if host.ScreenSize != "" {
		args = append(args, fmt.Sprintf("/size:%s", host.ScreenSize))
	}
	if host.ColorDepth > 0 {
		args = append(args, fmt.Sprintf("/depth:%d", host.ColorDepth))
	}

	// 常用增强选项
	// 注意：+dynamic-resolution 和 /smart-sizing 不能同时使用，这里使用 +dynamic-resolution 来实现自动调整分辨率
	args = append(args,
		"/cert:ignore",      // 忽略证书验证
		"/gfx",              // 启用图形加速
		"+dynamic-resolution", // 启用动态分辨率调整（窗口大小改变时自动调整）
		"/clipboard",          // 启用剪贴板同步
		"/sound",              // 启用音频
		"+window-drag",      // 启用窗口拖动功能
	)

	return exec.Command(b.toolInfo.Path, args...), nil
}

// GetToolInfo 获取工具信息
func (b *FreeRDPBuilder) GetToolInfo() *ToolInfo {
	return b.toolInfo
}
