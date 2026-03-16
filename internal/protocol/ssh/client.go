package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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

// SSHClient SSH客户端接口，支持直连和代理连接
type SSHClient interface {
	NewSession() (*ssh.Session, error)
	Close() error
}

// ProxiedClient 包装通过代理的 SSH 客户端
type ProxiedClient struct {
	*ssh.Client
	proxyClient *ssh.Client
}

// Close 关闭连接（包括代理连接）
func (pc *ProxiedClient) Close() error {
	var errs []error
	if pc.Client != nil {
		if err := pc.Client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if pc.proxyClient != nil {
		if err := pc.proxyClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Client SSH客户端
type Client struct {
	host      *models.Host
	allHosts  []*models.Host // 所有主机配置，用于查找跳板机
	client    SSHClient      // SSH客户端接口，支持直连和代理连接
	status    protocol.ConnectionStatus
	err       error
	startTime *time.Time
	ctx       context.Context // 连接上下文，用于取消连接
}

// NewClient 创建SSH客户端
func NewClient(host *models.Host, allHosts []*models.Host) *Client {
	return &Client{
		host:     host,
		allHosts: allHosts,
		status:   protocol.StatusIdle,
		ctx:      context.Background(), // 默认使用 background context
	}
}

// NewClientWithContext 创建带上下文的SSH客户端
func NewClientWithContext(ctx context.Context, host *models.Host, allHosts []*models.Host) *Client {
	return &Client{
		host:     host,
		allHosts: allHosts,
		status:   protocol.StatusIdle,
		ctx:      ctx,
	}
}

// Connect 建立SSH连接
func (c *Client) Connect() error {
	c.status = protocol.StatusConnecting
	c.startTime = &time.Time{}
	*c.startTime = time.Now()

	// 根据连接方式选择连接方法
	switch c.host.ConnectVia {
	case "proxyjump":
		return c.connectViaProxyJump()
	case "proxyserver":
		return c.connectViaProxyServer()
	default:
		return c.connectDirect()
	}
}

// connectDirect 直连目标服务器
func (c *Client) connectDirect() error {
	// 检查上下文是否已取消
	if err := c.ctx.Err(); err != nil {
		c.status = protocol.StatusDisconnected
		c.err = fmt.Errorf("连接已取消")
		return c.err
	}

	// 构建SSH配置
	config, err := c.buildSSHConfig(c.host)
	if err != nil {
		c.status = protocol.StatusError
		c.err = err
		return err
	}

	// 连接服务器 - 使用 DialContext 支持取消
	address := fmt.Sprintf("%s:%d", c.host.Host, c.host.GetPort())
	var d net.Dialer
	conn, err := d.DialContext(c.ctx, "tcp", address)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			c.status = protocol.StatusDisconnected
			c.err = fmt.Errorf("连接已取消")
		} else {
			c.status = protocol.StatusError
			c.err = fmt.Errorf("SSH连接失败: %w", err)
		}
		return c.err
	}

	// 在连接上建立 SSH 会话
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close()
		c.status = protocol.StatusError
		c.err = fmt.Errorf("建立SSH连接失败: %w", err)
		return c.err
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)
	c.status = protocol.StatusConnected
	c.err = nil

	return nil
}

// connectViaProxyJump 通过跳板机连接
func (c *Client) connectViaProxyJump() error {
	// 检查上下文是否已取消
	if err := c.ctx.Err(); err != nil {
		c.status = protocol.StatusDisconnected
		c.err = fmt.Errorf("连接已取消")
		return c.err
	}

	// 查找跳板机配置
	var proxyHost *models.Host
	for _, h := range c.allHosts {
		if h.Name == c.host.ProxyJump {
			proxyHost = h
			break
		}
	}
	if proxyHost == nil {
		c.status = protocol.StatusError
		c.err = fmt.Errorf("跳板机 %s 未找到", c.host.ProxyJump)
		return c.err
	}

	// 连接跳板机 - 使用 DialContext 支持取消
	proxyConfig, err := c.buildSSHConfig(proxyHost)
	if err != nil {
		c.status = protocol.StatusError
		c.err = fmt.Errorf("构建跳板机SSH配置失败: %w", err)
		return c.err
	}

	proxyAddress := fmt.Sprintf("%s:%d", proxyHost.Host, proxyHost.GetPort())
	var d net.Dialer
	proxyConn, err := d.DialContext(c.ctx, "tcp", proxyAddress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			c.status = protocol.StatusDisconnected
			c.err = fmt.Errorf("连接已取消")
		} else {
			c.status = protocol.StatusError
			c.err = fmt.Errorf("连接跳板机 %s 失败: %w", proxyHost.Name, err)
		}
		return c.err
	}

	// 在跳板机连接上建立 SSH 会话
	proxySSHConn, proxyChans, proxyReqs, err := ssh.NewClientConn(proxyConn, proxyAddress, proxyConfig)
	if err != nil {
		proxyConn.Close()
		c.status = protocol.StatusError
		c.err = fmt.Errorf("建立跳板机SSH连接失败: %w", err)
		return c.err
	}
	proxyClient := ssh.NewClient(proxySSHConn, proxyChans, proxyReqs)

	// 再次检查上下文
	if err := c.ctx.Err(); err != nil {
		proxyClient.Close()
		c.status = protocol.StatusDisconnected
		c.err = fmt.Errorf("连接已取消")
		return c.err
	}

	// 通过跳板机连接目标
	targetConfig, err := c.buildSSHConfig(c.host)
	if err != nil {
		proxyClient.Close()
		c.status = protocol.StatusError
		c.err = err
		return c.err
	}

	targetAddress := fmt.Sprintf("%s:%d", c.host.Host, c.host.GetPort())
	conn, err := proxyClient.Dial("tcp", targetAddress)
	if err != nil {
		proxyClient.Close()
		c.status = protocol.StatusError
		c.err = fmt.Errorf("通过跳板机连接目标 %s:%d 失败: %w", c.host.Host, c.host.GetPort(), err)
		return c.err
	}

	// 在连接上建立 SSH 会话
	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetAddress, targetConfig)
	if err != nil {
		conn.Close()
		proxyClient.Close()
		c.status = protocol.StatusError
		c.err = fmt.Errorf("建立SSH连接失败: %w", err)
		return c.err
	}

	// 使用 ProxiedClient 包装，确保关闭时同时关闭代理连接
	c.client = &ProxiedClient{
		Client:      ssh.NewClient(ncc, chans, reqs),
		proxyClient: proxyClient,
	}
	c.status = protocol.StatusConnected
	c.err = nil

	return nil
}

