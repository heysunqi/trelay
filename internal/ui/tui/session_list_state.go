package tui

import (
	sshpkg "trelay/internal/protocol/ssh"

	tea "github.com/charmbracelet/bubbletea"
)

// SessionListState 后台会话列表模式状态
// 处理后台 SSH 会话的管理
type SessionListState struct{}

func newSessionListState() State {
	return &SessionListState{}
}

func (s *SessionListState) StateName() string {
	return "SessionListState"
}

func (s *SessionListState) GetStateType() StateType {
	return StateSessionList
}

// OnEnter 进入后台会话列表模式
func (s *SessionListState) OnEnter() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.showSessionList = true
	a.sessionListCursor = 0
	return nil, nil
}

// OnExit 退出后台会话列表模式
func (s *SessionListState) OnExit() (tea.Model, tea.Cmd) {
	a := stateManager.app
	a.showSessionList = false
	return nil, nil
}

// HandleKey 处理后台会话列表模式的键盘事件
func (s *SessionListState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := stateManager.app

	bgSessions := a.connManager.GetBackgroundSessions()

	switch msg.String() {
	case "esc", "q":
		a.showSessionList = false
		// 切换回普通模式
		stateManager.SetState(NewNormalState())
		return a, nil

	case "up", "k":
		if a.sessionListCursor > 0 {
			a.sessionListCursor--
		}
		return a, nil

	case "down", "j":
		if a.sessionListCursor < len(bgSessions)-1 {
			a.sessionListCursor++
		}
		return a, nil

	case "enter":
		// 切回前台
		if a.sessionListCursor < len(bgSessions) {
			session := bgSessions[a.sessionListCursor]
			if ptySession, ok := session.(*sshpkg.PTYSession); ok && ptySession.IsAlive() {
				a.showSessionList = false
				// 切换回普通模式，然后附加会话
				stateManager.SetState(NewNormalState())
				return a, a.attachSSHSession(ptySession, true) // 从后台恢复
			}
		}
		return a, nil

	case "d", "D":
		// 断开选中的后台会话
		if a.sessionListCursor < len(bgSessions) {
			session := bgSessions[a.sessionListCursor]
			a.connManager.RemoveSession(session.GetHostID())
			// 更新光标
			newSessions := a.connManager.GetBackgroundSessions()
			if len(newSessions) == 0 {
				a.showSessionList = false
				// 切换回普通模式
				stateManager.SetState(NewNormalState())
			} else if a.sessionListCursor >= len(newSessions) {
				a.sessionListCursor = len(newSessions) - 1
			}
		}
		return a, nil
	}

	return a, nil
}

// Render 渲染后台会话列表模式的视图
// 后台会话列表模式不单独渲染，委托给 App 的 View 方法
func (s *SessionListState) Render() string {
	return ""
}

// NewSessionListState 创建后台会话列表状态实例
func NewSessionListState() State {
	return newSessionListState()
}
