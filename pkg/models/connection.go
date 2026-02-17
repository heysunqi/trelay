package models

import (
	"time"
)

// ConnectionStatus 表示连接状态
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusError        ConnectionStatus = "error"
	StatusReconnecting ConnectionStatus = "reconnecting"
)

// Connection 表示一个活跃的连接
type Connection struct {
	Host        *Host
	Status      ConnectionStatus
	StartTime   time.Time
	EndTime     *time.Time
	Error       error
	SessionID   string // 会话标识符
	Pid         int    // 子进程PID（如果适用）
}

// NewConnection 创建新连接
func NewConnection(host *Host) *Connection {
	return &Connection{
		Host:      host,
		Status:    StatusDisconnected,
		StartTime: time.Now(),
	}
}

// Connect 标记为连接中
func (c *Connection) Connect() {
	c.Status = StatusConnecting
	c.StartTime = time.Now()
	c.Error = nil
}

// Connected 标记为已连接
func (c *Connection) Connected(sessionID string, pid int) {
	c.Status = StatusConnected
	c.SessionID = sessionID
	c.Pid = pid
}

// Disconnect 断开连接
func (c *Connection) Disconnect() {
	now := time.Now()
	c.Status = StatusDisconnected
	c.EndTime = &now
}

// SetError 设置错误状态
func (c *Connection) SetError(err error) {
	c.Status = StatusError
	c.Error = err
	now := time.Now()
	c.EndTime = &now
}

// IsActive 检查连接是否活跃
func (c *Connection) IsActive() bool {
	return c.Status == StatusConnected || c.Status == StatusConnecting
}

// Duration 返回连接持续时间
func (c *Connection) Duration() time.Duration {
	if c.EndTime != nil {
		return c.EndTime.Sub(c.StartTime)
	}
	return time.Since(c.StartTime)
}