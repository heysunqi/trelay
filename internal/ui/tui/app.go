package tui

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"remote-desktop-manager/internal/config"
	"remote-desktop-manager/internal/protocol"
	sshclient "remote-desktop-manager/internal/protocol/ssh"
	"remote-desktop-manager/internal/ui/dialogs"
	"remote-desktop-manager/pkg/models"
)

// 终端IOCTL常量（跨平台）
// macOS (Darwin): TIOCGETA = 0x404C7413, TIOCSETA = 0x804C7414
// Linux: TCGETS = 0x5401, TCSETS = 0x5402
var (
	tcGetTermios = uint(getTCGETS())
	tcSetTermios = uint(getTCSETS())
)

func getTCGETS() uintptr {
	if runtime.GOOS == "darwin" {
		return 0x404C7413 // TIOCGETA on macOS
	}
	return unix.TCGETS
}

func getTCSETS() uintptr {
	if runtime.GOOS == "darwin" {
		return 0x804C7414 // TIOCSETA on macOS
	}
	return unix.TCSETS
}

// App 表示TUI应用程序
type App struct {
	config         *models.Config
	logger         *zap.Logger
	configMgr      *config.ConfigManager
	width          int
	height         int
	ready          bool
	selected       int // 当前选中的主机索引
	hosts          []*models.Host // 扁平化的主机列表
	filteredHosts  []*models.Host // 过滤后的主机列表
	grouped        map[string][]*models.Host // 分组的主机
	groups         []string // 分组名称列表
	currentGroup   string // 当前选中的分组
	quitting       bool

	// 搜索相关字段
	searchQuery    string
	searchMode     bool
	searchCursor   int // 搜索框光标位置

	// 状态刷新相关字段
	lastStatusCheck time.Time

	// 连接相关字段
	connManager    *protocol.Manager
	showDialog     bool // 是否显示对话框
	connectDialog  *dialogs.ConnectDialog
	connecting    bool // 是否正在连接，防止重复触发
}

// NewApp 创建新的应用程序实例
func NewApp(logger *zap.Logger) (*App, error) {
	// 加载配置
	configMgr := config.NewConfigManager(logger)
	cfg, err := configMgr.Load()
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	app := &App{
		config:        cfg,
		logger:        logger,
		configMgr:     configMgr,
		connManager:   protocol.NewManager(),
		selected:      0,
		searchQuery:   "",
		searchMode:    false,
		searchCursor:  0,
		showDialog:    false,
	}

	// 初始化主机数据
	app.refreshHosts()

	// 初始化最后状态检查时间
	app.lastStatusCheck = time.Now()

	return app, nil
}

// refreshHosts 刷新主机列表
func (a *App) refreshHosts() {
	// 获取分组的主机
	a.grouped = a.config.GetGroupedHosts()

	// 获取分组名称
	a.groups = make([]string, 0, len(a.grouped))
	for groupName := range a.grouped {
		a.groups = append(a.groups, groupName)
	}

	// 如果没有当前分组，设置第一个分组
	if a.currentGroup == "" && len(a.groups) > 0 {
		a.currentGroup = a.groups[0]
	}

	// 扁平化当前分组的主机
	if hosts, ok := a.grouped[a.currentGroup]; ok {
		a.hosts = hosts
	} else {
		a.hosts = []*models.Host{}
	}

	// 应用搜索过滤
	a.applySearchFilter()

	// 确保选中索引在有效范围内
	if len(a.filteredHosts) > 0 {
		if a.selected >= len(a.filteredHosts) {
			a.selected = len(a.filteredHosts) - 1
		}
	} else {
		a.selected = 0
	}
}

// applySearchFilter 应用搜索过滤
func (a *App) applySearchFilter() {
	if a.searchQuery == "" {
		a.filteredHosts = a.hosts
		return
	}

	query := strings.ToLower(a.searchQuery)
	var filtered []*models.Host

	for _, host := range a.hosts {
		// 搜索主机名、描述、IP地址
		if strings.Contains(strings.ToLower(host.Name), query) ||
			strings.Contains(strings.ToLower(host.Description), query) ||
			strings.Contains(strings.ToLower(host.Host), query) {
			filtered = append(filtered, host)
		}
	}

	a.filteredHosts = filtered
}

