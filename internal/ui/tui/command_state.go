package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// CommandState 命令模式状态
// 处理命令输入
type CommandState struct{}

func newCommandState() State {
	return &CommandState{}
}

func (s *CommandState) StateName() string {
	return "CommandState"
}

func (s *CommandState) GetStateType() StateType {
	return StateCommand
}

// OnEnter 进入命令模式
func (s *CommandState) OnEnter() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.commandMode = true
	a.commandInput.Reset()
	a.commandInput.Placeholder = "输入命令..."
	a.commandInput.Focus()
	return nil, textinput.Blink
}

// OnExit 退出命令模式
func (s *CommandState) OnExit() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.commandMode = false
	a.commandInput.Blur()
	a.commandInput.Reset()
	return nil, nil
}

// HandleKey 处理命令模式的键盘事件
func (s *CommandState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := stateManager.app

	switch msg.String() {
	case "ctrl+c":
		// 命令模式下忽略 Ctrl+C，不退出
		return a, nil

	case "esc":
		a.commandMode = false
		a.commandInput.Blur()
		a.commandInput.Reset()
		// 切换回普通模式
		stateManager.SetState(NewNormalState())

	case "enter":
		cmd := a.commandInput.Value()
		if cmd == "q" {
			// 输入 q 后按回车退出程序
			a.quitting = true
			return a, tea.Quit
		} else if cmd == "group" {
			// 进入分组选择模式
			a.groupSelectMode = true
			a.groupSelectCursor = 0
			a.groupList = a.groups
			a.filteredGroupList = a.groups
			a.groupSearchQuery = ""
			stateManager.SetState(NewGroupSelectState())
		} else {
			// 不识别的命令：清除命令模式
			a.commandMode = false
			a.commandInput.Blur()
			a.commandInput.Reset()
			// 切换回普通模式
			stateManager.SetState(NewNormalState())
		}

	default:
		var cmd tea.Cmd
		a.commandInput, cmd = a.commandInput.Update(msg)
		return a, cmd
	}

	return a, nil
}

// Render 渲染命令模式的视图
// 命令模式不单独渲染，委托给 App 的 View 方法
func (s *CommandState) Render() string {
	return ""
}

// NewCommandState 创建命令状态实例
func NewCommandState() State {
	return newCommandState()
}
