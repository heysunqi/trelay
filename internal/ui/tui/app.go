package tui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"trelay/internal/config"
	"trelay/internal/keymgr"
	"trelay/internal/protocol"
	sshpkg "trelay/internal/protocol/ssh"
	"trelay/internal/ui/dialogs"
	"trelay/pkg/models"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// 终端IOCTL常量（跨平台）
// macOS (Darwin): TIOCGETA = 0x404C7413, TIOCSETA = 0x804C7414
// Linux: TCGETS = 0x5401, TCSETS = 0x5402
var (
	tcGetTermios = uint(getTCGETS())
	tcSetTermios = uint(getTCSETS())
)

// GlobalErrorMessage 全局错误信息（从 main.go 传递到 TUI）
var GlobalErrorMessage string

func getTCGETS() uintptr {
	if runtime.GOOS == "darwin" {
		return 0x404C7413 // TIOCGETA on macOS
	}
	return 0x5401
}

func getTCSETS() uintptr {
	if runtime.GOOS == "darwin" {
		return 0x804C7414 // TIOCSETA on macOS
	}
	return 0x5402
}

// App 表示TUI应用程序
type App struct {
	config        *models.Config
	logger        *zap.Logger
	configMgr     *config.ConfigManager
	width         int
	height        int
	ready         bool
	selected      int                       // 当前选中的主机索引
	hosts         []*models.Host            // 扁平化的主机列表
	filteredHosts []*models.Host            // 过滤后的主机列表
	grouped       map[string][]*models.Host // 分组的主机
	groups        []string                  // 分组名称列表
	currentGroup  string                    // 当前选中的分组
	quitting      bool

	// 搜索相关字段
	searchQuery    string
	searchMode     bool
	searchBoxVisible bool // 搜索输入框持久可见（独立于 searchMode）

	// 分页相关
	paginator paginator.Model
	pageSize  int // 每页显示数量，默认 10

	// 命令输入框相关（Shift+: 触发）
	commandMode  bool            // 是否处于命令模式
	commandInput textinput.Model // 命令/搜索统一输入框

	// 分组选择模式（:group 命令触发）
	groupSelectMode   bool     // 是否处于分组选择模式
	groupSelectCursor int      // 分组列表光标
	groupList         []string // 所有分组列表
	filteredGroupList    []string // 搜索过滤后的分组列表
	groupSearchMode      bool     // 分组列表的搜索模式
	groupSearchQuery     string   // 分组搜索关键词
	groupSearchBoxVisible bool    // 分组搜索输入框持久可见

	// 版本号
	version string

	// 状态刷新相关字段
	lastStatusCheck time.Time

	// 连接相关字段
	connManager      *protocol.Manager
	connecting       bool          // 是否正在连接，防止重复触发
	connectingHost   string        // 正在连接的主机名（用于 spinner 显示）
	connectingCancel context.CancelFunc // 连接取消函数
	spinner          spinner.Model // 连接中的 spinner

	// 新建连接对话框相关字段
	showNewConnectionDialog bool                         // 是否显示新建连接对话框
	newConnectionDialog     *dialogs.NewConnectionDialog // 新建连接对话框实例

	// 密码输入对话框相关字段
	showPasswordDialog bool                    // 是否显示密码输入对话框
	passwordDialog     *dialogs.PasswordDialog // 密码输入对话框实例

	// 错误提示对话框相关字段
	showErrorDialog bool                 // 是否显示错误提示对话框
	errorDialog     *dialogs.ErrorDialog // 错误提示对话框实例

	// 新建分组对话框相关字段
	showNewGroupDialog bool                    // 是否显示新建分组对话框
	newGroupDialog     *dialogs.NewGroupDialog // 新建分组对话框实例

	// 编辑连接对话框相关字段
	showEditDialog bool                          // 是否显示编辑连接对话框
	editDialog     *dialogs.EditConnectionDialog // 编辑连接对话框实例

	// 后台会话相关字段
	showSessionList    bool // 是否显示后台会话列表
	sessionListCursor  int  // 后台会话列表光标
	pendingSSHSession  *sshpkg.PTYSession // 等待 attach 的 SSH 会话
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
		config:       cfg,
		logger:       logger,
		configMgr:    configMgr,
		connManager:  protocol.NewManager(),
		selected:     0,
		searchQuery:  "",
		searchMode:   false,
		version:      "1.0.0",
		pageSize:     1, // 初始占位，View() 首次渲染时自动计算
	}

	// 初始化 spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	app.spinner = s

	// 初始化 paginator
	pg := paginator.New()
	pg.Type = paginator.Dots
	pg.PerPage = 1 // 初始占位，View() 首次渲染时自动计算
	pg.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render("●")
	pg.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render("○")
	app.paginator = pg

	// 初始化命令输入框
	ti := textinput.New()
	ti.Placeholder = "输入命令..."
	ti.CharLimit = 50
	app.commandInput = ti

	// 初始化主机数据
	app.refreshHosts()

	// 初始化最后状态检查时间
	app.lastStatusCheck = time.Now()

	// 如果有错误信息，显示错误对话框
	if GlobalErrorMessage != "" {
		app.showErrorDialog = true
		app.errorDialog = dialogs.NewErrorDialog(GlobalErrorMessage, 80, 24) // 初始大小，会在 WindowSizeMsg 中更新
		GlobalErrorMessage = ""
	}

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

	// 排序分组：字母序，"未分组"始终在最后
	sort.Slice(a.groups, func(i, j int) bool {
		if a.groups[i] == "未分组" {
			return false // 未分组始终在最后
		}
		if a.groups[j] == "未分组" {
			return true // 未分组始终在最后
		}
		return a.groups[i] < a.groups[j] // 字母序
	})

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

	// 更新分页器
	a.updatePaginator()
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

// findHostGroup 查找主机所属分组
func (a *App) findHostGroup(hostName string) string {
	for _, group := range a.config.Groups {
		for _, profileName := range group.Profiles {
			if profileName == hostName {
				return group.Name
			}
		}
	}
	return ""
}

// hostStatusResult 异步状态检查结果
type hostStatusResult struct {
	statuses map[string]string // hostName -> "online"/"offline"
}

// sshSessionMsg SSH 会话完成/detach 消息
type sshSessionMsg struct {
	hostID string
	err    error
}

// sshConnectResultMsg SSH 连接结果消息
type sshConnectResultMsg struct {
	session  *sshpkg.PTYSession
	err      error
	host     *models.Host
	canceled bool // 标记是否被取消
}

