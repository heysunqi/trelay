package tui

import (
	"trelay/internal/ui/dialogs"

	"github.com/charmbracelet/bubbles/textinput"
	sshpkg "trelay/internal/protocol/ssh"
	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
)

// NormalState 普通模式状态
// 处理主机列表浏览和快捷键
type NormalState struct{}

func newNormalState() State {
	return &NormalState{}
}

func (s *NormalState) StateName() string {
	return "NormalState"
}

func (s *NormalState) GetStateType() StateType {
	return StateNormal
}

// OnEnter 进入普通模式（不需要特殊处理）
func (s *NormalState) OnEnter() (tea.Model, tea.Cmd) {
	return nil, nil
}

// OnExit 退出普通模式（不需要特殊处理）
func (s *NormalState) OnExit() (tea.Model, tea.Cmd) {
	return nil, nil
}

// HandleKey 处理普通模式的键盘事件
func (s *NormalState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := stateManager.app

	switch msg.String() {
	case "q", "ctrl+c":
		a.quitting = true
		return a, tea.Quit

	case ":":
		// 进入命令模式
		a.commandMode = true
		a.commandInput.Reset()
		a.commandInput.Placeholder = "输入命令..."
		a.commandInput.Focus()
		return a, textinput.Blink

	case "/":
		// 进入搜索模式
		a.searchMode = true
		a.searchBoxVisible = true
		if a.searchQuery == "" {
			a.commandInput.Reset()
		}
		a.commandInput.Placeholder = "搜索主机..."
		a.commandInput.Focus()
		return a, textinput.Blink

	case "up", "k":
		if a.selected > 0 {
			a.selected--
			a.paginator.Page = a.selected / a.pageSize
		}
		return a, nil

	case "down", "j":
		if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts)-1 {
			a.selected++
			a.paginator.Page = a.selected / a.pageSize
		}
		return a, nil

	case "left", "h":
		if a.paginator.Page > 0 {
			a.paginator.Page--
			a.selected = a.paginator.Page * a.pageSize
		}
		return a, nil

	case "right", "l":
		if a.paginator.Page < a.paginator.TotalPages-1 {
			a.paginator.Page++
			a.selected = a.paginator.Page * a.pageSize
		}
		return a, nil

	case "enter":
		return s.handleEnterKey(a)

	case "tab":
		return s.handleTabKey(a)

	case "N", "n":
		return s.handleNewConnection(a)

	case "G", "g":
		return s.handleNewGroup(a)

	case "B", "b":
		return s.handleShowSessionList(a)

	case "E", "e":
		return s.handleEditConnection(a)

	case "r":
		return s.handleRefresh(a)
	}

	return a, nil
}

// handleEnterKey 处理回车键 - 连接主机
func (s *NormalState) handleEnterKey(a *App) (tea.Model, tea.Cmd) {
	if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts) && !a.connecting {
		host := a.filteredHosts[a.selected]

		// 检查是否已有后台会话可复用
		if session, ok := a.connManager.GetSession(host.Name); ok {
			if ptySession, ok := session.(*sshpkg.PTYSession); ok {
				if ptySession.IsConnected() && !ptySession.IsAttached() && ptySession.IsAlive() {
					a.connecting = true
					return a, a.attachSSHSession(ptySession, true)
				}
			}
		}

		a.connecting = true
		a.connectingHost = host.Name

		if host.Protocol == "ssh" && host.Password == "" && host.KeyPath == "" {
			a.showPasswordDialog = true
			a.passwordDialog = dialogs.NewPasswordDialog(host, a.width, a.height)
		} else {
			return a, a.executeConnection(host)
		}
	}
	return a, nil
}

// handleTabKey 处理 Tab 键 - 切换分组
func (s *NormalState) handleTabKey(a *App) (tea.Model, tea.Cmd) {
	if len(a.groups) > 1 {
		currentIndex := -1
		for i, group := range a.groups {
			if group == a.currentGroup {
				currentIndex = i
				break
			}
		}
		if currentIndex >= 0 {
			nextIndex := (currentIndex + 1) % len(a.groups)
			a.currentGroup = a.groups[nextIndex]
			a.refreshHosts()
		}
	}
	return a, nil
}

// handleNewConnection 处理 N 键 - 新建连接
func (s *NormalState) handleNewConnection(a *App) (tea.Model, tea.Cmd) {
	a.showNewConnectionDialog = true
	var groupNames []string
	for _, group := range a.config.Groups {
		groupNames = append(groupNames, group.Name)
	}
	// 获取可用作跳板机的主机列表
	var availableProxies []string
	for _, host := range a.config.Profiles {
		if host.Protocol == "ssh" {
			availableProxies = append(availableProxies, host.Name)
		}
	}
	a.newConnectionDialog = dialogs.NewNewConnectionDialog(groupNames, availableProxies, a.width, a.height)
	return a, nil
}

// handleNewGroup 处理 G 键 - 新建分组
func (s *NormalState) handleNewGroup(a *App) (tea.Model, tea.Cmd) {
	a.showNewGroupDialog = true
	a.newGroupDialog = dialogs.NewNewGroupDialog(a.width, a.height)
	return a, nil
}

// handleShowSessionList 处理 B 键 - 显示后台会话列表
func (s *NormalState) handleShowSessionList(a *App) (tea.Model, tea.Cmd) {
	bgSessions := a.connManager.GetBackgroundSessions()
	if len(bgSessions) > 0 {
		a.sessionListCursor = 0
		// 切换到后台会话列表状态
		stateManager.SetState(NewSessionListState())
	}
	return a, nil
}

// handleEditConnection 处理 E 键 - 编辑连接
func (s *NormalState) handleEditConnection(a *App) (tea.Model, tea.Cmd) {
	if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts) {
		host := a.filteredHosts[a.selected]
		hostGroup := a.findHostGroup(host.Name)
		var groupNames []string
		for _, group := range a.config.Groups {
			groupNames = append(groupNames, group.Name)
		}
		// 获取可用作跳板机的主机列表
		var availableProxies []string
		for _, h := range a.config.Profiles {
			if h.Protocol == "ssh" && h.Name != host.Name {
				availableProxies = append(availableProxies, h.Name)
			}
		}
		a.showEditDialog = true
		a.editDialog = dialogs.NewEditConnectionDialog(host, groupNames, hostGroup, availableProxies, a.width, a.height)
	}
	return a, nil
}

// handleRefresh 处理 R 键 - 刷新配置
func (s *NormalState) handleRefresh(a *App) (tea.Model, tea.Cmd) {
	if cfg, err := a.configMgr.Load(); err == nil {
		a.config = cfg
		a.refreshHosts()
		a.logger.Info("配置已刷新")
	} else {
		a.logger.Error("刷新配置失败", zap.Error(err))
	}
	return a, a.checkHostStatusAsync()
}

// Render 渲染普通模式的视图
// 普通模式不单独渲染，委托给 App 的 View 方法
func (s *NormalState) Render() string {
	return ""
}
