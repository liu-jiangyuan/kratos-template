# 02. 业务模块复制与新增开发指南

本模板中的 `internal/todo`（配合 `api/todo/v1` 和 `ent/schema/todo.go`）是标准的模块参考蓝本。AI 或开发者在新增业务模块（例如 `user` 模块）时，请严格按以下 6 个步骤复制与修改。

---

## 步骤 1：定义 API Proto 契约 (`api/<module>/v1/`)

创建 `api/user/v1/error_reason.proto` 和 `api/user/v1/user.proto`：

- `go_package` 使用相对路径格式：`option go_package = "api/user/v1;v1";`
- 定义资源错误原因枚举 `ErrorReason`（如 `USER_UNSPECIFIED`、`USER_NOT_FOUND`、`USER_INVALID_ARGUMENT`）。
- 定义 RPC 服务 `UserService` 并添加 `google.api.http` 注解。
- 生成代码：
  ```bash
  make api
  ```

---

## 步骤 2：定义数据库 Schema (`ent/schema/`)

在 `ent/schema/user.go` 中新建 Schema：

- 组合通用 Mixin：`IDMixin{}`（UUIDv7 主键）与 `TimeMixin{}`（`created_at` / `updated_at`）。
- **边界提醒**：`ent/schema` **严禁** import `internal/...`。状态枚举等字段在 schema 中使用 `field.Int32("status").Default(1)` 等基础类型，在 `model` 层再转为 `biz` 的强类型枚举。
- 生成 Ent 代码：
  ```bash
  make ent
  ```

---

## 步骤 3：创建模块目录与四层代码 (`internal/<module>/`)

复制 `internal/todo` 为 `internal/user`，包含以下固定结构：

```text
internal/user/
├── user.go             # 导出 wire.ProviderSet
├── biz/
│   └── user.go         # DO 实体、UserRepo 接口、领域错误、UserUsecase
├── model/
│   ├── user.go         # 实现 biz.UserRepo（注入 *core/data.Data，使用 r.data.DB(ctx) 访问 ent）
│   └── user_test.go    # 使用内存 SQLite 测试真实 Ent 查询
├── service/
│   ├── user.go         # 实现 v1.UserServiceServer，DTO <-> DO 转换
│   └── user_test.go    # 使用 fakeUserRepo 测试 service 层逻辑
└── server/
    └── server.go       # 实现 core/server.Registrar 接口
```

### 各层关键代码模板

#### 1) `internal/user/biz/user.go`
- 声明领域错误：`ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")`
- 声明 DO 结构体 `type User struct { ... }`
- 声明仓储接口 `type UserRepo interface { ... }`
- 实现 `type UserUsecase struct { repo UserRepo }` 与 `NewUserUsecase(repo UserRepo) *UserUsecase`

#### 2) `internal/user/model/user.go`
- 构造函数返回 `biz.UserRepo` 接口而非具体结构体：
  ```go
  func NewUserRepo(d *data.Data) biz.UserRepo {
      return &userRepo{data: d}
  }
  ```
- 所有查询通过 `r.data.DB(ctx).User...` 执行（这样当外层调用 `data.InTx` 时会自动走事务连接）。
- 将 `ent.IsNotFound(err)` 映射为 `biz.ErrUserNotFound`，不向上层暴露 `ent` 错误类型。

#### 3) `internal/user/service/user.go`
- 内嵌 `v1.UnimplementedUserServiceServer`，注入 `*biz.UserUsecase`。
- 负责入参校验与 DTO ↔ DO 转换。

#### 4) `internal/user/server/server.go`
- 实现 `core/server.Registrar` 接口，只注册路由，不创建 Server：
  ```go
  type Registrar struct {
      svc *service.UserService
  }
  var _ coreserver.Registrar = (*Registrar)(nil)

  func NewRegistrar(svc *service.UserService) *Registrar {
      return &Registrar{svc: svc}
  }
  func (r *Registrar) RegisterHTTP(srv *http.Server) {
      v1.RegisterUserServiceHTTPServer(srv, r.svc)
  }
  func (r *Registrar) RegisterGRPC(srv *grpc.Server) {
      v1.RegisterUserServiceServer(srv, r.svc)
  }
  ```

#### 5) `internal/user/user.go`
- 汇总本模块的四层构造函数：
  ```go
  var ProviderSet = wire.NewSet(
      model.NewUserRepo,
      biz.NewUserUsecase,
      service.NewUserService,
      server.NewRegistrar,
  )
  ```

---

## 步骤 4：在入口注册模块 (`cmd/<app>/`)

1. 在 `cmd/<app>/main.go` 的 `newRegistrars` 中加入新模块的 `*userserver.Registrar`：
   ```go
   func newRegistrars(
       todoReg *todoserver.Registrar,
       userReg *userserver.Registrar,
   ) []coreserver.Registrar {
       return []coreserver.Registrar{
           todoReg,
           userReg,
       }
   }
   ```
2. 在 `cmd/<app>/wire.go` 的 `wireApp` 中加入 `user.ProviderSet`：
   ```go
   panic(wire.Build(
       coreserver.ProviderSet,
       data.ProviderSet,
       todo.ProviderSet,
       user.ProviderSet,
       newRegistrars,
       newApp,
   ))
   ```
3. 重新生成 Wire 代码：
   ```bash
   make wire
   ```

---

## 步骤 5：校验架构边界与运行测试

```bash
make check
make test
```