// checkHostStatus 检查主机状态
func (a *App) checkHostStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for _, host := range a.hosts {
		// 跳过本地主机（localhost）的检查
		if host.Host == "localhost" || host.Host == "127.0.0.1" {
			host.Status = "online"
			continue
		}

		// 使用TCP连接检查端口是否可达
		address := fmt.Sprintf("%s:%d", host.Host, host.GetPort())

		dialer := &net.Dialer{
			Timeout: 2 * time.Second,
		}

		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			host.Status = "offline"
			continue
		}

		conn.Close()
		host.Status = "online"
	}

	a.lastStatusCheck = time.Now()
}

// statusCheckCmd 状态检查命令
func (a *App) statusCheckCmd() tea.Cmd {
	return tea.Every(3*time.Second, func(t time.Time) tea.Msg {
		return statusCheckMsg(t)
	})
}

// statusCheckMsg 状态检查消息
type statusCheckMsg time.Time

// toggleSearchMode 切换搜索模式
func (a *App) toggleSearchMode() {
	a.searchMode = !a.searchMode
	if !a.searchMode {
		a.searchQuery = ""
		a.searchCursor = 0
		a.applySearchFilter()
	}
}

// executeConnection 执行连接
func (a *App) executeConnection(host *models.Host) {
	// 退出TUI
	a.quitting = true

	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		a.logger.Error("获取可执行文件路径失败", zap.Error(err))
		return
	}

	// 使用goroutine在TUI退出后执行SSH连接
	go func() {
		// 等待一段时间让TUI完全退出
		time.Sleep(100 * time.Millisecond)

		// 清理终端
		fmt.Print("\033[2J\033[H") // 清屏和重置光标

		// 恢复终端到正常模式（cooked mode）
		// Bubble Tea 退出时应该已经恢复，但我们要确保终端在正常模式
		if fd := int(os.Stdin.Fd()); fd > 0 {
			// 读取当前终端设置
			termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
			if err == nil {
				// 设置为 cooked mode
				termios.Lflag |= unix.ICANON | unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHOCTL | unix.ECHOKE
				termios.Lflag &^= unix.ISIG
				unix.IoctlSetTermios(fd, unix.TCSETS, termios)
			}
		}

		// 创建SSH客户端
		client := sshclient.NewClient(host)
		a.logger.Info("正在连接SSH主机", zap.String("host", host.Name))

		// 连接
		if err := client.Connect(); err != nil {
			fmt.Printf("连接失败: %v\n", err)
			a.promptRestart(execPath)
			return
		}

		fmt.Printf("已连接到 %s\n\n", host.Name)
		a.logger.Info("SSH连接成功", zap.String("host", host.Name))

		// 启动交互式会话
		if err := client.StartInteractiveSession(); err != nil {
			a.logger.Error("SSH会话错误", zap.Error(err))
		}

		// 断开连接
		client.Disconnect()
		fmt.Printf("\n已断开与 %s 的连接\n", host.Name)

		// 提示重启程序
		a.promptRestart(execPath)
	}()
}

