package tui

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.uber.org/zap"
	"remote-desktop-manager/internal/config"
	"remote-desktop-manager/pkg/models"
)

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
		selected:      0,
		searchQuery:   "",
		searchMode:    false,
		searchCursor:  0,
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

		case "enter", " ":
			// 连接选中的主机
			if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts) {
				host := a.filteredHosts[a.selected]
				a.logger.Info("连接主机", zap.String("host", host.Name))
				// TODO: 实现连接逻辑
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

// stripANSI 移除字符串中的ANSI转义序列
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
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
			// 添加省略号（如果还有空间）
			if currentWidth+1 <= maxWidth {
				result += "…"
			}
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
	// 加上列之间的空格：每列之间1个空格，6个分隔符 = 6字符，总共95字符

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

	// 表头样式
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true).
		Background(lipgloss.Color("#003300"))

	// 渲染每一列
	renderedColumns := make([]string, len(headers))
	for i, header := range headers {
		width := widths[i]
		displayText := truncateByDisplayWidth(header, width)
		colStyle := headerStyle.Copy().Width(width).Align(lipgloss.Left)
		renderedColumns[i] = colStyle.Render(displayText)
	}

	// 组合各列
	separator := strings.Repeat(" ", colSpacing)
	return strings.Join(renderedColumns, separator)
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

	// 列内容
	columns := []string{
		selectionIndicator,
		protocolText,
		host.Name,
		host.Host,
		host.Username,
		groupName,
		statusText,
	}

	// 基础样式
	baseStyle := lipgloss.NewStyle()
	if selected {
		baseStyle = baseStyle.
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00ff00")).
			Bold(true)
	} else {
		baseStyle = baseStyle.Foreground(lipgloss.Color("#00ff00"))
	}

	// 渲染每一列
	renderedColumns := make([]string, len(columns))
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

		// 创建列样式
		colStyle := baseStyle.Copy().Width(width).Align(lipgloss.Left)

		// 状态列特殊着色
		if i == 6 { // 状态列
			colStyle = colStyle.Foreground(lipgloss.Color(statusColor)).Bold(true)
		}

		renderedColumns[i] = colStyle.Render(displayText)
	}

	// 组合各列
	separator := strings.Repeat(" ", colSpacing)
	return strings.Join(renderedColumns, separator)
}

// renderStatusBar 渲染状态栏
func (a *App) renderStatusBar() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(a.width).
		Align(lipgloss.Left)

	// 统计在线主机
	onlineCount := 0
	for _, host := range a.hosts {
		if host.Status == "online" {
			onlineCount++
		}
	}

	status := fmt.Sprintf("主机: %d/%d | 在线: %d/%d | 分组: %d | 选中: ",
		a.selected+1, len(a.filteredHosts), onlineCount, len(a.hosts), len(a.groups))

	if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts) {
		host := a.filteredHosts[a.selected]
		status += host.Name
	} else {
		status += "无"
	}

	// 添加搜索状态
	if a.searchQuery != "" {
		status += fmt.Sprintf(" | 搜索: '%s'", a.searchQuery)
	}

	// 添加最后状态检查时间
	lastCheckStr := a.lastStatusCheck.Format("15:04:05")
	status += fmt.Sprintf(" | 状态更新: %s", lastCheckStr)

	return style.Render(status)
}

// renderHelp 渲染帮助信息
func (a *App) renderHelp() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		Width(a.width).
		Align(lipgloss.Center)

	var help string
	if a.searchMode {
		help = "输入搜索词 | Esc: 退出搜索 | Enter: 确认搜索"
	} else {
		help = "↑/↓: 选择 | Enter: 连接 | Tab: 切换分组 | R: 刷新 | /: 搜索 | Q: 退出"
	}

	return style.Render(help)
}

// Run 启动TUI应用程序
func Run(logger *zap.Logger) error {
	app, err := NewApp(logger)
	if err != nil {
		return err
	}

	p := tea.NewProgram(app,
		tea.WithAltScreen(), // 使用备用屏幕
		tea.WithMouseCellMotion(), // 启用鼠标支持
	)

	if _, err := p.Run(); err != nil {
		logger.Error("TUI运行失败", zap.Error(err))
		return err
	}

	return nil
}