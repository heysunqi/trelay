package dialogs

import (
	"fmt"
	"net"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"trelay/pkg/models"
)

// EditConnectionDialog 编辑连接配置对话框
type EditConnectionDialog struct {
	// 输入字段
	nameInput        string // 服务器名称
	ipInput          string // IP地址
	portInput        string // 端口号
	usernameInput    string // 用户名
	protocol         string // 连接协议: ssh, rdp, vnc
	authMethod       string // 认证方式: password, key
	passwordInput    string // 密码
	keyPathInput     string // 密钥路径
	passphraseInput  string // 密钥密码
	groupInput       string // 服务器分组
	descriptionInput string // 描述

	// 聚焦索引和导航状态
	focusIndex int
	fields     []string // 字段列表

	// 下拉框状态
	protocolOptions []string // 协议选项
	authOptions     []string // 认证选项
	groupOptions    []string // 分组选项
	protocolFocus   bool     // 协议下拉框是否聚焦
	authFocus       bool     // 认证下拉框是否聚焦
	groupFocus      bool     // 分组下拉框是否聚焦
	protocolIndex   int      // 当前选中的协议索引
	authIndex       int      // 当前选中的认证索引
	groupIndex      int      // 当前选中的分组索引

	// 操作状态
	canceled bool // 是否取消操作
	closed   bool // 对话框是否应该关闭
	saved    bool // 是否成功保存配置

	// 原始主机名称（用于查找和更新）
	originalName string

	// 终端尺寸（用于居中显示）
	width  int
	height int
}

// NewEditConnectionDialog 创建编辑连接配置对话框
func NewEditConnectionDialog(host *models.Host, groups []string, hostGroup string, width, height int) *EditConnectionDialog {
	// 格式化分组选项，确保"未分组"选项可用
	var formattedGroups []string
	if len(groups) > 0 {
		formattedGroups = append(formattedGroups, "未分组")
		for _, group := range groups {
			if group != "未分组" {
				formattedGroups = append(formattedGroups, group)
			}
		}
	} else {
		formattedGroups = []string{"未分组"}
	}

	// 确定分组索引
	groupIndex := 0
	groupInput := "未分组"
	if hostGroup != "" {
		groupInput = hostGroup
		for i, opt := range formattedGroups {
			if opt == hostGroup {
				groupIndex = i
				break
			}
		}
	}

	// 确定协议索引
	protocolIndex := 0
	protocol := host.Protocol
	if protocol == "" {
		protocol = "ssh"
	}
	for i, opt := range []string{"ssh", "rdp", "vnc"} {
		if opt == protocol {
			protocolIndex = i
			break
		}
	}

	// 确定认证方式索引
	authIndex := 0
	authMethod := host.AuthMethod
	if authMethod == "" {
		authMethod = "password"
	}
	for i, opt := range []string{"password", "key"} {
		if opt == authMethod {
			authIndex = i
			break
		}
	}

	// 确定分组输入
	if hostGroup == "" {
		groupInput = "未分组"
		groupIndex = 0
	}

	dialog := &EditConnectionDialog{
		// 初始化字段列表
		fields: []string{
			"name", "ip", "port", "username", "protocol",
			"authMethod", "password", "keyPath", "passphrase", "group", "description",
		},
		focusIndex: 0,

		// 初始化协议和认证选项
		protocolOptions: []string{"ssh", "rdp", "vnc"},
		authOptions:     []string{"password", "key"},
		groupOptions:    formattedGroups,
		protocolFocus:   false,
		authFocus:       false,
		groupFocus:      false,
		protocolIndex:   protocolIndex,
		authIndex:       authIndex,
		groupIndex:      groupIndex,

		// 预填充数据
		nameInput:        host.Name,
		ipInput:          host.Host,
		portInput:        fmt.Sprintf("%d", host.Port),
		usernameInput:    host.Username,
		protocol:         protocol,
		authMethod:       authMethod,
		passwordInput:    host.Password,
		keyPathInput:     host.KeyPath,
		passphraseInput:  host.Passphrase,
		groupInput:       groupInput,
		descriptionInput: host.Description,

		originalName: host.Name,

		// 终端尺寸
		width:  width,
		height: height,
	}

	return dialog
}

