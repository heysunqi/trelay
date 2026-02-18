package dialogs

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"remote-desktop-manager/pkg/models"
)

// ConnectDialog 连接确认对话框
type ConnectDialog struct {
	host      *models.Host
	confirmed bool
	closed    bool // 对话框是否应该关闭
}

// NewConnectDialog 创建连接确认对话框
func NewConnectDialog(host *models.Host) *ConnectDialog {
	return &ConnectDialog{
		host: host,
	}
}

// Init 初始化对话框
func (d *ConnectDialog) Init() tea.Cmd {
	return nil
}

// Update 更新对话框状态
func (d *ConnectDialog) Update(msg tea.Msg) (*ConnectDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			d.confirmed = true
			d.closed = true
			// 不要返回tea.Quit，让app控制对话框关闭
			return d, nil
		case tea.KeyEsc:
			d.closed = true
			// 不需要返回tea.Quit，让app处理关闭
			return d, nil
		case tea.KeyCtrlC:
			// Ctrl+C应该退出整个程序
			return d, tea.Quit
		}
	}
	return d, nil
}

// View 渲染对话框
func (d *ConnectDialog) View() string {
	// 对话框样式
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00ff00")).
		Padding(1, 2).
		Background(lipgloss.Color("#001100"))

	// 标题样式
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)

	// 内容样式
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00"))

	// 提示样式
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	// 构建对话框内容
	var content strings.Builder

	// 标题
	content.WriteString(titleStyle.Render("确认连接"))
	content.WriteString("\n\n")

	// 主机信息
	content.WriteString(contentStyle.Render(fmt.Sprintf("主机名称: %s", d.host.Name)))
	content.WriteString("\n")
	content.WriteString(contentStyle.Render(fmt.Sprintf("协议类型: %s", strings.ToUpper(d.host.Protocol))))
	content.WriteString("\n")
	content.WriteString(contentStyle.Render(fmt.Sprintf("连接地址: %s:%d", d.host.Host, d.host.Port)))
	content.WriteString("\n")
	content.WriteString(contentStyle.Render(fmt.Sprintf("用户名称: %s", d.host.Username)))
	content.WriteString("\n")
	content.WriteString(contentStyle.Render(fmt.Sprintf("认证方式: %s", d.getAuthType())))

	// 描述（如果有）
	if d.host.Description != "" {
		content.WriteString("\n")
		content.WriteString(contentStyle.Render(fmt.Sprintf("描述: %s", d.host.Description)))
	}

	content.WriteString("\n\n")

	// 提示
	content.WriteString(hintStyle.Render("按 [Enter] 确认连接，按 [Esc] 取消"))

	// 渲染对话框
	return dialogStyle.Render(content.String())
}

// IsConfirmed 返回是否确认
func (d *ConnectDialog) IsConfirmed() bool {
	return d.confirmed
}

// Host 返回主机配置
func (d *ConnectDialog) Host() *models.Host {
	return d.host
}

// IsClosed 返回对话框是否应该关闭
func (d *ConnectDialog) IsClosed() bool {
	return d.closed
}

// getAuthType 获取认证方式
func (d *ConnectDialog) getAuthType() string {
	if d.host.Password != "" {
		return "密码认证"
	} else if d.host.KeyPath != "" {
		authType := "密钥认证"
		if d.host.Passphrase != "" {
			authType += " (带密码)"
		}
		return authType
	}
	return "未配置"
}
