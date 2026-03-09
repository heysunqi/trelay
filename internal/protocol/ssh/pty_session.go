package ssh

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"trelay/internal/protocol"
	"trelay/pkg/models"

	"github.com/muesli/cancelreader"
	"golang.org/x/crypto/ssh"
)

// PTYSession 基于SSH会话，支持前台/后台切换
type PTYSession struct {
	mu         sync.Mutex
	host       *models.Host
	sshClient  *ssh.Client
	session    *ssh.Session
	stdinPipe  io.WriteCloser
	stdoutPipe io.Reader
	stderrPipe io.Reader
	attached   bool
	done       chan struct{}
	status     protocol.ConnectionStatus
	err        error
	startTime  *time.Time

	// I/O 转发控制
	ioCancel context.CancelFunc
	ioWg     sync.WaitGroup

	// 持久化输出通道：由 Start() 中的 goroutine 写入，由 Attach() 中的 goroutine 读取
	// 整个会话生命周期只有一组读取 goroutine，避免 detach/re-attach 时多个 goroutine 竞争 pipe
	outputCh chan []byte

	// 残留数据：detach 时可能已从 stdin 读取但未发送的数据
	pendingInput []byte
}

// NewPTYSession 创建SSH会话
func NewPTYSession(host *models.Host, sshClient *ssh.Client) *PTYSession {
	now := time.Now()
	return &PTYSession{
		host:      host,
		sshClient: sshClient,
		done:      make(chan struct{}),
		status:    protocol.StatusConnected,
		startTime: &now,
	}
}

// Start 启动SSH会话（非阻塞，在goroutine中运行）
func (p *PTYSession) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取终端大小
	rows, cols := getTermSize()

	// 创建 SSH session
	session, err := p.sshClient.NewSession()
	if err != nil {
		p.status = protocol.StatusError
		p.err = fmt.Errorf("创建SSH会话失败: %w", err)
		return p.err
	}
	p.session = session

	// 请求远程 PTY
	// 远程 PTY 开启 ECHO，由远程服务器处理回显
	modes := ssh.TerminalModes{
		ssh.ECHO:          1, // 远程回显开启
		ssh.ECHOK:         1,
		ssh.ECHOE:         1, // 回显擦除字符
		ssh.ECHOKE:        1, // 回显擦除行
		ssh.ECHOCTL:       1, // 回显控制字符
		ssh.ICRNL:         1,
		ssh.IGNPAR:        1,
		ssh.IXON:          1,
		ssh.IXOFF:         1,
		ssh.OPOST:         1,
		ssh.ONLCR:         1,
		ssh.ISIG:          1,
		ssh.ICANON:        1,
		ssh.IEXTEN:        1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}
	if err := session.RequestPty("xterm-256color", int(rows), int(cols), modes); err != nil {
		session.Close()
		p.status = protocol.StatusError
		p.err = fmt.Errorf("请求远程PTY失败: %w", err)
		return p.err
	}

	// 不使用本地 PTY，直接使用管道
	// 这样可以完全控制 I/O 流，避免本地 PTY 的回显问题
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		session.Close()
		p.status = protocol.StatusError
		p.err = fmt.Errorf("创建stdin管道失败: %w", err)
		return p.err
	}
	p.stdinPipe = stdinPipe

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		p.status = protocol.StatusError
		p.err = fmt.Errorf("创建stdout管道失败: %w", err)
		return p.err
	}
	p.stdoutPipe = stdoutPipe

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		session.Close()
		p.status = protocol.StatusError
		p.err = fmt.Errorf("创建stderr管道失败: %w", err)
		return p.err
	}
	p.stderrPipe = stderrPipe

	// 启动远程 shell
	if err := session.Shell(); err != nil {
		session.Close()
		p.status = protocol.StatusError
		p.err = fmt.Errorf("启动远程shell失败: %w", err)
		return p.err
	}

	// 启动持久化管道读取 goroutine（整个会话生命周期只有一组）
	// 避免每次 Attach/Detach 都创建新的读取 goroutine 导致多个 goroutine 竞争同一 pipe
	p.outputCh = make(chan []byte, 256)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.stdoutPipe.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case p.outputCh <- data:
				case <-p.done:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.stderrPipe.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case p.outputCh <- data:
				case <-p.done:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// 在 goroutine 中等待 SSH session 结束
	go func() {
		_ = session.Wait()
		close(p.done)
		p.mu.Lock()
		p.status = protocol.StatusDisconnected
		p.mu.Unlock()
	}()

	return nil
}

