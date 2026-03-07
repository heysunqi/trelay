package protocol

import (
	"errors"
	"io"
	"time"
)

// ConnectionStatus 表示连接状态
type ConnectionStatus string

const (
	StatusIdle         ConnectionStatus = "idle"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusError        ConnectionStatus = "error"
	StatusBackground   ConnectionStatus = "background"
)

// ErrNotSupported 表示协议不支持该操作
var ErrNotSupported = errors.New("该协议不支持此操作")

// Session 表示一个远程会话
type Session interface {
	// Connect 建立连接
	Connect() error

	// Disconnect 断开连接
	Disconnect() error

	// IsConnected 返回是否已连接
	IsConnected() bool

	// GetStatus 返回连接状态
	GetStatus() ConnectionStatus

	// GetError 返回连接错误（如果有）
	GetError() error

	// GetHostID 返回主机标识
	GetHostID() string

	// GetStartTime 返回连接开始时间
	GetStartTime() *time.Time

	// GetDuration 返回连接持续时间
	GetDuration() time.Duration

	// Detach 将会话从终端分离（挂到后台）
	Detach() error

	// Attach 将会话附加到终端（从后台切回前台）
	// isResume: true 表示从后台恢复会话，会触发远程 shell 重绘
	Attach(stdin io.Reader, stdout io.Writer, isResume bool) error

	// IsAttached 返回会话是否已附加到终端
	IsAttached() bool
}
