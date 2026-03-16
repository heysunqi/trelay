package models

// Host 表示一个远程主机配置
type Host struct {
	// 基本信息
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Protocol    string `json:"protocol"` // ssh, rdp, vnc

	// 网络连接信息
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`

	// 认证信息
	Username   string `json:"username,omitempty"`
	AuthMethod string `json:"auth_method,omitempty" mapstructure:"auth_method"` // password, key, agent
	Password   string `json:"password,omitempty"`
	KeyPath    string `json:"key_path,omitempty" mapstructure:"key_path"`
	Passphrase string `json:"passphrase,omitempty"` // SSH密钥密码

	// RDP特定字段
	Domain     string `json:"domain,omitempty"`
	ScreenSize string `json:"screen_size,omitempty" mapstructure:"screen_size"` // 例如 "1920x1080"
	ColorDepth int    `json:"color_depth,omitempty" mapstructure:"color_depth"` // 例如 16, 24, 32

	// VNC特定字段
	ViewOnly bool `json:"view_only,omitempty" mapstructure:"view_only"`

	// SSH 代理配置
	ConnectVia      string `json:"connect_via,omitempty" mapstructure:"connect_via"`           // 连接方式: direct, proxyjump, proxyserver
	ProxyJump       string `json:"proxy_jump,omitempty" mapstructure:"proxy_jump"`             // 跳板机名称（引用配置中的其他主机）
	ProxyHost       string `json:"proxy_host,omitempty" mapstructure:"proxy_host"`             // 代理服务器地址
	ProxyPort       int    `json:"proxy_port,omitempty" mapstructure:"proxy_port"`             // 代理服务器端口
	ProxyUser       string `json:"proxy_user,omitempty" mapstructure:"proxy_user"`             // 代理服务器用户名
	ProxyAuthMethod string `json:"proxy_auth_method,omitempty" mapstructure:"proxy_auth_method"` // 代理服务器认证方式
	ProxyPassword   string `json:"proxy_password,omitempty" mapstructure:"proxy_password"`       // 代理服务器密码
	ProxyKeyPath    string `json:"proxy_key_path,omitempty" mapstructure:"proxy_key_path"`       // 代理服务器密钥路径

	// 连接选项
	Options map[string]interface{} `json:"options,omitempty"`

	// 内部状态（不序列化到JSON）
	Status      string `json:"-"` // online, offline, connecting, connected
	LastConnect string `json:"-"` // 最后连接时间
}

// DefaultPort 返回协议的默认端口
func (h *Host) DefaultPort() int {
	switch h.Protocol {
	case "ssh":
		return 22
	case "rdp":
		return 3389
	case "vnc":
		return 5900
	default:
		return 0
	}
}

// GetPort 获取端口，如果未设置则返回默认端口
func (h *Host) GetPort() int {
	if h.Port > 0 {
		return h.Port
	}
	return h.DefaultPort()
}

// Validate 验证主机配置是否有效
func (h *Host) Validate() error {
	if h.Name == "" {
		return NewValidationError("主机名称不能为空")
	}
	if h.Protocol == "" {
		return NewValidationError("协议类型不能为空")
	}
	if h.Host == "" {
		return NewValidationError("主机地址不能为空")
	}

	// 验证协议类型
	switch h.Protocol {
	case "ssh":
		if h.Username == "" {
			return NewValidationError("SSH协议需要用户名")
		}
	case "rdp":
		if h.Username == "" {
			return NewValidationError("RDP协议需要用户名")
		}
	case "vnc":
		// VNC只需要密码，可能不需要用户名
	default:
		return NewValidationError("不支持的协议类型: " + h.Protocol)
	}

	return nil
}

// ValidationError 验证错误
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) error {
	return &ValidationError{Message: message}
}
