package vnc

import (
	"fmt"
	"os/exec"
	"strings"

	"trelay/pkg/models"
)

// RemminaBuilder Remmina命令构建器
type RemminaBuilder struct {
	toolInfo *ToolInfo
}

// NewRemminaBuilder 创建Remmina构建器
func NewRemminaBuilder(toolInfo *ToolInfo) *RemminaBuilder {
	return &RemminaBuilder{
		toolInfo: toolInfo,
	}
}

// BuildCommand 构建Remmina命令
func (b *RemminaBuilder) BuildCommand(host *models.Host) (*exec.Cmd, error) {
	args := []string{}

	// 协议类型
	args = append(args, "--protocol=vnc")

	// 服务器地址
	address := BuildVNCAddress(host)
	args = append(args, "--server="+address)

	// 用户名（可选）
	if host.Username != "" {
		args = append(args, "--username="+host.Username)
	}

	// 密码（可选，但Remmina通常需要密码文件）
	if host.Password != "" {
		args = append(args, "--password="+host.Password)
	}

	// 只读模式
	if host.ViewOnly {
		args = append(args, "--view-only")
	}

	// 屏幕尺寸（可选）
	if host.ScreenSize != "" {
		args = append(args, "--resolution="+host.ScreenSize)
	}

	// Remmina可以使用 -c 参数指定一个临时的 .remmina 文件
	// 或者直接使用协议URL: vnc://[user]:[password]@[host]:[port]
	// 这里使用更简单的方式：通过环境变量或配置文件

	// 实际上更好的方式是创建临时 .remmina 文件
	// 但为了简化，我们使用命令行参数或直接使用 vnc URL 方式

	// 使用 vnc:// URL 方式连接
	vncURL := b.buildVNCURL(host)
	args = append(args, vncURL)

	cmd := exec.Command(b.toolInfo.Path, args...)
	return cmd, nil
}

// buildVNCURL 构建VNC URL
func (b *RemminaBuilder) buildVNCURL(host *models.Host) string {
	var sb strings.Builder
	sb.WriteString("vnc://")

	if host.Username != "" {
		sb.WriteString(host.Username)
		if host.Password != "" {
			sb.WriteString(":")
			sb.WriteString(host.Password)
		}
		sb.WriteString("@")
	}

	sb.WriteString(host.Host)

	if host.Port > 0 && host.Port != 5900 {
		sb.WriteString(":")
		sb.WriteString(fmt.Sprintf("%d", host.Port))
	}

	return sb.String()
}

// GetToolInfo 获取工具信息
func (b *RemminaBuilder) GetToolInfo() *ToolInfo {
	return b.toolInfo
}
