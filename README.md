# ExtendAI Lab CLI

面向多 LLM 的编码代理 CLI — TUI + 插件系统 + 缓存优先架构。

## 设计目标

- **缓存优先** — 共享前缀 → 子 agent fork → 全部命中缓存读取
- **多模型兼容** — ProviderProfile 模式，支持 DeepSeek / OpenAI / Anthropic / Google 等
- **双语言优化** — 自动检测模型家族 → 自动选择提示词语言（token 经济学）
- **TUI + VSCode + Web + Desktop** — 同构架构
- **开放插件系统** — 插件 SDK (MIT) 让第三方轻松扩展

## 项目结构

```
extendai-lab-cli/
├── LICENSE                        ← AGPL v3（覆盖全仓库默认）
├── packages/
│   ├── core/                      ← AGPL v3
│   │   ├── src/                   CLI 核心（TUI / Agent 循环 / 工具链）
│   │   └── test/
│   ├── rust-daemon/               ← AGPL v3
│   │   └── src/                   Rust 后台（MCP 池 / 缓存管理 / 状态持久化）
│   ├── plugin-sdk/                ← MIT
│   │   ├── LICENSE                MIT 许可证覆盖此包
│   │   ├── src/                   插件开发工具包（类型定义 / Hook API）
│   │   └── test/
└── docs/
```

## 双许可证策略

| 模块 | 许可证 | 说明 |
|------|--------|------|
| `packages/core/` | AGPL v3 | 核心 CLI — 修改须开源 |
| `packages/rust-daemon/` | AGPL v3 | 后台服务 — 修改须开源 |
| `packages/plugin-sdk/` | **MIT** | 插件 SDK — 可自由开发闭源/开源插件 |
| 第三方插件 | 自选 | 插件开发者自选许可证，不受 AGPL 限制 |

## 阶段路线

- **阶段 1** — ProviderProfile 多模型 + 共享 Context/Fork 机制
- **阶段 2** — Rust 后台 daemon（MCP 连接池 + 缓存 + 状态管理）
- **阶段 3** — 插件系统 + 扩展子 Agent 角色
- **阶段 4** — VSCode 扩展 / Web UI / Desktop
