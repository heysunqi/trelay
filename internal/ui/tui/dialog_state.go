package tui

import (
	"trelay/internal/ui/dialogs"
	"trelay/pkg/models"

	tea "github.com/charmbracelet/bubbletea"
)

// DialogType 对话框类型
type DialogType int

const (
	DialogPassword DialogType = iota
	DialogError
	DialogNewConnection
	DialogEditConnection
	DialogNewGroup
)

// DialogState 是对话框状态的通用接口
type DialogState interface {
	State
	GetDialog() interface{}
}

// BaseDialogState 提供对话框状态的基础实现
type BaseDialogState struct {
	dialogType DialogType
}

func (s *BaseDialogState) GetStateType() StateType {
	return StateDialog
}

func (s *BaseDialogState) StateName() string {
	return "DialogState"
}

func (s *BaseDialogState) Render() string {
	return ""
}

// PasswordDialogState 密码输入对话框状态
type PasswordDialogState struct {
	BaseDialogState
	dialog   *dialogs.PasswordDialog
	host     *models.Host
	width    int
	height   int
	onSubmit func(host *models.Host) (tea.Model, tea.Cmd)
	onClose  func() (tea.Model, tea.Cmd)
}

func NewPasswordDialogState(host *models.Host, width, height int) *PasswordDialogState {
	return &PasswordDialogState{
		dialog:   dialogs.NewPasswordDialog(host, width, height),
		host:     host,
		width:    width,
		height:   height,
	}
}

func (s *PasswordDialogState) OnEnter() (tea.Model, tea.Cmd) {
	return nil, nil
}

func (s *PasswordDialogState) OnExit() (tea.Model, tea.Cmd) {
	s.dialog = nil
	return nil, nil
}

func (s *PasswordDialogState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.dialog == nil {
		return stateManager.app, nil
	}

	updated, cmd := s.dialog.Update(msg)
	s.dialog = updated

	// 检查对话框是否关闭
	if s.dialog.IsClosed() {
		if s.dialog.IsSubmitted() && s.onSubmit != nil {
			// 获取密码并执行连接
			s.host.Password = s.dialog.GetPassword()
			return s.onSubmit(s.host)
		}
		// 关闭对话框
		if s.onClose != nil {
			return s.onClose()
		}
		// 默认切换回普通模式
		return stateManager.SetState(NewNormalState())
	}

	return stateManager.app, cmd
}

func (s *PasswordDialogState) Render() string {
	if s.dialog == nil {
		return ""
	}
	return s.dialog.View()
}

func (s *PasswordDialogState) GetDialog() interface{} {
	return s.dialog
}

func (s *PasswordDialogState) SetOnSubmit(f func(host *models.Host) (tea.Model, tea.Cmd)) {
	s.onSubmit = f
}

func (s *PasswordDialogState) SetOnClose(f func() (tea.Model, tea.Cmd)) {
	s.onClose = f
}

// ErrorDialogState 错误提示对话框状态
type ErrorDialogState struct {
	BaseDialogState
	dialog  *dialogs.ErrorDialog
	message string
	width   int
	height  int
	onClose func() (tea.Model, tea.Cmd)
}

func NewErrorDialogState(message string, width, height int) *ErrorDialogState {
	return &ErrorDialogState{
		dialog:  dialogs.NewErrorDialog(message, width, height),
		message: message,
		width:   width,
		height:  height,
	}
}

func (s *ErrorDialogState) OnEnter() (tea.Model, tea.Cmd) {
	return nil, nil
}

func (s *ErrorDialogState) OnExit() (tea.Model, tea.Cmd) {
	s.dialog = nil
	return nil, nil
}

func (s *ErrorDialogState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.dialog == nil {
		return stateManager.app, nil
	}

	updated, cmd := s.dialog.Update(msg)
	s.dialog = updated

	// 检查对话框是否关闭
	if s.dialog.IsClosed() {
		if s.onClose != nil {
			return s.onClose()
		}
		return stateManager.SetState(NewNormalState())
	}

	return stateManager.app, cmd
}

func (s *ErrorDialogState) Render() string {
	if s.dialog == nil {
		return ""
	}
	return s.dialog.View()
}

func (s *ErrorDialogState) GetDialog() interface{} {
	return s.dialog
}

func (s *ErrorDialogState) SetOnClose(f func() (tea.Model, tea.Cmd)) {
	s.onClose = f
}

// NewConnectionDialogState 新建连接对话框状态
type NewConnectionDialogState struct {
	BaseDialogState
	dialog           *dialogs.NewConnectionDialog
	groupNames       []string
	availableProxies []string
	width            int
	height           int
	onSave           func(host *models.Host) (tea.Model, tea.Cmd)
	onClose          func() (tea.Model, tea.Cmd)
}

func NewNewConnectionDialogState(groupNames, availableProxies []string, width, height int) *NewConnectionDialogState {
	return &NewConnectionDialogState{
		dialog:           dialogs.NewNewConnectionDialog(groupNames, availableProxies, width, height),
		groupNames:       groupNames,
		availableProxies: availableProxies,
		width:            width,
		height:           height,
	}
}

func (s *NewConnectionDialogState) OnEnter() (tea.Model, tea.Cmd) {
	return nil, nil
}

func (s *NewConnectionDialogState) OnExit() (tea.Model, tea.Cmd) {
	s.dialog = nil
	return nil, nil
}

