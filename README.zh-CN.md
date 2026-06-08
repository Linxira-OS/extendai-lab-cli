<p align="center">
  <img src="docs/logo.svg" alt="ExtendAI Lab" width="640"/>
</p>

<p align="center">
  <a href="./README.md">English</a>
  &nbsp;·&nbsp;
  <strong>简体中文</strong>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">规格</a>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/npm/l/extendai-lab.svg?style=flat-square&color=8b949e&labelColor=161b22" alt="license"/></a>
  <a href="https://github.com/Linxira-OS/extendai-lab-cli/stargazers"><img src="https://img.shields.io/github/stars/Linxira-OS/extendai-lab-cli.svg?style=flat-square&color=dbab09&labelColor=161b22&logo=github&logoColor=white" alt="GitHub stars"/></a>
  <a href="https://github.com/Linxira-OS/extendai-lab-cli/graphs/contributors"><img src="https://img.shields.io/github/contributors/Linxira-OS/extendai-lab-cli.svg?style=flat-square&color=bc8cff&labelColor=161b22&logo=github&logoColor=white" alt="contributors"/></a>
</p>

<br/>

<h3 align="center">面向终端的 DeepSeek 原生 AI coding agent。</h3>
<p align="center">由配置与插件驱动的极薄 harness——单一静态 Go 二进制，围绕 DeepSeek 的前缀缓存调优，长会话也能把 token 成本压低。</p>

<br/>

> [!NOTE]
> **上游项目：** 本项目 fork 自 [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)（main-v2 分支）。
> 我们在此基础上扩展了额外功能和定制。核心代码的所有功劳归于原作者。

<br/>

## 特性

- **配置驱动**：provider、agent、启用的工具、插件全部在 `extendai-lab.toml` 中声明，
  内核无硬编码模型。
- **多模型 · 可组合**：DeepSeek（flash/pro）与 MiMo 作为预设内置；任何 OpenAI 兼容
  端点都只是一条配置。可选让两个模型协同（执行器 + 规划器），各自独立、缓存稳定的 session。
- **插件驱动**：外部工具以子进程形式运行，通过 stdio JSON-RPC 通信（MCP 兼容）；
  内置工具在编译期自注册。
- **零摩擦分发**：`CGO_ENABLED=0` 单二进制；一条命令交叉编译到六个目标平台。
  唯一依赖是一个 TOML 解析库。

## 安装

```sh
npm i -g extendai-lab                  # 任意系统;自动拉取对应平台的原生二进制
```

预编译归档(`darwin|linux|windows × amd64|arm64`)和 `SHA256SUMS` 见每个
[GitHub release](https://github.com/Linxira-OS/extendai-lab-cli/releases)。

### 从源码构建

```sh
make build      # -> bin/extendai-lab
make cross      # -> dist/（darwin|linux|windows × amd64|arm64）
```

## 快速开始

```sh
extendai-lab setup                      # 配置向导 → ./extendai-lab.toml
export DEEPSEEK_API_KEY=sk-...  # 或写入 .env（见 .env.example）
extendai-lab chat                       # 然后在会话里运行 /init 生成 AGENTS.md（项目记忆）
extendai-lab run "把 main.go 里的 TODO 实现掉"
extendai-lab run --model mimo-pro "给这个函数补单元测试"
echo "解释这段代码" | extendai-lab run
```

## 配置

优先级：**flag > `./extendai-lab.toml` > `~/.config/extendai-lab/config.toml` > 内置默认值**。
密钥经环境变量通过 `api_key_env` 注入，绝不写入配置文件。

```toml
default_model = "deepseek-flash"   # 执行器；设 [agent].planner_model 可加规划器

[agent]
# planner_model = "mimo-pro"          # 可选的低频规划器
auto_plan = "ask"                  # off|ask|on；复杂聊天任务自动进入计划模式

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[tools]
enabled = []   # 省略/为空 = 全部内置工具

[permissions]
mode  = "ask"                                # 无规则命中时 writer 的兜底：ask|allow|deny
deny  = ["bash(rm -rf*)", "bash(git push*)"] # 任何模式下都硬阻断
allow = ["bash(go test*)"]                   # 从不询问

[sandbox]
# workspace_root = ""          # 文件写工具被限制在此目录；留空 = 当前目录

[[plugins]]
name    = "example"
command = "extendai-lab-plugin-example"
```

## 架构

三层可扩展性，全部藏在内核按名解析的 registry 之后：

1. **Registry**：`Provider` 与 `Tool` 是接口；内核没有 `switch model`。
2. **编译期内置**：provider（`provider/openai`）和 tool（`tool/builtin`）通过
   `init()` 自注册，`main` 用 blank import 拉入。新增内置 = 一个文件 + 一行 import。
3. **运行时插件**：配置里声明的可执行文件，通过 stdin/stdout 上的
   newline-delimited JSON-RPC 2.0（MCP stdio 约定）通信，每个远程 tool 适配成
   `Tool` 接口。

## 上游致谢

本项目基于 [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)
由 [esengine](https://github.com/esengine) 开发。我们感谢他们在构建核心 Go agent 框架方面的出色工作。

主要上游贡献者（按字母顺序）：
- [ctharvey](https://github.com/ctharvey)
- [dimasd-angga](https://github.com/dimasd-angga)
- [Evan-Pycraft](https://github.com/Evan-Pycraft)
- [ForeverYoungPp](https://github.com/ForeverYoungPp)
- [GTC2080](https://github.com/GTC2080)
- [kabaka9527](https://github.com/kabaka9527)
- [lisniuse](https://github.com/lisniuse)
- [wade19990814-hue](https://github.com/wade19990814-hue)
- [wviana](https://github.com/wviana)

Logo 设计遵循 [Linxira OS 品牌规范](https://linxira-os.github.io/brand/)。

<br/>

---

<p align="center">
  <sub>MIT —— 见 <a href="./LICENSE">LICENSE</a></sub>
  <br/>
  <sub>基于 <a href="https://github.com/esengine/DeepSeek-Reasonix">DeepSeek-Reasonix</a> by esengine</sub>
</p>
