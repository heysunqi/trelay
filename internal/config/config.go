package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"remote-desktop-manager/pkg/models"
)

// Manager 配置管理器接口
type Manager interface {
	Load() (*models.Config, error)
	Save(config *models.Config) error
	GetConfigPath() string
	SetConfigPath(path string)
}

// ConfigManager 配置管理器实现
type ConfigManager struct {
	configPath string
	logger     *zap.Logger
}

// NewConfigManager 创建新的配置管理器
func NewConfigManager(logger *zap.Logger) *ConfigManager {
	return &ConfigManager{
		logger: logger,
	}
}

// Load 加载配置文件
func (cm *ConfigManager) Load() (*models.Config, error) {
	configPath := cm.GetConfigPath()

	viper.SetConfigFile(configPath)
	viper.SetConfigType("json")

	// 设置默认值
	viper.SetDefault("version", "1.0")
	viper.SetDefault("ui.theme", "retro-green")

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		// 如果文件不存在，创建默认配置
		if os.IsNotExist(err) {
			cm.logger.Info("配置文件不存在，创建默认配置", zap.String("path", configPath))
			defaultConfig := models.DefaultConfig()
			if err := cm.Save(defaultConfig); err != nil {
				return nil, err
			}
			return defaultConfig, nil
		}
		return nil, err
	}

	// 解析配置
	var config models.Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 只在调试模式下打印配置加载信息，避免影响生产环境的日志输出
	if cm.logger.Core().Enabled(zapcore.DebugLevel) {
		cm.logger.Debug("配置文件加载成功",
			zap.String("path", configPath),
			zap.Int("profiles", len(config.Profiles)),
			zap.Int("groups", len(config.Groups)))
	}

	return &config, nil
}

// Save 保存配置文件
func (cm *ConfigManager) Save(config *models.Config) error {
	// 验证配置
	if err := config.Validate(); err != nil {
		return err
	}

	configPath := cm.GetConfigPath()

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 设置配置到viper
	viper.SetConfigFile(configPath)
	viper.SetConfigType("json")

	// 将结构体转换为map以便viper设置
	// 这里使用简单的序列化方式
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	cm.logger.Info("配置文件保存成功",
		zap.String("path", configPath),
		zap.Int("profiles", len(config.Profiles)))

	return nil
}

// GetConfigPath 获取配置文件路径
func (cm *ConfigManager) GetConfigPath() string {
	if cm.configPath != "" {
		return cm.configPath
	}

	// 默认配置文件路径
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果无法获取用户目录，使用当前目录
		return "remote-desktop-manager.json"
	}

	return filepath.Join(homeDir, ".config", "remote-desktop-manager", "config.json")
}

// SetConfigPath 设置配置文件路径
func (cm *ConfigManager) SetConfigPath(path string) {
	cm.configPath = path
}
