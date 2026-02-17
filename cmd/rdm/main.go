package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
func initLogger() error {
	var err error
	if debugMode {
		logger, err = zap.NewDevelopment()
	} else {
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
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
	// 初始化日志器
	if err := initLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 如果有指定配置文件路径，设置全局配置路径
	if configPath != "" {
		// 检查配置文件是否存在
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			logger.Warn("配置文件不存在，将创建默认配置", zap.String("path", configPath))
			// 创建默认配置
			mgr := config.NewConfigManager(logger)
			mgr.SetConfigPath(configPath)
			if _, err := mgr.Load(); err != nil {
				logger.Error("创建默认配置失败", zap.Error(err), zap.String("path", configPath))
				os.Exit(1)
			}
		}
	}

	// 显示欢迎信息
	logger.Info("启动远程桌面管理器",
		zap.Bool("debug", debugMode),
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