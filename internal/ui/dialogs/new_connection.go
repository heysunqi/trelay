package dialogs

import (
	"fmt"
	"net"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"remote-desktop-manager/pkg/models"
)

// NewConnectionDialog 新建连接配置对话框
type NewConnectionDialog struct {
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

	// 终端尺寸（用于居中显示）
	width  int
	height int
}

// NewNewConnectionDialog 创建新建连接配置对话框
func NewNewConnectionDialog(groups []string, width, height int) *NewConnectionDialog {
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

	return &NewConnectionDialog{
		// 初始化字段列表
		fields: []string{
			"name", "ip", "port", "username", "protocol",
			"authMethod", "password", "keyPath", "passphrase", "group", "description",
		},
		focusIndex: 0,

		// 初始化协议和认证选项
		protocolOptions: []string{"ssh", "rdp", "vnc"},
		authOptions: []string{"password", "key"},
		groupOptions: formattedGroups,
		protocolFocus: false,
		authFocus: false,
		groupFocus: false,
		protocolIndex: 0,
		authIndex: 0,
		groupIndex: 0,

		// 初始化默认协议为ssh，默认认证方式为password
		protocol: "ssh",
		authMethod: "password",
		groupInput: "未分组",

		// 终端尺寸
		width: width,
		height: height,
	}
}

// Init 初始化对话框
func (d *NewConnectionDialog) Init() tea.Cmd {
	return nil
}

// Update 更新对话框状态
func (d *NewConnectionDialog) Update(msg tea.Msg) (*NewConnectionDialog, tea.Cmd) {
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
			} else if d.authFocus {
				d.authMethod = d.authOptions[d.authIndex]
				d.authFocus = false
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
				switch d.fields[d.focusIndex] {
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
					if msg.Runes != nil && len(msg.Runes) > 0 {
						// 如果在分组字段输入文字，视为创建新分组
						if d.groupInput == "未分组" {
							d.groupInput = string(msg.Runes)
						} else {
							d.groupInput += string(msg.Runes)
						}
					}
				}
			}
		}
	}
	return d, nil
}

// View 渲染对话框
func (d *NewConnectionDialog) View() string {
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

	content.WriteString(titleStyle.Render("新建连接配置"))
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
func (d *NewConnectionDialog) renderTextField(label, value string, focused bool) string {
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
func (d *NewConnectionDialog) renderPasswordField(label, value string, focused bool) string {
	masked := strings.Repeat("•", len(value))
	return d.renderTextField(label, masked, focused)
}

// 渲染协议选择字段
func (d *NewConnectionDialog) renderProtocolSelect(label, value string, focused bool) string {
	var builder strings.Builder

	builder.WriteString(label)
	builder.WriteString(": ")

	// 显示当前选中的协议
	if d.protocolFocus {
		builder.WriteString("(")
	} else if focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.protocolOptions {
		var optStr string
		if d.protocolFocus && i == d.protocolIndex {
			optStr = fmt.Sprintf("• %s", strings.ToUpper(opt))
		} else {
			optStr = opt
		}
		builder.WriteString(optStr)
		if i < len(d.protocolOptions)-1 {
			builder.WriteString(" | ")
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
func (d *NewConnectionDialog) renderAuthSelect(label, value string, focused bool) string {
	var builder strings.Builder

	builder.WriteString(label)
	builder.WriteString(": ")

	// 显示当前选中的认证方式
	if d.authFocus {
		builder.WriteString("(")
	} else if focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.authOptions {
		var optStr string
		if d.authFocus && i == d.authIndex {
			optStr = fmt.Sprintf("• %s", strings.ToUpper(opt))
		} else {
			optStr = opt
		}
		builder.WriteString(optStr)
		if i < len(d.authOptions)-1 {
			builder.WriteString(" | ")
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
func (d *NewConnectionDialog) renderGroupSelect(label, value string, focused bool) string {
	var builder strings.Builder

	builder.WriteString(label)
	builder.WriteString(": ")

	// 显示当前选中的分组
	if d.groupFocus {
		builder.WriteString("(")
	} else if focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.groupOptions {
		var optStr string
		if d.groupFocus && i == d.groupIndex {
			optStr = fmt.Sprintf("• %s", strings.ToUpper(opt))
		} else {
			optStr = opt
		}
		builder.WriteString(optStr)
		if i < len(d.groupOptions)-1 {
			builder.WriteString(" | ")
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
func (d *NewConnectionDialog) validate() error {
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

// CreateHostConfig 创建主机配置
func (d *NewConnectionDialog) CreateHostConfig() *models.Host {
	host := &models.Host{
		Name:        strings.TrimSpace(d.nameInput),
		Host:        strings.TrimSpace(d.ipInput),
		Username:    strings.TrimSpace(d.usernameInput),
		Protocol:    strings.TrimSpace(d.protocol),
		Description: strings.TrimSpace(d.descriptionInput),
	}

	// 设置端口号
	if strings.TrimSpace(d.portInput) != "" {
		var port int
		fmt.Sscanf(d.portInput, "%d", &port)
		host.Port = port
	}

	// 设置认证信息
	if d.protocol == "ssh" {
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
func (d *NewConnectionDialog) GetGroup() string {
	if d.groupInput == "未分组" || strings.TrimSpace(d.groupInput) == "" {
		return ""
	}
	return strings.TrimSpace(d.groupInput)
}

// IsCanceled 返回是否取消操作
func (d *NewConnectionDialog) IsCanceled() bool {
	return d.canceled
}

// IsClosed 返回对话框是否应该关闭
func (d *NewConnectionDialog) IsClosed() bool {
	return d.closed
}

// IsSaved 返回是否成功保存配置
func (d *NewConnectionDialog) IsSaved() bool {
	return d.saved
}

// 获取可见字段列表
func (d *NewConnectionDialog) getVisibleFields() []string {
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
func (d *NewConnectionDialog) getVisibleFieldIndex(originalIndex int, fields []struct {
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
func (d *NewConnectionDialog) navigateNextField() {
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
func (d *NewConnectionDialog) navigatePreviousField() {
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

// 处理文本输入
func handleTextInput(current string, msg tea.KeyMsg) string {
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

// 处理端口输入
func handlePortInput(current string, msg tea.KeyMsg) string {
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

// 处理密码输入
func handlePasswordInput(current string, msg tea.KeyMsg) string {
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