// checkHostStatusAsync 异步检查主机状态，不阻塞 UI 线程
func (a *App) checkHostStatusAsync() tea.Cmd {
	type hostInfo struct {
		name string
		host string
		port int
	}
	var hosts []hostInfo
	for _, h := range a.hosts {
		hosts = append(hosts, hostInfo{name: h.Name, host: h.Host, port: h.GetPort()})
	}

	return func() tea.Msg {
		statuses := make(map[string]string)
		var mu sync.Mutex
		var wg sync.WaitGroup

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		for _, h := range hosts {
			if h.host == "localhost" || h.host == "127.0.0.1" {
				mu.Lock()
				statuses[h.name] = "online"
				mu.Unlock()
				continue
			}

			wg.Add(1)
			go func(hi hostInfo) {
				defer wg.Done()
				address := fmt.Sprintf("%s:%d", hi.host, hi.port)
				conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", address)
				mu.Lock()
				if err != nil {
					statuses[hi.name] = "offline"
				} else {
					conn.Close()
					statuses[hi.name] = "online"
				}
				mu.Unlock()
			}(h)
		}

		wg.Wait()
		return hostStatusResult{statuses: statuses}
	}
}

// statusCheckCmd 状态检查命令
func (a *App) statusCheckCmd() tea.Cmd {
	return tea.Every(3*time.Second, func(t time.Time) tea.Msg {
		return statusCheckMsg(t)
	})
}

// statusCheckMsg 状态检查消息
type statusCheckMsg time.Time

// updatePaginator 更新分页器状态
func (a *App) updatePaginator() {
	a.paginator.SetTotalPages(len(a.filteredHosts))
	if a.paginator.TotalPages > 0 && a.paginator.Page >= a.paginator.TotalPages {
		a.paginator.Page = a.paginator.TotalPages - 1
	}
}

// applyGroupSearchFilter 应用分组搜索过滤
func (a *App) applyGroupSearchFilter() {
	if a.groupSearchQuery == "" {
		a.filteredGroupList = a.groupList
	} else {
		query := strings.ToLower(a.groupSearchQuery)
		var filtered []string
		for _, group := range a.groupList {
			if strings.Contains(strings.ToLower(group), query) {
				filtered = append(filtered, group)
			}
		}
		a.filteredGroupList = filtered
	}
	// 确保光标不越界
	if len(a.filteredGroupList) == 0 {
		a.groupSelectCursor = 0
	} else if a.groupSelectCursor >= len(a.filteredGroupList) {
		a.groupSelectCursor = len(a.filteredGroupList) - 1
	}
}

// executeConnection 执行连接
func (a *App) executeConnection(host *models.Host) tea.Cmd {
	switch host.Protocol {
	case "ssh":
		// SSH 使用进程内 PTY 模式，支持后台化
		return tea.Batch(a.executeSSHConnection(host), a.spinner.Tick)
	case "rdp", "vnc":
		// RDP/VNC 仍使用 syscall.Exec 模式
		a.executeExternalConnection(host)
		return nil
	default:
		a.logger.Error("不支持的协议", zap.String("protocol", host.Protocol))
		return nil
	}
}

// executeSSHConnection 使用进程内 PTY 执行 SSH 连接
func (a *App) executeSSHConnection(host *models.Host) tea.Cmd {
	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	a.connectingCancel = cancel

	// 异步建立 SSH 连接
	return func() tea.Msg {
		// 使用带上下文的客户端
		client := sshpkg.NewClientWithContext(ctx, host, a.config.Profiles)
		if err := client.Connect(); err != nil {
			// 检查是否是取消导致的错误
			if errors.Is(err, context.Canceled) {
				return sshConnectResultMsg{canceled: true, host: host}
			}
			return sshConnectResultMsg{err: fmt.Errorf("SSH连接失败: %w", err), host: host}
		}

		// 创建后台 PTY 会话
		ptySession, err := client.StartBackgroundSession()
		if err != nil {
			client.Disconnect()
			return sshConnectResultMsg{err: fmt.Errorf("创建SSH会话失败: %w", err), host: host}
		}

		return sshConnectResultMsg{session: ptySession, host: host}
	}
}

// executeExternalConnection 使用 syscall.Exec 执行外部连接（RDP/VNC）
func (a *App) executeExternalConnection(host *models.Host) {
	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		a.logger.Error("获取可执行文件路径失败", zap.Error(err))
		return
	}

	// 根据协议类型构建命令
	var args []string
	args = append(args, execPath)

	switch host.Protocol {
	case "rdp":
		args = append(args, "--direct-rdp", host.Name)
	case "vnc":
		args = append(args, "--direct-vnc", host.Name)
		// 如果主机配置了密码，则传递密码参数
		if host.Password != "" {
			args = append(args, "--password", host.Password)
		}
	}

	args = append(args, "--return-to-trelay")

	// 使用 syscall.Exec 直接替换当前进程运行直接连接
	a.quitting = true
	err = syscall.Exec(execPath, args, os.Environ())

	// 如果 syscall.Exec 返回，说明执行失败
	if err != nil {
		a.logger.Error("启动直接连接失败", zap.Error(err))
		fmt.Printf("启动直接连接失败: %v\n", err)
	}
}

// attachSSHSession 将 SSH 会话附加到终端
// isResume: true 表示从后台恢复会话，false 表示首次连接
func (a *App) attachSSHSession(session *sshpkg.PTYSession, isResume bool) tea.Cmd {
	adapter := sshpkg.NewExecAdapter(session, isResume)
	hostID := session.GetHostID()

	return tea.Exec(adapter, func(err error) tea.Msg {
		return sshSessionMsg{hostID: hostID, err: err}
	})
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
	// 使用更安全的清屏方式，避免在某些终端上显示乱码
	fmt.Print("\n\n") // 简单换行清屏
	err := syscall.Exec(execPath, []string{execPath}, os.Environ())
	if err != nil {
		// syscall.Exec 失败通常不会返回，因为进程已经被替换
		// 这里只是为了编译通过
		fmt.Printf("重启失败: %v\n", err)
		os.Exit(1)
	}
}

// Init 初始化应用程序，返回初始命令
func (a *App) Init() tea.Cmd {
	// 首先获取终端尺寸，异步检查主机状态（不阻塞 UI 渲染）
	return tea.Batch(
		tea.WindowSize(),         // 获取终端尺寸命令
		a.checkHostStatusAsync(), // 异步状态检查
		a.statusCheckCmd(),       // 定时状态检查
		a.spinner.Tick,           // spinner 动画
	)
}