// promptRestart 提示用户重启程序
func (a *App) promptRestart(execPath string) {
	// 确保终端在正常模式
	if fd := int(os.Stdin.Fd()); fd > 0 {
		termios, err := unix.IoctlGetTermios(fd, tcGetTermios)
		if err == nil {
			// 设置为 cooked mode
			termios.Lflag |= unix.ICANON | unix.ECHO | unix.ISIG
			unix.IoctlSetTermios(fd, tcSetTermios, termios)
		}
	}

	// 清空输入缓冲区（非阻塞方式）
	if fd := int(os.Stdin.Fd()); fd > 0 {
		// 使用O_NON标志设置非阻塞读取
		flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
		if err == nil {
			unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags|unix.O_NONBLOCK)
			buf := make([]byte, 1024)
			for {
				_, readErr := os.Stdin.Read(buf)
				if readErr != nil {
					break // 没有更多数据可读
				}
			}
			// 恢复阻塞模式
			unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags)
		}
	}

	fmt.Println("\n按 Enter 键返回TUI，或按 Ctrl+C 退出...")

	// 等待Enter键
	readBuf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(readBuf)
		if err != nil {
			// 读取错误（可能是Ctrl+C）
			os.Exit(0)
		}
		if n > 0 {
			// 只响应Enter键
			if readBuf[0] == '\n' || readBuf[0] == '\r' {
				break
			}
		}
	}

	// 重新启动程序（使用exec替换当前进程）
	fmt.Print("\033[2J\033[H") // 清屏
	err := unix.Exec(execPath, []string{execPath}, os.Environ())
	if err != nil {
		fmt.Printf("重启失败: %v\n", err)
		os.Exit(1)
	}
}

// Init 初始化应用程序，返回初始命令
func (a *App) Init() tea.Cmd {
	// 初始状态检查
	a.checkHostStatus()

	// 返回定时状态检查命令
	return a.statusCheckCmd()
}

// Update 处理消息和更新状态
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// 处理窗口大小变化
		if !a.ready {
			a.ready = true
		}
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case statusCheckMsg:
		// 定时状态检查
		a.checkHostStatus()
		// 继续定时检查
		return a, a.statusCheckCmd()

	case tea.KeyMsg:
		// 如果对话框显示，转发消息给对话框
		if a.showDialog && a.connectDialog != nil {
			// 更新对话框
			newDialog, cmd := a.connectDialog.Update(msg)
			a.connectDialog = newDialog

			// 检查对话框是否应该关闭
			if a.connectDialog.IsClosed() {
				// 保存确认状态
				confirmed := a.connectDialog.IsConfirmed()

				// 关闭对话框
				a.showDialog = false
				host := a.connectDialog.Host()
				a.connectDialog = nil

				// 清除连接标志（如果取消连接）
				if !confirmed {
					a.connecting = false
				}

				// 如果确认，执行连接
				if confirmed {
					a.executeConnection(host)
				}
				return a, cmd
			}

			// 如果对话框返回非nil命令，执行它（可能是Quit）
			if cmd != nil {
				return a, cmd
			}

			return a, cmd
		}

		// 处理键盘输入
		switch msg.String() {
		case "q", "ctrl+c":
			// 退出程序
			a.quitting = true
			return a, tea.Quit

		case "/":
			// 进入/退出搜索模式
			a.toggleSearchMode()
			return a, nil

		case "esc":
			// 退出搜索模式
			if a.searchMode {
				a.toggleSearchMode()
			}
			return a, nil

		case "up", "k":
			// 上移选择
			if a.selected > 0 {
				a.selected--
			}
			return a, nil

		case "down", "j":
			// 下移选择
			if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts)-1 {
				a.selected++
			}
			return a, nil

		case "enter":
			// 显示连接确认对话框（只响应Enter键，不响应空格）
			if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts) && !a.connecting {
				host := a.filteredHosts[a.selected]
				a.connectDialog = dialogs.NewConnectDialog(host)
				a.showDialog = true
				a.connecting = true // 设置连接标志，防止重复触发
			}
			return a, nil

		case "tab":
			// 切换分组
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

		case "r":
			// 刷新配置
			if cfg, err := a.configMgr.Load(); err == nil {
				a.config = cfg
				a.refreshHosts()
				a.logger.Info("配置已刷新")
			} else {
				a.logger.Error("刷新配置失败", zap.Error(err))
			}
			return a, nil

		default:
			// 搜索模式下的输入处理
			if a.searchMode {
				switch msg.Type {
				case tea.KeyBackspace, tea.KeyDelete:
					// 删除字符
					if len(a.searchQuery) > 0 && a.searchCursor > 0 {
						a.searchQuery = a.searchQuery[:a.searchCursor-1] + a.searchQuery[a.searchCursor:]
						a.searchCursor--
						a.applySearchFilter()
					}
				case tea.KeyLeft:
					// 左移光标
					if a.searchCursor > 0 {
						a.searchCursor--
					}
				case tea.KeyRight:
					// 右移光标
					if a.searchCursor < len(a.searchQuery) {
						a.searchCursor++
					}
				case tea.KeyHome:
					// 移动到开头
					a.searchCursor = 0
				case tea.KeyEnd:
					// 移动到结尾
					a.searchCursor = len(a.searchQuery)
				default:
					// 普通字符输入
					if msg.Runes != nil && len(msg.Runes) > 0 {
						r := string(msg.Runes)
						a.searchQuery = a.searchQuery[:a.searchCursor] + r + a.searchQuery[a.searchCursor:]
						a.searchCursor += len(r)
						a.applySearchFilter()
					}
				}
				return a, nil
			}
		}
	}

	return a, nil
}

