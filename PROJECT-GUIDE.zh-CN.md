# pigeon 项目分析：结构、启动方式与依赖

> 本文是对本仓库（`github.com/hoki2020/pigeon`，v0.1.13xx 系列）的静态分析结论，
> 并结合了在 **Windows 10 + Go 1.26.8** 上的实际构建与启动验证。
> 仓库自带的权威文档见 [`README.md`](README.md)、[`docs/reference.md`](docs/reference.md)、
> [`docs/subsystems/`](docs/subsystems/README.md)。

---

## 1. 这是什么项目

**pigeon 是一个“把各种 AI 编程 Agent 的模型设置集中到一处”的本地工具。**

它做的事：

- 统一管理机器上几十种 Agent（Claude Code、Codex、Gemini CLI、OpenCode、Cursor、Zed、Cline…）的
  配置文件，只改“模型”这一个键，注释/顺序/格式保持不变，写入是原子的。
- 在本机跑一个 **本地 LLM 网关**（默认 `127.0.0.1:3425`），同时会说
  OpenAI Chat Completions / OpenAI Responses / Anthropic Messages / Gemini 四种协议，
  并在协议之间做流式翻译（含工具调用、推理内容）。
- 把“订阅账号”（Claude、ChatGPT、Copilot、Gemini…）变成所有 Agent 都能用的 Provider。
- 额度用尽时按 **Routing Group** 自动切到下一个账号/模型，Agent 无感。
- 请求离开本机前做 **隐私打码**（占位符替换敏感值，回复里再还原）。

形态：**单一二进制**，同时提供 macOS 菜单栏应用、Windows/Linux 桌面窗口、TUI（终端界面）、
纯 CLI、`pigeon web`（浏览器界面）、Docker 和 Termux。

仓库规模（当前 checkout）：

| 指标 | 数量 |
| --- | --- |
| Go 源文件 | 3,706 |
| 其中测试文件 | 2,570 |
| `internal/` 子包 | 约 75 个 |
| 内置插件 | 14 个（`plugins/*`） |
| 前端浏览器回归测试 | 895 个 `.test.cjs` |

> 注意：README 写明“本仓库不接受 Pull Request”，只允许维护者开 PR。

---

## 2. 技术栈

