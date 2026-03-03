package tui

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"trelay/internal/config"
	"trelay/internal/protocol"
	"trelay/internal/ui/dialogs"
	"trelay/pkg/models"

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
	searchQuery  string
	searchMode   bool
	searchCursor int // 搜索框光标位置

	// 状态刷新相关字段
	lastStatusCheck time.Time

	// 连接相关字段
	connManager *protocol.Manager
	connecting  bool // 是否正在连接，防止重复触发

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
	showNewGroupDialog bool                     // 是否显示新建分组对话框
	newGroupDialog     *dialogs.NewGroupDialog // 新建分组对话框实例

	// 编辑连接对话框相关字段
	showEditDialog bool                        // 是否显示编辑连接对话框
	editDialog     *dialogs.EditConnectionDialog // 编辑连接对话框实例
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
		searchCursor: 0,
	}

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
	case "ssh":
		args = append(args, "--direct-ssh", host.Name)
		// 如果主机配置了密码，则传递密码参数
		if host.Password != "" {
			args = append(args, "--password", host.Password)
		}
	case "rdp":
		args = append(args, "--direct-rdp", host.Name)
	default:
		a.logger.Error("不支持的协议", zap.String("protocol", host.Protocol))
		fmt.Printf("不支持的协议: %s\n", host.Protocol)
		return
	}

	args = append(args, "--return-to-trelay")

	// 使用 syscall.Exec 直接替换当前进程运行直接连接
	// 这样可以完全控制终端，避免与Bubble Tea事件循环的冲突
	a.quitting = true
	err = syscall.Exec(execPath, args, os.Environ())

	// 如果 syscall.Exec 返回，说明执行失败
	if err != nil {
		a.logger.Error("启动直接连接失败", zap.Error(err))
		fmt.Printf("启动直接连接失败: %v\n", err)
	}
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
		tea.WindowSize(),          // 获取终端尺寸命令
		a.checkHostStatusAsync(),  // 异步状态检查
		a.statusCheckCmd(),        // 定时状态检查
	)
}

// Update 处理消息和更新状态
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				// 执行连接
				a.executeConnection(host)
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

	case tea.KeyMsg:
		// 搜索模式下的输入处理优先
		if a.searchMode {
			switch msg.Type {
			case tea.KeyEsc:
				// 退出搜索模式
				a.toggleSearchMode()
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

		// 非搜索模式下的键盘事件处理
		switch msg.String() {
		case "q", "ctrl+c":
			// 退出程序
			a.quitting = true
			return a, tea.Quit

		case "/":
			// 进入搜索模式
			a.toggleSearchMode()
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
			// 直接连接到选中的主机
			if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts) && !a.connecting {
				host := a.filteredHosts[a.selected]
				a.connecting = true // 设置连接标志，防止重复触发

				// 检查是否是SSH协议且没有配置密码或密钥
				if host.Protocol == "ssh" && host.Password == "" && host.KeyPath == "" {
					// 显示密码输入对话框
					a.showPasswordDialog = true
					a.passwordDialog = dialogs.NewPasswordDialog(host, a.width, a.height)
				} else {
					// 直接执行连接
					a.executeConnection(host)
				}
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

		case "N", "n":
			// 显示新建连接对话框
			a.showNewConnectionDialog = true
			var groupNames []string
			for _, group := range a.config.Groups {
				groupNames = append(groupNames, group.Name)
			}
			a.newConnectionDialog = dialogs.NewNewConnectionDialog(groupNames, a.width, a.height)
			return a, nil

		case "G", "g":
			// 显示新建分组对话框
			a.showNewGroupDialog = true
			a.newGroupDialog = dialogs.NewNewGroupDialog(a.width, a.height)
			return a, nil

		case "E", "e":
			// 显示编辑连接对话框
			if len(a.filteredHosts) > 0 && a.selected < len(a.filteredHosts) {
				host := a.filteredHosts[a.selected]
				// 查找主机所属分组
				hostGroup := a.findHostGroup(host.Name)
				// 获取所有分组名称
				var groupNames []string
				for _, group := range a.config.Groups {
					groupNames = append(groupNames, group.Name)
				}
				a.showEditDialog = true
				a.editDialog = dialogs.NewEditConnectionDialog(host, groupNames, hostGroup, a.width, a.height)
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
	// 错误对话框优先显示（即使未初始化完成）
	if a.showErrorDialog && a.errorDialog != nil {
		var dialogView string
		dialogView += a.errorDialog.View()
		return dialogView
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

	var content string

	// 标题
	title := a.renderTitle()
	content += title + "\n\n"

	// 计算底部固定内容的高度
	bottomContentHeight := 3 // 状态栏 + 帮助信息 + 分隔行

	// 计算主机列表可用高度
	titleHeight := 1 + strings.Count(title, "\n") // 标题行数（包含换行）
	availableHeight := a.height - titleHeight - bottomContentHeight

	// 渲染主机列表（带高度限制）
	hostList := a.renderHostListWithHeight(availableHeight)
	content += hostList + "\n"

	// 填充剩余空间（如果需要）
	renderedHostHeight := 1 + strings.Count(hostList, "\n") // 主机列表行数
	if renderedHostHeight < availableHeight {
		content += strings.Repeat("\n", availableHeight-renderedHostHeight)
	}

	// 状态栏（固定在底部）
	statusBar := a.renderStatusBar()
	content += statusBar

	// 帮助信息（固定在底部）
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

	minWidths := []int{2, 4, 30, 15, 10, 20, 10}
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
	totalHosts := len(a.filteredHosts)
	selectedIndex := a.selected + 1
	status := fmt.Sprintf("%d/%d hosts | [↑/↓] 选择 | [Enter] 连接", selectedIndex, totalHosts)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00aa00")).
		Width(a.width - 4).
		Align(lipgloss.Center)

	return statusStyle.Render(status)
}

// renderHostListWithHeight 渲染带高度限制的主机列表
func (a *App) renderHostListWithHeight(maxHeight int) string {
	fullList := a.renderHostList()
	lines := strings.Split(fullList, "\n")

	// 如果列表高度不超过最大高度，直接返回
	if len(lines) <= maxHeight {
		return fullList
	}

	// 计算可显示的主机项目数
	// 标题 + 分隔线 + 主机项目数 + 最后一行空行
	headerLines := 2 // 标题 + 分隔线
	if maxHeight <= headerLines {
		return strings.Join(lines[:maxHeight], "\n")
	}

	maxItems := maxHeight - headerLines
	return strings.Join(lines[:headerLines+maxItems], "\n")
}

// renderHelp 渲染帮助信息
func (a *App) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true).
		Width(a.width - 4).
		Align(lipgloss.Center)
	helpText := "键盘: ↑↓ 选择 | Enter 连接 | Tab 分组 | / 搜索 | R 刷新 | N 新建 | E 编辑 | G 新建分组 | Q 退出"

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
	// 不使用备用屏幕，避免退出时终端状态混乱
	p := tea.NewProgram(
		app,
	)

	// 运行程序
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// RunWithMessage 带错误信息启动 TUI
func RunWithMessage(logger *zap.Logger, errorMessage string) error {
	GlobalErrorMessage = errorMessage
	return Run(logger)
}
