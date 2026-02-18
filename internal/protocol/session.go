package protocol

import (
	"time"
)

// ConnectionStatus 表示连接状态
type ConnectionStatus string

const (
	StatusIdle        ConnectionStatus = "idle"
	StatusConnecting  ConnectionStatus = "connecting"
	StatusConnected   ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusError       ConnectionStatus = "error"
)

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
}
