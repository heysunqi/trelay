package vnc

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"trelay/pkg/models"

	"go.uber.org/zap"
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

// Client VNC客户端，实现protocol.Session接口
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

// NewClient 创建VNC客户端
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

// Connect 建立VNC连接 (实现Session接口)
func (c *Client) Connect() error {
	c.status = StatusConnecting
	now := time.Now()
	c.startTime = &now

	c.logger.Info("=========================================")
	c.logger.Info("开始VNC连接流程", zap.String("host", c.host.Name))
	c.logger.Info("主机信息",
		zap.String("host", c.host.Host),
		zap.Int("port", c.host.Port),
		zap.String("username", c.host.Username),
		zap.Bool("view_only", c.host.ViewOnly))

	// 检查平台支持
	if !IsPlatformSupported() {
		c.status = StatusError
		c.err = &ErrorPlatformUnsupported{Platform: GetPlatformName()}
		c.logger.Error("平台不支持", zap.String("platform", GetPlatformName()))
		return c.err
	}

	// 1. 检测可用工具
	c.logger.Info("步骤1: 检测VNC工具...")
	toolInfo, err := c.detector.GetPreferredTool()
	if err != nil {
		c.status = StatusError
		c.err = fmt.Errorf("VNC工具检测失败: %w\n\n安装帮助:\n%s",
			err, c.installHelp.GetInstallHelp())
		c.logger.Error("VNC工具检测失败", zap.Error(err))
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
		c.err = fmt.Errorf("创建VNC命令构建器失败: %w", err)
		c.logger.Error("创建VNC命令构建器失败", zap.Error(err))
		return c.err
	}
	c.builder = builder

	// 3. 构建命令
	c.logger.Info("步骤3: 构建VNC命令...")
	cmd, err := builder.BuildCommand(c.host)
	if err != nil {
		c.status = StatusError
		c.err = fmt.Errorf("构建VNC命令失败: %w", err)
		c.logger.Error("构建VNC命令失败", zap.Error(err))
		return c.err
	}

	// 打印完整的命令
	c.logger.Info("构建的命令",
		zap.String("path", cmd.Path),
		zap.Strings("args", cmd.Args))

	// 设置输出
	c.stdout.Reset()
	c.stderr.Reset()
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr

	c.cmd = cmd

	// 4. 启动进程
	c.logger.Info("步骤4: 启动VNC进程...")
	c.logger.Info("执行命令", zap.String("full_command", strings.Join(cmd.Args, " ")))

	// 对于macOS的open命令，不需要等待子进程
	// 因为open是异步的，会立即返回
	if c.host.Protocol == "vnc" && GetPlatformName() == "macOS" {
		// macOS使用open命令，直接启动不等待
		if err := c.cmd.Start(); err != nil {
			c.status = StatusError
			c.err = fmt.Errorf("启动VNC进程失败: %w", err)
			c.logger.Error("启动VNC进程失败", zap.Error(err))
			return c.err
		}
		c.logger.Info("VNC进程启动成功 (macOS Screen Sharing)")
		c.status = StatusConnected
		c.err = nil
		return nil
	}

	// 其他平台正常启动
	if err := c.cmd.Start(); err != nil {
		c.status = StatusError
		c.err = fmt.Errorf("启动VNC进程失败: %w", err)
		c.logger.Error("启动VNC进程失败", zap.Error(err))
		return c.err
	}
	c.logger.Info("VNC进程启动成功", zap.Int("pid", c.cmd.Process.Pid))

	c.status = StatusConnected
	c.err = nil

	return nil
}

// Disconnect 断开连接 (实现Session接口)
func (c *Client) Disconnect() error {
	c.logger.Info("准备断开VNC连接...")
	if c.cmd == nil || c.cmd.Process == nil {
		c.logger.Info("VNC进程已不存在，无需断开")
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

// StartInteractiveSession 启动交互式VNC会话
func (c *Client) StartInteractiveSession() error {
	c.logger.Info("开始等待VNC会话结束...")

	// macOS使用open命令，不需要等待
	if GetPlatformName() == "macOS" {
		c.logger.Info("macOS屏幕共享已启动，请在屏幕共享应用中操作")
		c.logger.Info("按回车键返回...")
		// 等待用户按回车
		fmt.Scanln()
		c.status = StatusDisconnected
		return nil
	}

	if c.cmd == nil || c.cmd.Process == nil {
		c.logger.Error("VNC进程未启动")
		return fmt.Errorf("VNC进程未启动")
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

	// 分析错误输出
	if stderrStr != "" {
		lines := strings.Split(stderrStr, "\n")
		var errorLines []string
		var warnLines []string

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.Contains(line, "[ERROR]") || strings.Contains(line, "Error") {
				errorLines = append(errorLines, trimmed)
			} else if strings.Contains(line, "[WARN]") || strings.Contains(line, "Warning") {
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
			c.logger.Error("进程异常退出", zap.Int("exit_code", exitCode))
			return fmt.Errorf("VNC进程退出，代码: %d\n错误输出:\n%s", exitCode, stderrStr)
		}
	}

	c.status = StatusDisconnected
	c.logger.Info("VNC会话正常结束")
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

// Detach 将会话从终端分离（VNC不支持后台化）
func (c *Client) Detach() error {
	return fmt.Errorf("VNC协议不支持后台化")
}

// Attach 将会话附加到终端（VNC不支持后台化）
func (c *Client) Attach(stdin io.Reader, stdout io.Writer) error {
	return fmt.Errorf("VNC协议不支持后台化")
}

// IsAttached 返回会话是否已附加到终端
func (c *Client) IsAttached() bool {
	return false
}

// GetConnectionHint 获取连接提示
func (c *Client) GetConnectionHint() string {
	switch GetPlatformName() {
	case "macOS":
		return "屏幕共享应用已启动，请在应用中完成连接和认证"
	case "Linux":
		return "VNC客户端已启动，正在连接..."
	default:
		return "正在连接..."
	}
}

// formatErrorMessage 格式化错误消息
func (c *Client) formatErrorMessage(stderr string, exitCode int) string {
	var hints []string

	// 检查连接问题
	if strings.Contains(stderr, "connection refused") || strings.Contains(stderr, "Connection refused") {
		hints = append(hints, "")
		hints = append(hints, "连接被拒绝，可能原因：")
		hints = append(hints, "  - 目标主机未运行VNC服务")
		hints = append(hints, "  - 网络不可达")
		hints = append(hints, "  - 防火墙阻止连接")
		hints = append(hints, "  - VNC端口不正确")
		hints = append(hints, "")
	}

	// 检查认证问题
	if strings.Contains(stderr, "authentication") || strings.Contains(stderr, "Authentication") {
		hints = append(hints, "认证失败，可能原因：")
		hints = append(hints, "  - 密码不正确")
		hints = append(hints, "  - VNC服务器未设置密码")
		hints = append(hints, "  - 用户名/密码格式不正确")
		hints = append(hints, "")
	}

	// 检查显示问题
	if strings.Contains(stderr, "display") || strings.Contains(stderr, "Display") {
		hints = append(hints, "")
		hints = append(hints, "显示问题，请确保X服务器正常运行")
		hints = append(hints, "")
	}

	// 构建错误消息
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("VNC 连接失败 (退出码: %d)\n", exitCode))

	if len(hints) > 0 {
		sb.WriteString(strings.Join(hints, "\n"))
	}

	sb.WriteString("详细错误信息:\n")
	sb.WriteString(stderr)

	return sb.String()
}
