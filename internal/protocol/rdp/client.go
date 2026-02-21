package rdp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"trelay/pkg/models"
)

// ConnectionStatus 连接状态
type ConnectionStatus string

const (
	StatusIdle         ConnectionStatus = "idle"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusError        ConnectionStatus = "error"
)

// Client RDP客户端，实现protocol.Session接口
type Client struct {
	host        *models.Host
	cmd         *exec.Cmd
	builder     CmdBuilder
	status      ConnectionStatus
	err         error
	startTime   *time.Time
	detector    *ToolDetector
	installHelp *InstallHelper
	logger      *zap.Logger
	stdout      *bytes.Buffer
	stderr      *bytes.Buffer
}

// NewClient 创建RDP客户端
func NewClient(host *models.Host) *Client {
	logger, _ := zap.NewDevelopment()
	return &Client{
		host:        host,
		status:      StatusIdle,
		detector:    NewToolDetector(),
		installHelp: NewInstallHelper(),
		logger:      logger,
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
	}
}

// Connect 建立RDP连接 (实现Session接口)
func (c *Client) Connect() error {
	c.status = StatusConnecting
	now := time.Now()
	c.startTime = &now

	c.logger.Info("=========================================")
	c.logger.Info("开始RDP连接流程", zap.String("host", c.host.Name))
	c.logger.Info("主机信息",
		zap.String("host", c.host.Host),
		zap.Int("port", c.host.Port),
		zap.String("username", c.host.Username),
		zap.String("domain", c.host.Domain))

	// 检查环境变量（针对 freerdp 需要 X11）
	display := os.Getenv("DISPLAY")
	if display == "" {
		c.logger.Warn("DISPLAY 环境变量未设置，freerdp 需要 X11 显示支持")
	} else {
		c.logger.Info("DISPLAY 环境变量", zap.String("display", display))
	}

	// 1. 检测可用工具
	c.logger.Info("步骤1: 检测RDP工具...")
	toolInfo, err := c.detector.GetPreferredTool()
	if err != nil {
		c.status = StatusError
		c.err = fmt.Errorf("RDP工具检测失败: %w\n\n安装帮助:\n%s",
			err, c.installHelp.GetInstallHelp())
		c.logger.Error("RDP工具检测失败", zap.Error(err))
		return c.err
	}
	c.logger.Info("检测到工具",
		zap.String("name", toolInfo.Capability.Name),
		zap.String("path", toolInfo.Path),
		zap.String("executable", toolInfo.Executable))

	// 2. 创建命令构建器
	c.logger.Info("步骤2: 创建命令构建器...")
	builder, err := NewCmdBuilder(toolInfo)
	if err != nil {
		c.status = StatusError
		c.err = fmt.Errorf("创建RDP命令构建器失败: %w", err)
		c.logger.Error("创建RDP命令构建器失败", zap.Error(err))
		return c.err
	}
	c.builder = builder

	// 3. 构建命令
	c.logger.Info("步骤3: 构建RDP命令...")
	cmd, err := builder.BuildCommand(c.host)
	if err != nil {
		c.status = StatusError
		c.err = fmt.Errorf("构建RDP命令失败: %w", err)
		c.logger.Error("构建RDP命令失败", zap.Error(err))
		return c.err
	}

	// 打印完整的命令
	c.logger.Info("构建的命令",
		zap.String("path", cmd.Path),
		zap.Strings("args", cmd.Args))
	c.stdout.Reset()
	c.stderr.Reset()
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr

	c.cmd = cmd

	// 4. 启动进程
	c.logger.Info("步骤4: 启动RDP进程...")
	c.logger.Info("执行命令", zap.String("full_command", strings.Join(cmd.Args, " ")))
	if err := c.cmd.Start(); err != nil {
		c.status = StatusError
		c.err = fmt.Errorf("启动RDP进程失败: %w", err)
		c.logger.Error("启动RDP进程失败", zap.Error(err))
		return c.err
	}
	c.logger.Info("RDP进程启动成功", zap.Int("pid", cmd.Process.Pid))

	c.status = StatusConnected
	c.err = nil

	return nil
}

// Disconnect 断开连接 (实现Session接口)
func (c *Client) Disconnect() error {
	c.logger.Info("准备断开RDP连接...")
	if c.cmd == nil || c.cmd.Process == nil {
		c.logger.Info("RDP进程已不存在，无需断开")
		return nil
	}

	// 尝试优雅退出
	c.logger.Info("尝试发送SIGTERM信号...")
	err := c.cmd.Process.Signal(syscall.SIGTERM)
	c.logger.Info("发送信号完成", zap.Error(err))

	// 等待进程结束（最多5秒）
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case err := <-done:
		c.logger.Info("进程已退出", zap.Error(err))
	case <-time.After(5 * time.Second):
		c.logger.Warn("等待超时，强制终止进程")
		// 超时，强制终止
		c.cmd.Process.Kill()
	}

	c.status = StatusDisconnected
	return err
}

// IsConnected 返回是否已连接 (实现Session接口)
func (c *Client) IsConnected() bool {
	return c.status == StatusConnected &&
		c.cmd != nil &&
		c.cmd.Process != nil
}

// GetStatus 返回连接状态 (实现Session接口)
func (c *Client) GetStatus() ConnectionStatus {
	return c.status
}

