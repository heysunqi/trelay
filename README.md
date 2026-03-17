# Trelay - 远程连接管理器

一个基于终端的远程连接管理工具，支持 SSH、RDP、VNC 协议，采用复古终端界面风格。
<img width="1512" height="949" alt="image" src="https://github.com/user-attachments/assets/c9bcf97a-bdd7-4a07-836f-5c815efaaa23" />

## 功能特性

### 核心功能
- **复古终端界面**：黑底绿字经典终端风格，支持键盘导航
- **多协议支持**：
  - SSH（支持密码和密钥认证）
  - RDP（Windows 远程桌面）
  - VNC（虚拟网络控制台）
- **智能搜索**：按主机名、描述、IP 地址实时搜索
- **状态监控**：自动检测主机在线状态
- **分组管理**：按组组织主机，分组按字母序排列，未分组主机始终在最后
- **配置管理**：JSON 格式配置文件，支持热重载
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/e2788543-9850-43c2-b3f9-8bcbb367034a" />
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/64c37e50-772c-4f6d-a25f-c4b6720363cf" />
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/21815b8d-415b-4d92-9846-5d1dcf59cbf7" />
<img width="1512" height="949" alt="image" src="https://github.com/user-attachments/assets/344ab523-b6d8-4378-aa1b-6936417ab50a" />


### 连接方式
- **直接连接**：支持命令行参数直接连接（绕过 TUI）
  ```bash
  trelay --direct-ssh "hostname"
  trelay --direct-rdp "hostname"
  trelay --direct-vnc "hostname"
  ```

### SSH 会话
- **后台化支持**：按 `Ctrl+B` 挂起会话，随时切回继续操作
- **后台会话管理**：查看、切回或断开后台会话
- **连接取消**：在等待连接时按 `ESC` 可终止正在进行的连接尝试
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/d3efa62f-12bd-4387-aa43-dc1a978549df" />

### SSH 密钥管理
- **灵活的密钥配置**：
  - 支持使用已有密钥文件路径
  - 支持直接粘贴密钥内容，自动保存到本地
- **自动密钥托管**：粘贴的密钥自动保存到 `~/.config/trelay/keys/` 目录
- **密钥文件命名**：`<主机名>_<随机字符串>_key.pem` 格式，便于识别
- **安全存储**：密钥文件权限自动设置为 `0600`，目录权限 `0700`

### 外部工具支持
- **RDP**：Linux 优先使用 Remmina，macOS 使用 FreeRDP
- **VNC**：Linux 使用 Remmina/TigerVNC，macOS 使用系统屏幕共享
<img width="1512" height="949" alt="image" src="https://github.com/user-attachments/assets/97e13140-cd61-4e0c-bf23-321aa09bbb38" />


## 技术栈

| 类别 | 技术 |
|------|------|
| 编程语言 | Go 1.25 |
| TUI 框架 | Bubble Tea |
| 样式库 | Lipgloss |
| CLI 框架 | Cobra |
| 配置管理 | Viper |
| 日志系统 | Uber Zap |
| SSH 库 | golang.org/x/crypto/ssh |

## 快速开始

### 构建

```bash
make build
```

### 运行

```bash
# 启动 TUI
./trelay

# 指定配置文件
./trelay --config /path/to/config.json

# 启用调试模式
./trelay --debug
```

### 命令行参数

```
-c, --config string      配置文件路径（默认：~/.config/trelay/config.json）
-d, --debug              启用调试模式
    --direct-ssh        直接 SSH 连接（不启动 TUI）
    --direct-rdp        直接 RDP 连接（不启动 TUI）
    --direct-vnc        直接 VNC 连接（不启动 TUI）
    --return-to-trelay   连接结束后返回 TUI
-p, --password string   连接密码
-h, --help              查看帮助
```

## 项目结构

