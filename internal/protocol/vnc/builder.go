package vnc

import (
	"os/exec"

	"trelay/pkg/models"
)

// CmdBuilder VNC命令构建器接口
type CmdBuilder interface {
	// BuildCommand 根据主机配置构建VNC命令
	BuildCommand(host *models.Host) (*exec.Cmd, error)

	// GetToolInfo 获取工具信息
	GetToolInfo() *ToolInfo
}

// NewCmdBuilder 创建命令构建器
func NewCmdBuilder(toolInfo *ToolInfo) (CmdBuilder, error) {
	switch toolInfo.Type {
	case ToolRemmina:
		return NewRemminaBuilder(toolInfo), nil
	case ToolTigerVNC:
		return NewTigerVNCBuilder(toolInfo), nil
	case ToolMacScreen:
		return NewMacScreenConnector(toolInfo), nil
	default:
		return nil, ErrUnsupportedTool{Type: toolInfo.Type}
	}
}

// ErrUnsupportedTool 不支持的工具错误
type ErrUnsupportedTool struct {
	Type ToolType
}

func (e ErrUnsupportedTool) Error() string {
	return string(e.Type) + " 不受支持"
}

// BuildVNCAddress 构建VNC地址
func BuildVNCAddress(host *models.Host) string {
	address := host.Host
	if host.Port > 0 && host.Port != 5900 {
		address = address + ":" + intToString(host.Port)
	}
	return address
}

// intToString 整数转字符串
func intToString(i int) string {
	return string(rune('0'+i/1000)) + string(rune('0'+(i/100)%10)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10))
}
