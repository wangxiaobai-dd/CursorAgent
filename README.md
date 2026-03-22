# CursorAgent

配置驱动的任务调度：在一条或多条任务中绑定输入类型（SVN diff、本地文件、纯提示词）与输出目录，支持定时执行与结果上传。

**支持两种工作方式（可按任务切换）：**

1. **Agent CLI 模式**（默认）：由本进程调用 Cursor **`agent`**，通过 `-p` 传入合并后的提示词，在无界面环境下执行 Skill。
2. **Cursor 可执行文件 + 插件交互模式**：任务配置 `LaunchCursor: true` 时，由本进程启动 **Cursor 主程序**（如 Windows 下的 `cursor.cmd`），并与本机 **HTTP 服务**配合，供 **Cursor 扩展 / 插件**（exe 侧逻辑）通过接口查询状态、写结果、触发上传等。

详见下节「两种模式说明」对照表。

## 功能概览

| 能力 | 说明 |
|------|------|
| 多任务 | 单份 YAML 配置多条 `Tasks`，各自独立 Skill（若适用）、输入、输出与可选 Cron |
| 输入类型 | `svn_diff`：按日期范围取 SVN 提交并生成 diff；`file` / `prompt`：文件或模板提示词 |
| SVN + Agent | **Agent 模式**下，同一任务多个 revision **合并为一次** `agent -p` 调用，提示词结构与插件侧对齐 |
| 结果文件 | 默认写入 `<OutDir>/<OutPrefix>.<YYYY-MM-DD>`；任务运行前会**清空当日结果文件**再执行（见 `job`） |
| 日志 | 可选 `LogFile`：同时写 stderr 与日志文件（`util.SetupLogOutput`） |
| 定时 | Cron 表达式支持**秒**（`github.com/robfig/cron/v3`），格式：秒 分 时 日 月 周 |
| 插件 HTTP | 配置 `PluginListenerPort` 后，本进程暴露本地 API，供 **Cursor 插件**与 Go 侧交互（可与 Agent 模式并存监听） |
| 退出 | `SIGINT` / `SIGTERM` 时先停止 Cron 调度，再关闭插件 HTTP 服务（`PluginListener.Stop`） |

## 两种模式说明：Agent CLI 与 Cursor + 插件（exe）交互

| 对比项 | ① Agent CLI 模式 | ② Cursor + 插件交互模式 |
|--------|------------------|-------------------------|
| 任务开关 | `LaunchCursor: false`（默认） | `LaunchCursor: true` |
| 谁执行审查 | 本进程调用 **`agent`**（`AgentCommand`，默认 `agent`） | 唤起 **Cursor 可执行文件**（`CursorExePath`），由 **IDE 内插件**完成审查流程 |
| Skill | **必填** `Skill`（Cursor 中注册的技能名） | **可不填** `Skill`（校验以插件流程为准）；若仍填写可用于文档或其它用途 |
| 全局依赖 | `agent` 在 PATH 或可执行路径正确 | 必须配置 **`CursorExePath`**；建议配置 **`PluginListenerPort`**（默认 9150），插件与 Go 同端口通信 |
| 典型流程 | `GetInputs` → 合并 prompt → `RunSkill` / `agent -p` | `GetInputs` → 启动 Cursor → 插件通过 **HTTP** 调「是否已检查」「完成上传」等（见下节） |
| 适用场景 | 服务器/定时任务、无界面批量跑 Skill | 需要 IDE、diff 可视化、与已有 Cursor 扩展深度集成 |

说明：**②** 依赖你在 Cursor 侧安装并配置好对应插件；Go 端只负责拉起 Cursor、提供 HTTP 接口与可选上传。**①** 与 **②** 可在不同任务上混用（部分任务 Agent、部分任务 LaunchCursor）。

## 环境要求

- **Go**：见 `go.mod`（例如 1.25+）
- **Agent CLI 模式**：`agent` 在 `PATH` 中，或由配置项 `AgentCommand` 指定可执行文件
- **插件交互模式**：配置 `CursorExePath` 指向本机 Cursor 可执行文件（如 `cursor.cmd`）；插件需能访问 `PluginListenerPort` 对应的本机 HTTP 服务
- **SVN**（仅 `svn_diff`）：本机已安装 `svn`，且能访问配置的 `RepoURL`
- **Windows / Linux**：路径在 YAML 中按平台书写（Windows 可用 `D:\\path` 形式）

## 快速开始

```bash
cd CursorAgent
go build -o cursor-agent .
./cursor-agent -c config/option.yaml
```