```
trelay/
├── cmd/trelay/
│   └── main.go              # 程序入口
├── internal/
│   ├── config/              # 配置管理
│   │   ├── config.go        # 配置结构与加载
│   │   └── loader.go        # 配置加载器
│   ├── keymgr/              # SSH 密钥管理
│   │   └── manager.go       # 密钥文件管理
│   ├── protocol/            # 协议抽象层
│   │   ├── session.go       # Session 接口定义
│   │   ├── manager.go       # 连接管理器
│   │   ├── ssh/             # SSH 协议实现
│   │   │   ├── client.go         # SSH 客户端
│   │   │   ├── pty_session.go    # PTY 会话（支持后台挂起）
│   │   │   └── exec_adapter.go   # Bubble Tea 执行适配器
│   │   ├── rdp/             # RDP 协议实现
│   │   │   ├── client.go         # RDP 客户端
│   │   │   ├── detector.go       # RDP 工具检测
│   │   │   ├── selector.go       # 平台选择器
│   │   │   ├── builder.go        # 命令构建器
│   │   │   ├── freerdp_builder.go # FreeRDP 实现
│   │   │   └── remmina_builder.go # Remmina 实现
│   │   └── vnc/             # VNC 协议实现
│   │       ├── client.go         # VNC 客户端
│   │       ├── detector.go       # VNC 工具检测
│   │       ├── builder.go        # 命令构建器
│   │       └── tigervnc_builder.go # TigerVNC 实现
│   └── ui/
│       ├── dialogs/              # 对话框组件
│       │   ├── new_connection.go  # 新建连接对话框
│       │   ├── edit_connection.go # 编辑连接对话框
│       │   ├── new_group.go       # 新建分组对话框
│       │   ├── password.go        # 密码输入对话框
│       │   └── error.go            # 错误提示对话框
│       └── tui/                   # TUI 主界面
│           ├── app.go              # 主应用程序
│           ├── state.go            # 状态接口定义
│           ├── normal_state.go     # 普通模式
│           ├── search_state.go     # 搜索模式
│           ├── command_state.go    # 命令模式
│           ├── group_select_state.go # 分组选择模式
│           └── session_list_state.go # 后台会话列表
└── pkg/models/                # 数据模型
    ├── host.go                # 主机配置模型
    ├── config.go              # 配置模型
    └── connection.go          # 连接模型
```

## 界面操作

### 主界面快捷键（普通模式）

| 按键 | 功能 |
|------|------|
| `↑` / `↓` / `j` / `k` | 选择主机 |
| `Enter` | 连接选中主机 |
| `←` / `→` / `h` / `l` | 上/下翻页 |
| `Tab` | 切换分组 |
| `/` | 进入搜索模式 |
| `:` | 进入命令模式 |
| `N` | 新建连接 |
| `E` | 编辑连接 |
| `D` / `Delete` | 删除连接 |
| `G` | 新建分组 |
| `B` | 后台会话列表 |
| `R` | 刷新配置和状态 |
| `Ctrl+C` | 退出程序（第一次提示，第二次确认） |

### 搜索模式

| 按键 | 功能 |
|------|------|
| 任意字符 | 输入搜索关键词 |
| `Enter` | 退出搜索模式，保留搜索结果，进入列表区域 |
| `Esc` | 退出搜索模式，清空搜索词，恢复全部列表 |

### 命令模式

按 `:` 进入命令模式，支持以下命令：

| 命令 | 功能 |
|------|------|
| `q` + `Enter` | 退出程序 |
| `group` + `Enter` | 进入分组选择模式 |
| 其他 | 回车后无操作，返回普通模式 |

| 按键 | 功能 |
|------|------|
| `Esc` | 退出命令模式，返回普通模式 |
| `Enter` | 执行当前输入的命令 |
| `Ctrl+C` | 忽略（不退出） |

### 分组选择模式

| 按键 | 功能 |
|------|------|
| `↑` / `↓` / `j` / `k` | 选择分组 |
| `Enter` | 确认选择，切换到选中分组 |
| `Esc` / `q` | 退出分组选择，返回普通模式 |
| `/` | 进入分组搜索子模式 |

### 后台会话模式

| 按键 | 功能 |
|------|------|
| `↑` / `↓` / `j` / `k` | 选择会话 |
| `Enter` | 恢复选中的后台会话 |
| `d` / `D` | 断开选中的后台会话 |
| `Esc` / `q` | 退出后台会话列表 |

### SSH 会话模式

| 按键 | 功能 |
|------|------|
| `Ctrl+B` | 挂起会话到后台 |

### 连接等待界面

| 按键 | 功能 |
|------|------|
| `ESC` | 取消正在进行的连接 |

## 配置文件

### 配置位置

默认路径：`~/.config/trelay/config.json`

### 配置示例

