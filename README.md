# ExtendAI Lab CLI

> **Linxira-OS 生态中的 AI Agent 工作流引擎** — 多 LLM 编码代理 CLI
>
> 插件原生 · 缓存优先 · 模块化架构 · 从头构建

---

## 定位

ExtendAI Lab CLI 是 [Linxira OS](https://github.com/Linxira-OS) 生态中的 AI 开发层，与 [Linxira Pulse](https://github.com/Linxira-OS/linxira-pulse)（系统级 AI 运行时）协同工作。

**关键设计哲学**：
- **Plugin-Natured** — 插件不是附加功能，是整个架构的核心。任何核心组件都可以在运行时被插件替换
- **Runtime-First** — 一切在运行时决定
- **Cache-First** — 多 Agent 共享前缀 → KV 缓存命中 → 边际成本递减
- **Invasive Allowed** — 插件可以侵入性修改任意核心组件
- **Multi-LLM Native** — 以多模型支持为原生设计

## 模块结构

```
packages/
├── kernel/   @extendai/kernel  核心引擎：Session / Context / Provider / Tool
├── plugin/   @extendai/plugin  插件系统：Hooks / Registry / SDK
├── agent/    @extendai/agent   Agent 系统：Orchestrator / SubAgent / Council
├── tui/      @extendai/tui     终端 UI：组件 / 命令 / 主题
└── cli/      @extendai/cli     CLI 入口：依赖注入组装
```

## 快速开始（测试）

### 前置条件

```powershell
# 克隆并安装依赖
git clone https://github.com/Linxira-OS/extendai-lab-cli.git
cd extendai-lab-cli
npm install

# 构建
npx tsc --project tsconfig.json
```

### 方式一：环境变量配置（推荐）

```powershell
# PowerShell 中设置
$env:EXTENDAI_BASE_URL="https://api.suanli.cn/v1"
$env:EXTENDAI_API_KEY="sk-你的key"
$env:EXTENDAI_MODEL="free:QwQ-32B"

# 启动交互式聊天
npx tsx packages/cli/src/index.ts
```

### 方式二：配置文件持久化

```powershell
# 生成默认配置
npx tsx packages/cli/src/index.ts --init

# 编辑 ~/.extendai/config.json：
# {
#   "provider": {
#     "name": "suanli",
#     "type": "openai",
#     "apiKey": "sk-你的key",
#     "baseUrl": "https://api.suanli.cn/v1",
#     "model": "free:QwQ-32B",
#     ...
#   }
# }

# 启动
npx tsx packages/cli/src/index.ts
```

### 方式三：从其他目录运行（已构建后）

```powershell
# 在 D:\test 目录下
$env:EXTENDAI_BASE_URL="https://api.suanli.cn/v1"
$env:EXTENDAI_API_KEY="sk-你的key"
$env:EXTENDAI_MODEL="free:QwQ-32B"

node D:\path\to\extendai-lab-cli\dist\cli\src\index.js

# 或使用 tsx 开发模式（支持热更）
npx tsx D:\path\to\extendai-lab-cli\packages\cli\src\index.ts
```

### 命令行选项

```
extendai                     Start interactive chat
extendai --help              Show help
extendai --version           Show version
extendai --model <name>      Start with specific model
extendai --init              Create default config at ~/.extendai/config.json
extendai --init-git          Init git repo + .gitignore in current directory
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `EXTENDAI_API_KEY` | API 密钥（远程必填，本地/LAN 自动跳过） | `""` |
| `EXTENDAI_BASE_URL` | API 基础地址 | `https://api.openai.com/v1` |
| `EXTENDAI_MODEL` | 模型名称 | `gpt-4o` |

### 本地 Provider 示例

```powershell
# LM Studio
$env:EXTENDAI_BASE_URL="http://192.168.x.x:1234/v1"
$env:EXTENDAI_API_KEY=$env:EXTENDAI_MODEL  # key = model ID

# Ollama
$env:EXTENDAI_BASE_URL="http://localhost:11434/v1"
$env:EXTENDAI_API_KEY="ollama"
```

## 从 `D:\test` 测试的完整命令

```powershell
# 1. 设置环境变量（替换 sk-xxx 为你的密钥）
$env:EXTENDAI_BASE_URL="https://api.suanli.cn/v1"
$env:EXTENDAI_API_KEY="sk-你的密钥"
$env:EXTENDAI_MODEL="free:QwQ-32B"

# 2. 启动 CLI（开发模式，从任意目录）
npx tsx D:\-Users-\Documents\GitHub\chat-model\extendai-lab-cli\packages\cli\src\index.ts

# 或使用构建后的版本
node D:\-Users-\Documents\GitHub\chat-model\extendai-lab-cli\dist\cli\src\index.js
```

## 本地命令（聊天中支持）

| 命令 | 功能 |
|------|------|
| `/undo` | 回退到上一步（文件 + 对话） |
| `/snapshot <msg>` | 手动创建快照 |
| `/snapshots` | 列出快照历史 |
| `/fork` | 从当前点分支新会话 |
| `/exit` | 退出（或 Ctrl+C） |

## 构建

```bash
# 安装依赖
npm install

# 构建所有包
npm run build

# 开发运行（热加载）
npm run dev
```

## 项目状态

当前实现：**kernel 核心层**，包含：
- ✅ Session 树（分支 / 还原）
- ✅ Snapshot 系统（OpenCode 风格：外部 git repo + tree hash）
- ✅ 工具系统（14 个内置工具：read/write/edit/glob/grep 等）
- ✅ Provider 抽象（兼容 OpenAI API 格式）
- ✅ Config 加载（env > 配置文件 > 默认值）
- ✅ Worktree 检测
- ⬜ Agent / Orchestrator
- ⬜ 子代理系统
- ⬜ 插件 SDK
- ⬜ TUI 增强

## 许可证

AGPL-3.0

---

> 完整架构设计见 `DESIGN.md`（不在 git 追踪中）
