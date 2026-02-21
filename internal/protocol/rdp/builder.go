package rdp

import (
	"fmt"
	"os/exec"
	"trelay/pkg/models"
)

// CmdBuilder 命令构建器接口
type CmdBuilder interface {
	BuildCommand(host *models.Host) (*exec.Cmd, error)
	GetToolInfo() *ToolInfo
}

// NewCmdBuilder 根据工具信息创建对应的构建器
func NewCmdBuilder(toolInfo *ToolInfo) (CmdBuilder, error) {
	switch toolInfo.Capability.Type {
	case ToolRemmina:
		return NewRemminaBuilder(toolInfo), nil
	case ToolFreeRDP:
		return NewFreeRDPBuilder(toolInfo), nil
	default:
		return nil, fmt.Errorf("不支持的RDP工具: %s", toolInfo.Capability.Name)
	}
}
