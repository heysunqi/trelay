package ssh

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// ExecAdapter 将 PTYSession 适配为 tea.ExecCommand 接口
// 实现 Run() / SetStdin() / SetStdout() / SetStderr()
type ExecAdapter struct {
	Session  *PTYSession
	stdin    io.Reader
	stdout   io.Writer
	isResume bool // 是否是从后台恢复的会话
}

// NewExecAdapter 创建 ExecAdapter
// isResume: true 表示从后台恢复会话，false 表示首次连接
func NewExecAdapter(session *PTYSession, isResume bool) *ExecAdapter {
	return &ExecAdapter{
		Session:  session,
		isResume: isResume,
	}
}

// SetStdin 设置标准输入
func (e *ExecAdapter) SetStdin(r io.Reader) {
	e.stdin = r
}

// SetStdout 设置标准输出
func (e *ExecAdapter) SetStdout(w io.Writer) {
	e.stdout = w
}

// SetStderr 设置标准错误
func (e *ExecAdapter) SetStderr(w io.Writer) {
	// SSH session 通过 PTY 合并了 stderr 和 stdout，不需要单独处理
}

// Run 阻塞运行：将终端设为 raw mode，启动 I/O 转发，等待 detach 或 session 结束
func (e *ExecAdapter) Run() error {
	if e.stdin == nil || e.stdout == nil {
		return fmt.Errorf("stdin/stdout 未设置")
	}

	if !e.isResume {
		// 首次连接：清屏并移动光标到左上角
		// 这样 SSH shell 首次连接时内容会展示在屏幕最顶端
		fmt.Print("\033[2J\033[H")
	}

	// 重要：Bubble Tea 的 p.input 可能是 cancelreader，在 ReleaseTerminal 后被取消
	// 我们需要直接使用 os.Stdin 和 os.Stdout，而不是通过 cancelreader
	// 因为 cancelreader 被取消后会立即返回错误

	// 将终端设为 raw mode
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("设置 raw mode 失败: %w", err)
	}
	defer term.Restore(fd, oldState)

	// 监听终端窗口大小变化，同步到远程 PTY
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	go func() {
		for range sigCh {
			rows, cols := getTermSize()
			e.Session.ResizeTerminal(int(rows), int(cols))
		}
	}()

	// 直接使用 os.Stdin 和 os.Stdout，而不是 Bubble Tea 传递的 cancelreader
	// Attach 会阻塞，直到用户 Ctrl+B detach 或 SSH session 结束
	// 如果是恢复会话，Attach 内部会发送 Ctrl+L 让远程 shell 重绘
	return e.Session.Attach(os.Stdin, os.Stdout, e.isResume)
}