// Init 初始化对话框
func (d *EditConnectionDialog) Init() tea.Cmd {
	return nil
}

// Update 更新对话框状态
func (d *EditConnectionDialog) Update(msg tea.Msg) (*EditConnectionDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// 更新终端尺寸信息
		d.width = msg.Width
		d.height = msg.Height
		return d, nil
	case tea.KeyMsg:
		switch msg.Type {
		// 取消操作
		case tea.KeyEsc:
			d.canceled = true
			d.closed = true
			return d, nil
		case tea.KeyCtrlC:
			return d, tea.Quit

		// 确认操作
		case tea.KeyEnter:
			// 如果聚焦在下拉框上，确认选择
			if d.protocolFocus {
				d.protocol = d.protocolOptions[d.protocolIndex]
				d.protocolFocus = false
				// 根据协议更新认证方法选项
				if d.protocol != "ssh" {
					d.authMethod = ""
				} else if d.authMethod == "" {
					d.authMethod = "password"
					d.authIndex = 0
				}
				// 确保焦点索引有效
				d.ensureValidFocusIndex()
			} else if d.authFocus {
				d.authMethod = d.authOptions[d.authIndex]
				d.authFocus = false
				// 确保焦点索引有效
				d.ensureValidFocusIndex()
			} else if d.groupFocus {
				d.groupInput = d.groupOptions[d.groupIndex]
				d.groupFocus = false
			} else {
				// 验证输入并保存
				if err := d.validate(); err == nil {
					d.saved = true
					d.closed = true
				} else {
					// 这里可以显示验证错误，但为了简单起见，我们只打印到控制台
					return d, tea.Printf("验证失败: %v", err)
				}
			}
			return d, nil

		// 导航操作
		case tea.KeyTab:
			d.navigateNextField()
		case tea.KeyShiftTab:
			d.navigatePreviousField()
		case tea.KeyUp:
			if d.protocolFocus && d.protocolIndex > 0 {
				d.protocolIndex--
			} else if d.authFocus && d.authIndex > 0 {
				d.authIndex--
			} else if d.groupFocus && d.groupIndex > 0 {
				d.groupIndex--
			} else if !d.protocolFocus && !d.authFocus && !d.groupFocus {
				d.navigatePreviousField()
			}
		case tea.KeyDown:
			if d.protocolFocus && d.protocolIndex < len(d.protocolOptions)-1 {
				d.protocolIndex++
			} else if d.authFocus && d.authIndex < len(d.authOptions)-1 {
				d.authIndex++
			} else if d.groupFocus && d.groupIndex < len(d.groupOptions)-1 {
				d.groupIndex++
			} else if !d.protocolFocus && !d.authFocus && !d.groupFocus {
				d.navigateNextField()
			}

		// 处理协议、认证方式和分组字段的聚焦
		case tea.KeySpace:
			visibleFields := d.getVisibleFields()
			if d.focusIndex < len(visibleFields) {
				fieldName := visibleFields[d.focusIndex]

				if fieldName == "protocol" {
					d.protocolFocus = true
				} else if fieldName == "authMethod" && d.protocol == "ssh" {
					d.authFocus = true
				} else if fieldName == "group" {
					d.groupFocus = true
				}
			}

		// 文本输入操作
		default:
			if !d.protocolFocus && !d.authFocus && !d.groupFocus {
				visibleFields := d.getVisibleFields()
				if d.focusIndex < len(visibleFields) {
					fieldName := visibleFields[d.focusIndex]
					switch fieldName {
					case "name":
						d.nameInput = handleTextInput(d.nameInput, msg)
					case "ip":
						d.ipInput = handleTextInput(d.ipInput, msg)
					case "port":
						d.portInput = handlePortInput(d.portInput, msg)
					case "username":
						d.usernameInput = handleTextInput(d.usernameInput, msg)
					case "password":
						d.passwordInput = handlePasswordInput(d.passwordInput, msg)
					case "keyPath":
						d.keyPathInput = handleTextInput(d.keyPathInput, msg)
					case "passphrase":
						d.passphraseInput = handlePasswordInput(d.passphraseInput, msg)
					case "description":
						d.descriptionInput = handleTextInput(d.descriptionInput, msg)
					case "group":
						if msg.Type != tea.KeySpace {
							d.groupInput = handleTextInput(d.groupInput, msg)
						}
					}
				}
			}
		}
	}
	return d, nil
}

