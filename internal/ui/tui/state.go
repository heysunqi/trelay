package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// 全局状态管理器实例，供各状态类使用
var stateManager *StateManager

// StateType 状态类型枚举
type StateType int

const (
	StateNormal StateType = iota
	StateSearch
	StateCommand
	StateGroupSelect
	StateSessionList
	StateDialog
)

// State 状态接口
// 所有具体状态实现此接口
type State interface {
	// HandleKey 处理键盘事件
	HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)

	// Render 渲染状态对应的视图
	Render() string

	// OnEnter 进入状态时的钩子
	OnEnter() (tea.Model, tea.Cmd)

	// OnExit 退出状态时的钩子
	OnExit() (tea.Model, tea.Cmd)

	// StateName 返回状态名称
	StateName() string

	// StateType 返回状态类型
	GetStateType() StateType
}

// StateManager 状态管理器
// 负责管理当前状态和状态切换
type StateManager struct {
	currentState State
	app          *App
}

// NewStateManager 创建新的状态管理器
func NewStateManager(app *App) *StateManager {
	stateManager = &StateManager{
		app:          app,
		currentState: nil,
	}
	return stateManager
}

// SetState 设置新状态
// 会调用旧状态的 OnExit 和新状态的 OnEnter
func (m *StateManager) SetState(state State) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// 调用旧状态的 OnExit
	if m.currentState != nil {
		_, _ = m.currentState.OnExit()
	}

	// 设置新状态
	m.currentState = state

	// 调用新状态的 OnEnter
	if m.currentState != nil {
		_, cmd = m.currentState.OnEnter()
	}

	return m.app, cmd
}

// CurrentState 获取当前状态
func (m *StateManager) CurrentState() State {
	return m.currentState
}

// HandleKey 委托当前状态处理键盘事件
func (m *StateManager) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.currentState != nil {
		return m.currentState.HandleKey(msg)
	}
	return m.app, nil
}

// Render 委托当前状态渲染视图
func (m *StateManager) Render() string {
	if m.currentState != nil {
		return m.currentState.Render()
	}
	return ""
}

// IsInDialogMode 判断是否处于对话框模式
func (m *StateManager) IsInDialogMode() bool {
	if m.currentState != nil {
		return m.currentState.GetStateType() == StateDialog
	}
	return false
}
