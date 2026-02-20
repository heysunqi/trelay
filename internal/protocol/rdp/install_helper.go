package rdp

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// PackageManager 包管理器类型
type PackageManager string

const (
	PMApt   PackageManager = "apt"   // Ubuntu/Debian
	PMYum   PackageManager = "yum"   // CentOS/RHEL 7及以下
	PMDnf   PackageManager = "dnf"   // Fedora/RHEL 8+
	PMPacman PackageManager = "pacman" // Arch Linux
	PMApk   PackageManager = "apk"   // Alpine Linux
)

// PackageManagerInfo 包管理器信息
type PackageManagerInfo struct {
	Name    PackageManager
	Command string
	Install map[string]string // 工具名 -> 安装包名
}

// 包管理器安装命令映射
var packageManagers = []PackageManagerInfo{
	{
		Name:    PMApt,
		Command: "apt-get",
		Install: map[string]string{
			"remmina": "remmina remmina-plugin-rdp",
			"freerdp": "freerdp2-x11",
		},
	},
	{
		Name:    PMYum,
		Command: "yum",
		Install: map[string]string{
			"remmina": "remmina remmina-plugins-rdp",
			"freerdp": "freerdp",
		},
	},
	{
		Name:    PMDnf,
		Command: "dnf",
		Install: map[string]string{
			"remmina": "remmina remmina-plugins-rdp",
			"freerdp": "freerdp",
		},
	},
	{
		Name:    PMPacman,
		Command: "pacman",
		Install: map[string]string{
			"remmina": "remmina freerdp",
			"freerdp": "freerdp",
		},
	},
	{
		Name:    PMApk,
		Command: "apk",
		Install: map[string]string{
			"remmina": "remmina",
			"freerdp": "freerdp",
		},
	},
}

// InstallHelper 安装帮助器
type InstallHelper struct {
	os string
}

// NewInstallHelper 创建安装帮助器
func NewInstallHelper() *InstallHelper {
	return &InstallHelper{
		os: runtime.GOOS,
	}
}

// DetectPackageManager 检测系统包管理器
func (h *InstallHelper) DetectPackageManager() (PackageManager, bool) {
	for _, pm := range packageManagers {
		if _, err := exec.LookPath(pm.Command); err == nil {
			return pm.Name, true
		}
	}
	return "", false
}

// GetInstallCommands 获取安装命令
func (h *InstallHelper) GetInstallCommands() []InstallCommand {
	var commands []InstallCommand

	switch h.os {
	case "linux":
		if pm, found := h.DetectPackageManager(); found {
			for _, pmInfo := range packageManagers {
				if pmInfo.Name == pm {
					for tool, pkg := range pmInfo.Install {
						commands = append(commands, InstallCommand{
							Tool:    tool,
							Package: pkg,
							Command: h.formatCommand(pm, pkg),
						})
					}
				}
			}
		}

	case "darwin":
		commands = append(commands, InstallCommand{
			Tool:    "freerdp",
			Package: "freerdp",
			Command: "brew install freerdp",
		})
	}

	return commands
}

// formatCommand 格式化安装命令
func (h *InstallHelper) formatCommand(pm PackageManager, pkg string) string {
	switch pm {
	case PMApt:
		return "sudo apt update && sudo apt install -y " + pkg
	case PMYum:
		return "sudo yum install -y " + pkg
	case PMDnf:
		return "sudo dnf install -y " + pkg
	case PMPacman:
		return "sudo pacman -S --noconfirm " + pkg
	case PMApk:
		return "sudo apk add --no-cache " + pkg
	default:
		return "sudo " + string(pm) + " install " + pkg
	}
}

// GetInstallHelp 获取安装帮助信息
func (h *InstallHelper) GetInstallHelp() string {
	var help strings.Builder

	help.WriteString("未找到可用的RDP连接工具\n\n")

	switch h.os {
	case "linux":
		help.WriteString("请安装以下工具之一：\n\n")

		pm, found := h.DetectPackageManager()
		if found {
			help.WriteString(fmt.Sprintf("检测到包管理器: %s\n\n", pm))

			for _, cmd := range h.GetInstallCommands() {
				toolName := strings.ToUpper(cmd.Tool)
				help.WriteString(fmt.Sprintf("=== 安装 %s ===\n", toolName))
				help.WriteString(cmd.Command)
				help.WriteString("\n\n")
			}
		} else {
			help.WriteString("未能检测到系统包管理器。\n\n")
			help.WriteString("请访问以下链接获取安装指南：\n")
			help.WriteString("https://remmina.org/how-to-install-remmina/\n\n")
		}

	case "darwin":
		help.WriteString("请安装 FreeRDP：\n\n")
		help.WriteString("=== 安装 FreeRDP ===\n")
		help.WriteString("brew install freerdp\n\n")

		if _, err := exec.LookPath("brew"); err != nil {
			help.WriteString("提示: 未检测到 Homebrew\n")
			help.WriteString("请先安装 Homebrew: https://brew.sh/\n")
		}
	}

	help.WriteString("安装完成后，请重新启动程序。")

	return help.String()
}

// InstallCommand 安装命令
type InstallCommand struct {
	Tool    string // 工具名称
	Package string // 包名
	Command string // 完整安装命令
}
