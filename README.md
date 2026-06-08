<p align="center">
  <img src="docs/logo.svg" alt="ExtendAI Lab" width="640"/>
</p>

<p align="center">
  <strong>English</strong>
  &nbsp;·&nbsp;
  <a href="./README.zh-CN.md">简体中文</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">Spec</a>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/npm/l/extendai-lab.svg?style=flat-square&color=8b949e&labelColor=161b22" alt="license"/></a>
  <a href="https://github.com/Linxira-OS/extendai-lab-cli/stargazers"><img src="https://img.shields.io/github/stars/Linxira-OS/extendai-lab-cli.svg?style=flat-square&color=dbab09&labelColor=161b22&logo=github&logoColor=white" alt="GitHub stars"/></a>
  <a href="https://github.com/Linxira-OS/extendai-lab-cli/graphs/contributors"><img src="https://img.shields.io/github/contributors/Linxira-OS/extendai-lab-cli.svg?style=flat-square&color=bc8cff&labelColor=161b22&logo=github&logoColor=white" alt="contributors"/></a>
</p>

<br/>

<h3 align="center">A DeepSeek-native AI coding agent for your terminal.</h3>
<p align="center">A config- and plugin-driven harness — a single static Go binary, tuned around DeepSeek's prefix cache so token costs stay low across long sessions.</p>

<br/>

> [!NOTE]
> **Upstream:** This project is forked from [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) (main-v2 branch).
> We extend it with additional features and customizations. All credit for the core codebase goes to the original authors.

<br/>

## Features

- **Config-driven.** Providers, the agent, enabled tools, and plugins are all
  declared in `extendai-lab.toml`. No hardcoded models.
- **Multi-model & composable.** DeepSeek (flash/pro) and MiMo ship as presets;
  any OpenAI-compatible endpoint is a config entry, not new code. Optionally run
  two models together (executor + planner) in separate, cache-stable sessions.
- **Plugin-driven.** External tools run as subprocesses over stdio JSON-RPC
  (MCP-compatible). Built-in tools self-register at compile time.
- **Zero-friction distribution.** `CGO_ENABLED=0` single binary; cross-compile
  to six targets with one command. The only dependency is a TOML parser.

## Install

```sh
npm i -g extendai-lab                  # any OS; pulls the prebuilt native binary
```

Prebuilt archives (`darwin|linux|windows × amd64|arm64`) and `SHA256SUMS` are on
every [GitHub release](https://github.com/Linxira-OS/extendai-lab-cli/releases).

### Build from source

```sh
make build      # -> bin/extendai-lab
make cross      # -> dist/ (darwin|linux|windows × amd64|arm64)
```

## Quick start

```sh
extendai-lab setup                      # config wizard → ./extendai-lab.toml
export DEEPSEEK_API_KEY=sk-...  # or put it in .env (see .env.example)
extendai-lab chat                       # then run /init to generate AGENTS.md (project memory)
extendai-lab run "implement the TODOs in main.go"
extendai-lab run --model mimo-pro "add unit tests for this function"
echo "explain this code" | extendai-lab run
```

## Configuration

Resolution order: **flag > `./extendai-lab.toml` > `~/.config/extendai-lab/config.toml` >
built-in defaults**. Secrets come from the environment via `api_key_env` and are
never stored in config files.

```toml
default_model = "deepseek-flash"   # executor; set [agent].planner_model to add a planner
# language    = "zh"               # ui language; empty = auto-detect from $LANG

[agent]
# planner_model = "mimo-pro"          # optional low-frequency planner
auto_plan = "ask"                  # off|ask|on; complex chat tasks start in plan mode

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[tools]
enabled = []   # omit/empty = all built-ins

[permissions]
mode  = "ask"                                # writer fallback when no rule matches: ask|allow|deny
deny  = ["bash(rm -rf*)", "bash(git push*)"] # hard-blocked in every mode
allow = ["bash(go test*)"]                   # never prompted

[sandbox]
# workspace_root = ""          # file-writers confined here; empty = current dir

[[plugins]]
name    = "example"
command = "extendai-lab-plugin-example"
```

## Architecture

Three tiers of extensibility, all behind registries the core resolves by name:

1. **Registry** — `Provider` and `Tool` are interfaces; the core has no
   `switch model`.
2. **Compile-time built-ins** — providers (`provider/openai`) and tools
   (`tool/builtin`) self-register via `init()`; `main` blank-imports them.
   Adding a built-in is one file plus one import.
3. **Runtime plugins** — executables declared in config, spoken to over
   newline-delimited JSON-RPC 2.0 on stdin/stdout (the MCP stdio convention).
   Each remote tool is adapted to the `Tool` interface.

## Upstream Acknowledgments

This project is based on [DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)
by [esengine](https://github.com/esengine). We are grateful for their excellent work
in building the core Go agent framework.

Key upstream contributors (alphabetical):
- [ctharvey](https://github.com/ctharvey)
- [dimasd-angga](https://github.com/dimasd-angga)
- [Evan-Pycraft](https://github.com/Evan-Pycraft)
- [ForeverYoungPp](https://github.com/ForeverYoungPp)
- [GTC2080](https://github.com/GTC2080)
- [kabaka9527](https://github.com/kabaka9527)
- [lisniuse](https://github.com/lisniuse)
- [wade19990814-hue](https://github.com/wade19990814-hue)
- [wviana](https://github.com/wviana)

Logo design follows the [Linxira OS brand guidelines](https://linxira-os.github.io/brand/).

<br/>

---

<p align="center">
  <sub>MIT — see <a href="./LICENSE">LICENSE</a></sub>
  <br/>
  <sub>Based on <a href="https://github.com/esengine/DeepSeek-Reasonix">DeepSeek-Reasonix</a> by esengine</sub>
</p>
