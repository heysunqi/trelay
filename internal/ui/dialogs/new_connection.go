package dialogs

import (
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"trelay/pkg/models"
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

	// 密钥管理字段
	useExistingKey      bool            // true=使用已导入密钥路径, false=粘贴密钥内容
	useExistingOptions  []string        // ["是", "否"]
	useExistingFocus    bool            // 下拉框聚焦状态
	useExistingIndex    int             // 当前选中索引
	keyContentTextarea  textarea.Model  // 多行密钥内容输入框
	validationError     string          // 验证错误信息
	keyContentFocused   bool            // textarea 是否聚焦

	// 代理连接字段
	connectVia         string   // 连接方式: direct, proxyjump, proxyserver
	connectViaOptions  []string // ["直连", "ProxyJump", "ProxyServer"]
	connectViaFocus    bool     // 连接方式下拉框聚焦状态
	connectViaIndex    int      // 当前选中索引
	proxyJumpInput     string   // 跳板机名称
	proxyJumpListFocus bool     // 跳板机列表选择模式
	proxyJumpListIndex int      // 跳板机列表当前索引
	proxyHostInput     string   // 代理服务器地址
	proxyPortInput     string   // 代理服务器端口
	proxyUserInput     string   // 代理服务器用户名
	proxyPasswordInput string   // 代理服务器密码
	proxyKeyPathInput  string   // 代理服务器密钥路径
	availableProxies   []string // 可用的跳板机列表

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
func NewNewConnectionDialog(groups []string, availableProxies []string, width, height int) *NewConnectionDialog {
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

	// 初始化 textarea
	ta := textarea.New()
	ta.SetWidth(50)
	ta.SetHeight(6)
	ta.Placeholder = "粘贴 PEM 格式的私钥内容..."
	ta.CharLimit = 0 // 不限制字符数

	return &NewConnectionDialog{
		// 初始化字段列表
		fields: []string{
			"name", "ip", "port", "username", "protocol",
			"authMethod", "password", "useExistingKey", "keyPath", "keyContent", "passphrase",
			"connectVia", "proxyJump", "proxyHost", "proxyPort", "proxyUser", "proxyPassword", "proxyKeyPath",
			"group", "description",
		},
		focusIndex: 0,

		// 初始化协议和认证选项
		protocolOptions: []string{"ssh", "rdp", "vnc"},
		authOptions:     []string{"password", "key"},
		groupOptions:    formattedGroups,
		protocolFocus:   false,
		authFocus:       false,
		groupFocus:      false,
		protocolIndex:   0,
		authIndex:       0,
		groupIndex:      0,

		// 初始化默认协议为ssh，默认认证方式为password
		protocol:   "ssh",
		authMethod: "password",
		groupInput: "未分组",

		// 初始化密钥管理字段
		useExistingKey:      true, // 默认选择"是"
		useExistingOptions:  []string{"是", "否"},
		useExistingFocus:    false,
		useExistingIndex:    0,
		keyContentTextarea:  ta,
		validationError:     "",
		keyContentFocused:   false,

		// 初始化代理连接字段
		connectVia:         "direct",
		connectViaOptions:  []string{"直连", "ProxyJump", "ProxyServer"},
		connectViaFocus:    false,
		connectViaIndex:    0,
		availableProxies:   availableProxies,

		// 终端尺寸
		width:  width,
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
			// 如果跳板机列表选择模式，退出列表模式
			if d.proxyJumpListFocus {
				d.proxyJumpListFocus = false
				return d, nil
			}
			// 如果 textarea 聚焦，退出编辑模式
			if d.keyContentFocused {
				d.keyContentFocused = false
				d.keyContentTextarea.Blur()
				return d, nil
			}
			d.canceled = true
			d.closed = true
			return d, nil
		case tea.KeyCtrlC:
			return d, tea.Quit

		// 确认操作
		case tea.KeyEnter:
			// 如果跳板机列表选择模式，确认选择
			if d.proxyJumpListFocus {
				if len(d.availableProxies) > 0 {
					d.proxyJumpInput = d.availableProxies[d.proxyJumpListIndex]
				}
				d.proxyJumpListFocus = false
				return d, nil
			}
			// 如果 textarea 聚焦，插入换行
			if d.keyContentFocused {
				var cmd tea.Cmd
				d.keyContentTextarea, cmd = d.keyContentTextarea.Update(msg)
				return d, cmd
			}
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
			} else if d.useExistingFocus {
				d.useExistingKey = d.useExistingIndex == 0 // 0 = "是", 1 = "否"
				d.useExistingFocus = false
				d.ensureValidFocusIndex()
			} else if d.connectViaFocus {
				// 根据选择设置 connectVia 值
				switch d.connectViaIndex {
				case 0:
					d.connectVia = "direct"
				case 1:
					d.connectVia = "proxyjump"
				case 2:
					d.connectVia = "proxyserver"
				}
				d.connectViaFocus = false
				d.ensureValidFocusIndex()
			} else {
				// 验证输入并保存
				if err := d.validate(); err == nil {
					d.validationError = "" // 清除错误
					d.saved = true
					d.closed = true
				} else {
					// 设置验证错误信息
					d.validationError = err.Error()
				}
			}
			return d, nil

		// 导航操作
		case tea.KeyTab:
			// 如果 textarea 聚焦，退出编辑模式
			if d.keyContentFocused {
				d.keyContentFocused = false
				d.keyContentTextarea.Blur()
				d.navigateNextField()
				return d, nil
			}
			d.navigateNextField()
		case tea.KeyShiftTab:
			// 如果 textarea 聚焦，退出编辑模式
			if d.keyContentFocused {
				d.keyContentFocused = false
				d.keyContentTextarea.Blur()
				d.navigatePreviousField()
				return d, nil
			}
			d.navigatePreviousField()
		case tea.KeyUp:
			// 如果跳板机列表选择模式，导航列表
			if d.proxyJumpListFocus {
				if d.proxyJumpListIndex > 0 {
					d.proxyJumpListIndex--
				}
				return d, nil
			}
			// 如果 textarea 聚焦，传递给 textarea
			if d.keyContentFocused {
				var cmd tea.Cmd
				d.keyContentTextarea, cmd = d.keyContentTextarea.Update(msg)
				return d, cmd
			}
			if d.protocolFocus && d.protocolIndex > 0 {
				d.protocolIndex--
			} else if d.authFocus && d.authIndex > 0 {
				d.authIndex--
			} else if d.groupFocus && d.groupIndex > 0 {
				d.groupIndex--
			} else if d.useExistingFocus && d.useExistingIndex > 0 {
				d.useExistingIndex--
			} else if d.connectViaFocus && d.connectViaIndex > 0 {
				d.connectViaIndex--
			} else if !d.protocolFocus && !d.authFocus && !d.groupFocus && !d.useExistingFocus && !d.connectViaFocus {
				d.navigatePreviousField()
			}
		case tea.KeyDown:
			// 如果跳板机列表选择模式，导航列表
			if d.proxyJumpListFocus {
				if d.proxyJumpListIndex < len(d.availableProxies)-1 {
					d.proxyJumpListIndex++
				}
				return d, nil
			}
			// 如果 textarea 聚焦，传递给 textarea
			if d.keyContentFocused {
				var cmd tea.Cmd
				d.keyContentTextarea, cmd = d.keyContentTextarea.Update(msg)
				return d, cmd
			}
			if d.protocolFocus && d.protocolIndex < len(d.protocolOptions)-1 {
				d.protocolIndex++
			} else if d.authFocus && d.authIndex < len(d.authOptions)-1 {
				d.authIndex++
			} else if d.groupFocus && d.groupIndex < len(d.groupOptions)-1 {
				d.groupIndex++
			} else if d.useExistingFocus && d.useExistingIndex < len(d.useExistingOptions)-1 {
				d.useExistingIndex++
			} else if d.connectViaFocus && d.connectViaIndex < len(d.connectViaOptions)-1 {
				d.connectViaIndex++
			} else if !d.protocolFocus && !d.authFocus && !d.groupFocus && !d.useExistingFocus && !d.connectViaFocus {
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
				} else if fieldName == "useExistingKey" && d.protocol == "ssh" && d.authMethod == "key" {
					d.useExistingFocus = true
				} else if fieldName == "connectVia" && d.protocol == "ssh" {
					d.connectViaFocus = true
				} else if fieldName == "proxyJump" && d.connectVia == "proxyjump" && len(d.availableProxies) > 0 {
					// 进入跳板机列表选择模式
					d.proxyJumpListFocus = true
					// 如果当前有选中的跳板机，定位到该位置
					d.proxyJumpListIndex = 0
					for i, proxy := range d.availableProxies {
						if proxy == d.proxyJumpInput {
							d.proxyJumpListIndex = i
							break
						}
					}
				} else if fieldName == "keyContent" && !d.useExistingKey {
					// 进入 textarea 编辑模式
					d.keyContentFocused = true
					d.keyContentTextarea.Focus()
				} else {
					// 非下拉字段，将空格作为普通字符输入
					switch fieldName {
					case "name":
						d.nameInput += " "
					case "ip":
						d.ipInput += " "
					case "username":
						d.usernameInput += " "
					case "description":
						d.descriptionInput += " "
					case "keyPath":
						d.keyPathInput += " "
					case "proxyHost":
						d.proxyHostInput += " "
					case "proxyPort":
						d.proxyPortInput += " "
					case "proxyUser":
						d.proxyUserInput += " "
					case "proxyKeyPath":
						d.proxyKeyPathInput += " "
					}
				}
			}

		// 文本输入操作
		default:
			// 如果 textarea 聚焦，优先处理
			if d.keyContentFocused && !d.useExistingKey {
				var cmd tea.Cmd
				d.keyContentTextarea, cmd = d.keyContentTextarea.Update(msg)
				return d, cmd
			}

			if !d.protocolFocus && !d.authFocus && !d.groupFocus && !d.useExistingFocus && !d.connectViaFocus && !d.proxyJumpListFocus {
				visibleFields := d.getVisibleFields()
				if d.focusIndex < len(visibleFields) {
					fieldName := visibleFields[d.focusIndex]
					switch fieldName {
					case "name":
						d.nameInput = HandleTextInput(d.nameInput, msg)
					case "ip":
						d.ipInput = HandleTextInput(d.ipInput, msg)
					case "port":
						d.portInput = HandlePortInput(d.portInput, msg)
					case "username":
						d.usernameInput = HandleTextInput(d.usernameInput, msg)
					case "password":
						d.passwordInput = HandlePasswordInput(d.passwordInput, msg)
					case "keyPath":
						d.keyPathInput = HandleTextInput(d.keyPathInput, msg)
					case "passphrase":
						d.passphraseInput = HandlePasswordInput(d.passphraseInput, msg)
					case "description":
						d.descriptionInput = HandleTextInput(d.descriptionInput, msg)
					case "group":
						if msg.Type != tea.KeySpace {
							d.groupInput = HandleTextInput(d.groupInput, msg)
						}
					case "proxyHost":
						d.proxyHostInput = HandleTextInput(d.proxyHostInput, msg)
					case "proxyPort":
						d.proxyPortInput = HandlePortInput(d.proxyPortInput, msg)
					case "proxyUser":
						d.proxyUserInput = HandleTextInput(d.proxyUserInput, msg)
					case "proxyPassword":
						d.proxyPasswordInput = HandlePasswordInput(d.proxyPasswordInput, msg)
					case "proxyKeyPath":
						d.proxyKeyPathInput = HandleTextInput(d.proxyKeyPathInput, msg)
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
			name:     "useExistingKey",
			label:    "已导入密钥",
			visible:  d.protocol == "ssh" && d.authMethod == "key",
			getValue: func() string {
				if d.useExistingKey {
					return "是"
				}
				return "否"
			},
			render: d.renderUseExistingSelect,
		},
		{
			name:     "keyPath",
			label:    "密钥路径",
			visible:  d.protocol == "ssh" && d.authMethod == "key" && d.useExistingKey,
			getValue: func() string { return d.keyPathInput },
			render:   d.renderTextField,
		},
		{
			name:     "keyContent",
			label:    "密钥内容",
			visible:  d.protocol == "ssh" && d.authMethod == "key" && !d.useExistingKey,
			getValue: func() string { return d.keyContentTextarea.Value() },
			render:   d.renderKeyContentTextarea,
		},
		{
			name:     "passphrase",
			label:    "密钥密码",
			visible:  d.protocol == "ssh" && d.authMethod == "key",
			getValue: func() string { return d.passphraseInput },
			render:   d.renderPasswordField,
		},
		{
			name:     "connectVia",
			label:    "连接方式",
			visible:  d.protocol == "ssh",
			getValue: func() string {
				switch d.connectVia {
				case "proxyjump":
					return "ProxyJump"
				case "proxyserver":
					return "ProxyServer"
				default:
					return "直连"
				}
			},
			render: d.renderConnectViaSelect,
		},
		{
			name:     "proxyJump",
			label:    "跳板机",
			visible:  d.protocol == "ssh" && d.connectVia == "proxyjump",
			getValue: func() string { return d.proxyJumpInput },
			render:   d.renderProxyJumpInput,
		},
		{
			name:     "proxyHost",
			label:    "代理地址",
			visible:  d.protocol == "ssh" && d.connectVia == "proxyserver",
			getValue: func() string { return d.proxyHostInput },
			render:   d.renderTextField,
		},
		{
			name:     "proxyPort",
			label:    "代理端口",
			visible:  d.protocol == "ssh" && d.connectVia == "proxyserver",
			getValue: func() string { return d.proxyPortInput },
			render:   d.renderTextField,
		},
		{
			name:     "proxyUser",
			label:    "代理用户",
			visible:  d.protocol == "ssh" && d.connectVia == "proxyserver",
			getValue: func() string { return d.proxyUserInput },
			render:   d.renderTextField,
		},
		{
			name:     "proxyPassword",
			label:    "代理密码",
			visible:  d.protocol == "ssh" && d.connectVia == "proxyserver",
			getValue: func() string { return d.proxyPasswordInput },
			render:   d.renderPasswordField,
		},
		{
			name:     "proxyKeyPath",
			label:    "代理密钥路径",
			visible:  d.protocol == "ssh" && d.connectVia == "proxyserver",
			getValue: func() string { return d.proxyKeyPathInput },
			render:   d.renderTextField,
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

	// 渲染验证错误
	if d.validationError != "" {
		content.WriteString("\n\n")
		content.WriteString(d.renderValidationError())
	}

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
func (d *NewConnectionDialog) renderAuthSelect(label, value string, focused bool) string {
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
func (d *NewConnectionDialog) renderGroupSelect(label, value string, focused bool) string {
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

// 渲染已导入密钥选择字段
func (d *NewConnectionDialog) renderUseExistingSelect(label, value string, focused bool) string {
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

	if d.useExistingFocus || focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.useExistingOptions {
		isSelected := (i == 0 && d.useExistingKey) || (i == 1 && !d.useExistingKey)
		isCursor := d.useExistingFocus && i == d.useExistingIndex

		if isCursor {
			builder.WriteString(cursorStyle.Render(fmt.Sprintf(" %s ", opt)))
		} else if isSelected {
			builder.WriteString(selectedStyle.Render(fmt.Sprintf(" %s ", opt)))
		} else {
			builder.WriteString(fmt.Sprintf(" %s ", opt))
		}
		if i < len(d.useExistingOptions)-1 {
			builder.WriteString("|")
		}
	}

	if d.useExistingFocus || focused {
		builder.WriteString(")")
	} else {
		builder.WriteString(" ")
	}

	return builder.String()
}

// 渲染连接方式选择字段
func (d *NewConnectionDialog) renderConnectViaSelect(label, value string, focused bool) string {
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

	if d.connectViaFocus || focused {
		builder.WriteString("(")
	} else {
		builder.WriteString(" ")
	}

	for i, opt := range d.connectViaOptions {
		var isSelected bool
		switch i {
		case 0:
			isSelected = d.connectVia == "direct"
		case 1:
			isSelected = d.connectVia == "proxyjump"
		case 2:
			isSelected = d.connectVia == "proxyserver"
		}
		isCursor := d.connectViaFocus && i == d.connectViaIndex

		if isCursor {
			builder.WriteString(cursorStyle.Render(fmt.Sprintf(" %s ", opt)))
		} else if isSelected {
			builder.WriteString(selectedStyle.Render(fmt.Sprintf(" %s ", opt)))
		} else {
			builder.WriteString(fmt.Sprintf(" %s ", opt))
		}
		if i < len(d.connectViaOptions)-1 {
			builder.WriteString("|")
		}
	}

	if d.connectViaFocus || focused {
		builder.WriteString(")")
	} else {
		builder.WriteString(" ")
	}

	return builder.String()
}

// 渲染跳板机输入字段
func (d *NewConnectionDialog) renderProxyJumpInput(label, value string, focused bool) string {
	var builder strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#00ff00"))
	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#ffff00"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	builder.WriteString(labelStyle.Render(label + ":"))
	builder.WriteString("\n")

	if d.proxyJumpListFocus {
		// 列表选择模式：显示垂直列表
		for i, proxy := range d.availableProxies {
			if i == d.proxyJumpListIndex {
				// 当前光标位置
				builder.WriteString("  ")
				builder.WriteString(cursorStyle.Render("> " + proxy))
			} else if proxy == d.proxyJumpInput {
				// 已选中项
				builder.WriteString("  ")
				builder.WriteString(selectedStyle.Render("  " + proxy))
			} else {
				builder.WriteString("    ")
				builder.WriteString(proxy)
			}
			builder.WriteString("\n")
		}
		builder.WriteString(hintStyle.Render("  (↑/↓ 选择，Enter 确认，Esc 取消)"))
	} else {
		// 普通模式：显示当前选择 + 提示
		if len(d.availableProxies) > 0 {
			if value != "" {
				builder.WriteString(selectedStyle.Render("  " + value))
			} else {
				builder.WriteString(hintStyle.Render("  (未选择)"))
			}
			builder.WriteString("\n")
			if focused {
				builder.WriteString(hintStyle.Render("  (按空格选择跳板机)"))
			}
		} else {
			builder.WriteString(hintStyle.Render("  (无可用跳板机)"))
		}
	}

	return builder.String()
}

// 渲染密钥内容 textarea
func (d *NewConnectionDialog) renderKeyContentTextarea(label, value string, focused bool) string {
	var builder strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	builder.WriteString(labelStyle.Render(label + ":"))
	builder.WriteString("\n")

	// 渲染 textarea
	if d.keyContentFocused {
		builder.WriteString(d.keyContentTextarea.View())
	} else {
		// 非聚焦时显示预览
		content := d.keyContentTextarea.Value()
		lines := strings.Split(content, "\n")
		previewStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ff00")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00ff00")).
			Padding(0, 1)
		if len(content) > 0 {
			builder.WriteString(previewStyle.Render(fmt.Sprintf("%d 行密钥内容 (按空格编辑)", len(lines))))
		} else {
			placeholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
			builder.WriteString(placeholderStyle.Render("(按空格输入密钥内容)"))
		}
	}

	return builder.String()
}

// renderValidationError 渲染验证错误信息
func (d *NewConnectionDialog) renderValidationError() string {
	if d.validationError == "" {
		return ""
	}
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		Bold(true)
	return errorStyle.Render("✗ " + d.validationError)
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

		if d.authMethod == "key" {
			if d.useExistingKey {
				if strings.TrimSpace(d.keyPathInput) == "" {
					return models.NewValidationError("密钥认证需要输入密钥路径")
				}
			} else {
				content := d.keyContentTextarea.Value()
				if strings.TrimSpace(content) == "" {
					return models.NewValidationError("请输入密钥内容")
				}
				if !strings.Contains(content, "-----BEGIN") {
					return models.NewValidationError("密钥格式无效：缺少 PEM 头部标识")
				}
				if !strings.Contains(content, "-----END") {
					return models.NewValidationError("密钥格式无效：缺少 PEM 尾部标识")
				}
			}
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
			if d.useExistingKey {
				host.KeyPath = strings.TrimSpace(d.keyPathInput)
			}
			// 如果 !useExistingKey，KeyPath 由 app.go 在保存密钥文件后设置
			host.Passphrase = strings.TrimSpace(d.passphraseInput)
		}

		// 设置代理连接信息
		host.ConnectVia = d.connectVia
		if d.connectVia == "proxyjump" {
			host.ProxyJump = strings.TrimSpace(d.proxyJumpInput)
		} else if d.connectVia == "proxyserver" {
			host.ProxyHost = strings.TrimSpace(d.proxyHostInput)
			if strings.TrimSpace(d.proxyPortInput) != "" {
				var port int
				fmt.Sscanf(d.proxyPortInput, "%d", &port)
				host.ProxyPort = port
			}
			host.ProxyUser = strings.TrimSpace(d.proxyUserInput)
			host.ProxyPassword = strings.TrimSpace(d.proxyPasswordInput)
			host.ProxyKeyPath = strings.TrimSpace(d.proxyKeyPathInput)
		}
	} else {
		// RDP和VNC使用密码认证
		host.Password = strings.TrimSpace(d.passwordInput)
	}

	return host
}

// GetKeyContent 获取密钥内容
func (d *NewConnectionDialog) GetKeyContent() string {
	if d.useExistingKey {
		return "" // 使用已导入密钥路径，无需保存内容
	}
	return strings.TrimSpace(d.keyContentTextarea.Value())
}

// NeedsToSaveKey 判断是否需要保存密钥文件
func (d *NewConnectionDialog) NeedsToSaveKey() bool {
	return d.protocol == "ssh" && d.authMethod == "key" && !d.useExistingKey
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
		{name: "useExistingKey", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key" && d.useExistingKey},
		{name: "keyContent", visible: d.protocol == "ssh" && d.authMethod == "key" && !d.useExistingKey},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "connectVia", visible: d.protocol == "ssh"},
		{name: "proxyJump", visible: d.protocol == "ssh" && d.connectVia == "proxyjump"},
		{name: "proxyHost", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPort", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyUser", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPassword", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyKeyPath", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
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
		{name: "useExistingKey", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key" && d.useExistingKey},
		{name: "keyContent", visible: d.protocol == "ssh" && d.authMethod == "key" && !d.useExistingKey},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "connectVia", visible: d.protocol == "ssh"},
		{name: "proxyJump", visible: d.protocol == "ssh" && d.connectVia == "proxyjump"},
		{name: "proxyHost", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPort", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyUser", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPassword", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyKeyPath", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
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
		{name: "useExistingKey", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key" && d.useExistingKey},
		{name: "keyContent", visible: d.protocol == "ssh" && d.authMethod == "key" && !d.useExistingKey},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "connectVia", visible: d.protocol == "ssh"},
		{name: "proxyJump", visible: d.protocol == "ssh" && d.connectVia == "proxyjump"},
		{name: "proxyHost", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPort", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyUser", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPassword", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyKeyPath", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
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
func (d *NewConnectionDialog) ensureValidFocusIndex() {
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
		{name: "useExistingKey", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "keyPath", visible: d.protocol == "ssh" && d.authMethod == "key" && d.useExistingKey},
		{name: "keyContent", visible: d.protocol == "ssh" && d.authMethod == "key" && !d.useExistingKey},
		{name: "passphrase", visible: d.protocol == "ssh" && d.authMethod == "key"},
		{name: "connectVia", visible: d.protocol == "ssh"},
		{name: "proxyJump", visible: d.protocol == "ssh" && d.connectVia == "proxyjump"},
		{name: "proxyHost", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPort", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyUser", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyPassword", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
		{name: "proxyKeyPath", visible: d.protocol == "ssh" && d.connectVia == "proxyserver"},
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