// View 渲染界面
func (a *App) View() string {
	if !a.ready {
		return "正在初始化..."
	}

	if a.quitting {
		return "再见！\n"
	}

	var content string

	// 标题
	title := a.renderTitle()
	content += title + "\n\n"

	// 主机列表
	hostList := a.renderHostList()
	content += hostList + "\n\n"

	// 状态栏
	statusBar := a.renderStatusBar()
	content += statusBar

	// 帮助信息
	help := a.renderHelp()
	content += "\n" + help

	return content
}

// renderTitle 渲染标题
func (a *App) renderTitle() string {
	var titleContent string

	// 主标题
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ff00")).
		Padding(0, 1)

	title := "远程桌面管理器"
	if a.currentGroup != "" {
		title += " - " + a.currentGroup
	}
	titleContent = titleStyle.Render(title)

	// 搜索框
	if a.searchMode || a.searchQuery != "" {
		titleContent += "\n" + a.renderSearchBox()
	}

	// 边框样式
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Padding(0, 1).
		Width(a.width - 2)

	return style.Render(titleContent)
}

// renderSearchBox 渲染搜索框
func (a *App) renderSearchBox() string {
	searchLabel := "搜索: "
	cursor := "█"

	var displayText string
	if a.searchCursor >= len(a.searchQuery) {
		displayText = a.searchQuery + cursor
	} else {
		displayText = a.searchQuery[:a.searchCursor] + cursor + a.searchQuery[a.searchCursor:]
	}

	searchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Background(lipgloss.Color("#003300")).
		Padding(0, 1)

	return searchStyle.Render(searchLabel + displayText)
}

