# AI Agent Guidelines (AGENTS.md)

本文件定义了基于此 Kratos v3 模板开发时的硬性架构边界与模块规范。AI 助手（Claude Code / Cursor / Antigravity / Copilot 等）在生成、修改或复制模块代码时**必须**遵守本规范，所有详细说明均存放在 [`docs/`](./docs/) 目录下。

## 核心文档索引 (`docs/`)

在进行任何开发或扩展前，请先阅读 [`docs/`](./docs/) 下的对应规范文件：

- [`docs/01-architecture-and-boundaries.md`](./docs/01-architecture-and-boundaries.md)：目录分层职责（`pkg` / `ent` / `core` / `internal/<module>`）与硬性依赖边界规则（由 `test/arch` 强制校验）。
- [`docs/02-module-development-guide.md`](./docs/02-module-development-guide.md)：如何以 `internal/todo` 为蓝本复制/新增一个业务模块（`api` -> `ent/schema` -> `internal/<module>/{biz,model,service,server}` -> `cmd/*/wire.go`）。
- [`docs/03-core-and-extension-guide.md`](./docs/03-core-and-extension-guide.md)：如何在下游项目中按需扩展 `core`（中间件、数据库事务 `InTx`、Redis/注册中心等组件接入约定）与 `pkg`。

## 铁律速览（违反会导致 `make check` / `go test ./test/arch/...` 失败）

1. **`internal/` 只放业务模块目录**：每个业务模块是 `internal/<module>/` 下的独立目录，内部固定包含 `biz`、`model`、`service`、`server` 四个子包及顶层 `<module>.go`（导出 `ProviderSet`）。严禁在 `internal/` 下直接散落 `.go` 文件或非模块公共目录。
2. **模块内单向依赖**：`server` ──► `service` ──► `biz` ◄── `model` ──► `ent`
   - `biz` 只定义 DO（领域对象）、Repo 接口、Usecase，**严禁** import `ent`、`model`、`service`、`server`。
   - `model` 实现 `biz` 的 Repo 接口并调用 `ent` / `core/data`，**严禁** import `api`、`service`、`server`。
   - `service` 实现 `api` 的 gRPC/HTTP 接口并转换 DTO ↔ DO，**严禁** import `ent`、`model`、`server`。
   - `server` 实现 `core/server.Registrar` 注册本模块路由，**严禁** import `ent`、`model`、`biz`。
3. **严禁跨业务模块直接 import**：`internal/<a>/...` **严禁** import `internal/<b>/...`。
4. **基础库单向依赖**：
   - `pkg/...`：纯工具库，**严禁** import `core`、`ent`、`internal`、`api`、`cmd`。
   - `ent/...`：数据库 Schema 与生成代码，**严禁** import `core`、`internal`、`api`、`cmd`。
   - `core/...`：项目级基础设施，可依赖 `pkg` 和 `ent`，**严禁** import `internal`、`api`、`cmd`。

## 每次修改后的必跑验证

```bash
make check   # 运行 go vet ./... 与 go test -v ./test/arch/...
make test    # 运行全量单元测试与架构边界测试
make run     # 本地启动服务 (go run ./cmd/... -conf ./configs)
```
