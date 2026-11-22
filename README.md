# Kratos HTTP Demo Template

一个**开箱即用**的 Kratos v3 单体 HTTP 服务示例。核心目标：

1. **清晰的目录与功能分层**；
2. **可被 `test/arch` 强制执行依赖边界**；
3. **统一的 HTTP 响应契约**。

Demo 默认运行链路是：**HTTP + 内存 Repo + 日志**。不需要 gRPC、PostgreSQL、Redis 或任何外部服务，`make run` 直接启动。Ent/数据库代码保留为可选注入点；gRPC 可按需再扩展。

---

## 快速开始

环境要求：Go 1.25+。Windows 需要安装 Git for Windows；Makefile 会自动检测其中的 Git Bash 作为 `SHELL`，因此 cmd 和 Git Bash 下都可以执行 `make`。

```bash
# 安装缺失工具（PATH 已有则复用；缺失项安装到 .tools/）
make init

# 可选：如果工具安装在 .tools/bin，加入 PATH 后可直接使用 kratos/wire/ent
export PATH="$PWD/.tools/bin:$PATH"

# 运行测试（含架构边界测试）
make test

# 直接启动 demo（无需数据库）
make run
```

启动后访问 `http://127.0.0.1:8000/v1/todos`，会看到两条预置的示例数据。

### Demo 接口

```bash
# 列表（offset 分页）
curl http://127.0.0.1:8000/v1/todos

# 创建
curl -X POST http://127.0.0.1:8000/v1/todos \
  -H 'Content-Type: application/json' \
  -d '{"title":"write demo","content":"hello"}'

# 查询
curl http://127.0.0.1:8000/v1/todos/<id>

# 部分更新（update_mask 为 query 参数）
curl -X PATCH 'http://127.0.0.1:8000/v1/todos/<id>?update_mask=title,completed' \
  -H 'Content-Type: application/json' \
  -d '{"title":"renamed","completed":true}'

# 删除（软删除）
curl -X DELETE http://127.0.0.1:8000/v1/todos/<id>
```

基于模板创建新项目：

```bash
kratos new <your-project-name> -r https://github.com/liu-jiangyuan/kratos-template.git --branch dev
```

`kratos new` 会替换模块名，并把 `cmd/server` 重命名为 `cmd/<your-project-name>`。

### 工具链说明

工具解析顺序（`make api` / `make config`）：

1. **显式变量**：`PROTOC`、`PROTOC_INCLUDE`、`PROTOC_GEN_GO`、`PROTOC_GEN_GO_HTTP`、`PROTOC_GEN_OPENAPI`（环境变量或命令行）；
2. **PATH 中已有的同名工具**；
3. 仓库下 `.tools/` 中由 `make init` 安装的兜底版本。

说明：

- **只有 `make init` 会安装/下载工具**；`make api` / `make config` 不会自动下载。工具缺失时会提示执行 `make init`。
- `make init` 只补缺失项：PATH 或 `.tools` 中已有的工具会直接复用，不会覆盖或重复安装。
- `protoc` 优先使用 PATH 中的系统版本；只有找不到时才下载官方 release 到 `.tools/protoc`。
- 使用系统 protoc 时自动推断 include 目录（`<protoc>/../include` 或 `/usr/include`）；推断不对时显式设置 `PROTOC_INCLUDE`。
- `.tools/bin` 不再强制加入 PATH，不会覆盖你已有的插件版本。
- 查看当前实际解析结果：

  ```bash
  make tools-env
  ```

- 显式指定工具：

  ```bash
  make api PROTOC=protoc PROTOC_INCLUDE=/usr/include

  # 或使用环境变量
  export PROTOC=/opt/protoc/bin/protoc
  export PROTOC_INCLUDE=/opt/protoc/include
  export PROTOC_GEN_GO=/path/to/protoc-gen-go
  export PROTOC_GEN_GO_HTTP=/path/to/protoc-gen-go-http
  export PROTOC_GEN_OPENAPI=/path/to/protoc-gen-openapi
  ```

