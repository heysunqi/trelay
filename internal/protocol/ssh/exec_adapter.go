package ssh

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// ExecAdapter 将 PTYSession 适配为 tea.ExecCommand 接口
// 实现 Run() / SetStdin() / SetStdout() / SetStderr()
type ExecAdapter struct {
	Session *PTYSession
	stdin   io.Reader
	stdout  io.Writer
}

// NewExecAdapter 创建 ExecAdapter
func NewExecAdapter(session *PTYSession) *ExecAdapter {
	return &ExecAdapter{
		Session: session,
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

	// 直接使用 os.Stdin 和 os.Stdout，而不是 Bubble Tea 传递的 cancelreader
	// Attach 会阻塞，直到用户 Ctrl+B detach 或 SSH session 结束
	return e.Session.Attach(os.Stdin, os.Stdout)
}
