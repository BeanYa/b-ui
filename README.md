# B-UI

基于 [b-ui](https://github.com/alireza0/b-ui) 的定制化 fork。当前仓库保留上游后端兼容安装布局，并持续维护 `BeanYa/b-ui` 的发布、安装脚本、前端源码与文档。

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                         B-UI Binary                          │
│  ┌──────────────────────┐  ┌──────────────────────────────┐ │
│  │     Panel Server     │  │     Subscription Server      │ │
│  │  (session auth API)  │  │  (separate port, link/subs)  │ │
│  └──────────┬───────────┘  └──────────────────────────────┘ │
│  ┌──────────▼───────────────────────────────────────────────┐│
│  │                     HTTP Transport                       ││
│  │  api/ (v1 + v2 handlers)  │  sub/  │  middleware/        ││
│  └──────────────────────────────┬───────────────────────────┘│
│  ┌──────────────────────────────▼───────────────────────────┐│
│  │                    Domain Layer                          ││
│  │  services/           │  core/        │  jobs/            ││
│  │  cluster/*, panel,   │  sing-box     │  cron scheduler   ││
│  │  client, inbound,    │  runtime      │  (10 scheduled    ││
│  │  outbound, endpoint, │               │   jobs)           ││
│  │  warp, stats, ...    │               │                   ││
│  └──────────────────────────────────────────────────────────┘│
│  ┌──────────────────────────────────────────────────────────┐│
│  │                    Infrastructure                        ││
│  │  infra/db/  │  infra/logging/  │  infra/network/  │web/ ││
│  │  SQLite WAL │  structured log  │  HTTP client     │SPA  ││
│  └──────────────────────────────────────────────────────────┘│
│  ┌──────────────────────────────────────────────────────────┐│
│  │  cli/  │  shared/  │  config/                          ││
│  │ admin, │  helpers, │  settings,                         ││
│  │ setting│  utils    │  TLS, certs                        ││
│  └──────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│                   Frontend (Vue 3 SPA)                       │
│  Vuetify 4.0 + Vite 8 + TypeScript 6                        │
│  Pinia stores (auth, data, ping, remoteNode)                 │
│  features/  │  views/  │  components/  │  locales/ (6 lang) │
│  dashboard, │ cluster, │ shared UI    │  en, zh-CN, zh-TW,  │
│  webterminal│ settings,│ components   │  ru, vi, fa          │
│  panelUpdate│ clients, │              │                      │
│  theme      │ outbounds│              │                      │
└──────────────────────────────────────────────────────────────┘
```

### Backend architecture

The Go backend follows a layered domain-driven design:

- **`internal/domain/`** — Business logic with no framework dependencies
  - `services/` — Domain services: client, inbound, outbound, endpoint, service,
    TLS, WARP, stats, setting, user, panel update, cluster (membership, peer
    messaging, reachability, mesh ping, hub client, sync, crypto)
  - `core/` — sing-box runtime integration: live hot-reload of
    inbounds/outbounds/endpoints/services without restart, connection tracking,
    stats collection
  - `jobs/` — Cron scheduler with 10 scheduled jobs (stats collection, client
    depletion, core health check, WAL checkpoint, domain hints, cluster
    version poll, reachability probe, mesh ping, peer schedule, old stats cleanup)
  - `config/` — Application configuration models
- **`internal/http/`** — HTTP transport layer
  - `api/` — v1 (session auth) and v2 (Bearer token auth) API handlers
  - `sub/` — Subscription server: raw links, base64, JSON, Clash YAML output formats
  - `middleware/` — Session auth, token auth, CORS
- **`internal/infra/`** — Infrastructure
  - `db/` — SQLite with WAL mode, GORM models, migrations
  - `logging/` — Structured logging
  - `network/` — HTTP client utilities
  - `web/` — Embedded SPA static assets
- **`internal/cli/`** — CLI commands: `admin`, `setting`, `uri`, `migrate`
- **`internal/app/`** — Application bootstrap and wiring
- **`internal/shared/`** — Cross-cutting utilities

The proxy engine is [sing-box](https://sing-box.sagernet.org/) v1.13, compiled
with build tags for QUIC, gRPC, uTLS, ACME, gVisor, Naive outbound, and
Tailscale support.

### Frontend architecture

- **Vue 3.5** + **Vuetify 4.0** + **Vite 8** + **TypeScript 6**
- **Pinia** stores: `auth`, `data`, `ping`, `remoteNode`
- **Feature-based organization**: `features/dashboard/`, `features/webterminal/`,
  `features/panelUpdate/`, `features/theme/`, `features/settings/`, `features/data/`
- **6-language i18n**: English, Simplified Chinese, Traditional Chinese, Russian,
  Vietnamese, Persian
- 10-second data polling interval for dashboard refresh

### Build pipeline

- **Linux**: Fully static binaries via musl libc, 7 architectures (amd64, arm64,
  armv7, armv6, armv5, 386, s390x), cronet toolchain for Naive outbound
- **Windows**: amd64 (CGO) and arm64 (pure Go), Windows service wrapper
- **Docker**: Multi-arch images for 5 platforms pushed to GHCR
- Release automation in `.github/workflows/release.yml`

## 安装与快速开始

Linux 默认通过仓库根目录的 `install.sh` 进入 `scripts/release/install.sh` 完成安装。全新安装后，默认名称和路径如下：

- 管理命令：`b-ui`
- systemd 服务名：`b-ui`
- 安装根目录：`/usr/local/b-ui`
- 数据库路径：`/usr/local/b-ui/db/b-ui.db`

### 全新安装

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)
```

### 安装指定版本

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) v0.0.1
```

### Docker 引导入口

Docker 模式使用 `scripts/release/install-docker.sh`，默认拉取 `ghcr.io/beanya/b-ui:latest`。脚本会交互式收集面板端口、路径、管理员凭据，以及可选的协议引导信息，然后在当前目录下生成 `deploy/docker-compose.yml`、`deploy/db/`、`deploy/cert/` 并启动容器。

```sh
bash ./scripts/release/install-docker.sh
```

- 如需使用指定版本、fork 镜像、私有 registry 或 digest 固定镜像，可通过 `IMAGE_REF` 覆盖默认值：

```sh
IMAGE_REF=ghcr.io/beanya/b-ui:v0.1.14 bash ./scripts/release/install-docker.sh
```

- 默认面板访问方式是直接 `http://<server-ip>:<panel-port><panel-path>`，不依赖宿主机 Nginx
- 脚本不会为面板自动申请 ACME 证书；如需可信证书，请把文件挂载到 `deploy/cert/` 后在面板中改用对应路径
- 可选协议引导支持 `VLESS + TLS`、`VLESS + Reality`、`Hysteria2`，也可以跳过只部署面板
- 为了让首次引导更容易跑通，`VLESS + TLS` 和 `Hysteria2` 生成的客户端 TLS 侧默认会保留 `insecure: true`；如果你已经换成可信证书，请回到面板里把对应客户端或模板收紧
- 更完整的 Docker 说明见 [`docs/manual.md`](./docs/manual.md)

### 快速开始

1. 以 root 运行安装命令。
2. 安装完成后确认服务状态：`systemctl status b-ui`。
3. 先阅读完整用户手册 [`docs/manual.md`](./docs/manual.md)，按手册完成 TLS、客户端和入站配置。
4. 后续更新使用 `b-ui update`，强制重装当前版本使用 `b-ui update --force`。

## 功能速览

### 核心代理管理

- **多协议入站**：支持 VLESS、VMess、Trojan、Shadowsocks、Hysteria2、Naive、Tailscale 等多种入站协议，每个用户可绑定多个入站。
- **出站管理**：独立的出站实体管理，支持延迟测试（URL-based latency check）。
- **服务管理**：独立的服务实体（Services），支持 TLS 绑定和 sing-box 核心热重载。
- **端点管理**：独立的端点实体（Endpoints），内置 Cloudflare WARP 集成（自动注册设备、生成 WireGuard 密钥、应用 License）。
- **DNS 配置**：独立的 DNS 服务器配置视图。
- **路由规则**：独立的路由规则管理视图。

### 订阅系统

- **多格式输出**：支持原始链接、Base64 编码、JSON、Clash YAML（内置 DNS/TUN/规则模板）。
- **链接转换**：支持 VMess/VLESS/Trojan 等协议的链接格式转换。
- **订阅转换**：支持订阅 URL 到多格式输出的转换。

### TLS 与安全

- **TLS 预设密钥对即时生成**：创建 TLS 预设时自动物化密钥对材料，无需创建后手动重新生成。
- **Reality 域名探测**：自动探测候选域名（YouTube、Cloudflare、Apple 等）的 TLS 1.3 支持和 ALPN 协商，分类为推荐/可用/受限/失败。
- **Reality 密钥对生成**：支持 Ed25519 Reality 密钥对的即时生成。

### 集群管控平面

- **Cluster Center**：提供多节点集群管理能力，支持 Hub 域注册、成员节点自动发现与手动同步、操作状态轮询与成员管理。可在同一面板内跨节点查看集群状态。
- **一键加入集群**：支持 `buihub://` 协议 URI 解析，配合 Hub URL 协议选择（https/http），可快速将当前节点注册到指定 Hub 域。
- **节点间通信协议**：节点通过 Hub 下发的成员表获取对端端点后直连通信。支持 Ed25519 签名消息验证、多种路由模式（direct/multicast/broadcast/chain/scheduled_broadcast）、ACK 级别（none/node/quorum/all）、目标选择器（include/exclude/capability）和链式工作流（step-by-step with `continueOnFailure`）。Hub 只负责域成员权威登记，不参与节点间消息转发。
- **节点可达性追踪**：完整的可达性状态机（unknown → reachable → suspect → unreachable），可配置探测间隔、失败阈值和退避策略。
- **Mesh Ping**：集群成员间 ICMP/TCP/HTTP 多位置探测，30 秒间隔自动执行。
- **集群面板更新编排**：支持跨域广播更新可用性、更新请求推送、更新状态追踪和协调更新。
- **集群节点详情**：独立的集群节点详情视图，包含**可折叠延迟卡片**（显示 ping 延迟及颜色编码，支持按成员筛选）。
- **集群活动日志**：实时集群活动日志查看器，支持按域名过滤，自动记录入站/出站/Hub/Cron 四类操作日志，环形缓冲区最多保留 2048 条。
- **代理配置自动上报**：入站 CRUD 操作后自动向 Hub 上报节点代理配置，用于域订阅链接生成。

### 面板管理

- **面板自更新**：内置自更新检测流程，支持 preflight 资源可用性检查（最多 30 次重试）、detached systemd-run 执行、日志追踪、版本协调和崩溃恢复。可直接在面板内触发版本升级，无需 SSH 登录手动执行更新命令。
- **多管理员**：支持多管理员账户管理，区分 admin/non-admin 角色。
- **API v2（机器访问）**：独立的 Bearer Token 认证 API 层，支持 Token 过期管理和 CRUD。
- **数据库导出/导入**：支持数据库导出和导入操作。
- **sing-box 配置导出**：支持导出完整 sing-box 运行配置。

### 运维工具

- **交互式 WebTerminal**：管理员可以在面板内直接打开 `/webterminal`，连接服务器本地 shell，进行实时键盘输入、光标交互、流式输出查看与终端窗口尺寸同步。
- **更安全的终端连接流程**：WebTerminal 页面默认不会自动连入；需要先点击 `Connect` 并确认。离开页面、刷新页面或关闭标签页前会再次提醒，并在确认后主动中断当前终端会话。
- **系统资源监控**：实时 CPU、内存、磁盘、磁盘 IO、交换空间、网络、数据库和 sing-box 状态监控。
- **在线连接追踪**：实时在线用户/入站/出站追踪（10 秒刷新）。
- **客户端到期/流量耗尽自动禁用**：每分钟检查客户端流量和到期时间，自动禁用超额客户端并重启受影响入站。

### 部署与平台

- **Docker 引导部署**：提供 `scripts/release/install-docker.sh` 作为交互式 Docker 引导入口，可直接生成 compose 文件、初始化面板并按需引导基础协议对象。
- **Windows 原生支持**：Windows 服务包装器、批处理安装/卸载脚本，支持 amd64 和 arm64。
- **SIGHUP 热重启**：Linux 下支持 SIGHUP 信号触发热重启，Windows 下使用进程重启。
- **与上游兼容的安装布局**：默认安装根目录、数据库路径和服务运行方式继续兼容上游 `b-ui` 布局，便于迁移与运维。

### 前端

- **前端性能优化**：支持代码分割、资源按需加载、MDI 图标字体原生渲染，减少首屏资源开销。
- **6 语言国际化**：支持英语、简体中文、繁体中文、俄语、越南语、波斯语。
- **暗色设计系统**：基于 Raycast 风格的暗色 UI 设计（详见 [`DESIGN.md`](./DESIGN.md)）。

## 文档导航

- 安装迁移上游 `b-ui`：[`MIGRATION.md`](./MIGRATION.md)
- 完整用户手册：[`docs/manual.md`](./docs/manual.md)
- 贡献与本地开发：[`CONTRIBUTING.md`](./CONTRIBUTING.md)
- 前端设计基线：[`DESIGN.md`](./DESIGN.md)
- 集群协议：`BeanYa/b-protocol`

## 集群协议

B-UI 节点把 Hub 视为域成员的权威中心。加入域、退出域、删除成员等成员变更先提交到 Hub；Hub 成功登记后，节点通过节点间协议广播 `domain.cluster.changed` 事件，域内节点再向 Hub 拉取最新成员快照并更新本地集群表。

节点间通信不经过 Hub。每个节点维护自己的事件队列、已处理事件列表和投递日志，请求进入队列后按顺序处理；`messageId` 和 `idempotencyKey` 用于重复请求去重，重复事件不会再次执行副作用。

协议由私有 `BeanYa/b-protocol` 仓库统一维护，并按通信边界拆分：

- Hub 协议：`b-protocol/protocol/v1/hub`，定义注册、删除、操作查询、版本查询和成员快照。
- Node 协议：`b-protocol/protocol/v1/node`，定义 peer message envelope、HTTP 状态码约定、幂等处理和业务响应码。

节点展示名称使用 `display_name`，用于管理面板和域详情列表增强可读性；传播、请求和存储仍以 `node_id` 作为唯一主键。加入域时如果没有显式填写 `display_name`，默认从 `base_url` 的域名段推导，例如 `https://abc.def.com:1000/bui` 推导为 `abc.def.com`。

## 更新

安装完成后的常用更新命令：

```sh
b-ui update
b-ui update --force
```

如果你需要直接调用安装脚本，对应模式如下：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --update
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update
```

- `b-ui update` / `--update`：仅在已安装 `b-ui` 且当前版本低于目标版本时执行更新
- `b-ui update --force` / `--force-update`：即使当前版本相同也重新安装目标版本
- 如果当前版本已经等于或高于目标版本，`--update` 会直接退出，并提示改用 `--force-update`
- 两种模式都支持显式版本，例如 `b-ui update v0.0.1`

## 从已安装的上游版本迁移

如果服务器已经安装上游 `b-ui`，请使用迁移模式，而不是普通更新模式：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate
```

迁移会保留现有安装根目录 `/usr/local/b-ui`，并把默认服务名和管理命令切换为 `b-ui`。如果只存在旧库 `/usr/local/b-ui/db/b-ui.db`，程序会在首次迁移/启动时自动迁移到 `/usr/local/b-ui/db/b-ui.db`。

更多迁移细节见 [`MIGRATION.md`](./MIGRATION.md)。

## 仓库结构

- `src/backend/cmd/b-ui/`: Go 后端可执行入口
- `src/backend/internal/domain/`: 领域层（services, core, jobs, config）
- `src/backend/internal/http/`: HTTP 传输层（api v1/v2, sub, middleware）
- `src/backend/internal/infra/`: 基础设施层（db, logging, network, web）
- `src/backend/internal/cli/`: CLI 命令（admin, setting, uri, migrate）
- `src/backend/internal/shared/`: 跨层工具函数
- `src/frontend/`: Vue 3 + Vuetify 前端源码
- `src/services/`: systemd、Windows 服务等运行资产
- `scripts/build/`, `scripts/dev/`, `scripts/release/`: 构建、开发、发布脚本
- `packaging/docker/`: Docker 打包定义

## 开发说明

开发者通常只需要先看这三处：

- 本地联调：`bash ./scripts/dev/run-local.sh`
- 前端单独开发：在 `src/frontend/` 下执行 `npm install && npm run dev`
- UI 改动前先读 [`DESIGN.md`](./DESIGN.md)

## Fork 说明

- 上游后端：[`alireza0/b-ui`](https://github.com/alireza0/b-ui)
- 上游前端：[`alireza0/b-ui-frontend`](https://github.com/alireza0/b-ui-frontend)
- 当前 fork 已将前端源码直接并入 `BeanYa/b-ui`
