# Kratos Custom Project Template

基于 **Kratos v3** 的自定义微服务项目模板，采用 **纵向模块化 (`internal/<module>`) + 基础设施核心库 (`core`) + 公共纯工具库 (`pkg`) + 根目录 `ent`** 的结构，并通过自动化架构测试 (`test/arch`) 强制守护依赖边界。

模板仅内置基础能力（HTTP/gRPC 服务启动、配置加载、Kratos 日志、PostgreSQL + Ent 数据库基础、完整 CRUD 示例模块 `todo`、protoc + Makefile 工具链），不预置任何三方可选组件（注册中心、Redis、链路追踪、鉴权、限流等），由下游项目按需扩展。

---

## 使用模板创建新项目

```bash
kratos new <your-project-name> -r https://github.com/liu-jiangyuan/kratos-template.git --branch dev
```

`kratos new` 会自动：
1. 拉取本仓库并把所有文件中的模块名 `github.com/liu-jiangyuan/kratos-template` 替换为 `<your-project-name>`；
2. 将 `cmd/server` 重命名为 `cmd/<your-project-name>`。

进入新项目即可直接编译、测试或启动服务：

```bash
cd <your-project-name>
make test
make run
```

---

## 目录结构与职责边界

```text
├── AGENTS.md / CLAUDE.md       # AI 助手入口规范（引导 AI 扫描 docs/ 下的架构与模块复制指南）
├── api/                        # Proto 契约与生成代码（对外可引用）
│   └── todo/v1/
├── cmd/server/                 # 程序入口与 Wire 组装（kratos new 自动改名为 cmd/<项目名>）
│   ├── main.go
│   ├── wire.go                 # 汇总 core 与各业务模块的 ProviderSet
│   └── wire_gen.go
├── configs/
│   └── config.yaml             # 本地运行配置（默认 PostgreSQL）
├── core/                       # 核心库：项目级基础设施（可依赖 pkg、ent，严禁依赖 internal、api）
│   ├── conf/                   #   conf.proto 配置结构与生成代码
│   ├── server/                 #   创建全局唯一的 HTTP / gRPC Server，定义模块路由注册接口 Registrar
│   └── data/                   #   数据库连接（ent.Client 构建、自动迁移、关闭、事务封装 InTx）
├── docs/                       # Markdown 文档文件夹（架构规范、模块复制开发指南、扩展指南，供开发者与 AI 扫描）
│   ├── 01-architecture-and-boundaries.md
│   ├── 02-module-development-guide.md
│   └── 03-core-and-extension-guide.md
├── ent/                        # Ent 根目录：统一管理所有 Schema 与生成的客户端代码
│   ├── schema/
│   └── generate.go
├── internal/                   # 纯粹的业务模块目录（除模块子目录外不放任何杂项文件）
│   └── todo/                   #   示例模块（删除整个目录即删除该模块）
│       ├── todo.go             #   模块唯一的对外 Wire ProviderSet 出口
│       ├── server/             #   实现 core/server.Registrar，注册本模块的 HTTP/gRPC 路由
│       ├── service/            #   实现 api 接口，负责参数校验与 DTO <-> DO 转换
│       ├── biz/                #   领域实体 (DO)、Repo 接口定义、UseCase 业务逻辑
│       └── model/              #   Repo 实现（调用 ent 与 core/data，负责 DO <-> PO 转换）
├── pkg/                        # 公共库：纯工具，零业务/框架耦合（仅可依赖标准库与三方库）
├── test/arch/                  # 架构与依赖边界自动化校验测试
│   └── arch_test.go
├── Makefile                    # protoc / ent / wire / build / run / test / check 工具链
└── go.mod
```

---

## AI 辅助开发与文档 (`docs/`)

根目录提供了 `AGENTS.md` / `CLAUDE.md`，并将详细规范统一存放在 `docs/` 文件夹下，方便后续项目用 AI 复制模块或扩展开发时直接扫描：

- [`docs/01-architecture-and-boundaries.md`](./docs/01-architecture-and-boundaries.md)：目录分层职责与硬性依赖边界规则。
- [`docs/02-module-development-guide.md`](./docs/02-module-development-guide.md)：如何以 `internal/todo` 为蓝本复制/新增一个业务模块。
- [`docs/03-core-and-extension-guide.md`](./docs/03-core-and-extension-guide.md)：如何在下游项目中按需扩展 `core`（中间件、事务 `InTx`、Redis/注册中心接入约定）与 `pkg`。

---

## 硬性依赖边界规则

由 `test/arch/arch_test.go` 在 `go test ./...` 或 `make check` 时自动扫描全仓 AST 进行校验，违反即报错：

| 包路径 | 允许依赖 | 严禁依赖 |
| :--- | :--- | :--- |
| `pkg/...` | 标准库、三方库 | `core`、`ent`、`internal`、`api`、`cmd` |
| `ent/...` | `pkg`、三方库 | `core`、`internal`、`api`、`cmd` |
| `core/...` | `pkg`、`ent`、三方库 | `internal`、`api`、`cmd` |
| `internal/<m>/biz` | `pkg`、`core`、`api`（仅错误原因枚举） | `ent`、本模块 `model`/`service`/`server`、其他业务模块 |
| `internal/<m>/model` | 本模块 `biz`、`ent`、`core`、`pkg` | `api`、本模块 `service`/`server`、其他业务模块 |
| `internal/<m>/service` | 本模块 `biz`、`api`、`pkg` | `ent`、本模块 `model`/`server`、其他业务模块 |
| `internal/<m>/server` | 本模块 `service`、`api`、`core` | `ent`、本模块 `model`/`biz`、其他业务模块 |
| `internal/<a>/...` | 本模块内部子包（按上述方向） | **任何** `internal/<b>/...`（模块间严禁直接 import） |

模块内单向依赖流：

```text
server ──► service ──► biz ◄── model ──► ent
```

---

## 常用命令

```bash
make init       # 安装 protoc 插件、wire、ent 等工具
make api        # 使用 protoc 生成 api/ 下的 proto 代码
make config     # 使用 protoc 生成 core/conf 下的配置 proto 代码
make ent        # 生成 ent 代码
make wire       # 生成 cmd/ 下的 wire 依赖注入代码
make generate   # 执行 go generate ./... 并 go mod tidy
make check      # 执行 go vet 与架构边界校验测试 (test/arch)
make test       # 运行所有单元测试与架构测试
make build      # 编译二进制文件到 ./bin/
make run        # 本地启动服务 (自动匹配 ./cmd/* -conf ./configs)
```
