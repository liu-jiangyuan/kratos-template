# 03. 核心库 (`core`) 与公共库 (`pkg`) 边界与扩展规范

---

## 1. `core` 与 `pkg` 的准入边界

新增非业务代码时，严格按以下标准归属：

- **归入 `pkg/<name>`**：
  - 不读取 `core/conf` 配置；
  - 不依赖 `ent`；
  - 不依赖框架传输层生命周期；
  - 仅依赖标准库或三方库，可独立复制到任意 Go 项目编译使用。
- **归入 `core/<name>`**：
  - 依赖 `core/conf` 配置初始化；
  - 或属于 HTTP/gRPC 中间件、日志系统、客户端连接池（数据库、缓存、消息队列、注册中心等）；
  - 需要通过 Wire 注入到 `core/server` 或各业务模块的 `biz` / `model` 中。

> **红线**：`core` 与 `pkg` 均**严禁** import `internal/...`、`api/...`、`cmd/...`。

---

## 2. 链路追踪、`request_id` 与日志规范 (`core/log` & `core/middleware/requestid`)

1. **链路追踪与 `request_id` 自动透传**：
   - `core/server` 默认挂载官方 `tracing.Server()`、自定义 `requestid.Server()` 和官方 `logging.Server(logger)` 中间件。
   - `requestid.Server()` 会从请求头读取 `X-Request-Id`；若客户端未携带，后端自动生成 UUIDv7，写回响应头 `X-Request-Id`，并通过 `kratos/v3/log.ContextWithAttrs` 注入 `request_id`。
   - 业务代码中调用 `log.InfoContext(ctx, "...")`（或注入 `core/log.Logger` 调用 `.Info(ctx, "...")`）时，日志会自动携带 `request_id`、`trace_id`、`span_id`。
2. **日志接口与 3 种输出实现 (`core/log`)**：
   - 统一接口：`core/log.Logger`（兼容 `slog.Handler` 与 `io.Closer`），默认导出 `core/log.ProviderSet`。
   - 3 种内置实现（通过 `configs/config.yaml` 的 `log.driver` 切换，默认 `console`）：
     - `console` (`ConsoleLogger`)：输出到终端，默认按日志级别对整行字符进行 ANSI 颜色着色（`debug` 默认颜色、`info` 蓝色、`warn` 黄色、`error` 红色）；
     - `file` (`FileLogger`)：输出到本地文件（无 ANSI 颜色转义字符）；
     - `database` (`DatabaseLogger`)：通过 HTTP 请求将日志发送到 Elasticsearch / ZincSearch 等日志数据库 API（配置 `log.database.endpoint`、`method`、`headers`、`timeout`）。
   - 自定义日志格式（`log.format`）：默认 `"json"`，同时支持 `"text"` 或任意自定义模板字符串（如 `"{time} [{level}] [req={request_id}] [trace={trace_id}] {caller} - {msg} {attrs}"`）。

---

## 3. 数据库事务 (`core/data.InTx`) 约束

`core/data/data.go` 提供基于 `context.Context` 的事务传播：

- `model` 层必须始终通过 `r.data.DB(ctx)` 获取 `*ent.Client`，严禁直接持有或绕过 `DB(ctx)` 调用裸连接。
- 跨表/跨 Repo 的事务通过 `d.InTx(ctx, func(ctx context.Context) error { ... })` 执行；若需在 `biz` 层编排事务，可在 `biz`（或 `core/data`）定义 `Transaction` 接口并通过 Wire 绑定 `*data.Data`。

---

## 4. 基础设施扩展约定

1. **新增配置项**：在 `core/conf/conf.proto` 中定义，执行 `make config` 生成 `conf.pb.go`。
2. **新增全局中间件**：在 `core/middleware/<name>/` 下实现，并在 `core/server/http.go` 或 `core/server/grpc.go` 中挂载。
3. **新增共享存储客户端（如 Redis）**：在 `core/data/data.go` 的 `Data` 中扩展字段，在 `NewData` 中初始化并在 `cleanup` 中释放。
4. **新增服务注册/发现**：在 `core/registry/` 下封装并在 `cmd/<app>/main.go` 的 `newApp` 中通过 `kratos.Registrar(...)` 注入。
