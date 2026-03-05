package vnc

import (
	"fmt"
	"os/exec"
	"strings"

	"trelay/pkg/models"
)

// MacScreenConnector macOS屏幕共享连接器
type MacScreenConnector struct {
	toolInfo *ToolInfo
}

// NewMacScreenConnector 创建macOS屏幕共享连接器
func NewMacScreenConnector(toolInfo *ToolInfo) *MacScreenConnector {
	return &MacScreenConnector{
		toolInfo: toolInfo,
	}
}

// BuildCommand 构建macOS屏幕共享命令
// 使用 open vnc:// URL 方式调用系统屏幕共享应用
func (b *MacScreenConnector) BuildCommand(host *models.Host) (*exec.Cmd, error) {
	vncURL := b.buildVNCURL(host)

	// 使用 open 命令打开VNC URL
	// 这会启动macOS的屏幕共享应用
	cmd := exec.Command("open", vncURL)

	return cmd, nil
}

// buildVNCURL 构建VNC URL
// 格式: vnc://[username[:password]@]hostname[:port]
func (b *MacScreenConnector) buildVNCURL(host *models.Host) string {
	var sb strings.Builder
	sb.WriteString("vnc://")

	// 添加用户名和密码
	if host.Username != "" {
		sb.WriteString(host.Username)
		if host.Password != "" {
			sb.WriteString(":")
			sb.WriteString(host.Password)
		}
		sb.WriteString("@")
	}

	// 添加主机地址
	sb.WriteString(host.Host)

	// 添加端口（VNC默认5900，如果使用其他端口需要转换）
	if host.Port > 0 && host.Port != 5900 {
		sb.WriteString(fmt.Sprintf(":%d", host.Port))
	}

	return sb.String()
}

// GetToolInfo 获取工具信息
func (b *MacScreenConnector) GetToolInfo() *ToolInfo {
	return b.toolInfo
}

// WaitForConnection 等待连接完成
// 注意：由于 open使用 命令，这个方法实际上不会等待连接
// 因为 open 是异步的，会立即返回
func (b *MacScreenConnector) WaitForConnection() error {
	// macOS的 open 命令是异步的，无法直接等待连接结果
	// 用户需要在打开的屏幕共享应用中完成认证
	return nil
}

// IsInteractive 是否是交互式连接
func (b *MacScreenConnector) IsInteractive() bool {
	// 使用系统屏幕共享应用，是交互式的
	return true
}

// GetConnectionHint 获取连接提示
func (b *MacScreenConnector) GetConnectionHint() string {
	return "屏幕共享应用已启动，请在应用中完成连接和认证"
}
