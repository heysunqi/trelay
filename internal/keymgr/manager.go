package keymgr

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// KeysDirName 密钥目录名称（相对于 ~/.config）
	KeysDirName = ".config/trelay/keys"
	// KeyFileSuffix 密钥文件后缀
	KeyFileSuffix = "_key.pem"
)

// GetKeysDir 获取密钥目录路径，如果不存在则创建
func GetKeysDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败: %w", err)
	}

	keysDir := filepath.Join(homeDir, KeysDirName)

	// 检查目录是否存在
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		// 创建目录，权限 0700
		if err := os.MkdirAll(keysDir, 0700); err != nil {
			return "", fmt.Errorf("创建密钥目录失败: %w", err)
		}
	}

	return keysDir, nil
}

// GenerateKeyFilename 根据主机名生成唯一的文件名
// 格式: <hostname>_<6位随机字符串>_key.pem
func GenerateKeyFilename(hostname string) (string, error) {
	// 生成6位随机字符串
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("生成随机字符串失败: %w", err)
	}
	randomStr := hex.EncodeToString(randomBytes)

	// 清理主机名中的特殊字符
	safeHostname := strings.TrimSpace(hostname)
	safeHostname = strings.ReplaceAll(safeHostname, " ", "_")
	safeHostname = strings.ReplaceAll(safeHostname, "/", "_")
	safeHostname = strings.ReplaceAll(safeHostname, "\\", "_")

	if safeHostname == "" {
		safeHostname = "unknown"
	}

	return safeHostname + "_" + randomStr + KeyFileSuffix, nil
}

// SaveKey 保存密钥内容到密钥目录
// hostname: 主机名，用于生成文件名
// 返回保存的文件完整路径
func SaveKey(hostname, keyContent string) (string, error) {
	keysDir, err := GetKeysDir()
	if err != nil {
		return "", err
	}

	// 清理密钥内容
	content := strings.TrimSpace(keyContent)
	if content == "" {
		return "", fmt.Errorf("密钥内容不能为空")
	}

	// 生成文件名
	filename, err := GenerateKeyFilename(hostname)
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(keysDir, filename)

	// 写入文件，权限 0600
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("写入密钥文件失败: %w", err)
	}

	return filePath, nil
}

// IsManagedKey 检查密钥路径是否在托管的密钥目录内
func IsManagedKey(keyPath string) bool {
	if keyPath == "" {
		return false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	keysDir := filepath.Join(homeDir, KeysDirName)
	absKeyPath, err := filepath.Abs(keyPath)
	if err != nil {
		return false
	}

	// 检查路径是否在密钥目录内
	return strings.HasPrefix(absKeyPath, keysDir)
}

// ReadKeyContent 读取密钥文件内容
func ReadKeyContent(keyPath string) (string, error) {
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("读取密钥文件失败: %w", err)
	}
	return string(content), nil
}