- `google/api`、`google/protobuf/descriptor.proto` 直接引用 **Kratos 官方 module** 的 `third_party`，仓库不维护本地 `third_party` 拷贝。
- `make ent` 会使用 `go.mod` 中声明的 Go 工具链（`GOTOOLCHAIN`）；本机没有对应版本时 Go 会自动下载。

---

## 目录结构

```text
├── api/<module>/v1/            # Proto 契约与生成代码（DTO）
├── cmd/server/                 # 程序入口与 Wire 组装（kratos new 后改名为 cmd/<project>）
├── configs/                    # 运行时配置
├── core/                       # 项目级基础设施（不依赖 internal/api/cmd）
│   ├── conf/                   #   配置 proto 与生成代码
│   ├── data/                   #   可选：数据库连接、事务上下文
│   ├── log/                    #   日志接口与 console/file/database 实现
│   ├── middleware/             #   通用中间件（requestid）
│   └── server/                 #   全局 HTTP Server、Registrar、统一响应
├── docs/                       # 架构边界、模块开发、扩展规范
├── ent/                        # 可选：Ent schema 与生成代码
├── internal/<module>/          # 业务模块（仅模块目录，禁止散落 .go）
│   ├── <module>.go             #   wire.ProviderSet 出口
│   ├── server/                 #   实现 Registrar，注册本模块 HTTP 路由
│   ├── service/                #   实现 api 接口，DTO <-> DO
│   ├── biz/                    #   DO、Repo 接口、Usecase、领域错误
│   └── model/                  #   Repo 实现（默认内存，可选 Ent）
├── pkg/                        # 纯公共工具库（零业务/框架耦合）
├── hack/                       # OpenAPI 后处理（servers + envelope）
├── test/arch/                  # AST 架构边界测试
├── test/integration/           # HTTP 集成测试
├── openapi.yaml                # 可选：make openapi 生成（含 envelope/servers）
├── .tools/                     # make init 安装的本地工具（gitignore）
└── Makefile
```

模块内依赖方向：

```text
server ──► service ──► biz ◄── model ──► (memory / ent)
```

Demo 默认使用 `internal/todo/model.NewMemoryTodoRepo`；Ent/Postgres 实现 `NewTodoRepo` 仍然保留，见「启用数据库」。

详见 [`docs/`](./docs/README.md)。

---

## HTTP 响应契约

所有响应 body 使用同一结构：

```json
{ "code": 1, "message": "success", "data": {} }
```

### 成功

- 请求进入业务处理并成功返回：**HTTP 200**，`code = 1`。

### 业务失败

参数校验失败、业务逻辑错误、资源不存在、冲突等**业务层错误**：

- **HTTP 200**
- `code = 0`
- `message` 为业务错误文案

对应 Kratos 状态码：400（校验/业务错误，reason 不是 `CODEC`）、404、409、412、422。`CODEC` 原因表示请求体/查询参数无法解码，属于协议错误，保持 400。

### 协议 / 认证 / 系统错误

以下情况使用**真实 HTTP 状态码**，body 仍为 `code = 0`：

| HTTP 状态 | 场景 |
| :--- | :--- |
| 400 | 请求体/查询参数无法解码（`CODEC`） |
| 401 / 403 | 未认证 / 无权限 |
| 429 | 限流 |
| 499 | 客户端断开 |
| 500 | 未预期错误、panic、基础设施错误 |
| 501 | 未实现 |
| 502 / 503 / 504 | 上游异常 / 服务不可用 / 超时 |

路由层 `404 Not Found`、`405 Method Not Allowed` 也保持真实状态码，并返回同样的 envelope。

