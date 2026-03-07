# Trelay - 远程连接管理器

一个基于终端的远程连接管理工具，支持 SSH、RDP、VNC 协议，采用复古终端界面风格。
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/4db7e4c5-9ed7-44d6-828a-3cd601e49c04" />

## 功能特性

### 核心功能
- **复古终端界面**：黑底绿字经典终端风格，支持键盘导航
- **多协议支持**：
  - SSH（支持密码和密钥认证）
  - RDP（Windows 远程桌面）
  - VNC（虚拟网络控制台）
- **智能搜索**：按主机名、描述、IP 地址实时搜索
- **状态监控**：自动检测主机在线状态
- **分组管理**：按组组织主机，支持分组切换
- **配置管理**：JSON 格式配置文件，支持热重载
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/e2788543-9850-43c2-b3f9-8bcbb367034a" />
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/64c37e50-772c-4f6d-a25f-c4b6720363cf" />
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/21815b8d-415b-4d92-9846-5d1dcf59cbf7" />
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/f1f39923-3373-4f1b-9030-6cea5ae758cb" />

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
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/d3efa62f-12bd-4387-aa43-dc1a978549df" />

### 外部工具支持
- **RDP**：Linux 优先使用 Remmina，macOS 使用 FreeRDP
- **VNC**：Linux 使用 Remmina/TigerVNC，macOS 使用系统屏幕共享
<img width="1308" height="854" alt="image" src="https://github.com/user-attachments/assets/28b0afd0-aa58-4948-9385-4a149e04e4d3" />

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
├── cmd/trelay/          # 程序入口
├── internal/
│   ├── config/         # 配置管理
│   ├── protocol/       # 协议实现
│   │   ├── ssh/        # SSH 协议
│   │   ├── rdp/        # RDP 协议
│   │   └── vnc/        # VNC 协议
│   └── ui/tui/         # TUI 界面
│       └── dialogs/    # 对话框组件
├── pkg/models/          # 数据模型
└── configs/            # 配置示例
```

## 界面操作

### 快捷键

| 按键 | 功能 |
|------|------|
| `↑/↓` | 选择主机 |
| `Enter` | 连接选中主机 |
| `Tab` | 切换分组 |
| `/` | 进入搜索模式 |
| `N` | 新建连接 |
| `E` | 编辑连接 |
| `G` | 新建分组 |
| `B` | 后台会话列表 |
| `D` | 删除连接 |
| `R` | 刷新配置 |
| `Q` | 退出程序 |

### SSH 会话模式

| 按键 | 功能 |
|------|------|
| `Ctrl+B` | 挂起到后台 |

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
      "profiles": ["web-server", "windows-server"]
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
| auth_method | string | 认证方式：password, key |
| password | string | 密码 |
| key_path | string | SSH 私钥路径 |
| domain | string | RDP 域 |
| screen_size | string | RDP 分辨率 |
| color_depth | int | RDP 颜色深度 |
| view_only | bool | VNC 只读模式 |

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
- 支持 Linux 和 macOS 系统
- 建议使用现代终端模拟器（iTerm2、kitty、alacritty）
