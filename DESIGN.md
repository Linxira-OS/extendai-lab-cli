# ExtendAI Lab CLI — 统一架构设计文档

> 基于 Claude Code、pi、oh-my-pi、deepseek-reasonix、CodeWhale、codex 六个项目的最佳实践

---

## 1. 核心架构

### 1.1 Agentic Loop（来自 Claude Code queryLoop + pi runLoop）

```
while (true):
  Phase 1: Pre-API
    - 获取 compact boundary 后的消息
    - 截断超大工具结果 (applyToolResultBudget)
    - 检查上下文是否需要压缩 (shouldCompact)
    - 构建 system prompt

  Phase 2: API Call
    - 发送 messages + tools + thinking config
    - 流式解析: text delta / reasoning delta / tool_calls

  Phase 3: Post-API
    if (无 tool_calls):
      → return completed  ← 循环结束
    if (有 tool_calls):
      → 执行工具 (并行读/串行写)
      → 收集 tool_result
      → messages = [...old, ...assistant, ...toolResults]
      → 继续循环
```

### 1.2 消息格式（来自 Claude Code）

```go
// API 请求中的 message 格式
type Message struct {
    Role       string     `json:"role"`                 // "system"|"user"|"assistant"|"tool"
    Content    string     `json:"content,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"` // assistant 角色
    ToolCallID string     `json:"tool_call_id,omitempty"` // tool 角色
}

type ToolCall struct {
    ID       string       `json:"id"`
    Type     string       `json:"type"` // "function"
    Function FunctionCall `json:"function"`
}

type FunctionCall struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"` // JSON string
}
```

### 1.3 Tool Result 发回格式（来自 Claude Code）

```go
// 执行完工具后，发回给模型
client.AddToolResult(toolCallID, content, isError)
// 内部创建: { role: "tool", content: "result", tool_call_id: "xxx" }
```

---

## 2. 工具系统

### 2.1 Tool Registry（来自 pi AgentTool + deepseek ToolRegistry）

```go
type ToolDefinition struct {
    Type     string       `json:"type"`
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Parameters  interface{} `json:"parameters"` // JSON Schema
}

type RegisteredTool struct {
    Definition ToolDefinition
    Execute    ToolFunc
    ReadOnly   bool   // 只读工具可并行执行
    Concurrency string // "shared" | "exclusive"
}
```

### 2.2 工具列表（来自所有项目）

| 工具 | 来源 | 只读 | 说明 |
|------|------|------|------|
| read_file | Claude Code + pi | ✅ | 读取文件内容 |
| write_file | Claude Code + pi | ❌ | 写入文件 |
| edit_file | Claude Code + pi | ❌ | 精确替换编辑 |
| list_dir | deepseek + pi | ✅ | 列出目录 |
| search_files | Claude Code + pi | ✅ | glob 搜索文件名 |
| grep | Claude Code + pi | ✅ | 正则搜索文件内容 |
| bash | Claude Code + pi + oh-my-pi | ❌ | 执行 shell 命令 |
| todo_write | Claude Code + deepseek | ❌ | 管理任务列表 |
| question | Claude Code | ❌ | 向用户提问 |
| web_search | deepseek + CodeWhale | ✅ | 网络搜索 |
| web_fetch | Claude Code + pi | ✅ | 获取 URL 内容 |

### 2.3 工具执行并发（来自 Claude Code + oh-my-pi）

```
只读工具 (read_file, list_dir, search_files, grep):
  → 并行执行 (最多 10 个)

写入工具 (write_file, edit_file, bash):
  → 串行执行 (一个完成后再执行下一个)
```

### 2.4 工具 Hooks（来自 pi）

```go
// beforeToolCall — 在执行前调用，可阻止执行
func beforeToolCall(toolName string, args map[string]interface{}) (block bool, reason string)