// Attach 将 SSH 会话连接到真实终端（前台模式）
// 返回时表示用户已 detach 或 session 已结束
// isResume: true 表示从后台恢复会话，会发送 Ctrl+L 触发远程 shell 重绘
func (p *PTYSession) Attach(stdin io.Reader, stdout io.Writer, isResume bool) error {
	p.mu.Lock()
	if p.stdinPipe == nil || p.stdoutPipe == nil {
		p.mu.Unlock()
		return fmt.Errorf("SSH会话未启动")
	}
	if p.attached {
		p.mu.Unlock()
		return fmt.Errorf("会话已经在前台")
	}

	// 检查 session 是否已结束
	select {
	case <-p.done:
		p.mu.Unlock()
		return fmt.Errorf("SSH会话已结束")
	default:
	}

	p.attached = true
	ctx, cancel := context.WithCancel(context.Background())
	p.ioCancel = cancel
	p.mu.Unlock()

	// 处理上次 detach 时残留的输入数据
	p.mu.Lock()
	if len(p.pendingInput) > 0 {
		p.stdinPipe.Write(p.pendingInput)
		p.pendingInput = p.pendingInput[:0] // 清空
	}
	p.mu.Unlock()

	// 使用 cancelreader 包装 stdin，这样可以在需要时取消阻塞的读取
	// 这是解决字符丢失问题的关键：退出时可以取消正在进行的读取
	cr, err := cancelreader.NewReader(stdin)
	if err != nil {
		cancel()
		return fmt.Errorf("创建 cancelreader 失败: %w", err)
	}
	defer cr.Close()

	// 启动双向 I/O 转发
	// stdin → SSH stdin（用户输入 → SSH）
	p.ioWg.Add(1)
	go func() {
		defer p.ioWg.Done()
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				// 取消正在进行的读取
				cr.Cancel()
				return
			case <-p.done:
				cr.Cancel()
				return
			default:
			}

			// 使用 cancelreader 进行阻塞读取
			n, err := cr.Read(buf)
			if err != nil {
				// 读取被取消或出错，退出
				return
			}
			if n == 0 {
				continue
			}

			// 复制数据
			data := make([]byte, n)
			copy(data, buf[:n])

			// 检测 detach 快捷键 Ctrl+B (0x02)
			for i := 0; i < len(data); i++ {
				if data[i] == 0x02 {
					// 写入 Ctrl+B 之前的数据
					if i > 0 {
						p.stdinPipe.Write(data[:i])
					}
					// 保存 Ctrl+B 之后的数据（如果有）
					if i+1 < len(data) {
						p.mu.Lock()
						p.pendingInput = append(p.pendingInput, data[i+1:]...)
						p.mu.Unlock()
					}
					// 触发 detach
					cancel()
					return
				}
			}

			if _, err := p.stdinPipe.Write(data); err != nil {
				return
			}
		}
	}()

	// SSH 输出 → 用户终端（从持久化 outputCh 通道读取，无 goroutine 泄漏风险）
	p.ioWg.Add(1)
	go func() {
		defer p.ioWg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.done:
				return
			case data := <-p.outputCh:
				if _, err := stdout.Write(data); err != nil {
					return
				}
			}
		}
	}()

	// 如果是恢复会话，发送回车让远程 shell 显示提示符
	// 必须在 I/O 转发启动后发送，这样输出才能被用户看到
	if isResume {
		// 等待一小段时间确保 I/O 转发 goroutine 已经开始工作
		time.Sleep(50 * time.Millisecond)
		p.mu.Lock()
		if p.stdinPipe != nil {
			// 发送回车触发远程 shell 显示提示符
			p.stdinPipe.Write([]byte("\r"))
		}
		p.mu.Unlock()
	}

	// 等待 detach 或 session 结束
	select {
	case <-ctx.Done():
		// 用户触发了 detach
	case <-p.done:
		// SSH session 自然结束
	}

	// 取消 context 并等待 goroutine 退出
	cancel()
	p.ioWg.Wait()

	p.mu.Lock()
	p.attached = false
	p.ioCancel = nil
	p.mu.Unlock()

	return nil
}

// Detach 将会话从终端分离
func (p *PTYSession) Detach() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.attached {
		return fmt.Errorf("会话未在前台")
	}

	if p.ioCancel != nil {
		p.ioCancel()
	}

	return nil
}

// IsAttached 返回是否已附加到终端
func (p *PTYSession) IsAttached() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.attached
}

// IsAlive 检查 SSH 会话是否仍然存活
func (p *PTYSession) IsAlive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// Done 返回会话结束的 channel
func (p *PTYSession) Done() <-chan struct{} {
	return p.done
}

// Connect 已在创建时完成
func (p *PTYSession) Connect() error {
	return nil
}

// Disconnect 断开连接并清理资源
func (p *PTYSession) Disconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 先触发 detach
	if p.ioCancel != nil {
		p.ioCancel()
	}

	// 关闭 stdin 管道
	if p.stdinPipe != nil {
		p.stdinPipe.Close()
		p.stdinPipe = nil
	}

	// 关闭 SSH session
	if p.session != nil {
		p.session.Close()
		p.session = nil
	}

	// 关闭 SSH 连接
	if p.sshClient != nil {
		p.sshClient.Close()
		p.sshClient = nil
	}

	p.status = protocol.StatusDisconnected
	return nil
}

// IsConnected 返回是否已连接
func (p *PTYSession) IsConnected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status == protocol.StatusConnected || p.status == protocol.StatusBackground
}

// GetStatus 返回连接状态
func (p *PTYSession) GetStatus() protocol.ConnectionStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetError 返回连接错误
func (p *PTYSession) GetError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// GetHostID 返回主机标识
func (p *PTYSession) GetHostID() string {
	return p.host.Name
}

// GetStartTime 返回连接开始时间
func (p *PTYSession) GetStartTime() *time.Time {
	return p.startTime
}

// GetDuration 返回连接持续时间
func (p *PTYSession) GetDuration() time.Duration {
	if p.startTime == nil {
		return 0
	}
	return time.Since(*p.startTime)
}

// GetHost 返回主机信息
func (p *PTYSession) GetHost() *models.Host {
	return p.host
}

// ResizeTerminal 调整远程终端大小
func (p *PTYSession) ResizeTerminal(rows, cols int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.session == nil {
		return fmt.Errorf("会话未启动")
	}

	return p.session.WindowChange(rows, cols)
}

// SendCtrlL 发送 Ctrl+L 到远程 shell，触发屏幕重绘
// 用于从后台恢复会话时刷新终端显示
func (p *PTYSession) SendCtrlL() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdinPipe != nil {
		p.stdinPipe.Write([]byte("\x0c")) // Ctrl+L = 0x0c
	}
}
