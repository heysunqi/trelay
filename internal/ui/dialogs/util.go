package dialogs

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleTextInput 处理文本输入
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
