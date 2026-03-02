package dialogs

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrorDialog 错误提示对话框
type ErrorDialog struct {
	message  string // 错误信息
	closed   bool   // 对话框是否应该关闭
	width    int    // 终端宽度
	height   int    // 终端高度
}

// NewErrorDialog 创建错误提示对话框
func NewErrorDialog(message string, width, height int) *ErrorDialog {
	return &ErrorDialog{
		message: message,
		width:   width,
		height:  height,
	}
}

// Init 初始化对话框
func (d *ErrorDialog) Init() tea.Cmd {
	return nil
}

// Update 更新对话框状态
func (d *ErrorDialog) Update(msg tea.Msg) (*ErrorDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// 更新终端尺寸信息
		d.width = msg.Width
		d.height = msg.Height
		return d, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			d.closed = true
			return d, nil
		case tea.KeyEsc:
			d.closed = true
			return d, nil
		case tea.KeyCtrlC:
			return d, tea.Quit
		}
	}
	return d, nil
}

// View 渲染对话框
func (d *ErrorDialog) View() string {
	// 对话框样式
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#ff4444")).
		Padding(1, 2).
		Background(lipgloss.Color("#1a0000"))

	// 标题样式
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff4444")).
		Bold(true)

	// 内容样式
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff6666"))

	// 提示样式
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	// 对错误信息进行换行处理
	wrapped := wrapText(d.message, d.width-8)

	var content strings.Builder
	content.WriteString(titleStyle.Render("连接失败"))
	content.WriteString("\n\n")
	content.WriteString(contentStyle.Render(wrapped))
	content.WriteString("\n\n")
	content.WriteString(hintStyle.Render("按 [Enter] 返回"))

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
func (d *ErrorDialog) IsClosed() bool {
	return d.closed
}

// wrapText 文本换行处理（支持中文和多行文本）
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}
	// 先按换行符分割成多行
	lines := strings.Split(text, "\n")
	var result strings.Builder

	for i, line := range lines {
		if i > 0 {
			result.WriteRune('\n')
		}
		currentWidth := 0
		for _, r := range line {
			rWidth := 1
			// 判断是否为中文字符（CJK统一汉字范围）
			if r >= 0x4e00 && r <= 0x9fff {
				rWidth = 2
			}
			// 如果当前行加上这个字符会超出最大宽度，则换行
			if currentWidth+rWidth > maxWidth {
				result.WriteRune('\n')
				currentWidth = 0
			}
			result.WriteRune(r)
			currentWidth += rWidth
		}
	}
	return result.String()
}