// View 渲染对话框
func (d *EditConnectionDialog) View() string {
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00ff00")).
		Padding(1, 2).
		Background(lipgloss.Color("#001100"))

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)


	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	var content strings.Builder

	content.WriteString(titleStyle.Render("编辑连接配置"))
	content.WriteString("\n\n")

	// 渲染各个字段
	fields := []struct {
		name     string
		label    string
		visible  bool
		getValue func() string
		render   func(label string, value string, focused bool) string
	}{
		{
			name:     "name",
			label:    "服务器名称",
			visible:  true,
			getValue: func() string { return d.nameInput },
			render:   d.renderTextField,
		},
		{
			name:     "ip",
			label:    "IP地址",
			visible:  true,
			getValue: func() string { return d.ipInput },
			render:   d.renderTextField,
		},
		{
			name:     "port",
			label:    "端口号",
			visible:  true,
			getValue: func() string { return d.portInput },
			render:   d.renderTextField,
		},
		{
			name:     "username",
			label:    "用户名",
			visible:  true,
			getValue: func() string { return d.usernameInput },
			render:   d.renderTextField,
		},
		{
			name:     "protocol",
			label:    "连接协议",
			visible:  true,
			getValue: func() string { return d.protocol },
			render:   d.renderProtocolSelect,
		},
		{
			name:     "authMethod",
			label:    "认证方式",
			visible:  d.protocol == "ssh",
			getValue: func() string { return d.authMethod },
			render:   d.renderAuthSelect,
		},
		{
			name:     "password",
			label:    "密码",
			visible:  d.protocol == "ssh" && d.authMethod == "password",
			getValue: func() string { return d.passwordInput },
			render:   d.renderPasswordField,
		},
		{
			name:     "keyPath",
			label:    "密钥路径",
			visible:  d.protocol == "ssh" && d.authMethod == "key",
			getValue: func() string { return d.keyPathInput },
			render:   d.renderTextField,
		},
		{
			name:     "passphrase",
			label:    "密钥密码",
			visible:  d.protocol == "ssh" && d.authMethod == "key",
			getValue: func() string { return d.passphraseInput },
			render:   d.renderPasswordField,
		},
		{
			name:     "group",
			label:    "服务器分组",
			visible:  true,
			getValue: func() string { return d.groupInput },
			render:   d.renderGroupSelect,
		},
		{
			name:     "description",
			label:    "描述",
			visible:  true,
			getValue: func() string { return d.descriptionInput },
			render:   d.renderTextField,
		},
	}

	for i, field := range fields {
		if !field.visible {
			continue
		}

		// 找到字段在可见列表中的实际索引
		visibleIndex := d.getVisibleFieldIndex(i, fields)
		if visibleIndex < 0 {
			continue
		}

		isFocused := d.focusIndex == visibleIndex
		value := field.getValue()
		rendered := field.render(field.label, value, isFocused)
		content.WriteString(rendered)
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(hintStyle.Render("使用 ↑/↓ 导航，Enter 确认，Tab 切换字段，Esc 取消"))

	// 渲染对话框内容
	dialogContent := dialogStyle.Render(content.String())

	// 如果有终端尺寸信息，使用lipgloss.Place实现完美居中
	if d.width > 0 && d.height > 0 {
		return lipgloss.Place(
			d.width, d.height,
			lipgloss.Center, lipgloss.Center,
			dialogContent,
		)
	}

	// 如果没有终端尺寸信息，直接返回原始内容
	return dialogContent
}

