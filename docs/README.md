# 架构与开发规范索引

本目录只保留模板的核心约束：**目录职责、依赖边界、模块分层、接口契约**。这些规则由 `test/arch` 与 `core/server` 共同保证，下游项目可以在此基础上按需扩展，但不要削弱边界。

Demo 默认是 **HTTP 单体 + 内存 Repo**，`make run` 不需要数据库；Ent/Postgres 代码作为可选扩展保留，gRPC 需要时再扩展。

## 文档列表

- [`01-architecture-and-boundaries.md`](./01-architecture-and-boundaries.md)：目录职责与硬性依赖边界。
- [`02-module-development-guide.md`](./02-module-development-guide.md)：新增业务模块的标准结构。
- [`03-core-and-extension-guide.md`](./03-core-and-extension-guide.md)：`core` / `pkg` 准入、日志、事务与扩展点。

## 铁律速览

1. **`internal/` 只放业务模块目录**：每个模块固定包含 `biz`、`model`、`service`、`server` 四个子包和顶层 `<module>.go`（导出 `wire.ProviderSet`）。禁止在 `internal/` 下散落 `.go` 文件或公共目录。
2. **模块内单向依赖**：`server ──► service ──► biz ◄── model ──► (memory / ent)`
   - `biz` 只定义 DO、Repo 接口、Usecase、领域错误；禁止依赖 `ent`、本模块 `model`/`service`/`server`。
   - `model` 实现 Repo 接口（Demo 默认 memory，可选 ent）；禁止依赖 `api`、本模块 `service`/`server`。
   - `service` 实现 `api` 接口并转换 DTO ↔ DO；禁止依赖 `ent`、本模块 `model`/`server`。
   - `server` 只实现 `core/server.Registrar`；禁止依赖 `ent`、本模块 `model`/`biz`。
3. **禁止跨业务模块直接 import**：`internal/<a>/...` 不得 import `internal/<b>/...`。模块间协作通过事件、契约包或 API client 显式完成。
4. **基础库单向依赖**：
   - `pkg/...`：纯工具库，不得 import `core`、`ent`、`internal`、`api`、`cmd`。
   - `ent/...`：不得 import `core`、`internal`、`api`、`cmd`。
   - `core/...`：可依赖 `pkg`、`ent`，不得 import `internal`、`api`、`cmd`。

## HTTP 响应契约

统一响应与状态码映射定义在 [`README.md`](../README.md) 的「HTTP 响应契约」一节。简要规则：

- 进入业务处理后的成功/业务失败：**HTTP 200**，`code=1/0`。
- 协议、认证、系统错误：使用真实 HTTP 状态码，`code=0`。

## 修改后的必跑验证

```bash
make check   # gofmt + go vet + test/arch
make test    # 单元测试 + 架构测试 + HTTP 集成测试
make run     # 本地启动（无需数据库）
```