```json
{
  "version": "1.0",
  "log_level": "error",
  "profiles": [
    {
      "name": "web-server",
      "description": "Web 服务器",
      "protocol": "ssh",
      "host": "192.168.1.100",
      "port": 22,
      "username": "admin",
      "auth_method": "password",
      "password": "your_password"
    },
    {
      "name": "db-server",
      "description": "数据库服务器",
      "protocol": "ssh",
      "host": "192.168.1.101",
      "port": 22,
      "username": "admin",
      "auth_method": "key",
      "key_path": "~/.ssh/id_rsa",
      "passphrase": "key_passphrase"
    },
    {
      "name": "bastion",
      "description": "跳板机",
      "protocol": "ssh",
      "host": "bastion.example.com",
      "port": 22,
      "username": "admin",
      "auth_method": "key",
      "key_path": "~/.ssh/id_rsa"
    },
    {
      "name": "internal-server",
      "description": "内网服务器（通过跳板机）",
      "protocol": "ssh",
      "host": "192.168.1.200",
      "port": 22,
      "username": "admin",
      "auth_method": "password",
      "password": "your_password",
      "connect_via": "proxyjump",
      "proxy_jump": "bastion"
    },
    {
      "name": "windows-server",
      "description": "Windows 服务器",
      "protocol": "rdp",
      "host": "192.168.1.102",
      "port": 3389,
      "username": "Administrator",
      "password": "windows_password"
    }
  ],
  "groups": [
    {
      "name": "production",
      "profiles": ["web-server", "db-server", "windows-server"]
    }
  ]
}
```

### 字段说明

**主机配置 (profiles)**

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 主机唯一标识 |
| description | string | 主机描述 |
| protocol | string | 协议：ssh, rdp, vnc |
| host | string | 主机地址 |
| port | int | 端口号 |
| username | string | 用户名（SSH/RDP） |
| auth_method | string | 认证方式：password, key, agent |
| password | string | 密码 |
| key_path | string | SSH 私钥路径 |
| passphrase | string | SSH 私钥密码 |
| domain | string | RDP 域 |
| screen_size | string | RDP 分辨率（如 "1920x1080"） |
| color_depth | int | RDP 颜色深度（如 16, 24, 32） |
| view_only | bool | VNC 只读模式 |
| connect_via | string | 连接方式：direct, proxyjump, proxyserver |
| proxy_jump | string | 跳板机名称（引用配置中的其他主机） |
| proxy_host | string | 代理服务器地址 |
| proxy_port | int | 代理服务器端口 |
| proxy_user | string | 代理服务器用户名 |
| proxy_auth_method | string | 代理服务器认证方式 |
| proxy_password | string | 代理服务器密码 |
| proxy_key_path | string | 代理服务器密钥路径 |

## SSH 密钥管理

### 密钥配置方式

在新建或编辑 SSH 连接时，选择密钥认证后会显示"已导入密钥"选项：

| 选项 | 说明 |
|------|------|
| 是 | 使用已有的密钥文件路径（如 `~/.ssh/id_rsa`） |
| 否 | 直接粘贴密钥内容，自动保存到本地 |

### 托管密钥目录

粘贴的密钥内容会自动保存到 `~/.config/trelay/keys/` 目录：

```
~/.config/trelay/keys/
├── web-server_a1b2c3_key.pem
├── db-server_d4e5f6_key.pem
└── ...
```

**文件命名规则**：`<主机名>_<6位随机字符>_key.pem`

### 安全说明

- 托管密钥目录权限：`0700`
- 密钥文件权限：`0600`
- 密钥内容必须是有效的 PEM 格式（包含 `-----BEGIN` 和 `-----END` 标识）

**分组配置 (groups)**

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 分组名称 |
| profiles | array | 主机名称列表 |

## 开发

### 添加新协议

1. 在 `internal/protocol/` 下创建新协议目录
2. 实现 `Session` 接口
3. 更新配置模型和 TUI

### 测试

```bash
go test ./...
```

## 注意事项

- 配置文件中的密码以明文存储，建议设置文件权限为 `600`
- 托管的 SSH 密钥自动设置安全权限（文件 `0600`，目录 `0700`）
- 支持 Linux 和 macOS 系统
- 建议使用现代终端模拟器（iTerm2、kitty、alacritty）
