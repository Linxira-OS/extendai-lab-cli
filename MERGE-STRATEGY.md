# 上游合并策略

本项目 fork 自 [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)（main-v2 分支）。

## 核心原则

1. **Import 路径保持 `reasonix/internal/...`** — 上游合并零冲突
2. **只改用户可见字符串** — CLI 输出、帮助文本、i18n 消息
3. **不改核心逻辑** — 新功能放独立 package 或 MCP 插件
4. **每次合并后立即测试**

## 分支结构

| 分支 | 用途 |
|---|---|
| `main` | 稳定版本 |
| `upstream-sync` | 上游同步专用 |
| `feature/*` | 我们的功能分支 |

## 合并流程

### Step 1: upstream-sync 拉上游

```bash
git checkout upstream-sync
git fetch upstream main-v2
git merge upstream/main-v2
```

### Step 2: 测试

```bash
go build ./...
go test ./...
```

### Step 3: feature 分支 rebase

```bash
git checkout feature/rebrand
git rebase upstream-sync
# 解决冲突（通常只有用户可见字符串）
```

### Step 4: 合并到 main

```bash
git checkout main
git merge feature/rebrand
git push origin main
```

## 冲突预期

| 文件类型 | 冲突概率 | 原因 |
|---|---|---|
| `internal/cli/*.go` | 中 | 帮助文本、i18n 消息 |
| `internal/i18n/*.go` | 中 | 翻译字符串 |
| `cmd/extendai-lab/main.go` | 低 | 包名注释 |
| `internal/agent/*.go` | 低 | 系统提示字符串 |
| 其他核心文件 | 极低 | 我们没改 |

## 我们的改动清单

### 用户可见字符串（需要每次合并后检查）
- CLI 命令名：`reasonix` → `extendai-lab`
- 配置文件名：`reasonix.toml` → `extendai-lab.toml`
- 配置目录：`.reasonix/` → `.extendai-lab/`
- 帮助文本、i18n 消息

### 保持不变（上游兼容）
- Go 模块名：`reasonix`
- Import 路径：`reasonix/internal/...`
- 核心逻辑：与上游一致

## 防冲突原则

1. **新功能放独立 package** — 不侵入 `internal/agent/`、`internal/tool/` 等核心目录
2. **用 MCP 插件扩展** — 通过配置添加功能，不改代码
3. **字符串改动最小化** — 只改用户可见部分，不改注释和日志
4. **测试覆盖** — 每次合并后运行完整测试套件

## 快速检查清单

合并上游后，检查以下文件是否还有 `reasonix`（不含 import 路径）：

```bash
grep -rn "reasonix" -- "*.go" | grep -v "reasonix/internal" | grep -v "//.*reasonix"
```

如果有，批量替换：
```powershell
Get-ChildItem -Recurse -Filter "*.go" | ForEach-Object {
    $content = Get-Content $_ -Raw
    if ($content -match 'reasonix' -and $content -notmatch '"reasonix/') {
        $newContent = $content -replace 'reasonix', 'extendai-lab'
        Set-Content $_ $newContent -NoNewline
    }
}
```

## 上游 Fork 信息

- **上游仓库**：https://github.com/esengine/DeepSeek-Reasonix
- **上游分支**：main-v2
- **我们的 Fork**：https://github.com/Linxira-OS/extendai-lab-cli
- **Fork 关系**：GitHub 自动同步，可提 PR 回上游
