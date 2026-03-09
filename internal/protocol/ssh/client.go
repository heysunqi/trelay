package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"trelay/internal/protocol"
	"trelay/pkg/models"

	"golang.org/x/crypto/ssh"
)

// winsize 终端窗口大小
type winsize struct {
	Rows    uint16
	Cols    uint16
	Xpixels uint16
	Ypixels uint16
}

// getTermSize 获取终端窗口大小
func getTermSize() (uint16, uint16) {
	ws := &winsize{}

	// 跨平台的 TIOCGWINSZ 常量
	const (
		TIOCGWINSZ_LINUX = 0x5413
		TIOCGWINSZ_MAC   = 0x40087468 // TIOCGWINSZ on macOS
	)

	var ioctlCmd uintptr
	if runtime.GOOS == "darwin" {
		ioctlCmd = TIOCGWINSZ_MAC
	} else {
		ioctlCmd = TIOCGWINSZ_LINUX
	}

	retCode, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		ioctlCmd,
		uintptr(unsafe.Pointer(ws)),
		0, 0, 0,
	)

	if int(retCode) == -1 || err != 0 {
		// 如果获取失败，使用默认值
		return 24, 80
	}

	// 确保我们返回合理的大小
	if ws.Rows == 0 || ws.Cols == 0 {
		return 24, 80
	}

	return ws.Rows, ws.Cols
}

// Client SSH客户端
type Client struct {
	host      *models.Host
	client    *ssh.Client
	status    protocol.ConnectionStatus
	err       error
	startTime *time.Time
}

// NewClient 创建SSH客户端
func NewClient(host *models.Host) *Client {
	return &Client{
		host:   host,
		status: protocol.StatusIdle,
	}
}