// 渲染文本输入字段
func (d *EditConnectionDialog) renderTextField(label, value string, focused bool) string {
	fieldStyle := lipgloss.NewStyle()
	if focused {
		fieldStyle = fieldStyle.
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00ff00"))
	} else {
		fieldStyle = fieldStyle.
			Foreground(lipgloss.Color("#00ff00"))
	}

	var builder strings.Builder

	builder.WriteString(label)
	builder.WriteString(": ")
	builder.WriteString(fieldStyle.Render(value))
	if focused {
		builder.WriteString("_") // 显示光标
	}

	return builder.String()
}

// 渲染密码输入字段
func (d *EditConnectionDialog) renderPasswordField(label, value string, focused bool) string {
	masked := strings.Repeat("•", len(value))
	return d.renderTextField(label, masked, focused)
}

// 渲染协议选择字段
func (d *EditConnectionDialog) renderProtocolSelect(label, value string, focused bool) string {
	var builder strings.Builder

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#00ff00")).
		Bold(true)

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#ffff00")).
		Bold(true)

	builder.WriteString(label)
	builder.WriteString(": ")

	if d.protocolFocus || focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.protocolOptions {
		isSelected := opt == d.protocol
		isCursor := d.protocolFocus && i == d.protocolIndex

		if isCursor {
			builder.WriteString(cursorStyle.Render(fmt.Sprintf(" %s ", strings.ToUpper(opt))))
		} else if isSelected {
			builder.WriteString(selectedStyle.Render(fmt.Sprintf(" %s ", opt)))
		} else {
			builder.WriteString(fmt.Sprintf(" %s ", opt))
		}
		if i < len(d.protocolOptions)-1 {
			builder.WriteString("|")
		}
	}

	if d.protocolFocus || focused {
		builder.WriteString(")")
	} else {
		builder.WriteString(" ")
	}

	return builder.String()
}

// 渲染认证方式选择字段
func (d *EditConnectionDialog) renderAuthSelect(label, value string, focused bool) string {
	var builder strings.Builder

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#00ff00")).
		Bold(true)

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#ffff00")).
		Bold(true)

	builder.WriteString(label)
	builder.WriteString(": ")

	if d.authFocus || focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.authOptions {
		isSelected := opt == d.authMethod
		isCursor := d.authFocus && i == d.authIndex

		if isCursor {
			builder.WriteString(cursorStyle.Render(fmt.Sprintf(" %s ", strings.ToUpper(opt))))
		} else if isSelected {
			builder.WriteString(selectedStyle.Render(fmt.Sprintf(" %s ", opt)))
		} else {
			builder.WriteString(fmt.Sprintf(" %s ", opt))
		}
		if i < len(d.authOptions)-1 {
			builder.WriteString("|")
		}
	}

	if d.authFocus || focused {
		builder.WriteString(")")
	} else {
		builder.WriteString(" ")
	}

	return builder.String()
}

// 渲染分组选择字段
func (d *EditConnectionDialog) renderGroupSelect(label, value string, focused bool) string {
	var builder strings.Builder

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#00ff00")).
		Bold(true)

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#ffff00")).
		Bold(true)

	builder.WriteString(label)
	builder.WriteString(": ")

	if d.groupFocus || focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.groupOptions {
		isSelected := opt == d.groupInput
		isCursor := d.groupFocus && i == d.groupIndex

		if isCursor {
			builder.WriteString(cursorStyle.Render(fmt.Sprintf(" %s ", strings.ToUpper(opt))))
		} else if isSelected {
			builder.WriteString(selectedStyle.Render(fmt.Sprintf(" %s ", opt)))
		} else {
			builder.WriteString(fmt.Sprintf(" %s ", opt))
		}
		if i < len(d.groupOptions)-1 {
			builder.WriteString("|")
		}
	}

	if d.groupFocus || focused {
		builder.WriteString(")")
	} else {
		builder.WriteString(" ")
	}

	return builder.String()
}

