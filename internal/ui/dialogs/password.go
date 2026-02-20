package dialogs

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"remote-desktop-manager/pkg/models"
)

// PasswordDialog 密码输入对话框
type PasswordDialog struct {
	host     *models.Host       // 要连接的主机
	password string             // 用户输入的密码
	closed   bool               // 对话框是否应该关闭
	submitted bool              // 用户是否提交了密码
	width    int                // 终端宽度
	height   int                // 终端高度
}

// NewPasswordDialog 创建密码输入对话框
func NewPasswordDialog(host *models.Host, width, height int) *PasswordDialog {
	return &PasswordDialog{
		host:   host,
		width:  width,
		height: height,
	}
}

// Init 初始化对话框
func (d *PasswordDialog) Init() tea.Cmd {
	return nil
}

// Update 更新对话框状态
func (d *PasswordDialog) Update(msg tea.Msg) (*PasswordDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// 更新终端尺寸信息
		d.width = msg.Width
		d.height = msg.Height
		return d, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			d.submitted = true
			d.closed = true
			return d, nil
		case tea.KeyEsc:
			d.closed = true
			return d, nil
		case tea.KeyCtrlC:
			return d, tea.Quit
		case tea.KeyBackspace:
			if len(d.password) > 0 {
				d.password = d.password[:len(d.password)-1]
			}
		default:
			if msg.Runes != nil && len(msg.Runes) > 0 {
				char := string(msg.Runes)
				switch char {
				case "\n", "\t", "\r":
					// 忽略换行和制表符
				default:
					d.password += char
				}
			}
		}
	}
	return d, nil
}

// View 渲染对话框
func (d *PasswordDialog) View() string {
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
	content.WriteString(titleStyle.Render("SSH密码输入"))
	content.WriteString("\n\n")

	// 主机信息
	content.WriteString(contentStyle.Render("主机名称: " + d.host.Name))
	content.WriteString("\n")
	port := d.host.GetPort() // 使用GetPort()获取默认端口
	content.WriteString(contentStyle.Render("连接地址: " + d.host.Host + ":" + fmt.Sprintf("%d", port)))
	content.WriteString("\n")
	content.WriteString(contentStyle.Render("用户名称: " + d.host.Username))
	content.WriteString("\n\n")

	// 密码输入
	maskedPassword := strings.Repeat("•", len(d.password))
	content.WriteString(contentStyle.Render("密码: " + maskedPassword))
	if len(d.password) == 0 {
		content.WriteString("_") // 显示光标
	}

	content.WriteString("\n\n")

	// 提示
	content.WriteString(hintStyle.Render("按 [Enter] 确认，按 [Esc] 取消"))

	// 渲染对话框
	dialogContent := dialogStyle.Render(content.String())

	// 如果有终端尺寸信息，使用lipgloss.Place实现完美居中
	if d.width > 0 && d.height > 0 {
		return lipgloss.Place(
			d.width, d.height,
			lipgloss.Center, lipgloss.Center,
			dialogContent,
		)
	}

	// 如果没有终端尺寸信息，直接返回原始内容
	return dialogContent
}

// GetPassword 返回用户输入的密码
func (d *PasswordDialog) GetPassword() string {
	return d.password
}

// IsSubmitted 返回用户是否提交了密码
func (d *PasswordDialog) IsSubmitted() bool {
	return d.submitted
}

// IsClosed 返回对话框是否应该关闭
func (d *PasswordDialog) IsClosed() bool {
	return d.closed
}

// Host 返回主机配置
func (d *PasswordDialog) Host() *models.Host {
	return d.host
}
