package vnc

import (
	"fmt"
	"runtime"
	"strings"
)

// InstallHelper VNC工具安装帮助
type InstallHelper struct{}

// NewInstallHelper 创建安装帮助实例
func NewInstallHelper() *InstallHelper {
	return &InstallHelper{}
}

// GetInstallHelp 获取安装帮助信息
func (h *InstallHelper) GetInstallHelp() string {
	var sb strings.Builder

	sb.WriteString("==============================================\n")
	sb.WriteString("VNC工具未找到，请先安装VNC客户端\n")
	sb.WriteString("==============================================\n\n")

	switch runtime.GOOS {
	case "linux":
		h.getLinuxInstallHelp(&sb)
	case "darwin":
		h.getMacOSInstallHelp(&sb)
	default:
		sb.WriteString("当前平台: ")
		sb.WriteString(runtime.GOOS)
		sb.WriteString("\n请手动安装VNC客户端\n")
	}

	return sb.String()
}

// getLinuxInstallHelp 获取Linux安装帮助
func (h *InstallHelper) getLinuxInstallHelp(sb *strings.Builder) {
	sb.WriteString("【Linux 安装指南】\n\n")

	// Remmina安装
	sb.WriteString("方案1: 安装 Remmina (推荐)\n")
	sb.WriteString("  Ubuntu/Debian:\n")
	sb.WriteString("    sudo apt update\n")
	sb.WriteString("    sudo apt install remmina remmina-plugin-vnc\n\n")
	sb.WriteString("  Fedora:\n")
	sb.WriteString("    sudo dnf install remmina remmina-vnc\n\n")
	sb.WriteString("  Arch Linux:\n")
	sb.WriteString("    sudo pacman -S remmina\n\n")

	// TigerVNC安装
	sb.WriteString("方案2: 安装 TigerVNC\n")
	sb.WriteString("  Ubuntu/Debian:\n")
	sb.WriteString("    sudo apt install tigervnc-viewer\n\n")
	sb.WriteString("  Fedora:\n")
	sb.WriteString("    sudo dnf install tigervnc\n\n")
	sb.WriteString("  Arch Linux:\n")
	sb.WriteString("    sudo pacman -S tigervnc\n\n")
}

// getMacOSInstallHelp 获取macOS安装帮助
func (h *InstallHelper) getMacOSInstallHelp(sb *strings.Builder) {
	sb.WriteString("【macOS 安装指南】\n\n")

	sb.WriteString("方案1: 使用系统屏幕共享 (推荐)\n")
	sb.WriteString("  macOS自带屏幕共享功能，无需安装\n")
	sb.WriteString("  使用方法: 通过VNC地址连接即可\n\n")

	sb.WriteString("方案2: 安装 TigerVNC\n")
	sb.WriteString("  使用Homebrew:\n")
	sb.WriteString("    brew install tigervnc\n\n")

	sb.WriteString("方案3: 安装其他VNC客户端\n")
	sb.WriteString("  - RealVNC: https://www.realvnc.com/\n")
	sb.WriteString("  - Chicken of the VNC: brew install --cask chicken\n\n")

	sb.WriteString("注意: macOS屏幕共享需要远程主机开启VNC服务\n")
	sb.WriteString("  开启方法: 系统偏好设置 → 共享 → 屏幕共享\n")
}

// GetToolDetectionResult 获取工具检测结果信息
func (h *InstallHelper) GetToolDetectionResult(available []*ToolInfo) string {
	var sb strings.Builder

	if len(available) == 0 {
		sb.WriteString("未检测到任何VNC工具")
	} else {
		sb.WriteString("已检测到以下VNC工具:\n")
		for _, tool := range available {
			sb.WriteString("  - ")
			sb.WriteString(tool.Capability.Name)
			sb.WriteString(": ")
			sb.WriteString(tool.Path)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// GetPlatformRecommendation 获取平台推荐
func (h *InstallHelper) GetPlatformRecommendation() string {
	switch runtime.GOOS {
	case "linux":
		return "推荐使用 Remmina (如果可用)，否则使用 TigerVNC"
	case "darwin":
		return "推荐使用系统内置屏幕共享 (无需安装)"
	default:
		return "请根据您的系统选择合适的VNC客户端"
	}
}

// GetErrorSuggestion 获取错误建议
func (h *InstallHelper) GetErrorSuggestion(err error) string {
	var sb strings.Builder
	sb.WriteString("错误: ")
	sb.WriteString(err.Error())
	sb.WriteString("\n\n")

	sb.WriteString(h.GetInstallHelp())

	sb.WriteString("\n\n")
	sb.WriteString(h.GetPlatformRecommendation())

	return sb.String()
}

// GetVNCServerSetup 获取VNC服务器设置指南
func (h *InstallHelper) GetVNCServerSetup() string {
	var sb strings.Builder

	sb.WriteString("==============================================\n")
	sb.WriteString("VNC服务器设置指南\n")
	sb.WriteString("==============================================\n\n")

	switch runtime.GOOS {
	case "linux":
		sb.WriteString("【Linux VNC服务器】\n\n")
		sb.WriteString("方案1: 使用vino (GNOME桌面)\n")
		sb.WriteString("  # 启用桌面共享\n")
		sb.WriteString("  vino-preferences\n")
		sb.WriteString("  # 或通过dconf配置\n")
		sb.WriteString("  gsettings set org.gnome.desktop.remote-desktop.vnc true\n\n")

		sb.WriteString("方案2: 使用TigerVNC服务器\n")
		sb.WriteString("  sudo apt install tigervnc-standalone-server\n")
		sb.WriteString("  vncserver :1 -geometry 1920x1080 -depth 24\n\n")

		sb.WriteString("方案3: 使用x11vnc\n")
		sb.WriteString("  sudo apt install x11vnc\n")
		sb.WriteString("  x11vnc -display :0 -shared\n\n")

	case "darwin":
		sb.WriteString("【macOS VNC服务器】\n\n")
		sb.WriteString("方法1: 系统偏好设置\n")
		sb.WriteString("  1. 打开 系统偏好设置 → 共享\n")
		sb.WriteString("  2. 勾选 屏幕共享\n")
		sb.WriteString("  3. 点击 电脑设置...\n")
		sb.WriteString("  4. 勾选 VNC viewers may control screen with password\n")
		sb.WriteString("  5. 设置密码并确定\n\n")

		sb.WriteString("方法2: 命令行启用\n")
		sb.WriteString("  sudo /System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -activate -configure -access -on -restart -agent -privs -all\n\n")

	case "windows":
		sb.WriteString("【Windows VNC服务器】\n\n")
		sb.WriteString("方案1: 使用第三方软件\n")
		sb.WriteString("  - RealVNC Server\n")
		sb.WriteString("  - TightVNC\n")
		sb.WriteString("  - UltraVNC\n\n")

		sb.WriteString("方案2: 使用Windows远程桌面(非VNC)\n")
		sb.WriteString("  Windows远程桌面使用RDP协议，不是VNC协议\n")
		sb.WriteString("  trelay已支持RDP协议连接\n")
	}

	return sb.String()
}

// FormatHint 格式化提示信息
func (h *InstallHelper) FormatHint(toolName, suggestion string) string {
	return fmt.Sprintf("建议安装 %s: %s", toolName, suggestion)
}
