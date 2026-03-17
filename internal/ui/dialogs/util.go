package dialogs

import (
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// 公共样式常量
var (
	// DialogBorderStyle 对话框边框样式
	DialogBorderStyle = lipgloss.RoundedBorder()

	// DialogBorderColor 对话框边框颜色
	DialogBorderColor = lipgloss.Color("#00ff00")

	// DialogBackground 对话框背景色
	DialogBackground = lipgloss.Color("#001100")

	// PrimaryTextColor 主文字颜色
	PrimaryTextColor = lipgloss.Color("#00ff00")

	// SecondaryTextColor 次要文字颜色
	SecondaryTextColor = lipgloss.Color("#888888")

	// HighlightBackground 高亮背景色（选中项）
	HighlightBackground = lipgloss.Color("#00ff00")

	// CursorBackground 光标背景色
	CursorBackground = lipgloss.Color("#ffff00")

	// ErrorColor 错误颜色
	ErrorColor = lipgloss.Color("#ff0000")
)

// DialogStyle 返回标准对话框样式
func DialogStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(DialogBorderStyle).
		BorderForeground(DialogBorderColor).
		Padding(1, 2).
		Background(DialogBackground)
}

// TitleStyle 返回标题样式
func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(PrimaryTextColor).
		Bold(true)
}

// HintStyle 返回提示文字样式
func HintStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(SecondaryTextColor).
		Italic(true)
}

// ErrorStyle 返回错误文字样式
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ErrorColor).
		Bold(true)
}

// SelectedStyle 返回选中项样式
func SelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(HighlightBackground).
		Bold(true)
}

// CursorStyle 返回光标样式
func CursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(CursorBackground).
		Bold(true)
}

// HandleTextInput 处理文本输入
func HandleTextInput(current string, msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(current) > 0 {
			return current[:len(current)-1]
		}
	case tea.KeyDelete:
		return current
	case tea.KeyHome:
		return current
	case tea.KeyEnd:
		return current
	default:
		if len(msg.Runes) > 0 {
			char := string(msg.Runes)
			switch char {
			case "\n", "\t", "\r":
				return current
			default:
				return current + char
			}
		}
	}
	return current
}

// handlePortInput 处理端口输入（仅数字）
func HandlePortInput(current string, msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(current) > 0 {
			return current[:len(current)-1]
		}
	case tea.KeyDelete:
		return current
	case tea.KeyHome:
		return current
	case tea.KeyEnd:
		return current
	default:
		if len(msg.Runes) > 0 {
			char := string(msg.Runes)
			if char >= "0" && char <= "9" {
				return current + char
			}
		}
	}
	return current
}

// handlePasswordInput 处理密码输入
func HandlePasswordInput(current string, msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(current) > 0 {
			return current[:len(current)-1]
		}
	case tea.KeyDelete:
		return current
	case tea.KeyHome:
		return current
	case tea.KeyEnd:
		return current
	default:
		if len(msg.Runes) > 0 {
			char := string(msg.Runes)
			switch char {
			case "\n", "\t", "\r":
				return current
			default:
				return current + char
			}
		}
	}
	return current
}
