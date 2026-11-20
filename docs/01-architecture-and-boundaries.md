# 01. 架构分层与硬性依赖边界

所有边界规则均由 `test/arch/arch_test.go` 通过 AST 静态扫描强制执行。

---

## 1. 目录职责约束

```text
├── api/<module>/v1/            # Proto 契约与生成代码（DTO）
├── cmd/<app>/                  # 程序入口与全局 Wire 组装
├── configs/                    # 运行时配置文件
├── core/                       # 核心基础设施库（与具体业务无关）
│   ├── conf/                   #   配置 proto 定义与生成代码
│   ├── data/                   #   数据库连接 (ent.Client)、事务封装 InTx
│   ├── log/                    #   日志接口、自定义格式化器及 3 种输出实现（终端/文件/HTTP 日志数据库）
│   ├── middleware/             #   通用中间件（如 requestid）
│   └── server/                 #   全局唯一的 HTTP / gRPC Server，定义 Registrar 接口
├── ent/                        # Ent 根目录：统一存放所有表的 schema 与生成代码
│   └── schema/
├── internal/<module>/          # 业务模块目录（除模块子目录外严禁放其他文件）
│   ├── <module>.go             #   导出本模块的 wire.ProviderSet
│   ├── server/                 #   实现 core/server.Registrar 注册本模块路由
│   ├── service/                #   实现 api 接口，参数校验，DTO <-> DO 转换
│   ├── biz/                    #   领域实体 (DO)、Repo 接口、领域错误、Usecase
│   └── model/                  #   Repo 实现（调用 core/data 与 ent，DO <-> PO 转换）
├── pkg/                        # 纯公共工具库（零业务、零框架耦合）
└── test/arch/                  # 架构依赖边界自动化校验测试
```

---

## 2. 硬性 Import 依赖矩阵

| 源包 (`Source Package`) | 允许 Import 的本仓包 | 严禁 Import 的本仓包 |
| :--- | :--- | :--- |
| `pkg/...` | 标准库、三方库、`pkg/...` | `core`、`ent`、`internal`、`api`、`cmd` |
| `ent/...` | `pkg/...`、`ent/...` | `core`、`internal`、`api`、`cmd` |
| `core/...` | `pkg/...`、`ent/...`、`core/...` | `internal`、`api`、`cmd` |
| `internal/<m>/biz` | `pkg/...`、`core/...`、`api/<m>/...`（仅错误原因枚举） | `ent`、本模块 `model`/`service`/`server`、其他模块 `internal/<other>` |
| `internal/<m>/model` | `internal/<m>/biz`、`ent/...`、`core/...`、`pkg/...` | `api`、本模块 `service`/`server`、其他模块 `internal/<other>` |
| `internal/<m>/service` | `internal/<m>/biz`、`api/<m>/...`、`pkg/...` | `ent`、本模块 `model`/`server`、其他模块 `internal/<other>` |
| `internal/<m>/server` | `internal/<m>/service`、`api/<m>/...`、`core/server` | `ent`、本模块 `model`/`biz`、其他模块 `internal/<other>` |
| `internal/<m>` (根包) | 本模块的 `biz`、`model`、`service`、`server`（仅用于组装 `ProviderSet`） | 其他模块 `internal/<other>`、`cmd` |
| `cmd/<app>` | `core/...`、`internal/<m>`、`internal/<m>/server` | —（全局唯一的组装层） |

---

## 3. 数据模型流转边界

```text
Client ──► DTO (api) ──► service ──► DO (biz) ──► biz ──► DO (biz) ──► model ──► PO (ent) ──► Database
                                                   ▲                     │
                                                   └─── Repo Interface ──┘
```

1. **DTO (`api/<module>/v1`)**：Protobuf 生成的 Request / Response，仅在 `service` 层使用，严禁传入 `biz` 或 `model`。
2. **DO (`internal/<module>/biz`)**：纯净的领域对象（Go struct），不带 protobuf 或 ent 依赖。
   - **Schema 边界**：`ent/schema` 中的字段必须使用基础类型（如 `field.Int32("status")`），**严禁**在 `ent/schema` 中通过 `.GoType(biz.SomeType)` 反向 import `internal/<module>/biz`；类型转换统一在 `internal/<module>/model` 的 `toBiz()` 中完成。
3. **PO (`ent`)**：`ent` 生成的持久化实体，仅允许在 `internal/<module>/model` 层内部使用，严禁泄漏到 `biz` 或 `service`。