| 层 | 技术 |
| --- | --- |
| 语言 | Go **1.26.8**（`go.mod` 明确固定到补丁版本） |
| 桌面窗口 | [Wails v3](https://wails.io) `v3.0.0-beta.24`，用系统 WebView（macOS WebKit / Windows WebView2 / Linux WebKitGTK） |
| 终端界面 | Charm 的 `bubbletea` + `bubbles` + `lipgloss` |
| 前端 | 原生 JS/CSS，**无框架、无打包器**；资源用 `//go:embed assets` 打进二进制（`internal/gui/assets`） |
| 数据库 | `modernc.org/sqlite`（**纯 Go，不需要 cgo**） |
| 插件运行时 | 内嵌 [moejs](https://github.com/Calcium-Ion/moejs) 跑 JS 中间件；npm 上的 OpenCode/pi 插件跑在**按需下载的 Bun** 上 |
| 平台 API | `go-ole`（Windows COM）、`godbus`（Linux D-Bus）、`go-toast`（通知） |
| 网站 | `site/` 是 Cloudflare Worker（`worker.js` + `wrangler.jsonc`），与主程序无关 |

---

## 3. 目录结构

```
qypigeon/
├── main.go                  # 唯一入口：只做 cli.Main(version)
├── go.mod / go.sum          # 模块 github.com/hoki2020/pigeon, go 1.26.8
├── Makefile                 # build / cli / app / release / test / dev 等配方
├── Dockerfile               # 两阶段：golang:1.26-alpine 编译 → debian:12-slim 运行
├── AGENTS.md / CLAUDE.md    # 给编码 Agent 的项目约定（含“哪些代码真正在跑”）
├── LESSONS.md               # 历史事故复盘：改代码前的规矩清单
│
├── internal/                # 程序主体（约 75 个包）
├── plugins/                 # 内置插件（编译期注册，不是外部插件）
├── native/privacy-filter/   # 本地隐私模型 pf-serve 的构建脚本（CMake + Vulkan/Metal）
├── build/                   # 平台构建资源：darwin/、windows/、icon/、dev.sh、race-shards.sh
├── docs/                    # reference.md（完整手册）、subsystems/（各子系统设计文档）、code-standards.md
└── site/                    # usemagpie.ai 官网（Cloudflare Worker，与二进制无关）
```

### 3.1 `internal/` 关键包

按职责分组（每个包首行注释即其定位）：

**入口与界面**

| 包 | 职责 |
| --- | --- |
| `cli` | 所有命令的分发，`usage` 帮助文本，`serve` / `web` / `tui` / `app` 入口 |
| `gui` | 桌面外壳：Wails 窗口、托盘与面板、`/api/*` JSON 接口、`assets/` 前端、`pigeon web` |
| `tui` | 终端界面；没有其他 pigeon 时它自己托管网关 |
| `appdir` | 决定 pigeon 自己的文件放哪（`~/.config/pigeon`、`~/.cache/pigeon`，或便携模式 `data/`） |

**核心域**

| 包 | 职责 |
| --- | --- |
| `gateway` | 本地 LLM 网关：路由、协议翻译、流式、故障转移、用量记账、CORS/来源校验 |
| `provider` | Provider / API Key / 订阅账号 / 额度，`migrate*.go` 负责把旧内置订阅迁到插件 |
| `agent` | 每个 Agent 的配置文件位置、格式、读写（几十个 `*_cli.go` / `*.go` 适配器） |
| `catalog` | 从 models.dev 拉模型目录与价格 |
| `access` | 网关对外的具名 Key（`caller-keys.json`） |
| `usage` / `usageapi` | `usage.jsonl` 记账与 Usage 页面数据 |
| `settings` / `profile` / `library` | 设置、Agent 配置快照（profile）、指令/MCP/skill 库 |

**插件体系**

| 包 | 职责 |
| --- | --- |
| `pluginrt` | 内置插件注册表与生命周期（每个插件一个 `Manifest`） |
| `plugin` | 外部插件宿主：下载 Bun、跑 `host.js`、npm 包安装、中间件 |
| `middleware` / `gwmw` / `gwtool` / `gwbudget` | 网关的三个“插件点”：中间件、工具、限额 |
| `agentplug` / `jsmod` | 给 Agent 用的插件模块，及 moejs 的编译封装 |

**隐私与安全**

| 包 | 职责 |
| --- | --- |
| `redact` / `pii` / `masking` / `maskrec` | 打码规则、本地模型（pf-serve）调用、打码记录 |
| `netguard` / `netproxy` | 下载与代理的统一管控（含 DNS rebinding 防护） |
| `mcpauth` | 远程 MCP 服务器登录 |

**运行支撑**

`filelock`（跨进程文件锁）、`filememo`（文件解析缓存）、`steady` / `lastgood`（半写文件保护）、
`beat`（定时循环）、`daemon`（systemd/launchd 服务）、`autostart`、`update`、`handoff`（网关端口热交接）、
`sqlitedb`、`sessions`、`stats`、`otlp`、`davsync` / `backup`（同步与加密备份）、`wslrun`（Windows 下驱动 WSL）。

### 3.2 `plugins/` 内置插件（编译进主程序）

每个插件在 `init()` 里 `pluginrt.Register(pluginrt.Manifest{...})`，可在 GUI 的
Plugins › Built-in 里开关，配置写到 `plugins.yml`。

| 目录 | 名称 | 是否常驻（Essential） |
| --- | --- | --- |
| `budget` | Key limits（网关 Key 限额） | ✅ |
| `gateway` | Gateway 页面 | ✅ |
| `middleware` | JS 网关插件 | ✅ |
| `redact` | 隐私打码 | ✅ |
| `rotation` | 轮换策略 | ✅ |
| `routing` | Routing 页面 | ✅ |
| `search` | 联网搜索 | ✅ |
| `tools` | 插件工具（MCP） | ✅ |
| `usage` | Usage 页面 | ✅ |
| `groups` | 路由分组 | — |
| `library` | 指令/MCP/Skill 库 | — |
| `otel` | OpenTelemetry 导出 | — |
| `sessions` | Sessions 页面 | — |
| `warmups` | 每日预热与签到 | — |

`plugins/all/all.go` 是所有内置插件的聚合注册点，**只有 `main.go` 导入它**（核心不依赖插件）。

---

## 4. 架构与运行时路径

```
main.go
  └─ internal/cli.Main(version)
       ├─ 无参数        → GUI（有 GUI 构建）/ TUI（nogui 构建）
       ├─ pigeon app|tray|panel → internal/gui（Wails 窗口 + 托盘）
       │        └─ startBackend → 本进程起 gateway（或发现已有 pigeon 在服务）
       │        └─ Handler: 内嵌 assets/ + /api/* JSON 接口
       ├─ pigeon web    → 同一套页面，浏览器访问（默认 127.0.0.1:3430，一次性 key 登录）
       ├─ pigeon serve  → 只跑网关（可加 --web 同时出页面）
       ├─ pigeon tui    → 终端界面，必要时托管网关
       └─ 其余命令      → providers / groups / accounts / usage / quota / plugin / daemon …
```

关键点：

- **网关端口**：`127.0.0.1:3425`（`gateway.DefaultAddr`，`MAGPIE_ADDR` 可改）。
  `pigeon web` 页面端口 `127.0.0.1:3430`。开发模式用 `3426`–`3429`。测试代码避开 `3425`–`3437`。
- **前端**：`internal/gui/assets/` 的 `app.js`、`app.css`、`i18n.js` 等由 `//go:embed` 打进二进制；
  页面与 Go 之间只有 JSON API（`/api/state`、`/api/set`…），**没有代码生成、没有 npm 构建**。
- **Agent 配置改写**：`internal/agent` 里每个 Agent 一个适配器，只改模型相关的那一个键。
- **插件**：内置插件是 Go 代码；外部插件（npm 包 / pi 包 / `*.middleware.js`）跑在
  按需下载的 **Bun 1.4.2** 上（缓存在 `~/.cache/magpie/bun/<version>/pigeon-bun`）。
- **隐私模型**：`native/privacy-filter` 是 C++（ggml）程序 `pf-serve`，正常**不随源码编译**，
  而是首次启用时按 SHA-256 校验下载预编译版（约 1.6GB 模型）。

---

## 5. 数据与端口

| 位置 | 内容 |
| --- | --- |
| `~/.config/magpie/`（`XDG_CONFIG_HOME`） | `settings.json`、`providers.json`、`logins.json`、`profiles.json`、`plugins.yml`、`caller-keys.json`、`usage.jsonl`、`routing/`、`affinity.json`… |
| `~/.cache/magpie/`（`XDG_CACHE_HOME`） | Bun、`models.json`（models.dev 目录）、各厂商模型列表、图标缓存 |
| 便携模式 | 可执行文件旁存在 `data/` 目录或 `.portable` 文件时，所有数据都写进 `data/` |
| 端口 | 3425 网关 · 3430 `pigeon web` · 3426–3429 开发模式 |

常用环境变量：`MAGPIE_ADDR`（网关地址）、`MAGPIE_NO_ACCOUNTS=1`（本次运行不读取各 Agent 的登录态）、
`DO_NOT_TRACK=1`（关闭每日匿名统计）、`MAGPIE_PROXY` / `MAGPIE_MIRROR`（下载代理与镜像）、
`MAGPIE_THEME=light|dark`、`MAGPIE_DEBUG=1`。

---

## 6. 启动方式

### 6.1 直接用发行版（推荐给“只想用”的场景）

从 [usemagpie.ai](https://usemagpie.ai) 下载，或：

```sh
curl -fsSL https://usemagpie.ai/install.sh | sh
go install github.com/hoki2020/pigeon@latest
```

### 6.2 从源码构建

```sh
make build      # ./pigeon，带桌面窗口（需要平台 webview；Linux 需 cgo）
make cli        # 纯终端版，CGO_ENABLED=0，可任意交叉编译
make app        # macOS：pigeon.app（无 Dock 图标的菜单栏应用）
make release-windows  # dist/：Windows 桌面程序（amd64 + arm64，交叉编译）
make release-linux    # dist/：本机架构的 Linux 桌面程序
```

**没有 make 时（Windows 上很常见）直接调 Go：**

```powershell
# 桌面版（Windows 不需要 cgo / gcc）
go build -o pigeon.exe .

# 纯 CLI 版
$env:CGO_ENABLED=0; go build -tags nogui -o pigeon-cli.exe .

# 带版本号
go build -trimpath -ldflags "-s -w -X main.version=v0.1.1300" -o pigeon.exe .
```

> 你当前的机器上 `make` **没有安装**，所以要么装 make（`choco install make` / `winget`），
> 要么直接用上面的 `go build`。我实测两条命令都能在 Windows 上构建成功。

### 6.3 运行

```powershell
.\pigeon.exe                # 打开窗口 + 托盘
.\pigeon.exe tray           # 只开托盘
.\pigeon.exe web --no-open  # 浏览器界面（终端会打印带一次性 key 的链接）
.\pigeon.exe serve          # 只跑网关，127.0.0.1:3425
.\pigeon.exe tui            # 终端界面
.\pigeon.exe ls             # 列出检测到的 Agent 及其当前模型
.\pigeon.exe --help         # 全部命令
```

### 6.4 开发模式（改前端自动刷新）

```sh
make dev        # 需要 fswatch 才有 Go 热重启；否则退化为 dev-once
```

`make dev` 用 `-tags dev` 编译，前端直接从 `internal/gui/assets` 读取（改 CSS/JS 保存即刷新），
并使用独立端口 3426–3429，不会打断你正在用的 pigeon。

### 6.5 其他形态

```sh
pigeon daemon install   # Linux systemd --user / macOS launchd 常驻服务
docker run -p 3425:3425 -v pigeon:/config ghcr.io/yetone/magpie
```

---

## 7. 你的电脑需要装什么依赖？

你的环境我实际检查过：**Go 1.26.8 ✅、Node v22.22.2 ✅、Git 2.46 ✅、WebView2 155.0.4283.62 ✅，
`gcc` / `make` / `bun` ❌**。

### 7.1 只运行官方发行版

**不需要任何依赖。** Windows 的 WebView2 随系统（Win10/11 自带，你的机器已有）。
Bun 会在第一次用插件时自动下载；隐私模型在启用时才下载。

### 7.2 在你这台 Windows 上从源码构建并运行

| 依赖 | 现状 | 说明 |
| --- | --- | --- |
| **Go 1.26.8** | ✅ 已装且版本完全匹配 | `go.mod` 写死 `go 1.26.8`；装了别的版本时 `GOTOOLCHAIN=auto` 会自动拉取，但网络受限时建议手动装 1.26.8 |
| **WebView2 Runtime** | ✅ 已有 155.x | Win10 1803+ 通常自带；缺失时程序会弹窗提示并给出下载页 |
| **gcc / cgo** | ❌ 不需要 | **Windows 上的 GUI 走 WebView2，不用 cgo**，`CGO_ENABLED=0` 即可构建。cgo 只在 Linux（GTK3/WebKitGTK）和 macOS 上需要 |
| **make** | ❌ 未安装 | 只影响 `Makefile` 里的快捷命令；直接 `go build` 等价 |
| **Git** | ✅ 已有 | 仅用于 `git describe` 生成版本号；没有也能构建（版本变成 `dev`） |
| **Bun** | ❌ 未安装 | 运行期按需自动下载；只有跑 `internal/plugin` 的端到端测试才需要手动装 |

**结论：你这台机器现在已经可以直接 `go build` 出可运行的 Windows 版 pigeon（我已实测通过）。**
唯一建议补的是 `make`（可选，图方便）。

### 7.3 开发 / 测试才需要的

| 用途 | 依赖 |
| --- | --- |
| Go 单测 | `go test ./...`；笔记本上建议 `-timeout 45m`（`internal/gateway` 很重） |
| 前端回归测试（895 个 `*.test.cjs`） | **Node 22**（已有）+ `npx playwright install chromium webkit` |
| 插件端到端测试 | **Bun 1.4.2**（CI 用 `oven-sh/setup-bun`） |
| Race 检测 | `go test -race`；CI 在非 Windows 上跑（Windows 只做“编译测试”） |
| 重新编译 `native/privacy-filter` | CMake、C++ 工具链、Vulkan SDK / Metal（一般不需要，官方有预编译产物） |
| Linux 桌面构建 | `libgtk-3-dev`、`libwebkit2gtk-4.1-dev`，且必须加 `-tags gtk3`、开 cgo |

### 7.4 依赖速查（`go.mod` 直接依赖）

运行时库全部是纯 Go 或已内置：Wails v3（窗口）、bubbletea/bubbles/lipgloss（TUI）、
moejs（JS 插件）、modernc sqlite（数据库）、cobra 无关的自研 CLI、gjson/jsonc（JSON）、
go-toml/yaml、klauspost/compress、coder/websocket、golang.org/x/{net,sys,term,text}。

---

## 8. 构建标签（build tags）说明

| 标签 | 作用 |
| --- | --- |
| （无） | 默认构建，**包含桌面 GUI**（Windows/macOS 可行；Linux 需要 `gtk3`+cgo） |
| `nogui` | 纯终端版：`internal/gui` 的 Wails 文件不参与编译，`runGUI` 报错并提示用 `pigeon tui` |
| `gtk3` | Linux 桌面：用 GTK3 + WebKitGTK 4.1（而非 Wails 默认的 GTK4/WebKitGTK6） |
| `dev` | 开发模式：前端从磁盘读取、可热刷新、独立端口 |
| `production` | Wails 的生产构建标签（`Makefile` 与 `release-windows` 使用），Go 代码里没有自己的 `//go:build production` |

---

## 9. 我做的实际验证（供你复现）

```bash
# 1) 纯 CLI 构建（CGO 关闭）
go build -tags nogui -o pigeon-cli.exe .          # ✅ 成功，71.7 MB
# 2) 桌面版构建（Windows 无 cgo）
go build -o pigeon.exe .                          # ✅ 成功，81.6 MB
# 3) 运行
./pigeon.exe --version                            # pigeon dev
./pigeon.exe ls                                   # ✅ 正确列出本机 Claude Code/OpenCode/VS Code/Cline/ZCode 等
# 4) 用隔离的配置目录起 web + 网关
XDG_CONFIG_HOME=/tmp/mp-cfg XDG_CACHE_HOME=/tmp/mp-cache \
MAGPIE_ADDR=127.0.0.1:3445 ./pigeon.exe web --no-open --addr 127.0.0.1:3446
curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3446/api/state   # 401（未带链接里的 key，符合预期）
curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3445/v1/models   # 200 ✅ 网关正常
```

> 测试期间我用了临时的 `XDG_CONFIG_HOME`/`XDG_CACHE_HOME` 并已清理，没有写你的 `~/.config/pigeon`。
> 但请注意：**pigeon 启动时默认会读取各 Agent 已登录的账号**（日志里能看到它尝试刷新 Codex/Gemini 登录态）。
> 实验时建议同时隔离 `USERPROFILE`/`HOME`，或用 `MAGPIE_NO_ACCOUNTS=1` 关掉账号发现。

---

## 10. 上手建议（针对你）

1. **先跑发行版**确认功能和端口（`pigeon web` 最省事，不需要桌面环境）。
2. **要改代码**：在这台 Windows 上直接
   `go build -o pigeon.exe .` → `.\pigeon.exe`；改前端时用
   `make dev`（先装 make + fswatch）或手动 `-tags dev`。
3. **改之前先读**：`AGENTS.md` → `LESSONS.md` → `docs/code-standards.md`，
   以及 `docs/subsystems/` 里对应子系统的设计文档（仓库明确要求同步更新这些文档）。
4. **提交前跑**：`gofmt -l`、`go vet ./...`、`go build`、`go test -timeout 45m ./...`；
   改了 GUI 还要跑 Node + Playwright 的 895 个浏览器回归测试。
5. **注意仓库规则**：上游不接受外部 PR；本地修改请自建分支，别直接往 `main` 推。

---

## 11. 参考文档索引

| 文档 | 内容 |
| --- | --- |
| [`README.md`](README.md) / [`README.zh-CN.md`](README.zh-CN.md) | 功能总览 |
| [`docs/reference.md`](docs/reference.md) | 完整手册：每个 Agent、Provider 选项、网关端点、CLI、文件位置（3,164 行） |
| [`docs/subsystems/`](docs/subsystems/README.md) | 14 个子系统设计文档（网关路由、插件运行时、GUI 外壳、账号、同步备份…） |
| [`docs/code-standards.md`](docs/code-standards.md) | 代码规范与验收标准 |
| [`LESSONS.md`](LESSONS.md) | 历史事故复盘（改代码前的必读清单） |
| [`AGENTS.md`](AGENTS.md) | 内置订阅迁移到插件的现状（“哪些代码真正在跑”） |
| [`docs/integrating.md`](docs/integrating.md) | 让第三方 App/Agent 接入网关 |
