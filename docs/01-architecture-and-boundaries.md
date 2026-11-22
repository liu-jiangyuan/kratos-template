# 01. 架构分层与硬性依赖边界

所有边界规则由 `test/arch/arch_test.go` 通过 AST 静态扫描强制执行。

---

## 1. 目录职责

```text
├── api/<module>/v1/            # Proto 契约与生成代码（DTO）
├── cmd/<app>/                  # 程序入口与全局 Wire 组装
├── configs/                    # 运行时配置
├── core/                       # 核心基础设施（与具体业务无关）
│   ├── conf/                   #   配置 proto 定义与生成代码
│   ├── data/                   #   可选：数据库连接 (ent.Client)、事务封装 InTx
│   ├── log/                    #   日志接口与 console / file / database 实现
│   ├── middleware/             #   通用中间件（requestid）
│   └── server/                 #   全局 HTTP Server、Registrar、统一响应
├── ent/                        # 可选：Ent schema 与生成代码
├── internal/<module>/          # 业务模块
│   ├── <module>.go             #   本模块 wire.ProviderSet 出口
│   ├── server/                 #   实现 core/server.Registrar（HTTP）
│   ├── service/                #   实现 api 接口，DTO <-> DO
│   ├── biz/                    #   领域对象 (DO)、Repo 接口、领域错误、Usecase
│   └── model/                  #   Repo 实现（默认 memory，可选 Ent）
├── pkg/                        # 纯公共工具库
├── hack/                       # 生成后处理（OpenAPI servers + envelope）
├── test/arch/                  # 架构边界自动化校验
├── test/integration/           # HTTP 集成测试
└── openapi.yaml                # 可选：make openapi 生成
```

Demo 默认使用 `internal/todo/model.NewMemoryTodoRepo`，启动时不需要数据库。

---

## 2. 硬性 Import 依赖矩阵

| 源包 | 允许 Import | 严禁 Import |
| :--- | :--- | :--- |
| `pkg/...` | 标准库、三方库、`pkg/...` | `core`、`ent`、`internal`、`api`、`cmd` |
| `ent/...` | `pkg/...`、`ent/...` | `core`、`internal`、`api`、`cmd` |
| `core/...` | `pkg/...`、`ent/...`、`core/...` | `internal`、`api`、`cmd` |
| `internal/<m>/biz` | `pkg/...`、`core/...`、`api/<m>/...`（仅错误原因枚举） | `ent`、本模块 `model`/`service`/`server`、其他模块 |
| `internal/<m>/model` | 本模块 `biz`、`ent/...`、`core/...`、`pkg/...` | `api`、本模块 `service`/`server`、其他模块 |
| `internal/<m>/service` | 本模块 `biz`、`api/<m>/...`、`pkg/...` | `ent`、本模块 `model`/`server`、其他模块 |
| `internal/<m>/server` | 本模块 `service`、`api/<m>/...`、`core/server` | `ent`、本模块 `model`/`biz`、其他模块 |
| `internal/<m>` (根包) | 本模块四个子包（仅组装 `ProviderSet`） | 其他模块、`cmd` |
| `cmd/<app>` | `core/...`、`internal/<m>` | —（全局唯一组装层） |

模块内依赖流：

```text
server ──► service ──► biz ◄── model ──► (memory / ent)
```

---

## 3. 数据模型流转

```text
Client ──► DTO (api) ──► service ──► DO (biz) ──► model ──┬──► memory (demo)
                                  ▲                       └──► PO (ent) ──► Database（可选）
                                  └── Repo Interface ──┘
```

1. **DTO (`api/<module>/v1`)**：Protobuf 生成的 Request/Response，仅存在于 `service` 层。
2. **DO (`internal/<module>/biz`)**：纯 Go 领域对象，不依赖 protobuf 或 ent。
3. **PO (`ent`)**：可选。使用 Ent 时，PO 只允许出现在 `model` 层。

`ent/schema` 禁止反向 import `internal/<module>/biz`；状态枚举等字段使用基础类型（如 `field.Int32`），转换在 `model` 的 `toBiz()` 中完成。

---

## 4. 路由与响应

- 每个模块的 `server` 实现 `core/server.Registrar`，把 HTTP 路由注册到全局 Server。
- 全局响应编码器位于 `core/server/response.go`：
  - 成功：HTTP 200 + `{code:1, message:"success", data:...}`；
  - 业务失败：HTTP 200 + `{code:0, message:"..."}`；
  - 协议/认证/系统错误：真实 HTTP 状态码 + `{code:0, message:"..."}`；
  - 路由级 404/405 也返回统一 body。

详细映射见 [`README.md`](../README.md#http-响应契约)。