// connectViaProxyServer 通过代理服务器连接
func (c *Client) connectViaProxyServer() error {
	// 检查上下文是否已取消
	if err := c.ctx.Err(); err != nil {
		c.status = protocol.StatusDisconnected
		c.err = fmt.Errorf("连接已取消")
		return c.err
	}

	// 构建代理服务器配置
	proxyHost := &models.Host{
		Host:       c.host.ProxyHost,
		Port:       c.host.ProxyPort,
		Username:   c.host.ProxyUser,
		AuthMethod: c.host.ProxyAuthMethod,
		Password:   c.host.ProxyPassword,
		KeyPath:    c.host.ProxyKeyPath,
	}
	if proxyHost.Port == 0 {
		proxyHost.Port = 22
	}

	proxyConfig, err := c.buildSSHConfig(proxyHost)
	if err != nil {
		c.status = protocol.StatusError
		c.err = fmt.Errorf("构建代理服务器SSH配置失败: %w", err)
		return c.err
	}

	// 连接代理服务器 - 使用 DialContext 支持取消
	proxyAddress := fmt.Sprintf("%s:%d", proxyHost.Host, proxyHost.GetPort())
	var d net.Dialer
	proxyConn, err := d.DialContext(c.ctx, "tcp", proxyAddress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			c.status = protocol.StatusDisconnected
			c.err = fmt.Errorf("连接已取消")
		} else {
			c.status = protocol.StatusError
			c.err = fmt.Errorf("连接代理服务器 %s 失败: %w", proxyHost.Host, err)
		}
		return c.err
	}

	// 在代理连接上建立 SSH 会话
	proxySSHConn, proxyChans, proxyReqs, err := ssh.NewClientConn(proxyConn, proxyAddress, proxyConfig)
	if err != nil {
		proxyConn.Close()
		c.status = protocol.StatusError
		c.err = fmt.Errorf("建立代理SSH连接失败: %w", err)
		return c.err
	}
	proxyClient := ssh.NewClient(proxySSHConn, proxyChans, proxyReqs)

	// 再次检查上下文
	if err := c.ctx.Err(); err != nil {
		proxyClient.Close()
		c.status = protocol.StatusDisconnected
		c.err = fmt.Errorf("连接已取消")
		return c.err
	}

	// 通过代理服务器连接目标
	targetConfig, err := c.buildSSHConfig(c.host)
	if err != nil {
		proxyClient.Close()
		c.status = protocol.StatusError
		c.err = err
		return c.err
	}

	targetAddress := fmt.Sprintf("%s:%d", c.host.Host, c.host.GetPort())
	conn, err := proxyClient.Dial("tcp", targetAddress)
	if err != nil {
		proxyClient.Close()
		c.status = protocol.StatusError
		c.err = fmt.Errorf("通过代理服务器连接目标 %s:%d 失败: %w", c.host.Host, c.host.GetPort(), err)
		return c.err
	}

	// 在连接上建立 SSH 会话
	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetAddress, targetConfig)
	if err != nil {
		conn.Close()
		proxyClient.Close()
		c.status = protocol.StatusError
		c.err = fmt.Errorf("建立SSH连接失败: %w", err)
		return c.err
	}

	// 使用 ProxiedClient 包装
	c.client = &ProxiedClient{
		Client:      ssh.NewClient(ncc, chans, reqs),
		proxyClient: proxyClient,
	}
	c.status = protocol.StatusConnected
	c.err = nil

	return nil
}

// buildSSHConfig 构建 SSH 客户端配置
func (c *Client) buildSSHConfig(host *models.Host) (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:            host.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应该验证主机密钥
		Timeout:         30 * time.Second,
	}

	// 配置认证方式
	if host.Password != "" {
		// 密码认证
		config.Auth = []ssh.AuthMethod{
			ssh.Password(host.Password),
		}
	} else if host.KeyPath != "" {
		// 密钥认证
		key, err := c.parsePrivateKey(host.KeyPath, host.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else {
		return nil, errors.New("未配置密码或私钥")
	}

	return config, nil
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
