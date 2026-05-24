# Fork + Cache 机制设计

## 问题

多 Agent 架构的核心矛盾：

```
传统做法（oh-my-claudecode / oh-my-openagent 等）:
  主 agent → spawn A（新 prefix → 缓存写入）
          → spawn B（新 prefix → 缓存写入）
          → spawn C（新 prefix → 缓存写入）
                        ↓
                  3 次缓存写入，全部全价
                  子 agent 越多，经济损失越大
```

## 解法：Shared Prefix Fork

### 核心思路

让所有子 agent 共享同一个 prefix，这一段的 LLM 输出缓存（KV Cache）可以被所有子 agent 复用：

```
我们的做法：
  主 agent → 调查项目背景
           → 序列化为 SharedContext JSON
           → 标记"项目背景调查完成"标签
           → 冻结为共享 prefix
           → spawn A ─┐
           → spawn B ─┼── 共享 prefix → 缓存命中（便宜）
           → spawn C ─┘
                        ↓
                  1 次缓存写入 + 3 次缓存读取
                  每多一个子 agent，边际成本近乎为零
```

### 数据结构

```typescript
// 共享上下文
interface SharedContext {
  // 版本和元数据
  version: string;
  timestamp: number;
  
  // 项目调查结果
  project: {
    root: string;
    language: string;
    framework?: string;
    description: string;
  };
  
  // 已经做过的调查（防止重复）
  investigations: {
    [key: string]: {
      done: boolean;
      result: string;
      tags: string[];
      timestamp: number;
    };
  };
  
  // 共享知识
  knowledge: {
    key: string;
    content: string;
    source: string;
    tags: string[];
  }[];
  
  // 标签索引
  tags: {
    [tag: string]: string[]; // tag → key 列表
  };
}
```

### Hook 注入点

对于 oh-my-pi 架构，缓存机制通过 hook 注入：

```typescript
// 伪代码：SharedContext hook
hooks: {
  // 1. 当 agent 即将开始工作时，注入共享上下文
  before_agent_start: async (ctx) => {
    const shared = await loadSharedContext(ctx.sessionID);
    if (shared) {
      ctx.injectSystemMessage(renderSharedContext(shared));
      // 所有子 agent 共享同一段 prefix → 缓存命中
    }
  },
  
  // 2. 当 agent 完成调查时，标记并持久化
  agent_end: async (ctx) => {
    const tags = extractInvestigationTags(ctx.messages);
    if (tags.length > 0) {
      await saveSharedContext(ctx.sessionID, {
        investigations: tags,
        knowledge: extractKnowledge(ctx.messages),
        timestamp: Date.now(),
      });
    }
  },
}
```

### 和 CacheFirstLoop 的关系

deepseek-reasonix 的 CacheFirstLoop 设计给了我们参考：

```
CacheFirstLoop 三分架构:
┌─────────────────────────────────────────┐
│ IMMUTABLE PREFIX                        │ ← 固定，缓存命中目标
│   system + tool_specs + few_shots       │
├─────────────────────────────────────────┤
│ APPEND-ONLY LOG                         │ ← 增长，可安全 fork
│   [assistant₁][tool₁][assistant₂]...    │
├─────────────────────────────────────────┤
│ VOLATILE SCRATCH                        │ ← 每个 agent 独有
│   R1 thought, transient plan state      │
└─────────────────────────────────────────┘
```

对 oh-my-pi 的改造：
- **SharedContext** 加入 IMMUTABLE PREFIX 之前，作为额外一层
- 子 agent spawn 时自动附加 SharedContext 作为 prefix 一部分
- fork_context（byte-identical prefix）参考 CodeWhale 的实现

### 标签系统

每次调查完成后，结果需要被标记，以便后续 agent 判断是否重复工作：

```typescript
interface InvestigationTag {
  area: string;       // 调查领域，如 "project-structure", "dependencies"
  status: 'done' | 'in-progress' | 'stale';
  summary: string;    // 简短摘要
  fullResult: string; // 完整调查结果
  timestamp: number;
  ttl: number;        // 过期时间（分钟），过期后需重新调查
}
```

## 收益

| 场景 | 无缓存 | 有缓存 | 节省 |
|------|-------|-------|------|
| 1 主 agent + 3 子 agent | 4 次全价 | 1 次全价 + 3 次缓存读取 | ~50-70% |
| 蜂群 5 agent 并行 | 5 次全价 | 1 次全价 + 4 次缓存读取 | ~60-80% |
| 跨 session 复用上下文 | 每次全价 | 已有缓存则全部命中 | ~90%+ |
