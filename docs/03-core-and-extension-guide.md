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
  - 或属于 HTTP/gRPC 中间件、客户端连接池（数据库、缓存、消息队列、注册中心等）；
  - 需要通过 Wire 注入到 `core/server` 或各业务模块的 `biz` / `model` 中。

> **红线**：`core` 与 `pkg` 均**严禁** import `internal/...`、`api/...`、`cmd/...`。

---

## 2. 数据库事务 (`core/data.InTx`) 约束

`core/data/data.go` 提供基于 `context.Context` 的事务传播：

- `model` 层必须始终通过 `r.data.DB(ctx)` 获取 `*ent.Client`，严禁直接持有或绕过 `DB(ctx)` 调用裸连接。
- 跨表/跨 Repo 的事务通过 `d.InTx(ctx, func(ctx context.Context) error { ... })` 执行；若需在 `biz` 层编排事务，可在 `biz`（或 `core/data`）定义 `Transaction` 接口并通过 Wire 绑定 `*data.Data`。

---

## 3. 基础设施扩展约定

1. **新增配置项**：在 `core/conf/conf.proto` 中定义，执行 `make config` 生成 `conf.pb.go`。
2. **新增全局中间件**：在 `core/middleware/<name>/` 下实现，并在 `core/server/http.go` 或 `core/server/grpc.go` 中挂载。
3. **新增共享存储客户端（如 Redis）**：在 `core/data/data.go` 的 `Data` 中扩展字段，在 `NewData` 中初始化并在 `cleanup` 中释放。
4. **新增服务注册/发现**：在 `core/registry/` 下封装并在 `cmd/<app>/main.go` 的 `newApp` 中通过 `kratos.Registrar(...)` 注入。