// GetError 返回连接错误 (实现Session接口)
func (c *Client) GetError() error {
	return c.err
}

// GetHostID 返回主机标识 (实现Session接口)
func (c *Client) GetHostID() string {
	return c.host.Name
}

// GetStartTime 返回连接开始时间 (实现Session接口)
func (c *Client) GetStartTime() *time.Time {
	return c.startTime
}

// GetDuration 返回连接持续时间 (实现Session接口)
func (c *Client) GetDuration() time.Duration {
	if c.startTime == nil {
		return 0
	}
	return time.Since(*c.startTime)
}

// StartInteractiveSession 启动交互式RDP会话
func (c *Client) StartInteractiveSession() error {
	c.logger.Info("开始等待RDP会话结束...")
	if c.cmd == nil || c.cmd.Process == nil {
		c.logger.Error("RDP进程未启动")
		return errors.New("RDP进程未启动")
	}

	// 等待进程结束
	c.logger.Info("调用 cmd.Wait()...")
	err := c.cmd.Wait()
	c.logger.Info("cmd.Wait() 返回", zap.Error(err))

	// 打印输出
	stdoutStr := c.stdout.String()
	stderrStr := c.stderr.String()
	if stdoutStr != "" {
		c.logger.Info("进程标准输出", zap.String("output", stdoutStr))
	}

	// 分离 WARN 和 ERROR 日志
	hasError := false
	if stderrStr != "" {
		lines := strings.Split(stderrStr, "\n")
		var errorLines []string
		var warnLines []string

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.Contains(line, "[ERROR]") {
				hasError = true
				errorLines = append(errorLines, trimmed)
			} else if strings.Contains(line, "[WARN]") {
				warnLines = append(warnLines, trimmed)
			}
		}

		if len(warnLines) > 0 {
			c.logger.Warn("进程警告", zap.Strings("warnings", warnLines))
		}
		if len(errorLines) > 0 {
			c.logger.Error("进程错误输出", zap.Strings("errors", errorLines))
		}
	}

	// 检查退出状态
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		if exitCode != 0 {
			if hasError {
				c.logger.Error("进程异常退出", zap.Int("exit_code", exitCode))
				return fmt.Errorf(c.formatErrorMessage(stderrStr, exitCode))
			}
			c.logger.Warn("进程非零退出", zap.Int("exit_code", exitCode))
			return fmt.Errorf("RDP进程退出，代码: %d\n警告输出:\n%s", exitCode, stderrStr)
		}
	}

	c.status = StatusDisconnected
	c.logger.Info("RDP会话正常结束")
	return nil
}

// GetToolName 获取当前使用的工具名称
func (c *Client) GetToolName() string {
	if c.builder != nil {
		return c.builder.GetToolInfo().Capability.Name
	}
	return ""
}

// GetInstallHelp 获取安装帮助信息
func (c *Client) GetInstallHelp() string {
	return c.installHelp.GetInstallHelp()
}

// formatErrorMessage 格式化错误消息，提供友好的提示
func (c *Client) formatErrorMessage(stderr string, exitCode int) string {
	var hints []string

	// 检查 X11 显示问题
	if strings.Contains(stderr, "failed to open display") {
		hints = append(hints, "")
		hints = append(hints, "检测到 X11 显示问题。freerdp 需要 X11 显示环境才能运行。")
		hints = append(hints, "")
		hints = append(hints, "macOS 解决方案：")
		hints = append(hints, "  1. 安装 XQuartz: brew install --cask xquartz")
		hints = append(hints, "  2. 启动 XQuartz 应用程序")
		hints = append(hints, "  3. 确保 DISPLAY 环境变量已设置 (通常为 :0 或 :1)")
		hints = append(hints, "")
		hints = append(hints, "Linux 解决方案：")
		hints = append(hints, "  1. 确保 X server 正在运行")
		hints = append(hints, "  2. 检查 DISPLAY 环境变量")
		hints = append(hints, "")
		hints = append(hints, "或者使用 SSH X11 转发: ssh -X user@host")
		hints = append(hints, "")
	}

	// 检查连接问题
	if strings.Contains(stderr, "ERRCONNECT") || strings.Contains(stderr, "freerdp_connect_begin") {
		hints = append(hints, "RDP 连接失败，可能原因：")
		hints = append(hints, "  - 目标主机未运行 RDP 服务")
		hints = append(hints, "  - 网络不可达")
		hints = append(hints, "  - 防火墙阻止连接")
		hints = append(hints, "  - 用户名/密码不正确")
		hints = append(hints, "")
	}

	// 检查证书问题
	if strings.Contains(stderr, "certificate") || strings.Contains(stderr, "SSL") {
		hints = append(hints, "证书/SSL 问题。已在命令中使用 /cert:ignore 参数。")
		hints = append(hints, "如果问题持续，可能是服务器证书配置问题。")
		hints = append(hints, "")
	}

	// 构建 error message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("RDP 连接失败 (退出码: %d)\n", exitCode))

	if len(hints) > 0 {
		sb.WriteString(strings.Join(hints, "\n"))
	}

	sb.WriteString("详细错误信息:\n")
	sb.WriteString(stderr)

	return sb.String()
}
