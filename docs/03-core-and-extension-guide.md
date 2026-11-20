# 03. 核心库 (`core`) 与公共库 (`pkg`) 扩展指南

本模板只内置最小基础设施集。下游项目在基于本模板开发时，应将与具体业务无关的基础设施扩展放入 `core/`，将零耦合的纯工具放入 `pkg/`。

---

## 1. `core` 与 `pkg` 的准入判定

在新增一个公共包前，用以下标准决定放哪里：

- **放入 `pkg/<name>`**：
  - 不需要读取项目的 `core/conf` 配置；
  - 不依赖 `ent`；
  - 不涉及 Kratos 的 Server / Transport 生命周期；
  - 可以直接复制到任何其他 Go 项目中独立编译使用（例如：加密哈希、字符串/切片辅助函数、通用时间处理、与业务无关的算法）。
- **放入 `core/<name>`**：
  - 需要依赖 `core/conf` 中的配置项进行初始化；
  - 或者属于 Kratos HTTP/gRPC 中间件、客户端连接池（数据库、Redis、消息队列、注册中心、OSS 等）；
  - 需要通过 Wire 注入到 `core/server` 或各业务模块的 `biz` / `model` 中。

> **红线**：无论 `core` 还是 `pkg`，都**绝对不能** import `internal/...` 或 `api/...`。

---

## 2. 数据库事务 (`core/data.InTx`) 用法

`core/data/data.go` 已经内置了基于 `context.Context` 的事务传播机制：

- 在 `model` 层中，始终使用 `r.data.DB(ctx)` 获取 `*ent.Client`。
- 如果某个业务用例需要在同一个事务中跨多个表（或跨多个 Repo 方法）写数据：
  1. 在 `core/data`（或 `biz` 层定义的 `Transaction` 接口，由 `core/data.Data` 实现，因为 `Data` 已经实现了 `InTx(ctx context.Context, fn func(ctx context.Context) error) error`）中注入事务执行器；
  2. 在 `biz` 层定义：
     ```go
     type Transaction interface {
         InTx(ctx context.Context, fn func(ctx context.Context) error) error
     }
     ```
  3. 在 `core/data/data.go` 的 `ProviderSet` 中绑定 `wire.Bind(new(Transaction), new(*Data))`（或在 `core/data` 中导出通用的 `Transaction` 接口供 `biz` 使用，因为 `biz` 允许 import `core`）。

---

## 3. 在下游项目中扩展中间件（JWT / 限流 / 链路追踪等）

1. 在 `core/middleware/<name>/` 下编写中间件构造函数。
2. 如果中间件需要配置，在 `core/conf/conf.proto` 中新增配置字段并执行 `make config`。
3. 在 `core/server/http.go` 和 `core/server/grpc.go` 的 `http.Middleware(...)` / `grpc.Middleware(...)` 列表中挂载该中间件。

---

## 4. 在下游项目中扩展三方客户端（如 Redis / 注册中心）

1. **Redis 等存储客户端**：
   - `core/conf/conf.proto` 中已预留 `Data.Redis` 配置结构；
   - 在 `core/data/data.go` 的 `Data` 结构体中增加 `rdb *redis.Client` 字段，在 `NewData` 中初始化并在 `cleanup` 中关闭。
2. **服务注册发现 (etcd / consul / nacos)**：
   - 在 `core/conf/conf.proto` 中新增 `Registry` 消息；
   - 在 `core/registry/` 下封装 `kratos/ registry.Registrar` 的构建函数并加入 `core` 的 `ProviderSet`；
   - 在 `cmd/<app>/main.go` 的 `newApp` 中传入 `kratos.Registrar(r)`。