// Update 处理消息和更新状态
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// 如果显示后台会话列表，先处理
	if a.showSessionList {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			bgSessions := a.connManager.GetBackgroundSessions()
			switch keyMsg.String() {
			case "esc", "q":
				a.showSessionList = false
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
					} else if a.sessionListCursor >= len(newSessions) {
						a.sessionListCursor = len(newSessions) - 1
					}
				}
				return a, nil
			}
		}
		// 仍需处理 WindowSizeMsg
		if wMsg, ok := msg.(tea.WindowSizeMsg); ok {
			a.width = wMsg.Width
			a.height = wMsg.Height
		}
		return a, nil
	}

	// 如果正在连接中，处理终止操作
	if a.connecting {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.Type == tea.KeyEsc || keyMsg.String() == "esc" {
				// 取消连接
				if a.connectingCancel != nil {
					a.connectingCancel()
					a.connectingCancel = nil
				}
				// 重置状态
				a.connecting = false
				a.connectingHost = ""
				return a, nil
			}
		}
	}

	// 如果显示密码输入对话框，先让对话框处理消息
	if a.showPasswordDialog && a.passwordDialog != nil {
		updated, cmd := a.passwordDialog.Update(msg)
		a.passwordDialog = updated

		// 检查对话框是否需要关闭
		if a.passwordDialog.IsClosed() {
			// 如果用户提交了密码，则执行连接
			if a.passwordDialog.IsSubmitted() {
				host := a.passwordDialog.Host()
				// 设置密码
				host.Password = a.passwordDialog.GetPassword()
				// 关闭对话框
				a.showPasswordDialog = false
				a.passwordDialog = nil
				// 执行连接
				return a, a.executeConnection(host)
			}
			// 关闭对话框
			a.showPasswordDialog = false
			a.passwordDialog = nil
			a.connecting = false // 重置连接标志
		}

		return a, cmd
	}

	// 如果显示错误对话框，先让对话框处理消息
	if a.showErrorDialog && a.errorDialog != nil {
		// 特殊处理 WindowSizeMsg，让主应用也能处理
		if wMsg, ok := msg.(tea.WindowSizeMsg); ok {
			// 先让对话框处理窗口大小
			updated, cmd := a.errorDialog.Update(msg)
			a.errorDialog = updated

			// 检查对话框是否需要关闭
			if a.errorDialog.IsClosed() {
				a.showErrorDialog = false
				a.errorDialog = nil
			}

			// 让主应用更新窗口大小
			a.width = wMsg.Width
			a.height = wMsg.Height
			if !a.ready {
				a.ready = true
			}

			return a, cmd
		}

		updated, cmd := a.errorDialog.Update(msg)
		a.errorDialog = updated

		// 检查对话框是否需要关闭
		if a.errorDialog.IsClosed() {
			a.showErrorDialog = false
			a.errorDialog = nil
			// 返回 WindowSize 命令以确保 ready 状态被设置
			return a, tea.Batch(cmd, tea.WindowSize())
		}

		return a, cmd
	}

	// 如果显示新建连接对话框，先让对话框处理消息
	if a.showNewConnectionDialog && a.newConnectionDialog != nil {
		updated, cmd := a.newConnectionDialog.Update(msg)
		a.newConnectionDialog = updated

		// 检查对话框是否需要关闭
		if a.newConnectionDialog.IsClosed() {
			// 如果对话框是保存操作，保存主机配置
			if a.newConnectionDialog.IsSaved() {
				// 创建新主机配置
				host := a.newConnectionDialog.CreateHostConfig()

				// 保存密钥内容（如果需要）
				if a.newConnectionDialog.NeedsToSaveKey() {
					keyContent := a.newConnectionDialog.GetKeyContent()
					if keyContent != "" {
						keyPath, err := keymgr.SaveKey(host.Name, keyContent)
						if err != nil {
							a.logger.Error("保存密钥失败", zap.Error(err))
							// 显示错误对话框
							a.showErrorDialog = true
							a.errorDialog = dialogs.NewErrorDialog(
								fmt.Sprintf("保存密钥失败: %v", err),
								a.width, a.height,
							)
							// 关闭新建对话框，显示错误
							a.showNewConnectionDialog = false
							a.newConnectionDialog = nil
							return a, cmd
						}
						host.KeyPath = keyPath
						a.logger.Info("密钥已保存", zap.String("path", keyPath))
					}
				}

				// 添加到配置中
				a.config.Profiles = append(a.config.Profiles, host)

				// 检查分组是否存在，不存在则创建
				groupName := a.newConnectionDialog.GetGroup()
				if groupName != "" {
					found := false
					for _, g := range a.config.Groups {
						if g.Name == groupName {
							found = true
							// 添加主机到分组
							g.Profiles = append(g.Profiles, host.Name)
							break
						}
					}
					if !found {
						// 创建新分组
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

				// 刷新主机列表
				a.refreshHosts()
			}

			// 关闭对话框
			a.showNewConnectionDialog = false
			a.newConnectionDialog = nil
		}

		return a, cmd
	}

	// 如果显示新建分组对话框，先让对话框处理消息
	if a.showNewGroupDialog && a.newGroupDialog != nil {
		updated, cmd := a.newGroupDialog.Update(msg)
		a.newGroupDialog = updated

		// 检查对话框是否需要关闭
		if a.newGroupDialog.IsClosed() {
			if a.newGroupDialog.IsConfirmed() {
				// 创建新分组
				groupName := strings.TrimSpace(a.newGroupDialog.GetGroupName())
				if groupName != "" && groupName != "未分组" {
					// 检查分组是否已存在
					exists := false
					for _, g := range a.config.Groups {
						if g.Name == groupName {
							exists = true
							break
						}
					}

					if !exists {
						// 创建新分组
						newGroup := &models.Group{
							Name:     groupName,
							Profiles: []string{},
						}
						a.config.Groups = append(a.config.Groups, newGroup)

						// 保存配置
						if err := a.configMgr.Save(a.config); err != nil {
							a.logger.Error("保存分组失败", zap.Error(err))
						} else {
							a.logger.Info("新建分组已保存", zap.String("name", groupName))
						}

						// 刷新主机列表
						a.refreshHosts()
					}
				}
			}

			// 关闭对话框
			a.showNewGroupDialog = false
			a.newGroupDialog = nil
		}

		return a, cmd
	}

	// 如果显示编辑连接对话框，先让对话框处理消息
	if a.showEditDialog && a.editDialog != nil {
		updated, cmd := a.editDialog.Update(msg)
		a.editDialog = updated

		// 检查对话框是否需要关闭
		if a.editDialog.IsClosed() {
			if a.editDialog.IsSaved() {
				// 获取原始主机名称
				originalName := a.editDialog.GetOriginalName()

				// 查找并更新配置中的主机
				for _, h := range a.config.Profiles {
					if h.Name == originalName {
						// 保存密钥内容（如果需要）
						if a.editDialog.NeedsToSaveKey() {
							keyContent := a.editDialog.GetKeyContent()
							if keyContent != "" {
								// 使用编辑后的主机名
								newHostName := a.editDialog.GetNameInput()
								keyPath, err := keymgr.SaveKey(newHostName, keyContent)
								if err != nil {
									a.logger.Error("保存密钥失败", zap.Error(err))
									// 显示错误对话框
									a.showErrorDialog = true
									a.errorDialog = dialogs.NewErrorDialog(
										fmt.Sprintf("保存密钥失败: %v", err),
										a.width, a.height,
									)
									// 关闭编辑对话框，显示错误
									a.showEditDialog = false
									a.editDialog = nil
									return a, cmd
								}
								h.KeyPath = keyPath
								a.logger.Info("密钥已保存", zap.String("path", keyPath))
							}
						}

						// 更新主机配置
						a.editDialog.UpdateHostConfig(h)
						break
					}
				}

				// 处理分组变化
				newGroupName := a.editDialog.GetGroup()
				oldGroupName := a.findHostGroup(originalName)

				// 如果主机名称改变，需要更新分组中的引用
				newName := a.editDialog.GetNameInput()

				// 如果分组有变化
				if newGroupName != oldGroupName {
					// 从原分组移除
					if oldGroupName != "" {
						for _, group := range a.config.Groups {
							if group.Name == oldGroupName {
								newProfiles := []string{}
								for _, hostName := range group.Profiles {
									if hostName != originalName {
										newProfiles = append(newProfiles, hostName)
									}
								}
								group.Profiles = newProfiles
								break
							}
						}
					}

					// 添加到新分组
					if newGroupName != "" {
						found := false
						for _, group := range a.config.Groups {
							if group.Name == newGroupName {
								// 使用新名称（如果改变了）
								hostNameToAdd := originalName
								if newName != originalName {
									hostNameToAdd = newName
								}
								group.Profiles = append(group.Profiles, hostNameToAdd)
								found = true
								break
							}
						}
						if !found {
							// 创建新分组
							hostNameToAdd := originalName
							if newName != originalName {
								hostNameToAdd = newName
							}
							a.config.Groups = append(a.config.Groups, &models.Group{
								Name:     newGroupName,
								Profiles: []string{hostNameToAdd},
							})
						}
					}
				} else if newName != originalName && oldGroupName != "" {
					// 如果只是名称改变，更新分组中的引用
					for _, group := range a.config.Groups {
						if group.Name == oldGroupName {
							for i, hostName := range group.Profiles {
								if hostName == originalName {
									group.Profiles[i] = newName
									break
								}
							}
							break
						}
					}
				}

				// 保存配置
				if err := a.configMgr.Save(a.config); err != nil {
					a.logger.Error("保存配置失败", zap.Error(err))
				} else {
					a.logger.Info("主机配置已更新", zap.String("name", newName))
				}

				// 刷新主机列表
				a.refreshHosts()
			}

			// 关闭对话框
			a.showEditDialog = false
			a.editDialog = nil
		}

		return a, cmd
	}

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
		// 定时状态检查（异步，不阻塞 UI）
		return a, tea.Batch(
			a.checkHostStatusAsync(),
			a.statusCheckCmd(),
		)

	case hostStatusResult:
		// 异步状态检查结果返回，更新主机状态
		for _, host := range a.hosts {
			if status, ok := msg.statuses[host.Name]; ok {
				host.Status = status
			}
		}
		a.lastStatusCheck = time.Now()
		return a, nil

	case spinner.TickMsg:
		if a.connecting {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return a, cmd
		}

	case sshConnectResultMsg:
		// SSH 异步连接完成
		a.connecting = false
		a.connectingHost = ""
		if msg.err != nil {
			a.showErrorDialog = true
			a.errorDialog = dialogs.NewErrorDialog(msg.err.Error(), a.width, a.height)
			return a, nil
		}
		// 连接成功，将会话加入管理器并 attach 到终端
		a.connManager.AddSession(msg.session)
		a.logger.Info("SSH连接成功，进入交互模式", zap.String("host", msg.host.Name))
		return a, a.attachSSHSession(msg.session, false) // 首次连接

	case sshSessionMsg:
		// SSH 会话从前台返回（detach 或结束）
		a.connecting = false
		if session, ok := a.connManager.GetSession(msg.hostID); ok {
			if ptySession, ok := session.(*sshpkg.PTYSession); ok {
				if ptySession.IsAlive() {
					// 会话仍然存活 → 已后台化
					a.logger.Info("SSH会话已挂起到后台", zap.String("host", msg.hostID))
				} else {
					// 会话已结束 → 清理
					a.connManager.RemoveSession(msg.hostID)
					a.logger.Info("SSH会话已结束", zap.String("host", msg.hostID))
				}
			}
		}
		// 清理已断开的会话
		a.connManager.CleanupDeadSessions()
		return a, nil

	case tea.KeyMsg:
		// 分组选择模式优先
		if a.groupSelectMode {
			if a.groupSearchMode {
				// 分组搜索子模式
				switch msg.Type {
				case tea.KeyEsc:
					// 退出搜索编辑，清空搜索词，恢复全部分组，输入框保留（空）
					a.groupSearchMode = false
					a.groupSearchQuery = ""
					a.commandInput.Blur()
					a.commandInput.Reset()
					a.applyGroupSearchFilter()
				case tea.KeyEnter:
					// 退出搜索编辑，保留筛选结果，输入框保留（含搜索词）
					a.groupSearchMode = false
					a.commandInput.Blur()
					if len(a.filteredGroupList) > 0 {
						a.groupSelectCursor = 0
					}
				default:
					var cmd tea.Cmd
					a.commandInput, cmd = a.commandInput.Update(msg)
					a.groupSearchQuery = a.commandInput.Value()
					a.applyGroupSearchFilter()
					return a, cmd
				}
				return a, nil
			}

			// 分组选择模式（非搜索）
			switch msg.Type {
			case tea.KeyEsc:
				a.groupSelectMode = false
				a.commandMode = false
				a.groupSearchMode = false
				a.groupSearchQuery = ""
				a.groupSearchBoxVisible = false
				a.commandInput.Blur()
				a.commandInput.Reset()
			default:
				switch msg.String() {
				case "up", "k":
					if a.groupSelectCursor > 0 {
						a.groupSelectCursor--
					}
				case "down", "j":
					if len(a.filteredGroupList) > 0 && a.groupSelectCursor < len(a.filteredGroupList)-1 {
						a.groupSelectCursor++
					}
				case "/":
					a.groupSearchMode = true
					a.groupSearchBoxVisible = true
					if a.groupSearchQuery == "" {
						a.commandInput.Reset()
					}
					a.commandInput.Placeholder = "搜索分组..."
					a.commandInput.Focus()
					return a, textinput.Blink
				case "enter":
					if len(a.filteredGroupList) > 0 && a.groupSelectCursor < len(a.filteredGroupList) {
						a.currentGroup = a.filteredGroupList[a.groupSelectCursor]
					}
					a.groupSelectMode = false
					a.commandMode = false
					a.groupSearchMode = false
					a.groupSearchQuery = ""
					a.groupSearchBoxVisible = false
					a.commandInput.Blur()
					a.commandInput.Reset()
					a.refreshHosts()
				case "q":
					a.groupSelectMode = false
					a.commandMode = false
					a.groupSearchMode = false
					a.groupSearchQuery = ""
					a.groupSearchBoxVisible = false
					a.commandInput.Blur()
					a.commandInput.Reset()
				}
			}
			return a, nil
		}

		// 命令模式
		if a.commandMode {
			switch msg.Type {
			case tea.KeyEsc:
				a.commandMode = false
				a.commandInput.Blur()
				a.commandInput.Reset()
			case tea.KeyEnter:
				cmd := a.commandInput.Value()
				if cmd == "group" {
					a.groupSelectMode = true
					a.groupSelectCursor = 0
					a.groupList = a.groups
					a.filteredGroupList = a.groups
					a.groupSearchQuery = ""
				}
				// 不识别的命令：清除命令模式
				if cmd != "group" {
					a.commandMode = false
					a.commandInput.Blur()
					a.commandInput.Reset()
				}
			default:
				var cmd tea.Cmd
				a.commandInput, cmd = a.commandInput.Update(msg)
				return a, cmd
			}
			return a, nil
		}

		// 主机搜索模式
		if a.searchMode {
			switch msg.Type {
			case tea.KeyEsc:
				// 退出搜索编辑，清空搜索词，恢复全部列表，输入框保留（空）
				a.searchMode = false
				a.searchQuery = ""
				a.commandInput.Blur()
				a.commandInput.Reset()
				a.applySearchFilter()
				a.updatePaginator()
			case tea.KeyEnter:
				// 退出搜索编辑，保留搜索结果，输入框保留（含搜索词）
				a.searchMode = false
				a.commandInput.Blur()
				if len(a.filteredHosts) > 0 {
					a.selected = 0
					a.paginator.Page = 0
				}
			default:
				var cmd tea.Cmd
				a.commandInput, cmd = a.commandInput.Update(msg)
				a.searchQuery = a.commandInput.Value()
				a.applySearchFilter()
				a.updatePaginator()
				return a, cmd
			}
			return a, nil
		}

		// 普通模式下的键盘事件处理
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

		case "tab":
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

		case "N", "n":
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

		case "G", "g":
			a.showNewGroupDialog = true
			a.newGroupDialog = dialogs.NewNewGroupDialog(a.width, a.height)
			return a, nil

		case "B", "b":
			bgSessions := a.connManager.GetBackgroundSessions()
			if len(bgSessions) > 0 {
				a.showSessionList = true
				a.sessionListCursor = 0
			}
			return a, nil

		case "E", "e":
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

		case "r":
			if cfg, err := a.configMgr.Load(); err == nil {
				a.config = cfg
				a.refreshHosts()
				a.logger.Info("配置已刷新")
			} else {
				a.logger.Error("刷新配置失败", zap.Error(err))
			}
			return a, a.checkHostStatusAsync()
		}
	}

	return a, nil
}

