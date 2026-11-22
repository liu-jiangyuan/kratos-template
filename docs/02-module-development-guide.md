# 02. 业务模块开发规范

新增或修改业务模块（以 `<module>` 为例，参考 `internal/todo`）按以下步骤进行。

---

## 1. API 契约 (`api/<module>/v1/`)

- 每个模块在 `api/<module>/v1/` 下定义 `<module>.proto` 与 `error_reason.proto`。
- 修改后执行 `make api`，禁止手改 `*.pb.go`。
- `make api` 的 `google/api` 依赖直接来自 Kratos 官方 module 的 `third_party`，仓库不维护本地 `third_party`。
- 需要 OpenAPI 时执行 `make openapi`；生成的 200 响应会包装为 `code/message/data` envelope，并写入 `servers`。
- HTTP 路由避免使用会被 `/{id}` 捕获的固定段；自定义方法建议使用 `:action` 形式（如 `/v1/<resource>:action`）。

---

## 2. 数据库 Schema (`ent/schema/`，可选)

Demo 默认使用内存 Repo，不涉及 Ent。需要数据库时：

- 在 `ent/schema/<module>.go` 中定义表结构，复用 `IDMixin{}`（UUIDv7）与 `TimeMixin{}`。
- **禁止反向依赖**：`ent/schema` 不得 import `internal/...`。状态枚举等字段使用 `field.Int32` 等基础类型，在 `model` 层转为 `biz` 类型。
- 修改后执行 `make ent`；除 `schema/` 与 `generate.go` 外不要手改生成文件。

---

## 3. 模块四层结构

```text
internal/<module>/
├── <module>.go         # 唯一出口：导出 wire.ProviderSet
├── biz/                # DO、Repo 接口、领域错误、Usecase
├── model/              # Repo 实现（Demo 默认 memory，可选 ent + core/data）
├── service/            # 实现 api 接口（DTO <-> DO）
└── server/             # 实现 core/server.Registrar（HTTP）
```

### 各层契约

1. **`biz`**
   - 领域错误使用 `api/<module>/v1` 的 `ErrorReason`，例如：
     ```go
     ErrNotFound = errors.NotFound(v1.ErrorReason_XXX_NOT_FOUND.String(), "not found")
     ```
   - 基础设施错误统一返回 `ErrInternal`（参考 `biz.ErrTodoInternal`），不要把 ent/SQL 错误暴露给上层。
   - Repo 接口的入参和返回值只使用 DO 或基础类型。

2. **`model`**
   - Demo 默认使用内存 Repo：`func NewMemory<Resource>Repo() biz.<Resource>Repo`，不需要数据库。
   - 启用 Ent 时，构造函数返回 `biz` 的 Repo 接口：`func New<Resource>Repo(d *data.Data) biz.<Resource>Repo`；所有数据库读写通过 `r.data.DB(ctx)`。
   - 非 `ent.IsNotFound` 的存储错误必须转换为 `biz.ErrInternal`，原始错误只写日志。

3. **`service`**
   - 只依赖 `*biz.<Resource>Usecase`，负责 DTO ↔ DO 转换、参数解析、字段校验。
   - 参数解析错误包装为业务 `InvalidArgument`，不要直接把库错误返回给传输层。
   - 业务错误（校验、逻辑、NotFound、Conflict）会映射为 HTTP 200 + `code=0`；协议/系统错误保留真实状态码。

4. **`server`**
   - 实现 `core/server.Registrar` 的 `RegisterHTTP`。
   - 只注册本模块 service，不新建端口或 Server 实例。

5. **`<module>.go`**

   Demo 默认使用内存 Repo：

   ```go
   var ProviderSet = wire.NewSet(
       model.NewMemory<Resource>Repo,
       biz.New<Resource>Usecase,
       service.New<Resource>Service,
       server.NewRegistrar,
   )
   ```

   接入 Ent/Postgres 时：

   1. 把 `model.NewMemory<Resource>Repo` 换成 `model.New<Resource>Repo`；
   2. 在 `cmd/<app>/wire.go` 导入并加入 `core/data.ProviderSet`；
   3. 在 `configs/config.yaml` 打开 `data` 配置；
   4. 执行 `make ent`。

---

## 4. 全局组装与验证

1. 在 `cmd/<app>/main.go` 的 `newRegistrars` 中加入 `<module>server.NewRegistrar` 的产物。
2. 在 `cmd/<app>/wire.go` 的 `wireApp` 中加入 `<module>.ProviderSet`。
3. 执行：

   ```bash
   make wire
   make api
   make check
   make test
   ```