> 实现位置：`core/server/response.go`。HTTP 响应固定使用 JSON，不跟随 `Accept` 协商；`protojson` 无法编码 envelope。请使用普通 JSON 客户端；生成的 Kratos HTTP client 按 proto 响应解码，不适合直接消费 envelope。
>
> `make openapi` 可选生成 `openapi.yaml`：200 响应已包装为 `code/message/data` envelope，并附带统一的 `default` 错误 envelope。生成时会写入 `servers: http://127.0.0.1:8000`（与默认 `server.http.addr` 的端口一致）；服务地址/端口变化时用 `make openapi OPENAPI_SERVER_URL=http://<host>:<port>` 覆盖。

---

## 启用数据库（可选）

Demo 默认不打开数据库。需要 Ent/Postgres 时：

1. 在 `configs/config.yaml` 取消 `data` 段落注释；
2. 在 `cmd/server/wire.go` 导入并加回 `data.ProviderSet`；
3. 在 `internal/todo/todo.go` 把 `model.NewMemoryTodoRepo` 换成 `model.NewTodoRepo`；
4. 执行 `make ent` 生成/更新 Ent 代码。

### 启用 gRPC（可选）

Demo 不生成、也不启动 gRPC。需要时可在 `core/server` 增加 `NewGRPCServer`、扩展 `Registrar`、恢复 `protoc-gen-go-grpc` 生成与 `cmd/server` 启动项。

---

## 常用命令

```bash
make init       # 补齐缺失工具（PATH 优先；缺失项安装到 .tools/）
make api        # 生成 api/ proto 代码（HTTP）
make openapi    # 可选：生成 openapi.yaml（含 envelope/servers）
make config     # 生成 core/conf 配置代码
make ent        # 生成 ent 代码
make wire       # 生成 wire 依赖注入代码
make generate   # 生成 ent/wire + go mod tidy
make build      # 编译到 ./bin/
make run        # 本地启动（无需数据库）
make test       # 单元测试 + 架构测试 + HTTP 集成测试
make check      # gofmt 检查 + go vet + 架构测试
make all        # api + config + generate + check
```

---

## 硬性依赖边界

由 `test/arch/arch_test.go` 通过 AST 扫描强制校验：

| 包路径 | 允许依赖 | 严禁依赖 |
| :--- | :--- | :--- |
| `pkg/...` | 标准库、三方库 | `core`、`ent`、`internal`、`api`、`cmd` |
| `ent/...` | `pkg`、三方库、`ent` | `core`、`internal`、`api`、`cmd` |
| `core/...` | `pkg`、`ent`、三方库、`core` | `internal`、`api`、`cmd` |
| `internal/<m>/biz` | `pkg`、`core`、`api/<m>`（仅错误枚举） | `ent`、本模块 `model`/`service`/`server`、其他模块 |
| `internal/<m>/model` | 本模块 `biz`、`ent`、`core`、`pkg` | `api`、本模块 `service`/`server`、其他模块 |
| `internal/<m>/service` | 本模块 `biz`、`api/<m>`、`pkg` | `ent`、本模块 `model`/`server`、其他模块 |
| `internal/<m>/server` | 本模块 `service`、`api/<m>`、`core/server` | `ent`、本模块 `model`/`biz`、其他模块 |
| `internal/<m>` (根包) | 本模块四个子包（仅组装 `ProviderSet`） | 其他模块、`cmd` |
| `cmd/<app>` | `core`、`internal/<m>` | —（全局唯一组装层） |

规范细节：

- [`docs/README.md`](./docs/README.md)：规范索引与铁律速览。
- [`docs/01-architecture-and-boundaries.md`](./docs/01-architecture-and-boundaries.md)：目录职责与依赖矩阵。
- [`docs/02-module-development-guide.md`](./docs/02-module-development-guide.md)：新增业务模块步骤。
- [`docs/03-core-and-extension-guide.md`](./docs/03-core-and-extension-guide.md)：`core` / `pkg` 准入与扩展方式。
