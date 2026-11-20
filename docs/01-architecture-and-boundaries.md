# 01. 架构设计与硬性依赖边界规范

本项目基于 **Kratos v3**，采用 **纵向业务模块化 (`internal/<module>`) + 根目录 `ent` + 核心基础设施库 (`core`) + 纯公共工具库 (`pkg`)** 的分层架构。所有边界规则均由 `test/arch/arch_test.go` 通过 AST 静态扫描自动强制执行。

---

## 1. 顶层目录职责

```text
├── api/                        # Proto 契约与生成代码（DTO，对外可引用）
│   └── <module>/v1/
├── cmd/<app>/                  # 程序入口与全局 Wire 组装（kratos new 自动将 cmd/server 改名为 cmd/<app>）
│   ├── main.go
│   ├── wire.go
│   └── wire_gen.go
├── configs/                    # 运行时配置文件 (config.yaml)
├── core/                       # 核心基础设施库（框架级组装，与具体业务无关）
│   ├── conf/                   #   配置 proto 定义与生成代码
│   ├── data/                   #   数据库连接 (ent.Client)、自动迁移、事务封装 InTx
│   └── server/                 #   创建全局唯一的 HTTP / gRPC Server，定义 Registrar 接口
├── docs/                       # 架构规范与 AI 辅助开发指南（Markdown 文档目录）
├── ent/                        # Ent 根目录：统一存放所有表的 schema 与生成的客户端代码
│   ├── schema/
│   └── generate.go
├── internal/                   # 纯粹的业务模块目录（除模块目录外不放任何其他内容）
│   └── <module>/               #   单个业务模块（高内聚，删除目录即移除模块）
│       ├── <module>.go         #   导出本模块的 wire.ProviderSet
│       ├── server/             #   向 core/server 注册本模块的 HTTP/gRPC 路由
│       ├── service/            #   实现 api 接口，参数校验，DTO <-> DO 转换
│       ├── biz/                #   领域实体 (DO)、Repo 接口、领域错误、Usecase 业务逻辑
│       └── model/              #   Repo 实现（调用 core/data 与 ent，DO <-> PO 转换）
├── pkg/                        # 纯公共工具库（零业务、零框架耦合，可原样复制到其他项目）
└── test/arch/                  # 架构依赖边界自动化校验测试
```

---

## 2. 硬性 Import 依赖矩阵

`test/arch/arch_test.go` 会在每次 `go test ./...` 或 `make check` 时校验以下规则：

| 源包 (`Source Package`) | 允许 Import 的本仓包 | 严禁 Import 的本仓包 |
| :--- | :--- | :--- |
| `pkg/...` | 仅标准库、三方库、`pkg/...` | `core`、`ent`、`internal`、`api`、`cmd` |
| `ent/...` | `pkg/...`、`ent/...` | `core`、`internal`、`api`、`cmd` |
| `core/...` | `pkg/...`、`ent/...`、`core/...` | `internal`、`api`、`cmd` |
| `internal/<m>/biz` | `pkg/...`、`core/...`、`api/<m>/...`（仅错误原因枚举） | `ent`、本模块 `model`/`service`/`server`、其他模块 `internal/<other>` |
| `internal/<m>/model` | `internal/<m>/biz`、`ent/...`、`core/...`、`pkg/...` | `api`、本模块 `service`/`server`、其他模块 `internal/<other>` |
| `internal/<m>/service` | `internal/<m>/biz`、`api/<m>/...`、`pkg/...` | `ent`、本模块 `model`/`server`、其他模块 `internal/<other>` |
| `internal/<m>/server` | `internal/<m>/service`、`api/<m>/...`、`core/server` | `ent`、本模块 `model`/`biz`、其他模块 `internal/<other>` |
| `internal/<m>` (根包) | 本模块的 `biz`、`model`、`service`、`server`（仅用于组装 `ProviderSet`） | 其他模块 `internal/<other>`、`cmd` |
| `cmd/<app>` | `core/...`、`internal/<m>`、`internal/<m>/server` | —（全局唯一的组装层） |

---

## 3. 三种数据模型与分层边界

```text
Client ──► DTO (api) ──► service ──► DO (biz) ──► biz ──► DO (biz) ──► model ──► PO (ent) ──► Database
                                                   ▲                     │
                                                   └─── Repo Interface ──┘
```

1. **DTO (`api/<module>/v1`)**：Protobuf 生成的 Request / Response 结构体，只出现在 `service` 层。
2. **DO (`internal/<module>/biz`)**：纯净的领域对象（Go struct），不带 protobuf 标签，不带 `ent` 依赖。
   - **重要约定**：`ent/schema` 中的字段必须使用基础类型（如 `field.Int32("status")`），**严禁**在 `ent/schema` 中通过 `.GoType(biz.SomeType)` 反向 import `internal/<module>/biz`，否则会破坏 `ent` 不依赖 `internal` 的边界；类型转换统一在 `internal/<module>/model` 的 `toBiz()` 中完成。
3. **PO (`ent`)**：由 `ent` 生成的持久化实体（如 `*ent.Todo`），只允许出现在 `internal/<module>/model` 层，绝不向上泄漏到 `biz` 或 `service`。
