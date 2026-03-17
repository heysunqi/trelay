package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SearchState 搜索模式状态
// 处理主机搜索
type SearchState struct{}

func newSearchState() State {
	return &SearchState{}
}

func (s *SearchState) StateName() string {
	return "SearchState"
}

func (s *SearchState) GetStateType() StateType {
	return StateSearch
}

// OnEnter 进入搜索模式
func (s *SearchState) OnEnter() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.searchMode = true
	a.searchBoxVisible = true
	if a.searchQuery == "" {
		a.commandInput.Reset()
	}
	a.commandInput.Placeholder = "搜索主机..."
	a.commandInput.Focus()
	return nil, textinput.Blink
}

// OnExit 退出搜索模式
func (s *SearchState) OnExit() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.searchMode = false
	a.commandInput.Blur()
	a.commandInput.Reset()
	return nil, nil
}

// HandleKey 处理搜索模式的键盘事件
func (s *SearchState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := stateManager.app

	switch msg.Type {
	case tea.KeyEsc:
		// 退出搜索编辑，清空搜索词，恢复全部列表，输入框保留（空）
		a.searchMode = false
		a.searchQuery = ""
		a.commandInput.Blur()
		a.commandInput.Reset()
		a.applySearchFilter()
		a.updatePaginator()
		// 切换回普通模式
		stateManager.SetState(NewNormalState())

	case tea.KeyEnter:
		// 退出搜索编辑，保留搜索结果，输入框保留（含搜索词）
		a.searchMode = false
		a.commandInput.Blur()
		if len(a.filteredHosts) > 0 {
			a.selected = 0
			a.paginator.Page = 0
		}
		// 切换回普通模式
		stateManager.SetState(NewNormalState())

	default:
		var cmd tea.Cmd
		a.commandInput, cmd = a.commandInput.Update(msg)
		a.searchQuery = a.commandInput.Value()
		a.applySearchFilter()
		a.updatePaginator()
		return a, cmd
	}

	return a, nil
}

// Render 渲染搜索模式的视图
// 搜索模式不单独渲染，委托给 App 的 View 方法
func (s *SearchState) Render() string {
	return ""
}

// NewNormalState 创建普通状态实例
func NewNormalState() State {
	return newNormalState()
}

// NewSearchState 创建搜索状态实例
func NewSearchState() State {
	return newSearchState()
}