// View 渲染界面
func (a *App) View() string {
	// 后台会话列表弹窗优先显示
	if a.showSessionList {
		return a.renderSessionList()
	}

	// 错误对话框优先显示（即使未初始化完成）
	if a.showErrorDialog && a.errorDialog != nil {
		var dialogView string
		dialogView += a.errorDialog.View()
		return dialogView
	}

	// 连接中 loading 状态
	if a.connecting && a.connectingHost != "" {
		return a.renderConnectingView()
	}

	// 密码输入对话框优先显示（即使未初始化完成）
	if a.showPasswordDialog && a.passwordDialog != nil {
		var dialogView string
		dialogView += a.passwordDialog.View()
		return dialogView
	}

	// 新建连接对话框优先显示（即使未初始化完成）
	if a.showNewConnectionDialog && a.newConnectionDialog != nil {
		var dialogView string
		dialogView += a.newConnectionDialog.View()
		return dialogView
	}

	// 新建分组对话框优先显示（即使未初始化完成）
	if a.showNewGroupDialog && a.newGroupDialog != nil {
		var dialogView string
		dialogView += a.newGroupDialog.View()
		return dialogView
	}

	// 编辑连接对话框优先显示（即使未初始化完成）
	if a.showEditDialog && a.editDialog != nil {
		var dialogView string
		dialogView += a.editDialog.View()
		return dialogView
	}

	if !a.ready {
		return "正在初始化..."
	}

	if a.quitting {
		return "再见！\n"
	}

	// 1. 头部（信息区 + 快捷键区）
	header := a.renderHeader()

	// 2. 命令/搜索输入框（条件显示）
	var cmdInput string
	if a.commandMode || a.searchMode || a.groupSearchMode || a.searchBoxVisible || a.groupSearchBoxVisible {
		cmdInput = a.renderCommandInput()
	}

	// 4. 状态栏（先渲染，以便计算剩余高度给主内容区）
	statusBar := a.renderStatusBar()

	// 计算主内容区可用高度，使主机列表边框撑满中间区域
	contentHeight := a.height - lipgloss.Height(header) - lipgloss.Height(statusBar)
	if cmdInput != "" {
		contentHeight -= lipgloss.Height(cmdInput)
	}
	if contentHeight < 3 {
		contentHeight = 3
	}

	// 动态计算每页显示条数，使主机列表刚好铺满可用区域
	// innerHeight = contentHeight - 2（列表边框上下各1行）
	// 固定行数：表头1 + 分隔线1 = 2
	innerHeight := contentHeight - 2
	fixedLines := 2 // 表头 + 分隔线
	newPageSize := innerHeight - fixedLines
	if newPageSize < 1 {
		newPageSize = 1
	}
	// 如果需要分页，预留2行给分页指示器（空行+dots）
	totalHosts := len(a.filteredHosts)
	if totalHosts > 0 && totalHosts > newPageSize {
		newPageSize = innerHeight - fixedLines - 2
		if newPageSize < 1 {
			newPageSize = 1
		}
	}
	if newPageSize != a.pageSize {
		a.pageSize = newPageSize
		a.paginator.PerPage = newPageSize
		a.updatePaginator()
		// 同步当前选中项所在页
		if a.pageSize > 0 {
			a.paginator.Page = a.selected / a.pageSize
		}
		// 重新渲染状态栏以反映新的分页信息
		statusBar = a.renderStatusBar()
	}

	// 3. 主内容区（传入可用高度）
	var content string
	if a.groupSelectMode {
		content = a.renderGroupSelectWithHeight(contentHeight)
	} else {
		content = a.renderHostListWithHeight(contentHeight)
	}

	var sections []string
	sections = append(sections, header)
	if cmdInput != "" {
		sections = append(sections, cmdInput)
	}
	sections = append(sections, content)
	sections = append(sections, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader 渲染头部（信息区 + 快捷键区）
func (a *App) renderHeader() string {
	// totalWidth = a.width，两个边框盒子并排总宽度 = a.width
	// 每个盒子: Width(X) 设置内容宽度 X，渲染宽度 = X + 2（左右边框各1）
	// info渲染宽度 = infoInner + 2, shortcut渲染宽度 = shortcutInner + 2
	// 总渲染宽度 = infoInner + shortcutInner + 4 = a.width
	// 所以 infoInner + shortcutInner = a.width - 4
	totalInner := a.width - 4
	if totalInner < 20 {
		totalInner = 20
	}
	infoInner := totalInner * 40 / 100
	shortcutInner := totalInner - infoInner

	// 信息区内容
	groupName := a.currentGroup
	if groupName == "" {
		groupName = "未分组"
	}
	infoLines := []string{
		fmt.Sprintf(" Trelay v%s", a.version),
		fmt.Sprintf(" 分组: %s", groupName),
		fmt.Sprintf(" 主机: %d", len(a.filteredHosts)),
	}
	infoContent := strings.Join(infoLines, "\n")

	// 快捷键区内容（自适应列数，<> 包围键名）
	type shortcut struct {
		key  string
		desc string
	}
	shortcuts := []shortcut{
		{"↑↓", "选择"},
		{"Enter", "连接"},
		{"←→", "翻页"},
		{"/", "搜索"},
		{":", "命令"},
		{"N", "新建"},
		{"E", "编辑"},
		{"B", "后台会话"},
		{"R", "刷新"},
		{"G", "新建分组"},
		{"D", "删除"},
		{"Ctrl+B", "挂起连接"},
		{"Q", "退出"},
	}

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// 计算每个条目的渲染宽度，找出最宽的作为列宽基准
	maxEntryW := 0
	for _, s := range shortcuts {
		plain := fmt.Sprintf(" <%s> %s", s.key, s.desc)
		w := displayWidth(plain)
		if w > maxEntryW {
			maxEntryW = w
		}
	}
	colW := maxEntryW + 2 // 列间距
	if colW < 14 {
		colW = 14
	}

	// 根据可用宽度自动计算列数
	availW := shortcutInner
	numCols := availW / colW
	if numCols < 2 {
		numCols = 2
	}
	// 重新平均分配列宽
	colW = availW / numCols

	// 计算行数
	numRows := (len(shortcuts) + numCols - 1) / numCols

	var shortcutLines []string
	for r := 0; r < numRows; r++ {
		var line string
		for c := 0; c < numCols; c++ {
			idx := c*numRows + r
			if idx < len(shortcuts) {
				s := shortcuts[idx]
				entry := fmt.Sprintf(" %s %s", keyStyle.Render("<"+s.key+">"), descStyle.Render(s.desc))
				entryW := displayWidth(stripANSI(entry))
				pad := colW - entryW
				if pad < 0 {
					pad = 0
				}
				line += entry + strings.Repeat(" ", pad)
			}
		}
		shortcutLines = append(shortcutLines, line)
	}
	shortcutContent := strings.Join(shortcutLines, "\n")

	infoStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Foreground(lipgloss.Color("#00ff00")).
		Width(infoInner).
		Height(numRows)

	shortcutStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Width(shortcutInner).
		Height(numRows)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		infoStyle.Render(infoContent),
		shortcutStyle.Render(shortcutContent),
	)
}

// renderCommandInput 渲染命令/搜索输入框
func (a *App) renderCommandInput() string {
	prefix := ""
	if a.commandMode && !a.groupSelectMode {
		prefix = ":"
	} else if a.searchMode || a.groupSearchMode || a.searchBoxVisible || a.groupSearchBoxVisible {
		prefix = "/"
	}

	prefixStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Width(a.width - 4)

	return inputStyle.Render(prefixStyle.Render(prefix) + a.commandInput.View())
}

// renderGroupSelectWithHeight 渲染分组选择列表（指定高度撑满）
func (a *App) renderGroupSelectWithHeight(height int) string {
	if len(a.filteredGroupList) == 0 {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Width(a.width - 8).
			Align(lipgloss.Center)

		message := "无匹配分组"

		innerHeight := height - 2
		if innerHeight < 1 {
			innerHeight = 1
		}
		listStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00aa00")).
			Width(a.width - 2).
			Height(innerHeight)
		return listStyle.Render(style.Render(message))
	}

	// 列宽计算：选中(2) + 分组名称(60%) + 主机数量(剩余)
	tableWidth := a.width - 6 // 边框2 + 左右padding各1
	colSpacing := 2
	selectW := 2
	nameW := (tableWidth - selectW - colSpacing*2) * 60 / 100
	countW := tableWidth - selectW - nameW - colSpacing*2

	// 表头
	headers := []string{" ", "分组名称", "主机数量"}
	widths := []int{selectW, nameW, countW}

	var headerBuilder strings.Builder
	for i, header := range headers {
		headerBuilder.WriteString(header)
		padding := widths[i] - displayWidth(header)
		if padding > 0 {
			headerBuilder.WriteString(strings.Repeat(" ", padding))
		}
		if i < len(headers)-1 {
			headerBuilder.WriteString(strings.Repeat(" ", colSpacing))
		}
	}
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true).
		Background(lipgloss.Color("#003300"))

	var list string
	list += headerStyle.Render(headerBuilder.String()) + "\n"

	// 分隔线
	sepWidth := selectW + nameW + countW + colSpacing*2
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00aa00"))
	list += separatorStyle.Render(strings.Repeat("─", sepWidth)) + "\n"

	// 数据行
	for i, group := range a.filteredGroupList {
		selected := i == a.groupSelectCursor

		indicator := " "
		if selected {
			indicator = "●"
		}

		hostCount := 0
		if hosts, ok := a.grouped[group]; ok {
			hostCount = len(hosts)
		}
		countText := fmt.Sprintf("%d", hostCount)

		// 截断分组名
		groupName := truncateByDisplayWidth(group, nameW)

		var rowBuilder strings.Builder
		// 选中标记
		rowBuilder.WriteString(indicator)
		if p := selectW - displayWidth(indicator); p > 0 {
			rowBuilder.WriteString(strings.Repeat(" ", p))
		}
		rowBuilder.WriteString(strings.Repeat(" ", colSpacing))
		// 分组名称
		rowBuilder.WriteString(groupName)
		if p := nameW - displayWidth(groupName); p > 0 {
			rowBuilder.WriteString(strings.Repeat(" ", p))
		}
		rowBuilder.WriteString(strings.Repeat(" ", colSpacing))
		// 主机数量
		rowBuilder.WriteString(countText)
		if p := countW - displayWidth(countText); p > 0 {
			rowBuilder.WriteString(strings.Repeat(" ", p))
		}

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
		list += style.Render(rowBuilder.String()) + "\n"
	}

	// 边框包裹，Height 撑满
	innerHeight := height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Width(a.width - 2).
		Height(innerHeight)

	return listStyle.Render(list)
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

// unescapeDescription 将描述中的转义字符还原为原始字符
// 处理常见的转义序列：\s -> 空格, \t -> 制表符, \n -> 换行, \\ -> 反斜杠
func unescapeDescription(s string) string {
	if s == "" {
		return s
	}

	// 使用 strings.NewReplacer 进行批量替换
	// 顺序很重要：先替换双反斜杠，再替换其他
	replacer := strings.NewReplacer(
		"\\n", "\n", // 换行
		"\\t", "\t", // 制表符
		"\\s", " ", // 空格（放在最后避免与其他冲突）
		"\\\\", "\\", // 反斜杠
	)

	return replacer.Replace(s)
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
	// 检查是否显示描述列
	showDescription := a.shouldShowDescriptionColumn()

	// 列定义：选中(2)、协议(4)、名称(30)、IP(15)、用户名(10)、分组(20)、状态(8)、描述(20)
	// 列定义：选中(2)、协议(4)、名称(20)、IP(15)、用户名(10)、分组(20)、状态(8)、描述(25)
	// 如果显示描述列，总宽度：2+4+20+15+10+20+8+25 = 104字符
	// 加上列之间的空格：每列之间1个空格，7个分隔符 = 7字符，总共111字符

	minWidths := []int{2, 4, 20, 15, 10, 20, 10}
	if showDescription {
		minWidths = append(minWidths, 25) // 描述列宽度
	}
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
		// 使用最小宽度，多余空间加到描述列或名称列
		copy(widths, minWidths)
		extraWidth := availableWidth - minTotalWidth
		if showDescription {
			widths[len(widths)-1] += extraWidth // 描述列获得额外空间
		} else {
			widths[2] += extraWidth // 无描述列时，名称列获得额外空间
		}
	}

	return widths, colSpacing
}

