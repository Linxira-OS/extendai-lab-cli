# CLI 项目全景分析

分析时间：2026-05-24
分析范围：6 个 CLI 项目 + 4 个插件项目

## 目标

评估各 CLI 作为 ExtendAI Lab CLI 基础的适用性，关键是：**缓存优先的多 Agent 协作架构**。

## 项目速览

| 项目 | 语言 | 代码量 | TUI 引擎 | 许可证 | 类型 |
|------|------|--------|---------|--------|------|
| deepseek-reasonix | TypeScript | ~59K | React/Ink | MIT | 独立 CLI |
| oh-my-pi | TypeScript + Rust | ~150K+ | 自定义 TUI 引擎 | MIT | 独立 CLI |
| openclaude | TypeScript | ~1200+ 文件 | 自定义 Ink fork | 部分 MIT（核心闭源） | Claude Code 分支 |
| opencode-dev | TypeScript (Effect.ts) | ~100K+ | OpenTUI + Solid.js | Apache 2.0 | 独立 CLI |
| CodeWhale | Rust | ~50K | ratatui | MIT | 独立 CLI |
| codex | Rust + TS + Python | ~116 crates | ratatui fork | Apache 2.0 | 独立 CLI |

## 缓存架构（核心维度）

### 缓存经济学基础

LLM API 调用中：
- **缓存写入（Cache Write）** — 首次遇到新的 prefix 时触发，昂贵
- **缓存读取（Cache Read）** — prefix 匹配已有缓存时触发，便宜（通常 50-90% 折扣）

多 Agent 架构的核心问题：每 spawn 一个子 agent 就产生一段新的 unique prefix → 多次缓存写入 → 经济灾难。

### 各项目缓存架构

| 项目 | ImmutablePrefix | Context/Memory 分离 | Fork 机制 | Subagent 缓存意识 |
|------|----------------|---------------------|-----------|------------------|
| **deepseek-reasonix** | ✅ CacheFirstLoop | ✅ 三分（Immutable/AppendOnly/Volatile） | ⚠️ 通过工具化 spawn | ❌ 无共享 prefix |
| **CodeWhale** | ⚠️ SubAgentForkContext | ⚠️ fork_context 和 fresh 两模式 | ✅ agent_open 持久化 | ✅ 有缓存意识 |
| **oh-my-pi** | ❌ 无 | ⚠️ IRC + AgentRegistry 共享 | ❌ 每次重建 prompt | ❌ 无 |
| **opencode-dev** | ❌ 无 | ⚠️ Effect Fiber 可搭 | ⚠️ task_id resume | ❌ 无 |
| **openclaude** | ⚠️ ForkSubagent | ❌ 上下文混在一起 | ✅ fork 模式 | ⚠️ 有限 |
| **oh-my-claudecode** (插件) | ❌ 无 | ❌ 无 | ❌ 独立 tmux pane | ❌ 无 |
| **oh-my-openagent** (插件) | ❌ 无 | ❌ 无 | ⚠️ 后台 task | ❌ 无 |

### 核心发现：最"完整"的插件反而是反例

oh-my-claudecode（19 角色并行 spawn）和 oh-my-openagent（54 hooks + team mode）的问题：

- 每 spawn 一个子 agent = 一段新的 unique prefix
- N 个子 agent = N 次缓存写入（昂贵）
- 并行写入时缓存分片，互相无效化
- 子 agent 越多，缓存经济效益越差

这正是"功能最完整"不等于"架构最好"的原因。

## Hook/扩展系统（生态维度）

| 项目 | Hook 数量 | Agent 生命周期 Hook | 插件系统成熟度 |
|------|----------|-------------------|--------------|
| **oh-my-pi** | 全面 | agent_start/end, turn_start/end, tool_call/result | ★★★★★ |
| **opencode-dev** | 关键点 | plugin hooks + chat.transform | ★★★★ |
| **oh-my-claudecode** (插件) | 30+ | PreToolUse/PostToolUse/SubagentStart/Stop | ★★★★ |
| **oh-my-openagent** (插件) | 54+ 五层 | Session/ToolGuard/Transform/Continuation/Skill | ★★★★★ |
| **openclaude** | 少 | SubagentStart/Stop | ★★ |
| **CodeWhale** | ❌ 无 | ❌ | ★ |
| **deepseek-reasonix** | ❌ 仅 shell hooks | ❌ subagent 硬编码 `hooks: []` | ★ |

## 多模型支持

| 项目 | 支持 Provider 数 | 模式 |
|------|-----------------|------|
| oh-my-pi | 50+ | Provider enum，自定义 provider |
| opencode-dev | 20+ | AI SDK 生态 |
| openclaude | 5+ (Claude/Vertex/Bedrock/OpenAI/Gemini) | ProviderProfile |
| CodeWhale | 11 | Provider 配置 |
| deepseek-reasonix | 1+ (DeepSeek + openaiShim) | 深度绑定 DeepSeek |
| codex | 1+ (OpenAI + openai-compatible) | API key + config |

## 目标架构

```
┌────────────────────────────────────────────────────┐
│           TS Client Layer (oh-my-pi base)           │
│  ┌─────────┐ ┌──────────┐ ┌──────────────────┐     │
│  │   TUI   │ │ Hook 系统 │ │  Agent 定义 + 调度 │     │
│  │ (React) │ │ Extension │ │  子 agent 工具化   │     │
│  └─────────┘ └──────────┘ └──────────────────┘     │
│                                                     │
│  ← 通过 JSON-RPC / stdio 连接 →                     │
└────────────────────────────────────────────────────┘
                        │
                        ▼
┌────────────────────────────────────────────────────┐
│          Rust Daemon (常驻后台服务)                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────────────┐     │
│  │ MCP 池   │ │ 缓存管理  │ │  Agent 状态持久化 │     │
│  │ 多窗口共享 │ │ 共享前缀  │ │  跨 session 存活  │     │
│  └──────────┘ └──────────┘ └──────────────────┘     │
└────────────────────────────────────────────────────┘
```

## 关键设计模式：Fork + Cache

```
Phase 1（缓存写入，只做一次）：
  主 agent → 调查项目背景 → 标记标签 → 输出 SharedContext JSON

Phase 2（缓存读取，做 N 次）：
  子 agent 们 → 共享 SharedContext 前缀 → 全部命中缓存读取
               → 每个子 agent 只付 unique 后缀的钱

效果：3 个子 agent = 1 次前缀写入 + 3 次后缀写入
      相当于 fork 了一个对话，fork 前全缓存命中
```

## 选定方向

**Base CLI：oh-my-pi**

选择原因：
- 最完整的 hook/extension 系统（agent 生命周期全覆盖）
- 从该架构可快速抽象出标准插件 SDK（MIT）
- 50+ provider 支持，生态最广
- 8 角色子 agent 定义清晰（markdown + frontmatter）
- IRC 系统提供灵活的 agent 间通信
- Swarm 扩展支持 DAG 管线

需要追加：
- CacheFirstLoop / ImmutablePrefix 机制（复用 deepseek-reasonix 的设计模式）
- Shared Context JSON + 标签系统
- Fork + Cache 的 hook 层注入
- ProviderProfile 模式（参考 openclaude）
- 中文 locale 强制（参考 CodeWhale）
