# 02. 业务模块开发规范

新增或修改业务模块（以 `<module>` 为例，参考 `internal/todo`）必须遵循以下结构与步骤。

---

## 1. API 契约约束 (`api/<module>/v1/`)

- 每个模块在 `api/<module>/v1/` 下定义 `<module>.proto` 与 `error_reason.proto`。
- `go_package` 统一使用相对路径：`option go_package = "api/<module>/v1;v1";`。
- 修改后执行 `make api` 生成代码，严禁手改 `*.pb.go`。

---

## 2. 数据库 Schema 约束 (`ent/schema/`)

- 在 `ent/schema/<module>.go` 中定义表结构，复用 `IDMixin{}`（UUIDv7 主键）与 `TimeMixin{}`（`created_at` / `updated_at`）。
- **禁止反向依赖**：`ent/schema` **严禁** import `internal/...`。状态枚举等字段在 schema 中使用 `field.Int32` 等基础类型，在 `model` 层再转为 `biz` 的领域类型。
- 修改后执行 `make ent` 生成代码，严禁手改 `ent/` 下除 `schema/` 和 `generate.go` 以外的生成文件。

---

## 3. 模块内部四层约束 (`internal/<module>/`)

每个模块固定由以下 4 个子包和 1 个根包组成，**严禁**在模块下创建其他子目录：

```text
internal/<module>/
├── <module>.go         # 唯一出口：导出 wire.ProviderSet
├── biz/                # 领域对象 (DO)、Repo 接口、领域错误、Usecase
├── model/              # Repo 实现（调用 core/data 与 ent，DO <-> PO 转换）
├── service/            # 实现 api 接口（参数校验，DTO <-> DO 转换）
└── server/             # 实现 core/server.Registrar（注册 HTTP/gRPC 路由）
```

### 各层必须遵守的代码契约

1. **`biz`**
   - 领域错误使用 `api/<module>/v1` 的 `ErrorReason` 定义（如 `errors.NotFound(v1.ErrorReason_XXX.String(), "...")`）。
   - Repo 接口在此声明，入参与返回值只能是 DO 或基本类型。
2. **`model`**
   - 构造函数签名必须返回 `biz` 的 Repo 接口：`func New<Resource>Repo(d *data.Data) biz.<Resource>Repo`。
   - 所有数据库读写必须通过 `r.data.DB(ctx)` 获取 `*ent.Client`（确保 `core/data.InTx` 事务上下文生效）。
   - 必须将 `ent.IsNotFound(err)` 等底层存储错误转换为 `biz` 领域错误，严禁向上层暴露 `ent` 错误。
3. **`service`**
   - 内嵌 `v1.Unimplemented<Resource>ServiceServer`，仅依赖 `*biz.<Resource>Usecase`。
   - 完成 DTO ↔ DO 转换，不写业务逻辑，不直接访问存储。
4. **`server`**
   - 实现 `core/server.Registrar` 接口（`RegisterHTTP` / `RegisterGRPC`）。
   - 只将本模块的 `service` 注册到全局 Server，严禁在此新建端口或 Server 实例。
5. **`<module>.go`**
   - 仅汇总导出本模块的 `ProviderSet`：
     ```go
     var ProviderSet = wire.NewSet(
         model.New<Resource>Repo,
         biz.New<Resource>Usecase,
         service.New<Resource>Service,
         server.NewRegistrar,
     )
     ```

---

## 4. 全局组装与校验 (`cmd/<app>/`)

1. 在 `cmd/<app>/main.go` 的 `newRegistrars` 参数与返回切片中加入新模块的 `*<module>server.Registrar`。
2. 在 `cmd/<app>/wire.go` 的 `wireApp` 中加入 `<module>.ProviderSet`。
3. 执行生成与架构校验：
   ```bash
   make wire
   make check
   make test
   ```
