package models

// Config 表示应用程序的完整配置
type Config struct {
	Version         string          `json:"version"`
	DefaultProfile  string          `json:"default_profile,omitempty"`
	Profiles        []*Host         `json:"profiles"`
	Groups          []*Group        `json:"groups,omitempty"`
	UI              *UIConfig       `json:"ui,omitempty"`
}

// Group 表示主机分组
type Group struct {
	Name     string   `json:"name"`
	Profiles []string `json:"profiles"` // 主机名称列表
}

// UIConfig 表示用户界面配置
type UIConfig struct {
	Theme      string              `json:"theme,omitempty"`
	Colors     *UIColorScheme      `json:"colors,omitempty"`
	Keybindings map[string]string  `json:"keybindings,omitempty"`
}

// UIColorScheme 表示颜色方案
type UIColorScheme struct {
	Background string `json:"background,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Selection  string `json:"selection,omitempty"`
	Border     string `json:"border,omitempty"`
	Error      string `json:"error,omitempty"`
	Success    string `json:"success,omitempty"`
	Warning    string `json:"warning,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Version: "1.0",
		Profiles: []*Host{},
		Groups: []*Group{},
		UI: &UIConfig{
			Theme: "retro-green",
			Colors: &UIColorScheme{
				Background: "#000000",
				Foreground: "#00ff00",
				Selection:  "#008800",
				Border:     "#00aa00",
				Error:      "#ff0000",
				Success:    "#00ff00",
				Warning:    "#ffff00",
			},
			Keybindings: map[string]string{
				"up":    "up",
				"down":  "down",
				"enter": "enter",
				"quit":  "q",
				"refresh": "r",
			},
		},
	}
}

// Validate 验证配置是否有效
func (c *Config) Validate() error {
	// 检查版本
	if c.Version == "" {
		return NewValidationError("配置版本不能为空")
	}

	// 验证所有主机配置
	hostNames := make(map[string]bool)
	for _, host := range c.Profiles {
		if err := host.Validate(); err != nil {
			return err
		}

		// 检查主机名称是否重复
		if hostNames[host.Name] {
			return NewValidationError("主机名称重复: " + host.Name)
		}
		hostNames[host.Name] = true
	}

	// 验证分组
	for _, group := range c.Groups {
		if group.Name == "" {
			return NewValidationError("分组名称不能为空")
		}

		// 检查分组中的主机是否存在
		for _, profileName := range group.Profiles {
			found := false
			for _, host := range c.Profiles {
				if host.Name == profileName {
					found = true
					break
				}
			}
			if !found {
				return NewValidationError("分组" + group.Name + "中的主机不存在: " + profileName)
			}
		}
	}

	// 验证默认配置文件是否存在
	if c.DefaultProfile != "" {
		found := false
		for _, host := range c.Profiles {
			if host.Name == c.DefaultProfile {
				found = true
				break
			}
		}
		if !found {
			return NewValidationError("默认配置文件不存在: " + c.DefaultProfile)
		}
	}

	return nil
}

// FindHostByName 根据名称查找主机
func (c *Config) FindHostByName(name string) *Host {
	for _, host := range c.Profiles {
		if host.Name == name {
			return host
		}
	}
	return nil
}

// GetGroupedHosts 返回按分组组织的主机列表
func (c *Config) GetGroupedHosts() map[string][]*Host {
	result := make(map[string][]*Host)

	// 初始化所有分组
	for _, group := range c.Groups {
		result[group.Name] = []*Host{}
	}

	// 添加"未分组"类别
	result["未分组"] = []*Host{}

	// 将主机分配到分组
	hostInGroup := make(map[string]bool)
	for _, group := range c.Groups {
		for _, profileName := range group.Profiles {
			host := c.FindHostByName(profileName)
			if host != nil {
				result[group.Name] = append(result[group.Name], host)
				hostInGroup[profileName] = true
			}
		}
	}

	// 将未分组的主机添加到"未分组"类别
	for _, host := range c.Profiles {
		if !hostInGroup[host.Name] {
			result["未分组"] = append(result["未分组"], host)
		}
	}

	// 如果没有未分组的主机，删除"未分组"类别
	if len(result["未分组"]) == 0 {
		delete(result, "未分组")
	}

	return result
}