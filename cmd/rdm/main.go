package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"remote-desktop-manager/internal/config"
	"remote-desktop-manager/internal/protocol/ssh"
	"remote-desktop-manager/internal/ui/tui"
	"remote-desktop-manager/pkg/models"
)

var (
	// 命令行参数
	configPath  string
	debugMode   bool
	directSSH   string // 直接SSH连接的主机名
	returnToRDM bool   // SSH连接结束后是否返回RDM界面

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
	rootCmd.PersistentFlags().BoolVarP(&returnToRDM, "return-to-rdm", "", false, "SSH连接结束后返回RDM界面")
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

// runRoot 运行根命令
func runRoot(cmd *cobra.Command, args []string) {
	// 创建配置管理器（先用临时logger加载配置）
	tempLogger, err := zap.NewDevelopment() // 使用临时logger
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

	// 处理直接SSH连接
	if directSSH != "" {
		logger.Info("直接SSH连接模式", zap.String("host", directSSH))

		// 查找主机配置
		var targetHost *models.Host
		for _, host := range cfg.Profiles {
			if host.Name == directSSH {
				targetHost = host
				break
			}
		}

		if targetHost == nil {
			logger.Error("主机配置未找到", zap.String("host", directSSH))
			fmt.Fprintf(os.Stderr, "主机配置未找到: %s\n", directSSH)
			os.Exit(1)
		}

		if targetHost.Protocol != "ssh" {
			logger.Error("主机协议不支持SSH", zap.String("host", directSSH), zap.String("protocol", targetHost.Protocol))
			fmt.Fprintf(os.Stderr, "主机 %s 的协议不支持SSH: %s\n", directSSH, targetHost.Protocol)
			os.Exit(1)
		}

		// 连接SSH
		client := ssh.NewClient(targetHost)

		if err := client.Connect(); err != nil {
			logger.Error("SSH连接失败", zap.String("host", directSSH), zap.Error(err))
			fmt.Fprintf(os.Stderr, "SSH连接失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("已连接到 %s\n", directSSH)
		logger.Debug("SSH连接成功", zap.String("host", directSSH))

		// 启动交互式会话
		if err := client.StartInteractiveSession(); err != nil {
			logger.Error("SSH会话错误", zap.String("host", directSSH), zap.Error(err))
		}

		client.Disconnect()
		fmt.Printf("\n已断开与 %s 的连接\n", directSSH)

		// 如果需要返回RDM界面，重新启动程序
		if returnToRDM {
			fmt.Println("\n正在返回RDM界面...")
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
				_ = syscall.Exec(execPath, args, os.Environ())
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
}

// main 程序入口
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
