package dialogs

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NewGroupDialog 新建分组对话框
type NewGroupDialog struct {
	groupName string // 分组名称
	closed    bool   // 对话框是否关闭
	confirmed bool   // 用户是否确认
	canceled  bool   // 用户是否取消
	width     int    // 终端宽度
	height    int    // 终端高度
}

// NewNewGroupDialog 创建新建分组对话框
func NewNewGroupDialog(width, height int) *NewGroupDialog {
	return &NewGroupDialog{
		width:  width,
		height: height,
	}
}

// Init 初始化对话框
func (d *NewGroupDialog) Init() tea.Cmd {
	return nil
}

// Update 更新对话框状态
func (d *NewGroupDialog) Update(msg tea.Msg) (*NewGroupDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// 更新终端尺寸信息
		d.width = msg.Width
		d.height = msg.Height
		return d, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if d.groupName != "" {
				d.confirmed = true
			}
			d.closed = true
			return d, nil
		case tea.KeyEsc:
			d.canceled = true
			d.closed = true
			return d, nil
		case tea.KeyCtrlC:
			return d, tea.Quit
		case tea.KeyBackspace:
			if len(d.groupName) > 0 {
				d.groupName = d.groupName[:len(d.groupName)-1]
			}
		case tea.KeyDelete:
			return d, nil
		case tea.KeyHome:
			return d, nil
		case tea.KeyEnd:
			return d, nil
		default:
			if msg.Runes != nil && len(msg.Runes) > 0 {
				char := string(msg.Runes)
				switch char {
				case "\n", "\t", "\r":
					// 忽略换行和制表符
				default:
					d.groupName += char
				}
			}
		}
	}
	return d, nil
}

// View 渲染对话框
func (d *NewGroupDialog) View() string {
	// 对话框样式（与新建连接对话框风格一致）
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

	var content strings.Builder

	// 标题
	content.WriteString(titleStyle.Render("新建分组"))
	content.WriteString("\n\n")

	// 分组名称输入
	content.WriteString(contentStyle.Render("分组名称: " + d.groupName))
	if len(d.groupName) == 0 {
		content.WriteString("_") // 显示光标
	}

	content.WriteString("\n\n")

	// 提示
	content.WriteString(hintStyle.Render("按 [Enter] 确认，按 [Esc] 取消"))

	// 渲染对话框内容
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

// IsClosed 返回对话框是否应该关闭
func (d *NewGroupDialog) IsClosed() bool {
	return d.closed
}

// IsConfirmed 返回用户是否确认
func (d *NewGroupDialog) IsConfirmed() bool {
	return d.confirmed
}

// IsCanceled 返回用户是否取消
func (d *NewGroupDialog) IsCanceled() bool {
	return d.canceled
}

// GetGroupName 返回分组名称
func (d *NewGroupDialog) GetGroupName() string {
	return strings.TrimSpace(d.groupName)
}