// displayWidth 计算字符串在终端中的显示宽度（中文字符2，英文字符1）
func displayWidth(s string) int {
	// 先移除ANSI序列
	s = stripANSI(s)
	width := 0
	for _, r := range s {
		if unicode.In(r, unicode.Han) || // 汉字
			unicode.In(r, unicode.Hiragana) || // 平假名
			unicode.In(r, unicode.Katakana) || // 片假名
			unicode.In(r, unicode.Hangul) { // 韩文
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

// truncateByDisplayWidth 按显示宽度截断字符串
func truncateByDisplayWidth(s string, maxWidth int) string {
	if displayWidth(s) <= maxWidth {
		return s
	}

	result := ""
	currentWidth := 0
	for _, r := range s {
		runeWidth := 1
		if unicode.In(r, unicode.Han) ||
			unicode.In(r, unicode.Hiragana) ||
			unicode.In(r, unicode.Katakana) ||
			unicode.In(r, unicode.Hangul) {
			runeWidth = 2
		}

		if currentWidth+runeWidth > maxWidth {
			result += "…"
			break
		}
		result += string(r)
		currentWidth += runeWidth
	}
	return result
}

// getColumnWidths 计算表格列宽
func (a *App) getColumnWidths() ([]int, int) {
	// 列定义：选中(2)、协议(4)、名称(30)、IP(15)、用户名(10)、分组(20)、状态(8)
	// 最小总宽度：2+4+30+15+10+20+8 = 89字符
	// 加上列之间的空格：每列之间1个空格，6个分隔符 = 6字符，总共93字符

	minWidths := []int{2, 4, 30, 15, 10, 20, 8}
	colSpacing := 1 // 列之间的空格数

	// 如果终端宽度足够，按比例分配额外空间
	availableWidth := a.width - 4 // 减去边框和内边距
	minTotalWidth := 0
	for _, w := range minWidths {
		minTotalWidth += w
	}
	minTotalWidth += colSpacing * (len(minWidths) - 1)

	// 如果可用宽度小于最小总宽度，等比例缩小各列
	widths := make([]int, len(minWidths))
	if availableWidth < minTotalWidth {
		// 等比例缩小
		scale := float64(availableWidth) / float64(minTotalWidth)
		for i, w := range minWidths {
			widths[i] = int(float64(w) * scale)
			if widths[i] < 2 { // 确保最小宽度为2
				widths[i] = 2
			}
		}
	} else {
			// 使用最小宽度，多余空间加到名称列
		copy(widths, minWidths)
		extraWidth := availableWidth - minTotalWidth
		widths[2] += extraWidth // 名称列获得额外空间
	}

	return widths, colSpacing
}

// renderTableHeader 渲染表格表头
func (a *App) renderTableHeader() string {
	widths, colSpacing := a.getColumnWidths()

	// 表头文本
	headers := []string{" ", "协议", "名称", "IP地址", "用户名", "分组", "状态"}

	// 构建表头行
	var headerBuilder strings.Builder
	for i, header := range headers {
		width := widths[i]
		headerDisplayWidth := displayWidth(header)
		// 左对齐显示表头
		headerBuilder.WriteString(header)
		padding := width - headerDisplayWidth
		if padding > 0 {
			headerBuilder.WriteString(strings.Repeat(" ", padding))
		}

		// 添加列分隔符（最后一列后不加）
		if i < len(headers)-1 {
			headerBuilder.WriteString(strings.Repeat(" ", colSpacing))
		}
	}

	// 表头样式
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true).
		Background(lipgloss.Color("#003300"))

	return headerStyle.Render(headerBuilder.String())
}

// renderHostList 渲染主机列表
func (a *App) renderHostList() string {
	if len(a.filteredHosts) == 0 {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Width(a.width - 4).
			Align(lipgloss.Center)

		message := "没有可用的主机配置"
		if a.searchQuery != "" {
			message = fmt.Sprintf("没有找到匹配 '%s' 的主机", a.searchQuery)
		}
		return style.Render(message)
	}

	var list string
	// 添加表头
	list += a.renderTableHeader() + "\n"

	// 添加分隔线（与表格宽度匹配）
	widths, colSpacing := a.getColumnWidths()
	tableWidth := 0
	for _, w := range widths {
		tableWidth += w
	}
	tableWidth += colSpacing * (len(widths) - 1)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00aa00"))
	list += separatorStyle.Render(strings.Repeat("─", tableWidth)) + "\n"

	for i, host := range a.filteredHosts {
		item := a.renderHostItem(host, i == a.selected)
		list += item + "\n"
	}

	return list
}

// renderHostItem 渲染单个主机项
func (a *App) renderHostItem(host *models.Host, selected bool) string {
	widths, colSpacing := a.getColumnWidths()

	// 选中状态指示器
	selectionIndicator := " "
	if selected {
		selectionIndicator = "●"
	}

	// 协议文本（替代图标）
	protocolText := ""
	switch host.Protocol {
	case "ssh":
		protocolText = "SSH"
	case "rdp":
		protocolText = "RDP"
	case "vnc":
		protocolText = "VNC"
	default:
		protocolText = host.Protocol
	}

	// 状态文字和颜色
	statusText := "unknown"
	statusColor := "#888888" // 默认灰色

	if host.Status != "" {
		switch host.Status {
		case "online":
			statusText = "online"
			statusColor = "#00ff00" // 绿色
		case "offline":
			statusText = "offline"
			statusColor = "#ff0000" // 红色
		case "connecting":
			statusText = "connecting"
			statusColor = "#ffff00" // 黄色
		case "connected":
			statusText = "connected"
			statusColor = "#00ff00" // 绿色
		default:
			statusText = host.Status
		}
	}

	// 分组信息
	groupName := a.currentGroup
	if groupName == "" {
		groupName = "未分组"
	}

	// 构建表格行
	var rowBuilder strings.Builder
	columns := []string{
		selectionIndicator,
		protocolText,
		host.Name,
		host.Host,
		host.Username,
		groupName,
		statusText,
	}

	for i, column := range columns {
		width := widths[i]
		displayText := column

		// 应用最大长度限制（基于显示宽度）
		if i == 2 { // 名称列
			displayText = truncateByDisplayWidth(displayText, 30)
		} else if i == 5 { // 分组列
			displayText = truncateByDisplayWidth(displayText, 20)
		}

		// 应用列宽限制
		displayText = truncateByDisplayWidth(displayText, width)

		// 添加文本和填充
		rowBuilder.WriteString(displayText)
		padding := width - displayWidth(displayText)
		if padding > 0 {
			rowBuilder.WriteString(strings.Repeat(" ", padding))
		}

		// 添加列分隔符（最后一列后不加）
		if i < len(columns)-1 {
			rowBuilder.WriteString(strings.Repeat(" ", colSpacing))
		}
	}

	// 应用样式
	var style lipgloss.Style
	if selected {
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00ff00")).
			Bold(true)
	} else {
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ff00"))
	}

	// 状态文字单独着色
	rowText := rowBuilder.String()
	// 找到状态列的位置并着色
	// 计算状态列的开始位置（最后一列）
	statusColIndex := len(columns) - 1 // 状态是最后一列
	statusStartPos := 0
	for i := 0; i < statusColIndex; i++ {
		statusStartPos += widths[i]
		if i < statusColIndex-1 {
			statusStartPos += colSpacing
		}
	}
	// 状态列的宽度是widths[statusColIndex]
	statusEndPos := statusStartPos + widths[statusColIndex]

	if statusStartPos < len(rowText) && statusEndPos <= len(rowText) {
		beforeStatus := rowText[:statusStartPos]
		statusCol := rowText[statusStartPos:statusEndPos]
		afterStatus := rowText[statusEndPos:]

		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(statusColor)).Bold(true)
		coloredStatus := statusStyle.Render(statusCol)

		rowText = beforeStatus + coloredStatus + afterStatus
	}

	return style.Render(rowText)
}

// stripANSI 移除字符串中的ANSI转义序列
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
		} else if r == 'm' {
			inEscape = false
		}
		if !inEscape {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// renderStatusBar 渲染状态栏
func (a *App) renderStatusBar() string {
	// 连接状态
	var status string
	if a.showDialog {
		status = "按 [Esc] 取消，[Enter] 确认"
	} else {
		totalHosts := len(a.filteredHosts)
		selectedIndex := a.selected + 1
		status = fmt.Sprintf("%d/%d hosts | [↑/↓] 选择 | [Enter] 连接", selectedIndex, totalHosts)
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00aa00")).
		Width(a.width - 4).
		Align(lipgloss.Right)

	return statusStyle.Render(status)
}

// renderHelp 渲染帮助信息
func (a *App) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)
	helpText := "键盘: ↑↓ 选择 | Enter 连接 | Tab 分组 | / 搜索 | R 刷新 | Q 退出"

	return helpStyle.Render(helpText)
}

// Run 运行TUI应用程序
func Run(logger *zap.Logger) error {
	// 创建应用程序实例
	app, err := NewApp(logger)
	if err != nil {
		return err
	}

	// 创建Bubble Tea程序
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),       // 使用备用屏幕
		tea.WithMouseCellMotion(), // 启用鼠标单元格运动
	)

	// 运行程序
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}