// afterToolCall — 在执行后调用，可覆盖结果
func afterToolCall(toolName string, result string, err error) (overrideResult string, overrideErr error)
```

---

## 3. 上下文管理

### 3.1 Token 估算（来自 deepseek countTokensBounded）

```go
// 中文: 1 token ≈ 1.5 字符
// 英文: 1 token ≈ 4 字符
// 代码: 1 token ≈ 3.5 字符
func estimateTokens(text string) int {
    // 统计中文字符数
    cjk := countCJKChars(text)
    other := len(text) - cjk*3 // CJK 占 3 字节
    return cjk + other/4
}
```

### 3.2 上下文压缩（来自 Claude Code autocompact + pi compact）

**触发条件**：
```go
func shouldCompact(messages []Message, contextWindow int) bool {
    tokens := estimateTotalTokens(messages)
    threshold := float64(contextWindow) * 0.8 // 80%
    return float64(tokens) > threshold
}
```

**压缩流程**：
1. 保留最近 N 条消息不压缩
2. 用 LLM 摘要旧消息
3. 创建 CompactionEntry 替换旧消息
4. 保留文件操作记录

### 3.3 上下文分层（来自 deepseek CacheFirstLoop）

```
Immutable Prefix (冻结):
  - system prompt
  - tool definitions
  → session 内不变 → 缓存命中

Append-Only Log (增长):
  - user messages
  - assistant messages
  - tool results
  → 可被 trim() 截取作为子 agent 共享 prefix

Volatile Scratch (临时):
  - thinking content
  - streaming state
  → 不持久化
```

---

## 4. 会话持久化

### 4.1 JSONL 格式（来自 pi + oh-my-pi）

```
<session-dir>/
  session.jsonl          # append-only log
  artifacts/             # 大输出存储
```

**Entry 类型**：
```go
const (
    EntryTypeSession   = "session"         // header
    EntryTypeMessage   = "message"         // 消息
    EntryTypeCompaction = "compaction"     // 压缩摘要
    EntryTypeModelChg  = "model_change"   // 模型变更
    EntryTypeThinkChg  = "thinking_level"  // 思维强度变更
    EntryTypeBranchSum = "branch_summary"  // 分支摘要
)
```

### 4.2 Session Tree（来自 pi）

```
Session = flat entry list with id/parentId forming a tree
LeafId = current position in the tree

操作:
  AppendMessage() — 添加消息到当前 leaf
  Branch(id)     — 移动 leaf 指针到指定 entry
  Fork()         — 复制当前 branch 到新 session
  getBranch()    — 从 leaf 遍历到 root
```

---

## 5. 后台任务

### 5.1 AsyncJobManager（来自 oh-my-pi）

```go
type JobManager struct {
    jobs map[string]*Job
}

type Job struct {
    ID          string
    Command     string
    Status      JobStatus // running | completed | failed | cancelled
    StartedAt   time.Time
    FinishedAt  time.Time
    Output      string
    Error       string
}
```

**生命周期**：
```
StartJob() → status: "running"
  ↓
执行完成 → status: "completed" → 通知 AI
执行失败 → status: "failed"    → 通知 AI
取消     → status: "cancelled"
```

**完成通知**：
```go
// 当后台任务完成时，注入系统消息到对话
MsgJobComplete{
    JobID:    "job_1",
    Command:  "npm test",
    ExitCode: 0,
    Output:   "All tests passed.",
}
```

---

## 6. TUI 布局（来自 CodeWhale）

### 6.1 整体结构

```
┌─ ExtendAI Lab ─ session-id ─ model · ctx% · ● Live ─────────┐
│                                                              │
│   ┃ user message                                             │
│   ╭ ▷ read_file                                              │
│   │ file content...                                          │
│   ╰                                                          │
│   assistant response...                                      │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│┃  > input area                                               │
│   model-name                                    input        │
├──────────────────────────────────────────────────────────────┤
│ user 320 · asst 1.2K · tool 450 · sys 80         12% · 1.5K │
└──────────────────────────────────────────────────────────────┘
```

### 6.2 工具卡片（来自 CodeWhale）

```
╭ ▷ read_file        ← 工具名 + 家族符号
│ file content...    ← 工具输出
╰                    ← 结束标记
```

**家族符号**：
- `▷` read (蓝色)
- `◆` patch/write (金色)
- `▶` run/shell (青色)
- `⌕` search/grep (橙色)
- `◐` agent (青色)
- `•` generic (灰色)

### 6.3 水波动画（来自 CodeWhale）

```
AI 工作时 footer 显示: ⠹ thinking  ▁▂▃▄▅▆▇█▅▃▂▁
公式: primary = sin(x*0.52 - t*8.0)
```

### 6.4 Header 状态芯片（来自 CodeWhale）

```
● ExtendAI Lab ─ abc123def456         free:QwQ-32B · 12% · ⠹ Live
```

---

## 7. 安全系统

### 7.1 权限引擎（来自 codex + Claude Code）

```go
type PermissionRule struct {
    Permission string // "file.read" | "file.write" | "shell" | "destructive"
    Pattern    string // glob 模式
    Action     string // "allow" | "deny" | "ask"
}
```

**内置规则**：
- 项目内读写 → allow
- .git/** 写入 → deny
- ~/.ssh/** 写入 → deny
- rm -rf / → deny
- shell 命令 → ask

### 7.2 沙箱（来自 codex）

```
三层隔离:
  Linux:   bubblewrap + landlock
  macOS:   Seatbelt (sandbox-exec)
  Windows: Restricted Token + Job Object
  Fallback: 路径白名单
