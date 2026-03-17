package tui

import (
	"trelay/internal/ui/dialogs"
	"trelay/pkg/models"
	"time"

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
	case "ctrl+c":
		// 第一次按 Ctrl+C：显示确认提示，设置超时重置
		// 第二次按 Ctrl+C：真正退出
		if a.quitConfirm {
			a.quitting = true
			return a, tea.Quit
		}
		a.quitConfirm = true
		// 3秒后自动重置确认状态
		return a, func() tea.Msg {
			time.Sleep(3 * time.Second)
			return quitConfirmTimeoutMsg{}
		}

	case ":":
		// 进入命令模式，切换到命令状态
		return stateManager.SetState(NewCommandState())

	case "/":
		// 进入搜索模式，切换到搜索状态
		return stateManager.SetState(NewSearchState())

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

	// 创建对话框状态
	dialogState := NewNewConnectionDialogState(groupNames, availableProxies, a.width, a.height)
	dialogState.SetOnSave(func(host *models.Host) (tea.Model, tea.Cmd) {
		// 保存主机配置
		a.config.Profiles = append(a.config.Profiles, host)

		// 检查分组是否存在
		groupName := dialogState.GetDialog().(*dialogs.NewConnectionDialog).GetGroup()
		if groupName != "" {
			found := false
			for _, g := range a.config.Groups {
				if g.Name == groupName {
					found = true
					g.Profiles = append(g.Profiles, host.Name)
					break
				}
			}
			if !found {
				newGroup := &models.Group{
					Name:     groupName,
					Profiles: []string{host.Name},
				}
				a.config.Groups = append(a.config.Groups, newGroup)
			}
		}

		// 保存配置
		if err := a.configMgr.Save(a.config); err != nil {
			a.logger.Error("保存配置失败", zap.Error(err))
		} else {
			a.logger.Info("新建主机配置已保存", zap.String("name", host.Name))
		}

		a.refreshHosts()
		return stateManager.SetState(NewNormalState())
	})
	dialogState.SetOnClose(func() (tea.Model, tea.Cmd) {
		return stateManager.SetState(NewNormalState())
	})

	return stateManager.SetState(dialogState)
}

// handleNewGroup 处理 G 键 - 新建分组
func (s *NormalState) handleNewGroup(a *App) (tea.Model, tea.Cmd) {
	dialogState := NewNewGroupDialogState(a.width, a.height)
	dialogState.SetOnSave(func(groupName string) (tea.Model, tea.Cmd) {
		// 检查分组是否已存在
		found := false
		for _, g := range a.config.Groups {
			if g.Name == groupName {
				found = true
				break
			}
		}
		if !found {
			newGroup := &models.Group{
				Name:     groupName,
				Profiles: []string{},
			}
			a.config.Groups = append(a.config.Groups, newGroup)
		}

		// 保存配置
		if err := a.configMgr.Save(a.config); err != nil {
			a.logger.Error("保存配置失败", zap.Error(err))
		} else {
			a.logger.Info("新建分组已保存", zap.String("name", groupName))
		}

		a.refreshHosts()
		return stateManager.SetState(NewNormalState())
	})
	dialogState.SetOnClose(func() (tea.Model, tea.Cmd) {
		return stateManager.SetState(NewNormalState())
	})

	return stateManager.SetState(dialogState)
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

		dialogState := NewEditConnectionDialogState(host, groupNames, hostGroup, availableProxies, a.width, a.height)
		dialogState.SetOnSave(func(originalName string, updatedHost *models.Host) (tea.Model, tea.Cmd) {
			// 更新配置中的主机
			for i, p := range a.config.Profiles {
				if p.Name == originalName {
					// 如果主机名改变，需要更新分组中的引用
					if originalName != updatedHost.Name {
						for _, g := range a.config.Groups {
							for j, pn := range g.Profiles {
								if pn == originalName {
									g.Profiles[j] = updatedHost.Name
								}
							}
						}
					}
					a.config.Profiles[i] = updatedHost
					break
				}
			}

			// 更新分组
			groupName := dialogState.GetDialog().(*dialogs.EditConnectionDialog).GetGroup()
			if groupName != "" {
				// 从旧分组中移除
				for _, g := range a.config.Groups {
					for i, pn := range g.Profiles {
						if pn == updatedHost.Name {
							g.Profiles = append(g.Profiles[:i], g.Profiles[i+1:]...)
						}
					}
				}
				// 添加到新分组
				found := false
				for _, g := range a.config.Groups {
					if g.Name == groupName {
						found = true
						g.Profiles = append(g.Profiles, updatedHost.Name)
						break
					}
				}
				if !found {
					newGroup := &models.Group{
						Name:     groupName,
						Profiles: []string{updatedHost.Name},
					}
					a.config.Groups = append(a.config.Groups, newGroup)
				}
			}

			// 保存配置
			if err := a.configMgr.Save(a.config); err != nil {
				a.logger.Error("保存配置失败", zap.Error(err))
			} else {
				a.logger.Info("主机配置已更新", zap.String("name", updatedHost.Name))
			}

			a.refreshHosts()
			return stateManager.SetState(NewNormalState())
		})
		dialogState.SetOnClose(func() (tea.Model, tea.Cmd) {
			return stateManager.SetState(NewNormalState())
		})

		return stateManager.SetState(dialogState)
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