func (s *NewConnectionDialogState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.dialog == nil {
		return stateManager.app, nil
	}

	updated, cmd := s.dialog.Update(msg)
	s.dialog = updated

	// 检查对话框是否关闭
	if s.dialog.IsClosed() {
		if s.dialog.IsSaved() {
			host := s.dialog.CreateHostConfig()
			if s.onSave != nil {
				return s.onSave(host)
			}
		}
		if s.onClose != nil {
			return s.onClose()
		}
		return stateManager.SetState(NewNormalState())
	}

	return stateManager.app, cmd
}

func (s *NewConnectionDialogState) Render() string {
	if s.dialog == nil {
		return ""
	}
	return s.dialog.View()
}

func (s *NewConnectionDialogState) GetDialog() interface{} {
	return s.dialog
}

func (s *NewConnectionDialogState) SetOnSave(f func(host *models.Host) (tea.Model, tea.Cmd)) {
	s.onSave = f
}

func (s *NewConnectionDialogState) SetOnClose(f func() (tea.Model, tea.Cmd)) {
	s.onClose = f
}

// EditConnectionDialogState 编辑连接对话框状态
type EditConnectionDialogState struct {
	BaseDialogState
	dialog           *dialogs.EditConnectionDialog
	originalHost    *models.Host // 保存原始主机引用
	groupNames       []string
	availableProxies []string
	width            int
	height           int
	onSave           func(originalName string, host *models.Host) (tea.Model, tea.Cmd)
	onClose          func() (tea.Model, tea.Cmd)
}

func NewEditConnectionDialogState(host *models.Host, groupNames []string, hostGroup string, availableProxies []string, width, height int) *EditConnectionDialogState {
	return &EditConnectionDialogState{
		dialog:           dialogs.NewEditConnectionDialog(host, groupNames, hostGroup, availableProxies, width, height),
		originalHost:     host,
		groupNames:       groupNames,
		availableProxies: availableProxies,
		width:            width,
		height:           height,
	}
}

func (s *EditConnectionDialogState) OnEnter() (tea.Model, tea.Cmd) {
	return nil, nil
}

func (s *EditConnectionDialogState) OnExit() (tea.Model, tea.Cmd) {
	s.dialog = nil
	return nil, nil
}

func (s *EditConnectionDialogState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.dialog == nil {
		return stateManager.app, nil
	}

	updated, cmd := s.dialog.Update(msg)
	s.dialog = updated

	// 检查对话框是否关闭
	if s.dialog.IsClosed() {
		if s.dialog.IsSaved() {
			originalName := s.dialog.GetOriginalName()
			updatedHost := s.dialog.UpdateHostConfig(s.originalHost)
			if s.onSave != nil {
				return s.onSave(originalName, updatedHost)
			}
		}
		if s.onClose != nil {
			return s.onClose()
		}
		return stateManager.SetState(NewNormalState())
	}

	return stateManager.app, cmd
}

func (s *EditConnectionDialogState) Render() string {
	if s.dialog == nil {
		return ""
	}
	return s.dialog.View()
}

func (s *EditConnectionDialogState) GetDialog() interface{} {
	return s.dialog
}

func (s *EditConnectionDialogState) SetOnSave(f func(originalName string, host *models.Host) (tea.Model, tea.Cmd)) {
	s.onSave = f
}

func (s *EditConnectionDialogState) SetOnClose(f func() (tea.Model, tea.Cmd)) {
	s.onClose = f
}

// NewGroupDialogState 新建分组对话框状态
type NewGroupDialogState struct {
	BaseDialogState
	dialog  *dialogs.NewGroupDialog
	width   int
	height  int
	onSave  func(groupName string) (tea.Model, tea.Cmd)
	onClose func() (tea.Model, tea.Cmd)
}

func NewNewGroupDialogState(width, height int) *NewGroupDialogState {
	return &NewGroupDialogState{
		dialog: dialogs.NewNewGroupDialog(width, height),
		width:  width,
		height: height,
	}
}

func (s *NewGroupDialogState) OnEnter() (tea.Model, tea.Cmd) {
	return nil, nil
}

func (s *NewGroupDialogState) OnExit() (tea.Model, tea.Cmd) {
	s.dialog = nil
	return nil, nil
}

func (s *NewGroupDialogState) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.dialog == nil {
		return stateManager.app, nil
	}

	updated, cmd := s.dialog.Update(msg)
	s.dialog = updated

	// 检查对话框是否关闭
	if s.dialog.IsClosed() {
		if s.dialog.IsConfirmed() {
			groupName := s.dialog.GetGroupName()
			if s.onSave != nil {
				return s.onSave(groupName)
			}
		}
		if s.onClose != nil {
			return s.onClose()
		}
		return stateManager.SetState(NewNormalState())
	}

	return stateManager.app, cmd
}

func (s *NewGroupDialogState) Render() string {
	if s.dialog == nil {
		return ""
	}
	return s.dialog.View()
}

func (s *NewGroupDialogState) GetDialog() interface{} {
	return s.dialog
}

func (s *NewGroupDialogState) SetOnSave(f func(groupName string) (tea.Model, tea.Cmd)) {
	s.onSave = f
}

func (s *NewGroupDialogState) SetOnClose(f func() (tea.Model, tea.Cmd)) {
	s.onClose = f
}
