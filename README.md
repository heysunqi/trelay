# 远程桌面管理器 (Remote Desktop Manager)

一个复古命令行界面的远程桌面管理工具，支持 SSH、RDP、VNC 协议，使用 JSON 配置文件管理主机。

## ✨ 功能特性

- **复古终端界面**：黑底绿字经典终端风格，支持键盘导航
- **多协议支持**：
  - SSH (支持密码和密钥认证)
  - RDP (Windows远程桌面)
  - VNC (虚拟网络控制台)
- **智能搜索**：按主机名、描述、IP地址实时搜索
- **状态监控**：3秒自动检测主机在线状态
- **分组管理**：按组组织主机，支持分组切换
- **配置管理**：JSON格式配置文件，支持热重载
- **直接SSH连接**：支持通过命令行参数直接连接到指定SSH主机（不启动TUI）
- **智能返回**：SSH连接结束后自动返回RDM界面
- **新增连接配置**：按下N或n键，显示交互式对话框，支持配置服务器名称、IP地址、用户名、连接协议、认证方式和分组
- **交互式表单**：
  - 连接协议支持SSH、RDP、VNC的下拉选择
  - SSH协议支持密码和密钥认证方式
  - 服务器分组支持下拉选择和自动创建
  - 完整的输入验证和格式校验
- **界面优化**：主机数量和键盘快捷键提示居中显示在底部

## 🏗️ 设计架构

### 整体架构
```
┌─────────────────────────────────┐
│            CLI/TUI 界面层                │
├─────────────────────────────────────────┤
│             业务逻辑层                   │
├──────────┬──────────┬──────────┬────────┤
│ 配置管理 │ 连接管理 │ 协议抽象 │ 界面渲染│
└──────────┴──────────┴──────────┴────────┘
```

