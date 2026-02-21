package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"remote-desktop-manager/internal/config"
	"remote-desktop-manager/internal/protocol/rdp"
	"remote-desktop-manager/internal/protocol/ssh"
	"remote-desktop-manager/internal/ui/tui"
	"remote-desktop-manager/pkg/models"
)

var (
	// 命令行参数
	configPath  string
	debugMode   bool
	directSSH   string // 直接SSH连接的主机名
	directRDP   string // 直接RDP连接的主机名
	returnToRDM bool   // 连接结束后是否返回RDM界面
	password    string // SSH密码参数

	// 全局日志器
	logger *zap.Logger
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "rdm",
	Short: "远程桌面管理器 - 复古命令行界面",
	Long: `远程桌面管理器 (RDM)

一个复古命令行界面的远程桌面管理工具。
支持 SSH、RDP、VNC 协议，使用 JSON 配置文件管理主机。`,
	Run: runRoot,
}

// init 初始化命令行参数
func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "配置文件路径")
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "启用调试模式")
	rootCmd.PersistentFlags().StringVarP(&directSSH, "direct-ssh", "", "", "直接连接到指定名称的SSH主机（不启动TUI）")
	rootCmd.PersistentFlags().StringVarP(&directRDP, "direct-rdp", "", "", "直接连接到指定名称的RDP主机（不启动TUI）")
	rootCmd.PersistentFlags().BoolVarP(&returnToRDM, "return-to-rdm", "", false, "连接结束后返回RDM界面")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "SSH连接密码（不推荐在命令行中使用，建议在TUI中输入）")
}

// initLogger 初始化日志器
func initLogger(logLevel string) error {
	var err error

	// 转换日志级别字符串为zapcore.Level
	var level zapcore.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn", "warning":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		// 默认使用error级别
		level = zapcore.ErrorLevel
	}

	// 如果启用调试模式，使用Development配置
	if debugMode {
		logger, err = zap.NewDevelopment()
	} else {
		// 使用Production配置，并设置指定的日志级别
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(level)
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
		logger, err = config.Build()
	}

	if err != nil {
		return fmt.Errorf("初始化日志器失败: %w", err)
	}

	return nil
}

// runDirectConnection 执行直接连接（SSH或RDP）
func runDirectConnection(host *models.Host, protocolType string) error {
	switch protocolType {
	case "ssh":
		// 如果命令行参数中提供了密码，则使用该密码
		if password != "" {
			host.Password = password
		}

		client := ssh.NewClient(host)

		if err := client.Connect(); err != nil {
			return fmt.Errorf("SSH连接失败: %w", err)
		}

		fmt.Printf("已连接到 %s\n", host.Name)
		logger.Debug("SSH连接成功", zap.String("host", host.Name))

		// 启动交互式会话
		if err := client.StartInteractiveSession(); err != nil {
			logger.Error("SSH会话错误", zap.String("host", host.Name), zap.Error(err))
		}

		client.Disconnect()
		fmt.Printf("\n已断开与 %s 的连接\n", host.Name)

	case "rdp":
		fmt.Printf("准备连接到 %s...\n", host.Name)
		logger.Info("开始RDP直接连接",
			zap.String("host", host.Name),
			zap.String("address", host.Host),
			zap.Int("port", host.Port))

		client := rdp.NewClient(host)

		if err := client.Connect(); err != nil {
			logger.Error("RDP连接失败", zap.Error(err))
			return err
		}

		toolName := client.GetToolName()
		fmt.Printf("正在使用 %s 连接到 %s...\n", toolName, host.Name)
		logger.Info("RDP连接启动成功",
			zap.String("host", host.Name),
			zap.String("tool", toolName))

		// 启动交互式会话
		logger.Info("进入RDP会话...")
		if err := client.StartInteractiveSession(); err != nil {
			logger.Error("RDP会话错误",
				zap.String("host", host.Name),
				zap.Error(err))
			fmt.Printf("\nRDP会话错误: %v\n", err)
		} else {
			logger.Info("RDP会话正常结束")
		}

		client.Disconnect()
		fmt.Printf("\n已断开与 %s 的连接\n", host.Name)

	default:
		return fmt.Errorf("不支持的协议: %s", protocolType)
	}

	return nil
}

