package protocol

import (
	"sync"
	"time"
)

// ConnectionHistory 连接历史记录
type ConnectionHistory struct {
	HostID    string
	Host      string
	Protocol  string
	Username  string
	Timestamp time.Time
	Status    ConnectionStatus
}

// Manager 连接管理器
type Manager struct {
	mu            sync.RWMutex
	sessions      map[string]Session  // hostID -> Session
	activeSession string              // 当前活跃的会话
	history       []ConnectionHistory // 连接历史
	maxHistory    int                 // 最大历史记录数
}

// NewManager 创建连接管理器
func NewManager() *Manager {
	return &Manager{
		sessions:   make(map[string]Session),
		maxHistory: 50, // 保留最近50条历史记录
	}
}

// AddSession 添加后台会话
func (m *Manager) AddSession(session Session) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hostID := session.GetHostID()
	m.sessions[hostID] = session
	m.addToHistory(session, StatusConnected)
}

// RemoveSession 移除会话
func (m *Manager) RemoveSession(hostID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[hostID]; ok {
		if session.IsConnected() {
			session.Disconnect()
		}
		m.addToHistory(session, StatusDisconnected)
		delete(m.sessions, hostID)
		if m.activeSession == hostID {
			m.activeSession = ""
		}
	}
}

// Connect 连接主机
func (m *Manager) Connect(session Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hostID := session.GetHostID()

	// 如果已有连接，先断开
	if existing, ok := m.sessions[hostID]; ok && existing.IsConnected() {
		_ = existing.Disconnect()
	}

	// 记录到历史
	m.addToHistory(session, StatusConnecting)

	// 执行连接
	err := session.Connect()
	if err != nil {
		m.addToHistory(session, StatusError)
		return err
	}

	m.sessions[hostID] = session
	m.activeSession = hostID
	m.addToHistory(session, StatusConnected)

	return nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect(hostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[hostID]; ok {
		if session.IsConnected() {
			err := session.Disconnect()
			m.addToHistory(session, StatusDisconnected)
			delete(m.sessions, hostID)
			if hostID == m.activeSession {
				m.activeSession = ""
			}
			return err
		}
	}

	return nil
}

// GetSession 获取会话
func (m *Manager) GetSession(hostID string) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[hostID]
	return session, ok
}

// GetActiveSession 获取当前活跃会话
func (m *Manager) GetActiveSession() (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeSession == "" {
		return nil, false
	}
	session, ok := m.sessions[m.activeSession]
	return session, ok
}

// GetHistory 获取连接历史
func (m *Manager) GetHistory() []ConnectionHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.history
}

// ClearHistory 清空连接历史
func (m *Manager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = nil
}

// GetActiveSessions 获取所有活跃会话
func (m *Manager) GetActiveSessions() []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []Session
	for _, session := range m.sessions {
		if session.IsConnected() {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// GetBackgroundSessions 获取所有后台会话（已连接但未附加到终端）
func (m *Manager) GetBackgroundSessions() []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []Session
	for _, session := range m.sessions {
		if session.IsConnected() && !session.IsAttached() {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// GetBackgroundCount 获取后台会话数量
func (m *Manager) GetBackgroundCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, session := range m.sessions {
		if session.IsConnected() && !session.IsAttached() {
			count++
		}
	}
	return count
}

// CleanupDeadSessions 清理已断开的会话
func (m *Manager) CleanupDeadSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for hostID, session := range m.sessions {
		if !session.IsConnected() {
			delete(m.sessions, hostID)
			if m.activeSession == hostID {
				m.activeSession = ""
			}
		}
	}
}

// DisconnectAll 断开所有连接
func (m *Manager) DisconnectAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for hostID, session := range m.sessions {
		if session.IsConnected() {
			if err := session.Disconnect(); err != nil {
				lastErr = err
			}
		}
		delete(m.sessions, hostID)
	}

	m.activeSession = ""
	return lastErr
}

// addToHistory 添加到历史记录
func (m *Manager) addToHistory(session Session, status ConnectionStatus) {
	// 获取主机信息（这里需要从session获取，暂时简化处理）
	history := ConnectionHistory{
		HostID:    session.GetHostID(),
		Timestamp: time.Now(),
		Status:    status,
	}

	m.history = append([]ConnectionHistory{history}, m.history...)
	if len(m.history) > m.maxHistory {
		m.history = m.history[:m.maxHistory]
	}
}
