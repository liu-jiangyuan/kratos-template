# 03. `core` / `pkg` 边界与扩展规范

---

## 1. 准入边界

- **归入 `pkg/<name>`**
  - 不读取 `core/conf`；
  - 不依赖 `ent`；
  - 不依赖框架传输层生命周期；
  - 仅依赖标准库或三方库，可独立复制到其他项目。

- **归入 `core/<name>`**
  - 依赖 `core/conf` 初始化；
  - 或属于全局中间件、日志、连接池、注册中心等基础设施；
  - 需要通过 Wire 注入到 `core/server` 或业务模块。

> `core` 与 `pkg` 均禁止 import `internal/...`、`api/...`、`cmd/...`。

---

## 2. `request_id` 与日志

- `core/server` 默认挂载 `requestid.Server()` 和 `logging.Server(logger)`。
- `requestid.Server()`：
  - 从 `X-Request-Id` 读取；不存在时生成 UUIDv7；
  - 写回响应头；
  - 通过 `kratos/v3/log.ContextWithAttrs` 注入 `request_id`。
- 业务代码使用 `log.InfoContext(ctx, "...")` 或注入的 `core/log.Logger.Info(ctx, ...)` 时，日志会自动携带 `request_id`。
- `trace_id` / `span_id` 字段存在，但**默认没有安装 OpenTelemetry exporter**，因此为空；需要链路追踪时按第 4 节扩展。
- `core/log` 支持三种输出：
  - `console`：终端输出，按级别着色；
  - `file`：本地文件；
  - `database`：通过 HTTP API 发送到 Elasticsearch / ZincSearch 等。
- 自定义格式通过 `log.format` 配置，支持 `json`、`text` 或模板字符串（`{time}`、`{level}`、`{request_id}`、`{trace_id}`、`{attrs}` 等）。
- `json` 模式下：`console` 输出形如 `[INFO] {json}` 便于终端阅读；`file` 与 `database` 输出标准单行 JSON（含 `"level"` 键值），便于直接采集与入库。

---

## 3. 事务 (`core/data.InTx`，可选)

Demo 默认使用内存 Repo，不涉及事务。启用 Ent 后：

- `model` 层必须通过 `r.data.DB(ctx)` 获取 `*ent.Client`，不要绕过事务上下文直接持有连接。
- 跨 Repo 的事务：

  ```go
  err := d.InTx(ctx, func(ctx context.Context) error {
      // 所有 repo 调用都使用这个 ctx
      return nil
  })
  ```

- 嵌套 `InTx` 会复用外层事务。

---

## 4. 扩展点

### 新增配置项

在 `core/conf/conf.proto` 定义，执行 `make config`。

### 新增全局中间件

在 `core/middleware/<name>/` 实现，并在 `core/server/http.go` 挂载。

### 新增统一响应规则

修改 `core/server/response.go`：

- envelope 字段为 `code` / `message` / `data`；
- `errorHTTPStatus` 决定 HTTP 状态码；
- `errorMessage` 决定客户端可见文案；
- 业务错误使用 HTTP 200，协议/认证/系统错误使用真实状态码。

### 新增链路追踪

模板默认不挂载 tracing。接入时：

1. 安装 OpenTelemetry SDK 与 exporter，并添加 `github.com/go-kratos/kratos/contrib/otel/v3`；
2. 在启动阶段配置全局 `TracerProvider`；
3. 在 `core/server/http.go` 的 middleware 中加入 `tracing.Server()`（`github.com/go-kratos/kratos/contrib/otel/v3/tracing`）。

此时日志中的 `trace_id` / `span_id` 会自动从 context 中提取。

### 新增存储客户端（Redis、MQ 等）

在 `core/data/data.go` 的 `Data` 中扩展字段，在 `NewData` 初始化并在 `cleanup` 中释放。

### 新增服务注册/发现

在 `core/registry/` 下封装，并在 `cmd/<app>/main.go` 的 `newApp` 中通过 `kratos.Registrar(...)` 注入。