// Connect 建立SSH连接
func (c *Client) Connect() error {
	c.status = protocol.StatusConnecting
	c.startTime = &time.Time{}
	*c.startTime = time.Now()

	// 构建SSH配置
	config := &ssh.ClientConfig{
		User:            c.host.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应该验证主机密钥
		Timeout:         30 * time.Second,
	}

	// 配置认证方式
	if c.host.Password != "" {
		// 密码认证
		config.Auth = []ssh.AuthMethod{
			ssh.Password(c.host.Password),
		}
	} else if c.host.KeyPath != "" {
		// 密钥认证
		key, err := c.parsePrivateKey(c.host.KeyPath, c.host.Passphrase)
		if err != nil {
			c.status = protocol.StatusError
			c.err = fmt.Errorf("解析私钥失败: %w", err)
			return c.err
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else {
		c.status = protocol.StatusError
		c.err = errors.New("未配置密码或私钥")
		return c.err
	}

	// 连接服务器
	address := fmt.Sprintf("%s:%d", c.host.Host, c.host.Port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		c.status = protocol.StatusError
		c.err = fmt.Errorf("SSH连接失败: %w", err)
		return c.err
	}

	c.client = client
	c.status = protocol.StatusConnected
	c.err = nil

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	if c.client == nil {
		return nil
	}

	err := c.client.Close()
	c.client = nil
	c.status = protocol.StatusDisconnected
	return err
}

// IsConnected 返回是否已连接
func (c *Client) IsConnected() bool {
	return c.client != nil
}

// GetStatus 返回连接状态
func (c *Client) GetStatus() protocol.ConnectionStatus {
	return c.status
}

// GetError 返回连接错误
func (c *Client) GetError() error {
	return c.err
}

// GetHostID 返回主机标识
func (c *Client) GetHostID() string {
	return c.host.Name
}

// GetStartTime 返回连接开始时间
func (c *Client) GetStartTime() *time.Time {
	return c.startTime
}

// GetDuration 返回连接持续时间
func (c *Client) GetDuration() time.Duration {
	if c.startTime == nil {
		return 0
	}
	return time.Since(*c.startTime)
}

// Detach 将会话从终端分离（基础SSH客户端不支持，需使用PTYSession）
func (c *Client) Detach() error {
	return protocol.ErrNotSupported
}

// Attach 将会话附加到终端（基础SSH客户端不支持，需使用PTYSession）
func (c *Client) Attach(stdin io.Reader, stdout io.Writer, isResume bool) error {
	return protocol.ErrNotSupported
}

// IsAttached 返回会话是否已附加到终端
func (c *Client) IsAttached() bool {
	return false
}

// StartBackgroundSession 启动后台SSH会话，返回可控的PTYSession
// 调用方通过 PTYSession.Attach()/Detach() 控制前台/后台切换
func (c *Client) StartBackgroundSession() (*PTYSession, error) {
	if c.client == nil {
		return nil, errors.New("未连接到SSH服务器")
	}

	session := NewPTYSession(c.host, c.client)
	if err := session.Start(); err != nil {
		return nil, err
	}

	// SSH连接的所有权转移给PTYSession，防止Client.Disconnect()关闭连接
	c.client = nil
	c.status = protocol.StatusConnected

	return session, nil
}

// StartInteractiveSession 启动交互式SSH会话
func (c *Client) StartInteractiveSession() error {
	if c.client == nil {
		return errors.New("未连接到SSH服务器")
	}

	// 获取终端窗口大小
	rows, cols := getTermSize()

	// 创建会话
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %w", err)
	}
	defer session.Close()

	// 请求伪终端 - 完整的终端模式配置
	// 这些模式设置与现代Linux和macOS终端高度兼容
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,      // 开启回显
		ssh.ECHOK:         1,      // 回显换行符
		ssh.ECHOE:         1,      // 回显擦除字符
		ssh.ECHOKE:        1,      // 回显删除字符序列
		ssh.ECHOCTL:       1,      // 回显控制字符（显示^C而不是Control-C）
		ssh.ICRNL:         1,      // 将CR转换为NL
		ssh.IGNPAR:        1,      // 忽略奇偶校验错误的字节
		ssh.IXON:          1,      // 启用XON/XOFF流量控制
		ssh.IXOFF:         1,      // 启用XON/XOFF输入控制
		ssh.OPOST:         1,      // 启用输出处理
		ssh.ONLCR:         1,      // 将NL转换为CR-NL
		ssh.ISIG:          1,      // 启用信号处理
		ssh.ICANON:        1,      // 启用规范模式（行缓冲）
		ssh.IEXTEN:        1,      // 启用输入扩展
		ssh.TTY_OP_ISPEED: 115200, // 输入速度 = 115200 baud
		ssh.TTY_OP_OSPEED: 115200, // 输出速度 = 115200 baud
	}
	if err := session.RequestPty("xterm-256color", int(rows), int(cols), modes); err != nil {
		return fmt.Errorf("请求伪终端失败: %w", err)
	}

	// 连接标准输入输出到SSH会话（必须在Shell之前设置）
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// 启动shell
	if err := session.Shell(); err != nil {
		return fmt.Errorf("启动shell失败: %w", err)
	}

	// 监听终端窗口大小变化，同步到远程 PTY
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	go func() {
		for range sigCh {
			rows, cols := getTermSize()
			session.WindowChange(int(rows), int(cols))
		}
	}()

	// 等待会话结束
	return session.Wait()
}

// parsePrivateKey 解析私钥
func (c *Client) parsePrivateKey(keyData, keyPassword string) (ssh.Signer, error) {
	// 如果keyData是文件路径，读取文件内容
	var keyBytes []byte
	if _, err := os.Stat(keyData); err == nil {
		// 是文件路径
		keyBytes, err = os.ReadFile(keyData)
		if err != nil {
			return nil, fmt.Errorf("读取私钥文件失败: %w", err)
		}
	} else {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}
	// 尝试无密码解析
	key, err := ssh.ParsePrivateKey(keyBytes)
	if err == nil {
		return key, nil
	}

	// 如果失败，尝试使用密码解析
	if keyPassword != "" {
		key, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(keyPassword))
		if err == nil {
			return key, nil
		}
	}

	return nil, fmt.Errorf("解析私钥失败，可能需要密码: %w", err)
}