// runRoot 运行根命令
func runRoot(cmd *cobra.Command, args []string) {
	// 创建配置管理器（先用临时logger加载配置）
	// 使用生产模式的日志器，默认级别为Error，避免打印不必要的调试信息
	tempConfig := zap.NewProductionConfig()
	tempConfig.Level.SetLevel(zapcore.ErrorLevel)
	tempLogger, err := tempConfig.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建临时日志器失败: %v\n", err)
		os.Exit(1)
	}
	mgr := config.NewConfigManager(tempLogger)

	// 设置配置文件路径（如果指定了）
	if configPath != "" {
		mgr.SetConfigPath(configPath)
	}

	// 加载配置
	cfg, err := mgr.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志器（使用配置中的日志级别）
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "error" // 默认使用error级别
	}
	if err := initLogger(logLevel); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 重新创建配置管理器（使用实际的logger）
	mgr = config.NewConfigManager(logger)

	// 处理直接SSH/RDP连接
	if directSSH != "" || directRDP != "" {
		// 确定连接类型和主机名
		var directHost string
		var protocolType string

		if directSSH != "" {
			directHost = directSSH
			protocolType = "ssh"
		} else {
			directHost = directRDP
			protocolType = "rdp"
		}

		logger.Info(fmt.Sprintf("直接%s连接模式", strings.ToUpper(protocolType)), zap.String("host", directHost))

		// 查找主机配置
		var targetHost *models.Host
		for _, host := range cfg.Profiles {
			if host.Name == directHost {
				targetHost = host
				break
			}
		}

		if targetHost == nil {
			logger.Error("主机配置未找到", zap.String("host", directHost))
			fmt.Fprintf(os.Stderr, "主机配置未找到: %s\n", directHost)
			os.Exit(1)
		}

		// 验证协议
		if targetHost.Protocol != protocolType {
			logger.Error(fmt.Sprintf("主机协议不支持%s", strings.ToUpper(protocolType)),
				zap.String("host", directHost), zap.String("protocol", targetHost.Protocol))
			fmt.Fprintf(os.Stderr, "主机 %s 的协议不支持%s: %s\n", directHost, strings.ToUpper(protocolType), targetHost.Protocol)
			os.Exit(1)
		}

		// 执行直接连接
		if err := runDirectConnection(targetHost, protocolType); err != nil {
			logger.Error(fmt.Sprintf("%s连接失败", strings.ToUpper(protocolType)),
				zap.String("host", directHost), zap.Error(err))
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		// SSH会话结束后，恢复终端状态
		restoreTerminal()

		// 如果需要返回RDM界面，重新启动程序
		if returnToRDM {
			fmt.Println("\n正在返回RDM界面...")
			logger.Info("准备返回RDM界面")
			// 获取当前可执行文件路径
			execPath, err := os.Executable()
			if err == nil {
				args := []string{execPath}
				if configPath != "" {
					args = append(args, "--config", configPath)
				}
				if debugMode {
					args = append(args, "--debug")
				}
				// 使用 syscall.Exec 重新启动RDM程序
				logger.Info("执行syscall.Exec重新启动RDM", zap.Strings("args", args))
				err = syscall.Exec(execPath, args, os.Environ())
				if err != nil {
					logger.Error("syscall.Exec失败", zap.Error(err))
					fmt.Fprintf(os.Stderr, "返回RDM界面失败: %v\n", err)
				}
			} else {
				logger.Error("获取可执行文件路径失败", zap.Error(err))
				fmt.Fprintf(os.Stderr, "无法返回RDM界面: %v\n", err)
			}
		}

		os.Exit(0)
	}

	// 显示欢迎信息
	logger.Info("启动远程桌面管理器",
		zap.Bool("debug", debugMode),
		zap.String("log_level", logLevel),
		zap.String("config", configPath))

	// 运行TUI应用程序
	if err := tui.Run(logger); err != nil {
		logger.Error("应用程序运行失败", zap.Error(err))
		os.Exit(1)
	}

	// TUI退出后恢复终端状态，确保终端输出格式正常
	restoreTerminal()
}

// restoreTerminal 恢复终端状态到正常模式
// 使用 stty sane 命令重置终端设置（最可靠的方式）
func restoreTerminal() {
	// 使用 stty sane 命令恢复终端到正常状态
	// stty sane 会重置所有终端属性到合理的默认值
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // 忽略错误，有些终端可能不支持所有选项

	// 额外的ANSI恢复序列
	fmt.Fprint(os.Stderr, "\033[?1049l") // 退出备用屏幕
	fmt.Fprint(os.Stderr, "\033[?25h")   // 显示光标
	fmt.Fprint(os.Stderr, "\033[0m")     // 重置所有终端属性
	os.Stderr.Sync()
}

// main 程序入口
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