```

---

## 8. 实现优先级

### P0 — 核心功能（已实现）
- [x] Agentic loop (while hasToolCalls)
- [x] Function calling (tools in request, tool_calls parsing)
- [x] Tool registry (7 tools)
- [x] Tool result feedback (AddToolResult)
- [x] Session persistence (JSONL)
- [x] Streaming (SSE, reasoning content)

### P1 — 上下文安全
- [x] Token 估算改进 (CJK-aware)
- [x] 上下文压缩 (shouldCompact + compact)
- [x] 工具结果截断 (applyToolResultBudget)
- [x] 上下文使用可视化

### P2 — 工具增强
- [ ] Tool hooks (beforeToolCall, afterToolCall)
- [ ] 并发控制 (只读并行, 写入串行)
- [ ] 异步后台任务集成到 agentic loop
- [ ] 命令拦截/修正

### P3 — 高级功能
- [ ] 子 agent (spawn_subagent)
- [ ] 记忆系统 (remember/forget/recall)
- [ ] 技能系统 (run_skill/install_skill)
- [ ] 计划系统 (submit_plan/mark_step_complete)

### P4 — UI 增强
- [ ] 工具卡片美化 (╭│╰ rail)
- [ ] 水波动画
- [ ] Header 状态芯片
- [ ] 上下文用量进度条

---

## 9. 参考项目映射

| 组件 | 主要参考 | 辅助参考 |
|------|----------|----------|
| Agentic loop | Claude Code queryLoop | pi runLoop |
| Tool system | pi AgentTool | deepseek ToolRegistry |
| Tool orchestration | Claude Code runTools | oh-my-pi concurrency |
| Streaming | Claude Code queryModel | pi streamAssistantResponse |
| Context compaction | Claude Code autocompact | pi compact |
| Session persistence | pi JSONL | oh-my-pi SessionStorage |
| Background jobs | oh-my-pi AsyncJobManager | opencode-pty PTYManager |
| Permission system | codex ExecPolicy | Claude Code permission |
| TUI layout | CodeWhale | OpenCode |
| Tool cards | CodeWhale tool_card | — |
| Water-spout | CodeWhale footer | — |

---

## 10. 关键文件结构

```
clients/tui/
├── internal/
│   ├── api/
│   │   ├── client.go      # API 客户端 (SendMessage, streaming, tool_calls)
│   │   └── registry.go    # 工具注册表 (7 tools, ToolRegistry)
│   ├── model/
│   │   ├── model.go       # 主模型 (agentic loop, View, Update)
│   │   ├── session.go     # 会话持久化 (JSONL, tree)
│   │   ├── message.go     # 消息类型 (user/assistant/tool/system/error)
│   │   ├── jobs.go        # 后台任务管理器
│   │   ├── footer.go      # Footer 渲染
│   │   ├── sidebar.go     # 侧边栏
│   │   └── dialog.go      # 对话框系统
│   ├── renderer/
│   │   └── markdown.go    # Markdown 渲染
│   ├── theme/
│   │   └── theme.go       # 主题系统
│   └── protocol/
│       └── types.go       # IPC 协议类型
└── main.go                # 入口
```
