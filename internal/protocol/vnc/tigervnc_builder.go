package vnc

import (
	"fmt"
	"os/exec"
	"strings"

	"trelay/pkg/models"
)

// TigerVNCBuilder TigerVNC命令构建器
type TigerVNCBuilder struct {
	toolInfo *ToolInfo
}

// NewTigerVNCBuilder 创建TigerVNC构建器
func NewTigerVNCBuilder(toolInfo *ToolInfo) *TigerVNCBuilder {
	return &TigerVNCBuilder{
		toolInfo: toolInfo,
	}
}

// BuildCommand 构建TigerVNC命令
func (b *TigerVNCBuilder) BuildCommand(host *models.Host) (*exec.Cmd, error) {
	args := []string{}

	// 服务器地址 (Host:Port 格式)
	address := BuildVNCAddress(host)
	args = append(args, address)

	// 用户名
	if host.Username != "" {
		args = append(args, "-User="+host.Username)
	}

	// 密码（通过密码文件或环境变量，这里简化处理）
	if host.Password != "" {
		// TigerVNC支持 -passwd 参数，但需要加密的密码文件
		// 这里我们记录日志，实际使用时可能需要创建临时密码文件
		// 为简化，我们尝试使用 -SecurityTypes 参数
		args = append(args, "-SecurityTypes=VncAuth")
	}

	// 只读模式
	if host.ViewOnly {
		args = append(args, "-viewonly")
	}

	// 屏幕尺寸
	if host.ScreenSize != "" {
		args = append(args, "-geometry="+host.ScreenSize)
	}

	// 颜色深度
	if host.ColorDepth > 0 {
		args = append(args, fmt.Sprintf("-bpp=%d", host.ColorDepth))
	}

	// 共享连接（允许多个客户端同时连接）
	args = append(args, "-shared")

	// 保持连接
	args = append(args, "-keepalive")

	cmd := exec.Command(b.toolInfo.Path, args...)
	return cmd, nil
}

// buildPasswordFile 创建临时密码文件（可选实现）
// 注意：VNC密码需要特定格式的DES加密，这里简化处理
func (b *TigerVNCBuilder) buildPasswordFile(password string) (string, error) {
	// 这是一个占位实现
	// 完整的实现需要使用DES加密密码
	// TigerVNC的 vncpasswd 工具可以生成密码文件
	return "", fmt.Errorf("密码文件功能需要额外实现")
}

// GetToolInfo 获取工具信息
func (b *TigerVNCBuilder) GetToolInfo() *ToolInfo {
	return b.toolInfo
}

// BuildFullCommand 构建完整的命令（包含密码处理）
func (b *TigerVNCBuilder) BuildFullCommand(host *models.Host) (*exec.Cmd, error) {
	// 检查是否有密码
	if host.Password == "" {
		// 无密码模式
		return b.BuildCommand(host)
	}

	// 有密码的情况
	// TigerVNC需要密码文件，这里我们尝试调用vncpasswd或给出提示
	// 由于vncpasswd需要交互输入，我们这里使用一个workaround
	// 通过环境变量或创建临时文件

	// 简化处理：先构建基本命令，密码通过后续方式处理
	cmd, err := b.BuildCommand(host)
	if err != nil {
		return nil, err
	}

	// 如果有密码，尝试查找 vncpasswd 工具来生成密码文件
	// 这里暂时不实现完整的密码文件生成

	return cmd, nil
}

// buildVNCAddress 构建VNC地址（Host:Display格式）
func (b *TigerVNCBuilder) buildVNCAddress(host *models.Host) string {
	var sb strings.Builder
	sb.WriteString(host.Host)

	if host.Port > 0 && host.Port != 5900 {
		// VNC端口转换：5900 + display号
		// 例如：5901 对应 display :1
		display := host.Port - 5900
		if display >= 0 {
			sb.WriteString(fmt.Sprintf(":%d", display))
		} else {
			sb.WriteString(fmt.Sprintf(":%d", host.Port))
		}
	}

	return sb.String()
}
