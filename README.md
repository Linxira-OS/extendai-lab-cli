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

## 构建

```bash
# 安装依赖
npm install

# 类型检查（ts-go）
npm run typecheck

# 开发运行
npm run dev
```

使用 **TypeScript Go 原生编译器**（`@typescript/native-preview`）进行构建，即微软 TypeScript 7.0 的 Go 移植版。

## 许可证

AGPL-3.0

---

> 完整架构设计见 `DESIGN.md`（不在 git 追踪中）
