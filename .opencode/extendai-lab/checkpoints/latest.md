CHECKPOINT CONTEXT
==================

SOURCE SESSION
--------------
- Session ID: current
- Created At: 2026-06-03
- Checkpoint Kind: heavy
- Trigger: manual

USER REQUESTS (AS-IS)
---------------------
1. "参考designer文档继续工作"
2. "进入全自动模式，把当前所有内容全部开发好，并且全部做好测试"
3. "研究 deepseek-reasonix、oh-my-pi、pi、opencode-dev、oh-my-openagent、oh-my-opencode-slim 等项目"
4. "查看 reasonix 项目，迁移它作为我们的上游的可行性"
5. "新项目看的如何了" (deepcode-cli)
6. "不对 https://github.com/esengine/DeepSeek-Reasonix 不是有一个沙箱吗?"

GOAL
----
全面调研 8 个参考项目，评估 DeepSeek-Reasonix Go 版本作为上游的可行性，决定下一步架构方向。

WORK COMPLETED
--------------
1. **P0-P2 功能实现**:
   - CJK-aware token 估算 (token.go)
   - 上下文压缩系统 (compaction.go)
   - 工具结果截断预算 (registry.go)
   - CacheFirstContext 三区域模型 (context.go)
   - parallelSafe 并行调度 (registry.go)
   - Tool hooks (beforeToolCall/afterToolCall)
   - bash_bg 后台任务工具
   - 命令安全规则 (command_safety.go)

2. **测试覆盖**:
   - 138 个测试用例，14 个测试文件
   - api/client_test.go, api/context_test.go, api/registry_test.go
   - model/ 下 11 个测试文件

3. **项目调研完成**:
   - DeepSeek-Reasonix Go 版本 (main-v2 分支) — **确认有 Go 1.0 重写**
   - pi, oh-my-pi, opencode-dev, opencode-pty, codex, CodeWhale, copilot-cli
   - deepcode-cli (LLM 匹配 + Karpathy Guidelines)

4. **关键发现**:
   - Reasonix main-v2 是 Go 1.25 + bubbletea v2 (和我们相同技术栈)
   - Reasonix 有完整沙箱系统 (macOS Seatbelt + Linux bubblewrap)
   - Reasonix 有完整功能: agent/tool/memory/skill/hook/permission/sandbox/LSP/MCP
   - 我们实现了约 35% 功能，Reasonix 有 100%

CURRENT STATE
-------------
- **代码**: Go TUI with bubbletea, 31 源文件, 14 测试文件, 138 测试
- **架构**: CacheFirstContext + parallelSafe dispatch + tool hooks
- **调研**: 8 个项目完整分析，功能对比矩阵完成
- **决策待定**: 继续 Go 开发 vs 基于 Reasonix fork vs Rust 重写

PENDING TASKS
-------------
1. 决定架构方向 (基于 Reasonix fork / 继续 Go / Rust 重写)
2. 如果 fork Reasonix: 去品牌、迁移其他项目功能
3. deepcode-cli 的 LLM 匹配和 Karpathy Guidelines 迁移
4. 更多测试覆盖

KEY FILES
---------
- clients/tui/internal/api/context.go — CacheFirstContext 三区域模型
- clients/tui/internal/api/registry.go — 工具注册表 + parallelSafe 调度
- clients/tui/internal/api/client.go — API 客户端 + CacheFirstContext 集成
- clients/tui/internal/model/model.go — 主模型 + agentic loop
- clients/tui/internal/model/compaction.go — 上下文压缩
- clients/tui/internal/model/token.go — CJK-aware token 估算
- clients/tui/internal/model/command_safety.go — 命令安全规则
- DESIGN.md — 统一架构设计文档
- studying/deepseek-reasonix/ — Reasonix Go 版本 (main-v2 分支)

IMPORTANT DECISIONS
-------------------
1. **技术栈**: Go + bubbletea v2 (与 Reasonix 相同)
2. **上下文模型**: CacheFirstContext (ImmutablePrefix + AppendOnlyLog + VolatileScratch)
3. **并行调度**: parallelSafe 标记 + 连续分组 + goroutine 并发
4. **工具钩子**: beforeToolCall/afterToolCall 模式
5. **待决定**: 是否 fork Reasonix 作为上游

RESUME INSTRUCTIONS
-------------------
1. 用户需要决定架构方向:
   - A: 继续 Go 开发 (基于现有代码)
   - B: Fork Reasonix Go 版本 (推荐，技术栈相同)
   - C: Rust 重写 (性能最优但工作量大)
   - D: 混合方案 (Go TUI + Rust 后端)

2. 如果选择 B (fork Reasonix):
   - 检出 main-v2 分支
   - 删除上游品牌 (reasonix.toml → extendai.toml)
   - 迁移其他项目的优秀功能
   - 添加 CJK-aware token、水波动画、工具卡片

3. 参考项目位置:
   - studying/deepseek-reasonix (Go 版本在 main-v2 分支)
   - studying/pi, studying/oh-my-pi
   - studying/codex, studying/CodeWhale (Rust)
   - studying/opencode-dev, studying/opencode-pty
   - studying/deepcode-cli (LLM 匹配创新)

4. 功能对比矩阵在之前的对话中，包含所有项目的完整功能对比
