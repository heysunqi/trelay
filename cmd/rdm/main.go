package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"remote-desktop-manager/internal/config"
	"remote-desktop-manager/internal/ui/tui"
)

var (
	// 命令行参数
	configPath string
	debugMode  bool

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