首次使用请复制并编辑 `config/option.yaml`：按所选模式填写——**Agent 模式**需 `AgentCommand` / `Skill` 等；**插件模式**需 `CursorExePath`、`LaunchCursor: true`，并建议开启 `PluginListenerPort`；`svn_diff` 还需 `RepoURL`、`ProjectPath`、`DiffDir` 等。

### 命令行参数

| 参数 | 说明 |
|------|------|
| `-c` / `--config` | 配置文件路径，默认 `config/option.yaml` |
| `--run-once` | 每个任务**只执行一次**后**不**启动 Cron；进程仍保持运行直至收到退出信号（与「无 CronTime 的任务启动时跑一遍」可同时使用） |

无 `CronTime` 的任务在启动时执行一次；带 `CronTime` 且未使用 `--run-once` 时由 Cron 调度。

## 配置说明（摘要）

全局常用字段：

| 字段 | 含义 |
|------|------|
| `AgentCommand` | 默认可执行名 `agent` |
| `LogFile` | 非空则日志同时追加到该文件 |
| `DefaultOutDir` / `DefaultUploadURL` | 任务未填 `Output` 时的默认输出目录与上传地址 |
| `CursorExePath` | `LaunchCursor` 时唤起 Cursor 的可执行路径（如 Windows 下 `cursor.cmd`） |
| `PluginListenerPort` | 插件 HTTP 监听端口，默认 `9150`；为 `0` 可不监听 |

单条 `Tasks` 常用字段：

| 字段 | 含义 |
|------|------|
| `Name` | 任务名；插件 HTTP 路径中的 `<task>` 与此一致 |
| `Skill` | Cursor 中注册的 **技能标识**，不是本地 `.md` 文件名 |
| `LaunchCursor` | `true`：走 **Cursor exe + 插件** 流程（见上文模式 ②）；`false`：走 **Agent CLI**（需配置 `Skill`） |
| `CronTime` | 可选，六位 Cron（含秒） |
| `Input` | `Type` 为 `svn_diff` \| `file` \| `prompt`，其余字段随类型不同（见仓库内 `config/option.yaml` 示例） |
| `Output.OutDir` / `OutPrefix` | 结果目录与文件名前缀；**当日结果文件**为 `OutPrefix.YYYY-MM-DD` |
| `Output.UploadURL` | 可选；插件在「完成」接口中可触发上传 |

完整校验逻辑见 `config/option.go` 中 `Validate`。

## `svn_diff` 与提示词

- 程序会为每个 revision 生成 diff 文件，并将**与插件一致的**提示片段拼接为**一条**合并提示（`BuildMergedDiffReviewPrompt`）：包含「请使用 skill 文件 … 检查 @…，只输出结果，不要其他操作」「结果追加写入文件：…，写入头部为：REVISION:…」等。
- 多 revision 时，开头会强调**唯一结果文件绝对路径**，避免写入工作区默认命名的其它文件。
- `IncludeSvnSource: true` 时会在 diff 基础上解析并在提示中附加 `@` 工作副本源码引用（与插件行为对齐）。

## 插件 HTTP（`PluginListener`）

当配置了 `PluginListenerPort`（且非 0）时，进程启动即监听 `http://localhost:<端口>/`。

| 方法 | 路径 | 作用 |
|------|------|------|
| `GET` | `/api/review/checked/<task>?date=YYYY-MM-DD` | 根据任务 `Name` 判断当日结果文件是否存在 |
| `POST` | `/api/review/done/<task>?date=YYYY-MM-DD` | 若配置了 `UploadURL`，尝试上传当日结果文件 |

`<task>` 必须与配置中的 `Tasks[].Name` 一致。

## 项目结构（简要）

```
CursorAgent/
  main.go                 # 入口：配置、日志、插件监听、Cron、信号与优雅退出
  config/                 # YAML 与 Option 加载、校验
  input/                  # NewSource；code（svn_diff）、generic（file/prompt）
  cursor/                 # 调用 Cursor CLI（RunSkill 等）
  job/                    # Job.Run：清空当日结果、执行 runner
  server/                 # 插件 HTTP
  util/                   # 日志、文件、diff 路径解析、编码、上传等
```

## 构建与依赖

```bash
go mod tidy
go build -o cursor-agent .
```

依赖见 `go.mod`（`cron/v3`、`yaml.v2`、`pflag` 等）。

## 扩展建议

- **新输入类型**：扩展 `config.TaskInput`、`input/source.go` 的 `NewSource`，并实现 `input/types` 中的 `InputSource`。
- **新 Skill**：在 Cursor 中注册后，将注册名填入 `Skill` 字段即可。