// 验证输入
func (d *EditConnectionDialog) validate() error {
	// 验证必填字段
	if strings.TrimSpace(d.nameInput) == "" {
		return models.NewValidationError("服务器名称不能为空")
	}

	if strings.TrimSpace(d.ipInput) == "" {
		return models.NewValidationError("IP地址不能为空")
	}

	if net.ParseIP(strings.TrimSpace(d.ipInput)) == nil {
		return models.NewValidationError("IP地址格式无效")
	}

	if strings.TrimSpace(d.usernameInput) == "" {
		return models.NewValidationError("用户名不能为空")
	}

	// 验证协议和认证方式
	if d.protocol == "ssh" {
		if strings.TrimSpace(d.authMethod) == "" {
			return models.NewValidationError("SSH协议需要选择认证方式")
		}

		if d.authMethod == "password" && strings.TrimSpace(d.passwordInput) == "" {
			return models.NewValidationError("密码认证需要输入密码")
		}

		if d.authMethod == "key" && strings.TrimSpace(d.keyPathInput) == "" {
			return models.NewValidationError("密钥认证需要输入密钥路径")
		}
	} else if d.protocol == "rdp" {
		if strings.TrimSpace(d.passwordInput) == "" {
			return models.NewValidationError("RDP协议需要输入密码")
		}
	} else if d.protocol == "vnc" {
		if strings.TrimSpace(d.passwordInput) == "" {
			return models.NewValidationError("VNC协议需要输入密码")
		}
	}

	// 验证端口号
	if strings.TrimSpace(d.portInput) != "" {
		port := 0
		_, err := fmt.Sscanf(d.portInput, "%d", &port)
		if err != nil {
			return models.NewValidationError("端口号必须是数字")
		}
		if port < 1 || port > 65535 {
			return models.NewValidationError("端口号必须在1-65535之间")
		}
	}

	return nil
}

// UpdateHostConfig 更新主机配置
func (d *EditConnectionDialog) UpdateHostConfig(host *models.Host) *models.Host {
	host.Name = strings.TrimSpace(d.nameInput)
	host.Host = strings.TrimSpace(d.ipInput)
	host.Username = strings.TrimSpace(d.usernameInput)
	host.Protocol = strings.TrimSpace(d.protocol)
	host.Description = strings.TrimSpace(d.descriptionInput)

	// 设置端口号
	if strings.TrimSpace(d.portInput) != "" {
		var port int
		fmt.Sscanf(d.portInput, "%d", &port)
		host.Port = port
	}

	// 设置认证信息
	if host.Protocol == "ssh" {
		host.AuthMethod = strings.TrimSpace(d.authMethod)
		if host.AuthMethod == "password" {
			host.Password = strings.TrimSpace(d.passwordInput)
		} else if host.AuthMethod == "key" {
			host.KeyPath = strings.TrimSpace(d.keyPathInput)
			host.Passphrase = strings.TrimSpace(d.passphraseInput)
		}
	} else {
		// RDP和VNC使用密码认证
		host.Password = strings.TrimSpace(d.passwordInput)
	}

	return host
}

// GetGroup 获取分组名称
func (d *EditConnectionDialog) GetGroup() string {
	if d.groupInput == "未分组" || strings.TrimSpace(d.groupInput) == "" {
		return ""
	}
	return strings.TrimSpace(d.groupInput)
}

// GetOriginalName 获取原始主机名称
func (d *EditConnectionDialog) GetOriginalName() string {
	return d.originalName
}

// GetNameInput 获取当前输入的服务器名称
func (d *EditConnectionDialog) GetNameInput() string {
	return strings.TrimSpace(d.nameInput)
}

// IsCanceled 返回是否取消操作
func (d *EditConnectionDialog) IsCanceled() bool {
	return d.canceled
}

// IsClosed 返回对话框是否应该关闭
func (d *EditConnectionDialog) IsClosed() bool {
	return d.closed
}