### 技术栈
- **TUI框架**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Go的TUI框架
- **配置管理**: 标准JSON配置，支持多级别日志
- **CLI框架**: [Cobra](https://github.com/spf13/cobra) - 命令行参数解析
- **日志系统**: [Zap](https://go.uber.org/zap) - 高性能日志库，支持多级别输出
- **SSH协议**: `golang.org/x/crypto/ssh` - Go标准SSH库
- **RDP协议**: 计划中
- **VNC协议**: 计划中

## 📁 代码目录结构

```
remote-desktop-manager/
├── cmd/                           # 命令行入口
│   └── rdm/
│       └── main.go               # 程序主入口
├── internal/                      # 内部包（不对外暴露）
│   ├── config/                   # 配置管理
│   │   ├── config.go            # 配置管理器
│   │   └── loader.go            # 配置加载器
│   ├── protocol/                 # 协议实现
│   │   ├── manager.go           # 连接管理器
│   │   ├── session.go           # 会话管理
│   │   └── ssh/                 # SSH协议实现
│   └── ui/                       # 用户界面
│       └── tui/
│           └── app.go           # TUI主逻辑
├── pkg/                          # 可对外暴露的包
│   └── models/                   # 数据模型
│       ├── host.go              # 主机模型
│       └── config.go            # 配置模型
├── configs/                      # 配置文件示例
│   └── config.json.example      # 配置示例文件
├── go.mod                       # Go模块定义
└── README.md                    # 本文档
```

## 🚀 构建与安装

### 环境要求
- Go 1.20+
- Linux 或 macOS 系统

### 从源码构建
```bash
# 克隆项目（如果适用）
# git clone <repository-url>
# cd remote-desktop-manager

# 下载依赖
go mod download

# 构建项目
go build ./cmd/rdm

# 或直接运行
go run ./cmd/rdm
```

### 安装到系统
```bash
# 安装到 $GOPATH/bin 或 $GOBIN
go install ./cmd/rdm

# 确保安装目录在PATH中
export PATH=$PATH:$(go env GOPATH)/bin
```

## 📖 使用方法

### 启动程序
```bash
# 使用默认配置
rdm

# 指定配置文件
rdm --config /path/to/config.json

# 启用调试模式（输出所有日志）
rdm --debug

# 直接连接到指定名称的SSH主机（不启动TUI）
rdm --direct-ssh "主机名称"

# 直接SSH连接后自动返回RDM界面（内部使用）
rdm --direct-ssh "主机名称" --return-to-rdm

# 查看帮助
rdm --help
```

### 界面操作
```
快捷键说明：
┌─────────────────────────────────────────────┐
│ 普通模式：                                   │
│   ↑/↓ : 选择主机                             │
│   Enter : 连接选中主机                       │
│   Tab : 切换分组                             │
│   R : 刷新配置                               │
│   N/n : 新建连接配置                         │
│   / : 进入搜索模式                           │
│   Q : 退出程序                               │
│                                             │
│ 搜索模式：                                   │
│   输入 : 搜索主机                            │
│   Esc : 退出搜索                             │
│   Backspace : 删除字符                       │
│   ←/→ : 移动光标                             │
│                                             │
│ 新建连接对话框：                             │
│   Tab/Shift+Tab : 导航字段                   │
│   Space : 打开下拉选择菜单                   │
│   ↑/↓ : 在下拉菜单中选择                     │
│   Enter : 确认选择/保存配置                  │
│   Esc : 取消操作/关闭菜单                    │
└─────────────────────────────────────────────┘
```

### 命令行参数详细说明
```
Flags:
  -c, --config string       配置文件路径（默认：~/.config/remote-desktop-manager/config.json）
  -d, --debug               启用调试模式（输出详细日志）
      --direct-ssh string   直接连接到指定名称的SSH主机（不启动TUI）
      --return-to-rdm       SSH连接结束后返回RDM界面（内部使用）
  -h, --help                查看帮助信息
```

### 新建连接配置
按下 `N` 或 `n` 键可以打开新建连接配置对话框。这是一个交互式表单，允许您配置新的远程服务器连接。

#### 对话框字段说明
| 字段 | 类型 | 说明 | 验证规则 |
|------|------|------|----------|
| **服务器名称** | 文本 | 必填，主机的唯一标识符 | 不能为空，长度限制 |
| **IP地址** | 文本 | 必填，服务器地址 | 必须是有效的IPv4地址格式 |
| **端口号** | 文本 | 可选，默认使用协议默认端口 | 1-65535之间的整数 |
| **用户名** | 文本 | SSH/RDP必填 | 不能为空 |
| **连接协议** | 下拉菜单 | 必填，选择连接协议类型 | SSH、RDP、VNC |
| **认证方式** | 下拉菜单 | SSH协议必填 | 密码、密钥（仅SSH协议可见） |
| **密码** | 密码 | 可选，根据协议和认证方式显示 | 隐藏输入，最小长度限制 |
| **密钥路径** | 文本 | SSH密钥认证必填 | 有效的文件路径（仅密钥认证可见） |
| **密钥密码** | 密码 | 可选，SSH密钥保护密码 | 隐藏输入（仅密钥认证可见） |
| **服务器分组** | 下拉菜单 | 可选，主机所属分组 | 支持自动创建不存在的分组 |
| **描述信息** | 文本 | 可选，主机描述 | 任意文本 |

#### 使用说明
1. **导航字段**：使用 `Tab` 键前进，`Shift+Tab` 键后退
2. **选择字段**：对于下拉菜单（连接协议、认证方式、服务器分组），按 `Space` 键打开菜单，使用 `↑/↓` 键选择，按 `Enter` 键确认选择
3. **文本输入**：对于文本和密码字段，直接输入内容，支持 `Backspace` 和 `Delete` 键
4. **保存配置**：填写完所有必填字段后，按 `Enter` 键保存配置
5. **取消操作**：按 `Esc` 键可以取消新建连接配置

#### 连接协议、认证方式和分组设置详解

##### 1. 连接协议设置
支持三种远程连接协议：

| 协议 | 说明 | 默认端口 | 字段显示逻辑 |
|------|------|----------|-------------|
| **SSH** | Secure Shell，最常用的远程登录协议 | 22 | 显示用户名、认证方式、密码/密钥路径 |
| **RDP** | Remote Desktop Protocol，Windows远程桌面 | 3389 | 显示用户名、密码、域、屏幕分辨率、颜色深度 |
| **VNC** | Virtual Network Computing，通用远程桌面 | 5900 | 显示密码、只读模式选项 |

**操作步骤**：
- 使用 `Space` 键打开协议选择菜单
- 按 `↑/↓` 键选择所需的协议
- 按 `Enter` 键确认选择
- 对话框会根据选择的协议显示相应的字段

##### 2. 认证方式设置（仅SSH协议）
SSH协议支持两种认证方式：

| 方式 | 说明 | 字段显示逻辑 |
|------|------|-------------|
| **密码** | 使用用户名和密码进行认证 | 显示密码输入字段 |
| **密钥** | 使用SSH密钥对进行认证 | 显示密钥路径和密钥密码字段 |

**操作步骤**：
- 确保连接协议已选择为SSH
- 使用 `Space` 键打开认证方式选择菜单
- 按 `↑/↓` 键选择认证方式
- 按 `Enter` 键确认选择
- 对话框会根据选择的认证方式显示相应的字段

**注意**：
- 密码认证需要填写密码字段
- 密钥认证需要填写密钥路径（默认 ~/.ssh/id_rsa），如果密钥有密码保护，还需要填写密钥密码
- 密钥路径需要是有效的文件路径，程序会验证文件是否存在

##### 3. 服务器分组设置
服务器分组功能帮助您更好地组织和管理主机：

**操作步骤**：
- 使用 `Space` 键打开分组选择菜单
- 使用 `↑/↓` 键选择分组（如果已有分组）
- 或者直接输入新的分组名称
- 如果输入的分组名称不存在，会自动创建新分组

**分组管理**：
- 分组名称可以包含字母、数字、空格和特殊字符
- 主机可以属于多个分组（通过配置文件手动设置）
- 分组信息会保存到配置文件中
- 新建分组会在配置中自动创建对应的分组条目

#### 输入验证和错误提示
对话框具有完整的输入验证功能，确保您输入的数据格式正确：

| 验证类型 | 检查内容 | 错误提示 |
|----------|----------|----------|
| 非空验证 | 必填字段不能为空 | 字段不能为空 |
| IP地址验证 | 主机地址是否为有效的IPv4格式 | 无效的IP地址格式 |
| 端口验证 | 端口号是否在有效范围内 | 无效的端口号（必须在1-65535之间） |
| 文件路径验证 | SSH密钥路径是否存在 | 文件不存在 |
| 格式验证 | 输入格式是否符合要求 | 格式不正确 |

如果输入有误，字段会显示红色边框，并在底部显示具体的错误信息。

#### 状态栏说明
```
主机: 1/2 | 在线: 2/2 | 分组: 2 | 选中: debian-server | 搜索: 'server' | 状态更新: 15:04:05
└───────┘ └────────┘ └──────────────────┘ └───────────────┘ └─────────────┘ └─────────────┘ └───────────────┘ └─────────────┘ └───────────────┘ └─────────────┘ └───────────────┘
 当前/过滤  在线/总数  分组数     当前选中主机        搜索关键词       最后状态检查
```

## ⚙️ 配置文件

### 配置文件位置
默认配置文件路径：`~/.config/remote-desktop-manager/config.json`

### 日志级别配置
程序支持多级别日志输出，通过 `log_level` 字段控制：

| 级别 | 说明 | 输出内容 |
|------|------|----------|
| `debug` | 所有日志 | 调试详细信息，包含所有级别 |
| `info` | 信息及以上 | 常规运行信息，含info、warn、error |
| `warn` | 警告及以上 | 警告和错误信息 |
| `error` | 仅错误 | 默认级别，只输出错误日志 |

**使用示例：**
```json
{
  "log_level": "debug",  // 或 "info", "warn", "error"
  ...
}
```

### 配置示例
```json
{
  "version": "1.0",
  "log_level": "error",
  "default_profile": "web-server",
  "profiles": [
    {
      "name": "web-server-ssh-password",
      "description": "Web服务器 - SSH密码登录",
      "protocol": "ssh",
      "host": "192.168.1.100",
      "port": 22,
      "username": "admin",
      "auth_method": "password",
      "password": "your_password_here"
    },
    {
      "name": "web-server-ssh-key",
      "description": "Web服务器 - SSH密钥登录",
      "protocol": "ssh",
      "host": "192.168.1.101",
      "port": 22,
      "username": "admin",
      "auth_method": "key",
      "key_path": "~/.ssh/id_rsa",
      "passphrase": "your_key_passphrase_here"
    },
    {
      "name": "windows-server-rdp",
      "description": "Windows远程桌面服务器",
      "protocol": "rdp",
      "host": "192.168.1.102",
      "port": 3389,
      "username": "Administrator",
      "password": "windows_password",
      "domain": "WORKGROUP",
      "screen_size": "1920x1080",
      "color_depth": 32
    },
    {
      "name": "kvm-vnc-server",
      "description": "KVM虚拟机VNC控制台",
      "protocol": "vnc",
      "host": "192.168.1.103",
      "port": 5900,
      "password": "vnc_password",
      "view_only": false
    }
  ],
  "groups": [
    {
      "name": "production",
      "profiles": ["web-server-ssh-password", "windows-server-rdp"]
    },
    {
      "name": "development",
      "profiles": ["web-server-ssh-key", "kvm-vnc-server"]
    }
  ],
  "ui": {
    "theme": "retro-green",
    "colors": {
      "background": "#000000",
      "foreground": "#00ff00",
      "selection": "#008800",
      "border": "#00aa00",
      "error": "#ff0000",
      "success": "#00ff00",
      "warning": "#ffff00"
    },
    "keybindings": {
      "up": "up",
      "down": "down",
      "enter": "enter",
      "quit": "q",
      "refresh": "r"
    }
  }
}
```

### 配置字段说明

#### 主机配置 (profiles)
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 主机唯一标识符 |
| `description` | string | 否 | 主机描述信息 |
| `protocol` | string | 是 | 协议类型：ssh, rdp, vnc |
| `host` | string | 是 | 主机地址或IP |
| `port` | integer | 否 | 端口号（默认使用协议默认端口） |
| `username` | string | SSH/RDP必填 | 用户名 |
| `auth_method` | string | SSH可选 | 认证方式：password, key, agent |
| `password` | string | 可选 | 密码（SSH/RDP/VNC） |
| `key_path` | string | SSH密钥可选 | SSH私钥路径 |
| `passphrase` | string | SSH密钥可选 | SSH私钥密码 |
| `domain` | string | RDP可选 | Windows域 |
| `screen_size` | string | RDP可选 | 屏幕分辨率，如"1920x1080" |
| `color_depth` | integer | RDP可选 | 颜色深度：16, 24, 32 |
| `view_only` | boolean | VNC可选 | 是否只读模式 |

#### 分组配置 (groups)
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 分组名称 |
| `profiles` | array | 是 | 包含的主机名称列表 |

#### UI配置 (ui)
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `theme` | string | 否 | 主题名称，默认"retro-green" |
| `colors` | object | 否 | 颜色方案 |
| `keybindings` | object | 否 | 快捷键映射 |

## 🔧 开发指南

### 添加新协议
1. 在 `internal/protocol/manager.go` 中定义协议接口
2. 在 `internal/protocol/` 下创建新协议目录
3. 实现协议接口
4. 在配置模型中添加协议特有字段
5. 在TUI中更新协议图标和显示逻辑

### 添加新功能
1. 在 `pkg/models/` 中添加或修改数据模型
2. 在 `internal/` 下实现业务逻辑
3. 在 `internal/ui/tui/app.go` 中更新界面逻辑
4. 添加相应的键盘快捷键处理

### 测试
```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/config

# 带覆盖率测试
go test -cover ./...
```

## 📝 注意事项

### 安全提醒
⚠️ **重要**：配置文件中的密码以明文形式存储。
- 建议设置配置文件权限为 `600`（仅当前用户可读）
- 生产环境中考虑使用SSH密钥认证
- 后续版本计划添加配置加密功能

### 兼容性
- 支持 Linux 和 macOS 系统
- 需要终端支持UTF-8字符和ANSI颜色
- 建议使用现代终端模拟器（如 kitty, alacritty, iTerm2）

### 性能说明
- 状态检查每3秒执行一次
- 搜索功能实时过滤，性能优化
- 内存占用低，适合长时间运行
- 日志级别可配置，默认只输出error级别，减少日志噪音

## 🤝 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

## 📄 许可证

[待添加]

## 👏 致谢

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 优秀的Go TUI框架
- [Zap](https://go.uber.org/zap) - 高性能日志库
- [Cobra](https://github.com/spf13/cobra) - Go命令行框架

---

**远程桌面管理器** - 复古终端，现代功能，高效管理您的远程连接。