// shouldShowDescriptionColumn 检查是否应该显示描述列
// 当所有主机的描述长度都小于5个字符时，不显示该列
func (a *App) shouldShowDescriptionColumn() bool {
	// 如果没有主机，默认不显示
	if len(a.filteredHosts) == 0 {
		return false
	}

	// 检查是否所有主机都没有描述或描述长度都小于5
	allShortOrEmpty := true
	for _, host := range a.filteredHosts {
		descLen := displayWidth(host.Description)
		if descLen >= 5 {
			allShortOrEmpty = false
			break
		}
	}

	// 如果所有描述都小于5个字符，不显示该列
	return !allShortOrEmpty
}

// renderTableHeader 渲染表格表头
func (a *App) renderTableHeader() string {
	widths, colSpacing := a.getColumnWidths()

	// 检查是否显示描述列
	showDescription := a.shouldShowDescriptionColumn()

	// 表头文本
	headers := []string{" ", "协议", "名称", "IP地址", "用户名", "分组", "状态"}
	if showDescription {
		headers = append(headers, "描述")
	}

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

// renderHostListWithHeight 渲染主机列表（带分页，指定高度撑满）
func (a *App) renderHostListWithHeight(height int) string {
	if len(a.filteredHosts) == 0 {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Width(a.width - 8).
			Align(lipgloss.Center)

		message := "没有可用的主机配置"
		if a.searchQuery != "" {
			message = fmt.Sprintf("没有找到匹配 '%s' 的主机", a.searchQuery)
		}

		innerHeight := height - 2
		if innerHeight < 1 {
			innerHeight = 1
		}
		listStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00aa00")).
			Width(a.width - 2).
			Height(innerHeight)
		return listStyle.Render(style.Render(message))
	}

	var list string
	// 添加表头
	list += a.renderTableHeader() + "\n"

	// 添加分隔线
	widths, colSpacing := a.getColumnWidths()
	tableWidth := 0
	for _, w := range widths {
		tableWidth += w
	}
	tableWidth += colSpacing * (len(widths) - 1)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00aa00"))
	list += separatorStyle.Render(strings.Repeat("─", tableWidth)) + "\n"

	// 通过 paginator 获取当前页数据
	start, end := a.paginator.GetSliceBounds(len(a.filteredHosts))
	pageHosts := a.filteredHosts[start:end]

	for i, host := range pageHosts {
		globalIndex := start + i
		item := a.renderHostItem(host, globalIndex == a.selected)
		list += item + "\n"
	}

	// 用边框包裹列表，Height 撑满剩余空间（减去边框上下各1行）
	innerHeight := height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	// 计算已用行数，插入空行将分页指示器推到列表底部
	// list 末尾已有 "\n"，所以 gap 个 "\n" 会产生 gap+1 个空行，需要减1修正
	contentLines := 2 + len(pageHosts) // 表头1 + 分隔线1 + 主机行数
	paginatorLines := 0
	if a.paginator.TotalPages > 1 {
		paginatorLines = 2 // 空行1 + dots 1
	}
	gap := innerHeight - contentLines - paginatorLines - 1 // -1 修正尾部 \n
	if gap > 0 {
		list += strings.Repeat("\n", gap)
	}

	// 分页指示器固定在列表最底部（仅当有多页时显示）
	if a.paginator.TotalPages > 1 {
		paginatorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00aa00")).
			Width(a.width - 8).
			Align(lipgloss.Center)
		list += "\n" + paginatorStyle.Render(a.paginator.View())
	}

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Width(a.width - 2).
		Height(innerHeight)

	return listStyle.Render(list)
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

	// 检查是否显示描述列
	showDescription := a.shouldShowDescriptionColumn()

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

	// 如果显示描述列，添加描述内容
	if showDescription {
		// 处理描述中的转义字符
		unescapedDesc := unescapeDescription(host.Description)
		columns = append(columns, unescapedDesc)
	}

	for i, column := range columns {
		width := widths[i]
		displayText := column

		// 应用最大长度限制（基于显示宽度）
		if i == 2 { // 名称列，限制为20字符
			displayText = truncateByDisplayWidth(displayText, 20)
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
	// 状态列固定为 index 6（选中、协议、名称、IP、用户名、分组、状态）
	statusColIndex := 6
	statusStartPos := 0
	for i := 0; i < statusColIndex; i++ {
		statusStartPos += widths[i]
		statusStartPos += colSpacing
	}
	// 状态列的宽度是widths[statusColIndex]
	statusEndPos := statusStartPos + widths[statusColIndex]

	if statusStartPos < len(rowText) && statusEndPos <= len(rowText) {
		beforeStatus := rowText[:statusStartPos]
		statusCol := rowText[statusStartPos:statusEndPos]
		afterStatus := rowText[statusEndPos:]

		// 状态列样式
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(statusColor)).Bold(true)
		if selected {
			statusStyle = statusStyle.Background(lipgloss.Color("#00ff00"))
			// 选中行时，绿色状态(online/connected)改用深绿色避免绿底绿字不可见
			if statusColor == "#00ff00" {
				statusStyle = statusStyle.Foreground(lipgloss.Color("#004400"))
			}
		}

		// 分段渲染避免 ANSI 重置码破坏后续文本样式
		return style.Render(beforeStatus) + statusStyle.Render(statusCol) + style.Render(afterStatus)
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

// renderStatusBar 渲染状态栏（三栏布局 + 边框）
func (a *App) renderStatusBar() string {
	totalHosts := len(a.config.Profiles)
	totalGroups := len(a.config.Groups)
	currentPage := a.paginator.Page + 1
	totalPages := a.paginator.TotalPages
	if totalPages == 0 {
		totalPages = 1
		currentPage = 1
	}

	totalWidth := a.width - 4 // 边框占用
	hostWidth := totalWidth * 25 / 100
	groupWidth := totalWidth * 25 / 100
	pageWidth := totalWidth - hostWidth - groupWidth

	hostText := fmt.Sprintf("主机: %d", totalHosts)
	groupText := fmt.Sprintf("分组: %d", totalGroups)
	pageText := fmt.Sprintf("页: %d/%d (每页%d条)", currentPage, totalPages, a.pageSize)

	bgCount := a.connManager.GetBackgroundCount()
	if bgCount > 0 {
		pageText += fmt.Sprintf("  后台: %d", bgCount)
	}

	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	hostCol := lipgloss.NewStyle().Width(hostWidth).Align(lipgloss.Left).Render(textStyle.Render(hostText))
	groupCol := lipgloss.NewStyle().Width(groupWidth).Align(lipgloss.Left).Render(textStyle.Render(groupText))
	pageCol := lipgloss.NewStyle().Width(pageWidth).Align(lipgloss.Right).Render(textStyle.Render(pageText))

	inner := lipgloss.JoinHorizontal(lipgloss.Top, hostCol, groupCol, pageCol)

	barStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Width(a.width - 2)

	return barStyle.Render(inner)
}

// renderConnectingView 渲染连接中的 loading 视图
func (a *App) renderConnectingView() string {
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00ff00")).
		Padding(1, 2).
		Background(lipgloss.Color("#001a00"))

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00cc00"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("正在连接"))
	content.WriteString("\n\n")
	content.WriteString(a.spinner.View())
	content.WriteString(contentStyle.Render(" " + a.connectingHost))
	content.WriteString("\n\n")
	content.WriteString(hintStyle.Render("请等待... (按 ESC 终止)"))

	dialogContent := dialogStyle.Render(content.String())

	if a.width > 0 && a.height > 0 {
		return lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			dialogContent,
		)
	}
	return dialogContent
}

// renderSessionList 渲染后台会话列表弹窗
func (a *App) renderSessionList() string {
	bgSessions := a.connManager.GetBackgroundSessions()

	// 弹窗样式
	dialogWidth := 60
	if a.width > 0 && a.width < dialogWidth+4 {
		dialogWidth = a.width - 4
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00aa00")).
		Padding(1, 2).
		Width(dialogWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#00ff00")).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	var lines []string
	lines = append(lines, titleStyle.Render("后台会话列表"))
	lines = append(lines, "")

	if len(bgSessions) == 0 {
		lines = append(lines, itemStyle.Render("  没有后台会话"))
	} else {
		for i, session := range bgSessions {
			duration := session.GetDuration()
			durationStr := formatDuration(duration)
			line := fmt.Sprintf("  %s  (%s)", session.GetHostID(), durationStr)

			if i == a.sessionListCursor {
				lines = append(lines, selectedStyle.Render(line))
			} else {
				lines = append(lines, itemStyle.Render(line))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Enter:切回 | D:断开 | Esc:关闭"))

	content := strings.Join(lines, "\n")
	dialog := borderStyle.Render(content)

	// 居中显示
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, dialog)
}

// formatDuration 格式化持续时间
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
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
		tea.WithAltScreen(),
	)

	// 运行程序
	if _, err := p.Run(); err != nil {
		return err
	}

	// TUI 退出时，清理所有后台会话
	app.connManager.DisconnectAll()

	return nil
}

// RunWithMessage 带错误信息启动 TUI
func RunWithMessage(logger *zap.Logger, errorMessage string) error {
	GlobalErrorMessage = errorMessage
	return Run(logger)
}