// IsSaved 返回是否成功保存配置
func (d *EditConnectionDialog) IsSaved() bool {
	return d.saved
}

// 获取可见字段列表
func (d *EditConnectionDialog) getVisibleFields() []string {
	var visibleFields []string
	fields := []struct {
		name    string
		visible bool
	}{
		{name: "name", visible: true},
		{name: "ip", visible: true},
		{name: "port", visible: true},
		{name: "username", visible: true},
		{name: "protocol", visible: true},
		{name: "authMethod", visible: d.protocol == "ssh"},
		{name: "password", visible: (d.protocol == "ssh" && d.authMethod == "password") || d.protocol == "rdp" || d.protocol == "vnc"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "group", visible: true},
		{name: "description", visible: true},
	}

	for _, field := range fields {
		if field.visible {
			visibleFields = append(visibleFields, field.name)
		}
	}
	return visibleFields
}

// 获取可见字段的索引
func (d *EditConnectionDialog) getVisibleFieldIndex(originalIndex int, fields []struct {
	name     string
	label    string
	visible  bool
	getValue func() string
	render   func(label string, value string, focused bool) string
}) int {
	index := 0
	for i := 0; i < originalIndex; i++ {
		if fields[i].visible {
			index++
		}
	}
	if fields[originalIndex].visible {
		return index
	}
	return -1
}

// 导航到下一个字段
func (d *EditConnectionDialog) navigateNextField() {
	// 首先计算可见字段的总数
	fields := []struct {
		name    string
		visible bool
	}{
		{name: "name", visible: true},
		{name: "ip", visible: true},
		{name: "port", visible: true},
		{name: "username", visible: true},
		{name: "protocol", visible: true},
		{name: "authMethod", visible: d.protocol == "ssh"},
		{name: "password", visible: (d.protocol == "ssh" && d.authMethod == "password") || d.protocol == "rdp" || d.protocol == "vnc"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "group", visible: true},
		{name: "description", visible: true},
	}

	totalVisible := 0
	for _, field := range fields {
		if field.visible {
			totalVisible++
		}
	}

	// 移动到下一个字段
	d.focusIndex = (d.focusIndex + 1) % totalVisible
}

// 导航到上一个字段
func (d *EditConnectionDialog) navigatePreviousField() {
	// 首先计算可见字段的总数
	fields := []struct {
		name    string
		visible bool
	}{
		{name: "name", visible: true},
		{name: "ip", visible: true},
		{name: "port", visible: true},
		{name: "username", visible: true},
		{name: "protocol", visible: true},
		{name: "authMethod", visible: d.protocol == "ssh"},
		{name: "password", visible: (d.protocol == "ssh" && d.authMethod == "password") || d.protocol == "rdp" || d.protocol == "vnc"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "group", visible: true},
		{name: "description", visible: true},
	}

	totalVisible := 0
	for _, field := range fields {
		if field.visible {
			totalVisible++
		}
	}

	// 移动到上一个字段
	d.focusIndex = (d.focusIndex - 1 + totalVisible) % totalVisible
}

// 确保焦点索引在有效范围内（当可见字段数量变化时调用）
func (d *EditConnectionDialog) ensureValidFocusIndex() {
	fields := []struct {
		name    string
		visible bool
	}{
		{name: "name", visible: true},
		{name: "ip", visible: true},
		{name: "port", visible: true},
		{name: "username", visible: true},
		{name: "protocol", visible: true},
		{name: "authMethod", visible: d.protocol == "ssh"},
		{name: "password", visible: (d.protocol == "ssh" && d.authMethod == "password") || d.protocol == "rdp" || d.protocol == "vnc"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "group", visible: true},
		{name: "description", visible: true},
	}

	totalVisible := 0
	for _, field := range fields {
		if field.visible {
			totalVisible++
		}
	}

	// 如果当前焦点索引超出范围，调整到最后一个字段
	if d.focusIndex >= totalVisible {
		d.focusIndex = totalVisible - 1
	}
}
