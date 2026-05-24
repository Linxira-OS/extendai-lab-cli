# Base CLI 选择：oh-my-pi

## 决策时间

2026-05-24

## 选项回顾

### deepseek-reasonix（最初推荐）

| 优势 | 劣势 |
|------|------|
| ✅ 纯 TS，~59K，代码量最小 | ❌ 无 hook/extension 系统 |
| ✅ CacheFirstLoop + ImmutablePrefix 三分架构 | ❌ 子 agent 硬编码 `hooks: []` |
| ✅ spawn_subagent 工具化 | ❌ 从零搭插件 SDK 需数月 |
| ✅ MIT 许可证 | ❌ 深度绑定 DeepSeek，多 provider 需重写 |

### oh-my-pi（最终选择）

| 优势 | 劣势 |
|------|------|
| ✅ 最完整的 hook/extension 系统 | ❌ 无 CacheFirstLoop（需追加） |
| ✅ 从该架构可快速抽象插件 SDK | ❌ 代码量大（~150K+） |
| ✅ 50+ provider，生态最广 | ❌ 有 Rust 原生组件 |
| ✅ 8 角色子 agent + markdown 定义 | ❌ 依赖 Bun 特定 API |
| ✅ IRC agent 间通信 | |
| ✅ Swarm DAG 管线 | |
| ✅ 跨编辑器格式支持 | |

## 选择逻辑

### 关键问题：哪边更难改？

```
deepseek-reasonix:
  CacheFirstLoop    ✅ 已存在（最难的部分）
  插件系统            ❌ 从零建（数月工作量）
  多 provider        ❌ 从零建
  Hook 体系          ❌ 从零建

oh-my-pi:
  CacheFirstLoop    ❌ 需追加（但可通过 hook 注入）
  插件系统            ✅ 已存在（最难的部分已完成）
  多 provider        ✅ 已存在（50+）
  Hook 体系          ✅ 已存在（最完整）
```

**结论**：给 oh-my-pi 加缓存机制（通过 hook 注入）比给 deepseek-reasonix 搭整套插件系统快得多。

### 缓存机制的追加路径

oh-my-pi 已有 hook 点足够完备：

| Hook 事件 | 用于缓存机制 |
|-----------|------------|
| `before_agent_start` | 注入 SharedContext 到系统提示词 |
| `agent_end` | 标记调查完成，持久化上下文 |
| `session_start` | 加载历史缓存 |
| `turn_start` | 检查缓存有效性 |
| `tool_call` / `tool_result` | 拦截调查类工具调用，自动打标签 |

### 插件 SDK 的抽象路径

oh-my-pi 的扩展系统：
- `ExtensionAPI`（sendMessage, setModel, getActiveTools 等）
- `HookRunner`（事件注册 + 分发）
- `SkillLoader`（markdown skill 发现 + 注入）
- `CustomTool`（工具注册）

抽象为 MIT 的 `@extendai-lab/plugin-sdk`：
- 类型定义 + 生命周期接口
- 和 CLI 核心（AGPL）解耦
- 第三方无需关心 AGPL 限制

## 下一步

1. 将 oh-my-pi 核心代码 fork 到 `packages/core/`
2. 加入 SharedContext / Fork + Cache 机制（hook 层注入）
3. 抽象插件 SDK 到 `packages/plugin-sdk/`
4. 用兼容层迁移现有 `openagent-labforge` 代码
