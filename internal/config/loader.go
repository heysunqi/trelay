package config

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"remote-desktop-manager/pkg/models"
)

// LoadConfig 从默认路径加载配置
func LoadConfig(logger *zap.Logger) (*models.Config, error) {
	manager := NewConfigManager(logger)
	return manager.Load()
}

// LoadConfigFromPath 从指定路径加载配置
func LoadConfigFromPath(path string, logger *zap.Logger) (*models.Config, error) {
	manager := NewConfigManager(logger)
	manager.SetConfigPath(path)
	return manager.Load()
}

// SaveConfig 保存配置到默认路径
func SaveConfig(config *models.Config, logger *zap.Logger) error {
	manager := NewConfigManager(logger)
	return manager.Save(config)
}

// SaveConfigToPath 保存配置到指定路径
func SaveConfigToPath(config *models.Config, path string, logger *zap.Logger) error {
	manager := NewConfigManager(logger)
	manager.SetConfigPath(path)
	return manager.Save(config)
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "remote-desktop-manager", "config.json"), nil
}

// CreateDefaultConfig 创建默认配置文件
func CreateDefaultConfig(logger *zap.Logger) error {
	config := models.DefaultConfig()
	manager := NewConfigManager(logger)
	return manager.Save(config)
}

// ConfigExists 检查配置文件是否存在
func ConfigExists() bool {
	path, err := GetDefaultConfigPath()
	if err != nil {
		return false
	}

	_, err = os.Stat(path)
	return err == nil
}
