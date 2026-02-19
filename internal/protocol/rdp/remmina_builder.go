package rdp

import (
	"fmt"
	"os/exec"
	"strings"
	"remote-desktop-manager/pkg/models"
)

// RemminaBuilder remmina命令构建器
type RemminaBuilder struct {
	toolInfo *ToolInfo
}

// NewRemminaBuilder 创建Remmina构建器
func NewRemminaBuilder(toolInfo *ToolInfo) *RemminaBuilder {
	return &RemminaBuilder{toolInfo: toolInfo}
}

// BuildCommand 构建remmina命令
// 格式: remmina --connect=rdp://[user[:password]@]host[:port][/domain]
func (b *RemminaBuilder) BuildCommand(host *models.Host) (*exec.Cmd, error) {
	var uri strings.Builder

	// 构建URI
	uri.WriteString("rdp://")

	// 用户名和密码
	if host.Username != "" {
		uri.WriteString(host.Username)
		if host.Password != "" {
			uri.WriteString(":")
			uri.WriteString(host.Password)
		}
		uri.WriteString("@")
	}

	// 主机和端口
	uri.WriteString(host.Host)
	if host.Port > 0 {
		uri.WriteString(fmt.Sprintf(":%d", host.Port))
	}

	// 域名
	if host.Domain != "" {
		uri.WriteString("/")
		uri.WriteString(host.Domain)
	}

	args := []string{"--connect=" + uri.String()}

	return exec.Command(b.toolInfo.Path, args...), nil
}

// GetToolInfo 获取工具信息
func (b *RemminaBuilder) GetToolInfo() *ToolInfo {
	return b.toolInfo
}
