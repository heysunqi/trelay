# Trelay TUI 重构计划

## Context

trelay 的 TUI 层目前全部集中在 [app.go](internal/ui/tui/app.go)（2306 行），随着功能增加维护成本急剧上升。本次重构通过引入设计模式，将代码拆分为职责清晰的模块，在不改变用户体验的前提下降低维护难度。同时修复已发现的 Bug 并实现缺失的删除功能。

---

## 一、现状梳理（仅供参考，不执行）

### 1.1 快捷键全景

| 模式 | 快捷键 | 功能 | 位置 |
|------|--------|------|------|
| **普通** | `q`/`Ctrl+C` | 退出 | [app.go:1155](internal/ui/tui/app.go#L1155) |
| | `:` | 命令模式 | [app.go:1163](internal/ui/tui/app.go#L1163) |
| | `/` | 搜索模式 | [app.go:1169](internal/ui/tui/app.go#L1169) |
| | `↑`/`k`, `↓`/`j` | 上下选择 | [app.go:1179](internal/ui/tui/app.go#L1179) |
| | `←`/`h`, `→`/`l` | 翻页 | [app.go:1205](internal/ui/tui/app.go#L1205) |
| | `Enter` | 连接 | [app.go:1225](internal/ui/tui/app.go#L1225) |
| | `Tab` | 切换分组 | [app.go:1233](internal/ui/tui/app.go#L1233) |
| | `N`/`n` | 新建连接 | [app.go:1250](internal/ui/tui/app.go#L1250) |
| | `G`/`g` | 新建分组 | [app.go:1266](internal/ui/tui/app.go#L1266) |
| | `B`/`b` | 后台会话 | [app.go:1271](internal/ui/tui/app.go#L1271) |
| | `E`/`e` | 编辑连接 | [app.go:1279](internal/ui/tui/app.go#L1279) |
| | `r` | 刷新 | [app.go:1299](internal/ui/tui/app.go#L1299) |
| | `D`/`d` | **删除（未实现）** | 头部显示但无处理器 |
| **搜索** | `Esc` | 退出清空 | [app.go:1129](internal/ui/tui/app.go#L1129) |
| | `Enter` | 确认保留 | [app.go:1136](internal/ui/tui/app.go#L1136) |
| **命令** | `Esc` | 退出 | [app.go:1097](internal/ui/tui/app.go#L1097) |
| | `Enter` | 执行(仅`group`) | [app.go:1102](internal/ui/tui/app.go#L1102) |
| **分组选择** | `Esc`/`q` | 退出 | [app.go:1043](internal/ui/tui/app.go#L1043) |
| | `↑`/`k`,`↓`/`j` | 导航 | [app.go:1054](internal/ui/tui/app.go#L1054) |
| | `/` | 搜索 | [app.go:1066](internal/ui/tui/app.go#L1066) |
| | `Enter` | 选择 | [app.go:1070](internal/ui/tui/app.go#L1070) |
| **会话列表** | `Esc`/`q` | 关闭 | [app.go:554](internal/ui/tui/app.go#L554) |
| | `↑↓`/`jk` | 导航 | [app.go:557](internal/ui/tui/app.go#L557) |
| | `Enter` | 重挂载 | [app.go:569](internal/ui/tui/app.go#L569) |
| | `d`/`D` | 断开会话 | [app.go:578](internal/ui/tui/app.go#L578) |
| **连接中** | `Esc` | 取消 | [app.go:607](internal/ui/tui/app.go#L607) |
| **SSH交互** | `Ctrl+B` | 后台分离 | pty_session.go |

### 1.2 问题清单

| # | 问题 | 严重度 |
|---|------|--------|
| P1 | `Update()` 764 行，混合状态分发/按键/业务逻辑/持久化 | 高 |
| P2 | 11 个 bool 标志隐式状态机，无互斥保护 | 高 |
| P3 | 分组选择退出清理逻辑重复 3 次 | 中 |
| P4 | NewConnectionDialog 与 EditConnectionDialog ~80% 重复(1448+1539行) | 高 |
| P5 | 对话框关闭处理直接操作 config 并 Save(257行业务逻辑在Update中) | 高 |
| P6 | `View()` 修改 pageSize/paginator 状态，违反纯函数约定 | 中 |
| P7 | `renderHostItem()` 显示宽度当字节索引，多字节字符越界 | 中 |
| B1 | 头部显示 "D 删除" 但无处理器 | Bug |
| B2 | 头部显示 "R" 但只处理 `r` | Bug |
| B3 | connecting 状态非 Esc 键泄漏到普通模式 | Bug |

---

## 二、重构方案（执行计划）

### Phase 0: Bug 修复 + 实现删除功能

**修改文件：** [app.go](internal/ui/tui/app.go)

#### 0a. 实现 D/Delete 删除连接功能（B1）

在普通模式 switch（[app.go:1155](internal/ui/tui/app.go#L1155)）中新增 `case "D", "d", "delete":` ：

1. 获取当前选中主机 `host := a.filteredHosts[a.selected]`
2. 新增 `showDeleteConfirm bool` + `deleteTargetHost *models.Host` 字段（Phase 1 中会转为状态枚举）
3. 利用现有 `ErrorDialog` 实现确认弹窗（复用 [error.go](internal/ui/dialogs/error.go)，将提示文本改为 `"确认删除连接 'xxx'？按 Enter 确认，Esc 取消"`），或新建一个简单的 `ConfirmDialog`
4. 确认后执行删除逻辑：
   - 从 `a.config.Profiles` 中移除该 host
   - 从所有 `a.config.Groups[].Profiles` 中移除该 host.Name 引用
   - 调用 `a.configMgr.Save(a.config)`
   - 调用 `a.refreshHosts()`
   - 钳制 `a.selected` 防止越界

**建议方案**：新建一个轻量的 `ConfirmDialog`（[internal/ui/dialogs/confirm.go](internal/ui/dialogs/confirm.go)），结构类似现有的 `ErrorDialog`（[error.go](internal/ui/dialogs/error.go) 仅 139 行），添加 `IsConfirmed() bool` 方法。这样后续编辑/新建等也可以复用确认弹窗。

#### 0b. 修复 R 大小写不一致（B2）

[app.go:1299](internal/ui/tui/app.go#L1299): `case "r":` → `case "r", "R":`

#### 0c. 修复 connecting 状态按键泄漏（B3）

[app.go:602-617](internal/ui/tui/app.go#L602-L617): 在 `if a.connecting` 块中，Esc 处理之后添加：
```go
// 连接中状态屏蔽所有其他按键
if _, ok := msg.(tea.KeyMsg); ok {
    return a, cmd
}
```

#### 0d. 修复 renderHostItem 字节索引 Bug（P7）

[app.go:2077-2088](internal/ui/tui/app.go#L2077-L2088): 改为逐列独立渲染+拼接，不再构建完整行字符串后按字节位置切片。每列单独 `lipgloss.NewStyle().Width(w).Render(text)`，状态列单独着色后与其他列 join。

---

### Phase 1: 显式状态机 + 文件拆分（State Pattern）

**解决：P1（764行Update）、P2（11个bool标志）、P3（重复清理逻辑）**

#### 1a. 新建 `state.go` — 状态枚举 + 转换

```go
type AppState int
const (
    StateNormal AppState = iota
    StateSearchMode
    StateCommandMode
    StateGroupSelect
    StateGroupSearch
    StateConnecting
    StatePasswordDialog
    StateErrorDialog
    StateNewConnectionDialog
    StateNewGroupDialog
    StateEditDialog
    StateSessionList
    StateDeleteConfirm      // Phase 0 新增
)
```

在 `App` 结构体中：
- 删除所有 `show*` bool 字段和 `*Mode` bool 字段
- 新增 `state AppState`
- 新增辅助方法：

```go
func (a *App) enterState(s AppState)  // 设置 a.state = s
func (a *App) exitToNormal()          // 重置为 StateNormal，清理临时状态
func (a *App) exitGroupSelect()       // 提取的清理逻辑（解决 P3）
```

#### 1b. 拆分文件

将 [app.go](internal/ui/tui/app.go) 的 2306 行拆分为：

| 新文件 | 内容 | 来源行号 |
|--------|------|----------|
| [app.go](internal/ui/tui/app.go) | App 结构体、NewApp()、Init()、Run() | 58-190, 2277-2306 |
| [state.go](internal/ui/tui/state.go) | AppState 枚举、状态转换方法 | 新建 |
| [messages.go](internal/ui/tui/messages.go) | 所有自定义 tea.Msg 类型 | 276-295 |
| [update.go](internal/ui/tui/update.go) | Update() 分发 + handleGlobalMsg() | 549-551, 943-1009 |
| [update_normal.go](internal/ui/tui/update_normal.go) | updateNormal() — 普通模式按键 | 1155-1308 |
| [update_dialog.go](internal/ui/tui/update_dialog.go) | updatePasswordDialog/ErrorDialog/NewConn/NewGroup/EditConn/DeleteConfirm | 620-941 |
| [update_modes.go](internal/ui/tui/update_modes.go) | updateSearchMode/CommandMode/GroupSelect/GroupSearch/SessionList/Connecting | 551-617, 1012-1151 |
| [view.go](internal/ui/tui/view.go) | View() 分发 | 1314-1436 |
| [view_normal.go](internal/ui/tui/view_normal.go) | renderHeader/TableHeader/HostListWithHeight/HostItem/StatusBar/CommandInput | 1438-2164 |
| [view_overlay.go](internal/ui/tui/view_overlay.go) | renderConnectingView/SessionList/GroupSelectWithHeight | 1574-1694, 2166-2275 |
| [connection.go](internal/ui/tui/connection.go) | executeSSHConnection/ExternalConnection/attachSSHSession/checkHostStatusAsync/promptRestart | 384-535 |
| [helpers.go](internal/ui/tui/helpers.go) | displayWidth/truncateByDisplayWidth/stripANSI/getColumnWidths/shouldShowDescription/unescapeDescription | 1696-1829, 2108-2123 |

**Update() 分发核心结构（update.go）：**

```go
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // 1. 全局消息不分状态都处理
    if model, cmd, handled := a.handleGlobalMsg(msg); handled {
        return model, cmd
    }
    // 2. 按状态分发
    switch a.state {
    case StateSessionList:          return a.updateSessionList(msg)
    case StateConnecting:           return a.updateConnecting(msg)
    case StatePasswordDialog:       return a.updatePasswordDialog(msg)
    case StateErrorDialog:          return a.updateErrorDialog(msg)
    case StateNewConnectionDialog:  return a.updateNewConnectionDialog(msg)
    case StateNewGroupDialog:       return a.updateNewGroupDialog(msg)
    case StateEditDialog:           return a.updateEditDialog(msg)
    case StateDeleteConfirm:        return a.updateDeleteConfirm(msg)
    case StateGroupSearch:          return a.updateGroupSearch(msg)
    case StateGroupSelect:          return a.updateGroupSelect(msg)
    case StateCommandMode:          return a.updateCommandMode(msg)
    case StateSearchMode:           return a.updateSearchMode(msg)
    default:                        return a.updateNormal(msg)
    }
}
```

**handleGlobalMsg 处理以下消息类型（不依赖状态）：**
- `tea.WindowSizeMsg` → 更新 width/height/ready
- `statusCheckMsg` → 触发异步状态检查
- `hostStatusResult` → 更新主机状态
- `spinner.TickMsg` → 仅在 connecting 时更新 spinner
- `sshConnectResultMsg` → SSH 连接完成
- `sshSessionMsg` → SSH 会话返回

---

### Phase 2: View 纯函数化

**解决：P6（View 中修改状态）**

**修改文件：** [update.go](internal/ui/tui/update.go)、[view.go](internal/ui/tui/view.go)

1. 新增 `recalcLayout()` 方法，计算 `contentHeight` 和 `pageSize`，在以下时机调用：
   - `handleGlobalMsg` 中处理 `WindowSizeMsg` 时
   - `refreshHosts()` 末尾
   - 任何改变 `searchBoxVisible`/`commandMode` 等影响布局的状态转换时

2. `View()` 中删除 [app.go:1390-1417](internal/ui/tui/app.go#L1390-L1417) 的 pageSize 计算代码，改为直接读取 `a.contentHeight`。

3. App 新增字段：
```go
contentHeight int // 预计算的主内容区高度
```

---

### Phase 3: 提取 HostService（Service Pattern）

**解决：P5（UI 与持久化紧耦合，257 行业务逻辑嵌入 Update）**

**新建文件：** [internal/service/host_service.go](internal/service/host_service.go)

```go
package service

type HostService struct {
    configMgr *config.ConfigManager
    config    *models.Config
    logger    *zap.Logger
}

// AddHost 添加主机、保存密钥、更新分组、持久化配置
// 对应 app.go:692-753 的 77 行逻辑
func (s *HostService) AddHost(host *models.Host, groupName, keyContent string) error

// UpdateHost 更新主机、处理分组变更、名称变更引用更新、密钥保存
// 对应 app.go:819-932 的 113 行逻辑
func (s *HostService) UpdateHost(origName string, host *models.Host, newGroup, keyContent string) error

// DeleteHost 删除主机、清理分组引用、持久化
// 对应 Phase 0 新增的删除逻辑
func (s *HostService) DeleteHost(hostName string) error

// AddGroup 创建新分组
// 对应 app.go:771-800 的 30 行逻辑
func (s *HostService) AddGroup(groupName string) error

// ReloadConfig 重新加载配置
func (s *HostService) ReloadConfig() error

// GetConfig 返回当前配置（只读）
func (s *HostService) GetConfig() *models.Config

// FindHostGroup 查找主机所属分组
// 复用现有 app.go 中的 findHostGroup 逻辑
func (s *HostService) FindHostGroup(hostName string) string
```

**修改文件：** [update_dialog.go](internal/ui/tui/update_dialog.go)

对话框关闭处理简化示例（新建连接）：
```go
// Before: 77 行内联业务逻辑（app.go:692-753）
// After:
if a.newConnectionDialog.IsSaved() {
    host := a.newConnectionDialog.CreateHostConfig()
    keyContent := ""
    if a.newConnectionDialog.NeedsToSaveKey() {
        keyContent = a.newConnectionDialog.GetKeyContent()
    }
    if err := a.hostService.AddHost(host, a.newConnectionDialog.GetGroup(), keyContent); err != nil {
        a.enterState(StateErrorDialog)
        a.errorDialog = dialogs.NewErrorDialog(err.Error(), a.width, a.height)
        return a, nil
    }
    a.config = a.hostService.GetConfig()
    a.refreshHosts()
}
```

App 结构体改动：
- 新增 `hostService *service.HostService`
- 删除直接持有的 `configMgr`（通过 hostService 间接访问）

---

### Phase 4: 合并连接对话框（消除 ~1400 行重复）

**解决：P4（NewConnectionDialog 与 EditConnectionDialog 80% 重复）**

**新建文件：** [internal/ui/dialogs/connection_form.go](internal/ui/dialogs/connection_form.go)
**删除文件：** [new_connection.go](internal/ui/dialogs/new_connection.go)、[edit_connection.go](internal/ui/dialogs/edit_connection.go)

#### 设计

```go
type FormMode int
const (
    FormModeNew  FormMode = iota
    FormModeEdit
)

type ConnectionFormDialog struct {
    mode         FormMode
    originalName string  // 仅 FormModeEdit 有值

    // 以下字段从 NewConnectionDialog 保留（两者完全相同）
    nameInput     textinput.Model
    ipInput       textinput.Model
    portInput     textinput.Model
    usernameInput textinput.Model
    passwordInput textinput.Model
    // ... 所有其他共享字段 ...

    canceled bool
    closed   bool
    saved    bool
    width    int
    height   int
}

// Functional Options 构造
type FormOption func(*ConnectionFormDialog)
func WithHost(host *models.Host) FormOption { ... }       // 编辑模式预填充
func WithGroups(groups []string) FormOption { ... }
func WithProxies(proxies []string) FormOption { ... }
func WithSize(w, h int) FormOption { ... }

func NewConnectionForm(mode FormMode, opts ...FormOption) *ConnectionFormDialog
```

#### 行为差异处理（仅以下几处需要 `if d.mode == FormModeEdit`）

1. **标题**: `"新建连接"` vs `"编辑连接"`（View 方法中）
2. **字段预填充**: 通过 `WithHost()` option 在编辑模式下预设值
3. **`GetOriginalName()`**: 仅编辑模式返回 `originalName`
4. **密钥加载**: 编辑模式可能从磁盘读取已有密钥内容

#### 统一对外接口（保持与现有调用方兼容）

```go
func (d *ConnectionFormDialog) IsClosed() bool
func (d *ConnectionFormDialog) IsSaved() bool
func (d *ConnectionFormDialog) IsCanceled() bool
func (d *ConnectionFormDialog) CreateHostConfig() *models.Host
func (d *ConnectionFormDialog) GetGroup() string
func (d *ConnectionFormDialog) NeedsToSaveKey() bool
func (d *ConnectionFormDialog) GetKeyContent() string
func (d *ConnectionFormDialog) GetOriginalName() string  // 编辑模式
func (d *ConnectionFormDialog) GetNameInput() string     // 编辑模式
```

#### 迁移步骤

1. 复制 [new_connection.go](internal/ui/dialogs/new_connection.go) 为 `connection_form.go`
2. 添加 `mode`/`originalName` 字段
3. 从 [edit_connection.go](internal/ui/dialogs/edit_connection.go) 中补充编辑特有逻辑（约 5-10 处 `if d.mode == FormModeEdit` 分支）
4. 更新 [update_dialog.go](internal/ui/tui/update_dialog.go) 中的两处调用
5. App 结构体合并 `newConnectionDialog`/`editDialog` 为 `connectionFormDialog *ConnectionFormDialog`
6. State 枚举合并 `StateNewConnectionDialog`/`StateEditDialog` 为 `StateConnectionForm`
7. 删除旧文件，编译验证

---

### Phase 5: 提取子模型（Composite Model Pattern）

**解决：进一步降低各 update/view 方法的复杂度**

#### 5a. HostListModel

**新建文件：** [internal/ui/tui/host_list.go](internal/ui/tui/host_list.go)

```go
type HostListModel struct {
    hosts         []*models.Host
    filteredHosts []*models.Host
    selected      int
    paginator     paginator.Model
    pageSize      int
    contentHeight int
    width         int
    searchQuery   string
    currentGroup  string
}

// 从 App 迁移的方法
func (m *HostListModel) SetHosts(grouped map[string][]*models.Host, group string)
func (m *HostListModel) ApplySearch(query string)
func (m *HostListModel) RecalcLayout(height int)
func (m *HostListModel) MoveUp()
func (m *HostListModel) MoveDown()
func (m *HostListModel) PageLeft()
func (m *HostListModel) PageRight()
func (m *HostListModel) SelectedHost() *models.Host
func (m *HostListModel) View() string   // 包含 renderTableHeader + renderHostItem + paginator
```

App 中 `selected`、`hosts`、`filteredHosts`、`paginator`、`pageSize` 等字段全部迁移到 `HostListModel`。

#### 5b. GroupSelectModel

**新建文件：** [internal/ui/tui/group_select.go](internal/ui/tui/group_select.go)

```go
type GroupSelectModel struct {
    groups         []string
    filteredGroups []string
    cursor         int
    searchQuery    string
    searchMode     bool
    commandInput   textinput.Model
    width          int
}

func (m *GroupSelectModel) Update(msg tea.Msg) (selected string, done bool, cmd tea.Cmd)
func (m *GroupSelectModel) View(height int) string
```

App 中 `groupSelectCursor`、`groupList`、`filteredGroupList`、`groupSearchQuery` 等字段迁移到 `GroupSelectModel`。同时 `StateGroupSelect` 和 `StateGroupSearch` 合并为一个状态，由 `GroupSelectModel` 内部管理搜索子模式。

---

## 三、最终文件结构

```
internal/
  ui/
    tui/
      app.go              ← App 结构体、NewApp、Init、Run（~200 行）
      state.go            ← AppState 枚举、转换方法（~60 行）
      messages.go         ← 自定义 tea.Msg 类型（~50 行）
      update.go           ← Update 分发 + handleGlobalMsg（~120 行）
      update_normal.go    ← 普通模式按键（~160 行）
      update_dialog.go    ← 对话框状态处理（~120 行，Phase 3 后大幅精简）
      update_modes.go     ← 搜索/命令/会话列表/连接中（~180 行）
      view.go             ← View 分发（~60 行）
      view_normal.go      ← header/table/statusbar 渲染（~500 行）
      view_overlay.go     ← 连接中/会话列表 视图（~150 行）
      connection.go       ← 连接执行逻辑（~200 行）
      helpers.go          ← 工具函数（~150 行）
      host_list.go        ← HostListModel（Phase 5，~300 行）
      group_select.go     ← GroupSelectModel（Phase 5，~200 行）
    dialogs/
      confirm.go          ← ConfirmDialog（Phase 0 新建，~150 行）
      connection_form.go  ← 统一的连接表单（Phase 4，~1500 行）
      error.go            ← ErrorDialog（保留）
      new_group.go        ← NewGroupDialog（保留）
      password.go         ← PasswordDialog（保留）
  service/
    host_service.go       ← HostService（Phase 3，~200 行）
```

## 四、执行顺序

```
Phase 0（Bug修复+删除功能）
    ↓
Phase 1（状态机+文件拆分）← 最大改动，建议单独一个commit
    ↓
Phase 2（View纯函数化）← 小改动，可与Phase 1同commit
    ↓
Phase 3（HostService抽取）
    ↓
Phase 4（对话框合并）← 独立于Phase 3，但建议在其后
    ↓
Phase 5（子模型提取）
```

每个 Phase 完成后独立可编译运行，用户体验不变。

## 五、验证方式

每个 Phase 完成后：
1. `make build` — 编译通过
2. `make vet && make fmt` — 无 lint 错误
3. `make run` — 手动验证：
   - 所有快捷键响应正确（逐一测试第一节表格中的按键）
   - `D` 键删除连接（含确认弹窗），配置文件同步更新
   - `R`/`r` 均可刷新
   - 连接中按其他键不触发意外行为
   - Tab 切换分组、`/` 搜索过滤、翻页、Enter 连接正常
   - 新建/编辑连接对话框正常（Phase 4 后合并为同一组件）
   - SSH 后台会话 Ctrl+B 分离 / B 键重新挂载正常
   - 终端缩放时布局自适应（验证 Phase 2 View 纯化）
