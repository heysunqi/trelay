package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// GroupSelectState 分组选择模式状态
// 处理分组选择
type GroupSelectState struct {
	isSearching bool // 是否处于分组搜索子模式
}

func newGroupSelectState() State {
	return &GroupSelectState{
		isSearching: false,
	}
}

func (s *GroupSelectState) StateName() string {
	return "GroupSelectState"
}

func (s *GroupSelectState) GetStateType() StateType {
	return StateGroupSelect
}

// OnEnter 进入分组选择模式
func (s *GroupSelectState) OnEnter() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.groupSelectMode = true
	a.groupSelectCursor = 0
	a.groupList = a.groups
	a.filteredGroupList = a.groups
	a.groupSearchQuery = ""
	s.isSearching = false
	return nil, nil
}

// OnExit 退出分组选择模式
func (s *GroupSelectState) OnExit() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.groupSelectMode = false
	a.groupSearchMode = false
	a.groupSearchQuery = ""
	a.groupSearchBoxVisible = false
	a.commandInput.Blur()
	a.commandInput.Reset()
	return nil, nil
}

// HandleKey 处理分组选择模式的键盘事件
func (s *GroupSelectState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := stateManager.app

	// 如果处于搜索子模式
	if s.isSearching {
		return s.handleSearchKey(msg, a)
	}

	// 分组选择模式（非搜索）
	switch msg.Type {
	case tea.KeyEsc:
		return s.exitToNormal(a)

	case tea.KeyEnter:
		if len(a.filteredGroupList) > 0 && a.groupSelectCursor < len(a.filteredGroupList) {
			a.currentGroup = a.filteredGroupList[a.groupSelectCursor]
		}
		a.refreshHosts()
		// 切换回普通模式
		stateManager.SetState(NewNormalState())

	default:
		switch msg.String() {
		case "up", "k":
			if a.groupSelectCursor > 0 {
				a.groupSelectCursor--
			}

		case "down", "j":
			if len(a.filteredGroupList) > 0 && a.groupSelectCursor < len(a.filteredGroupList)-1 {
				a.groupSelectCursor++
			}

		case "/":
			// 进入分组搜索子模式
			s.isSearching = true
			a.groupSearchMode = true
			a.groupSearchBoxVisible = true
			if a.groupSearchQuery == "" {
				a.commandInput.Reset()
			}
			a.commandInput.Placeholder = "搜索分组..."
			a.commandInput.Focus()
			return a, textinput.Blink

		case "q":
			return s.exitToNormal(a)
		}
	}

	return a, nil
}

// handleSearchKey 处理搜索子模式的按键
func (s *GroupSelectState) handleSearchKey(msg tea.KeyMsg, a *App) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// 退出搜索编辑，清空搜索词，恢复全部分组，输入框保留（空）
		s.isSearching = false
		a.groupSearchMode = false
		a.groupSearchQuery = ""
		a.commandInput.Blur()
		a.commandInput.Reset()
		a.applyGroupSearchFilter()

	case tea.KeyEnter:
		// 退出搜索编辑，保留筛选结果，输入框保留（含搜索词）
		s.isSearching = false
		a.groupSearchMode = false
		a.commandInput.Blur()
		if len(a.filteredGroupList) > 0 {
			a.groupSelectCursor = 0
		}

	default:
		var cmd tea.Cmd
		a.commandInput, cmd = a.commandInput.Update(msg)
		a.groupSearchQuery = ""
		a.applyGroupSearchFilter()
		return a, cmd
	}

	return a, nil
}

// exitToNormal 退回到普通模式
func (s *GroupSelectState) exitToNormal(a *App) (tea.Model, tea.Cmd) {
	a.groupSelectMode = false
	a.commandMode = false
	a.groupSearchMode = false
	a.groupSearchQuery = ""
	a.groupSearchBoxVisible = false
	a.commandInput.Blur()
	a.commandInput.Reset()
	// 切换回普通模式
	stateManager.SetState(NewNormalState())
	return a, nil
}

// Render 渲染分组选择模式的视图
// 分组选择模式不单独渲染，委托给 App 的 View 方法
func (s *GroupSelectState) Render() string {
	return ""
}

// NewGroupSelectState 创建分组选择状态实例
func NewGroupSelectState() State {
	return newGroupSelectState()
}
