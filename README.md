# B-UI

B-UI 是基于 [alireza0/b-ui](https://github.com/alireza0/b-ui) 的定制化运维面板。当前 fork 保留上游兼容的安装布局，整合 Go 后端、Vue 前端、sing-box 运行时、订阅服务、WebTerminal、Docker/Windows 发布资产，以及面向多节点域的集群管控能力。

当前文档以 `v0.1.96` 已发布能力为准，`v0.2.0` milestone 主要用于统一文档、版本线和发布入口。

## 项目定位

B-UI 面向需要长期运维多个代理节点的场景：

- 单节点部署时，它提供入站、出站、客户端、TLS、订阅、系统监控和面板自更新能力。
- 多节点部署时，它通过 `b-cluster-hub` 管理域成员，由 B-UI 节点直接执行节点间通信和域资源同步。
- 协议约定由 `b-protocol` 维护，B-UI 实现其中的 Hub API 消费端和 Node peer API。

## 特色功能

### 代理与订阅

- 支持 VLESS、VMess、Trojan、Shadowsocks、Hysteria2、Naive、Tailscale 等入站协议。
- 客户端可以绑定多个入站，入站可绑定 TLS/Reality 模板。
- 订阅输出支持原始链接、Base64、JSON、Clash YAML、Sing-box 等格式。
- 支持 VMess/VLESS/Trojan/Shadowsocks/Hysteria2/TUIC 等链接生成与转换。
- 支持出站、服务、端点、DNS、路由规则等 sing-box 运行对象管理。

### TLS、Reality 与安全

- TLS 模板集中管理，入站通过模板复用证书、ALPN、SNI、Reality、ECH 等配置。
- Reality 支持候选域名探测和 Ed25519 密钥对生成。
- API v2 使用 Bearer Token，适合机器访问和自动化。
- 多管理员账户区分管理员与普通角色。

### 运维工具

- Dashboard 提供 CPU、内存、磁盘、网络、数据库、sing-box 状态、在线连接等运行信息。
- WebTerminal 允许管理员在浏览器内连接服务器本地 shell。终端默认不自动连接，需要点击 `Connect` 并确认；离开页面前会提示并主动中断会话。
- 面板内置自更新流程，包含 preflight 检查、后台执行、日志追踪、版本协调和失败恢复。
- 数据库支持导出/导入，sing-box 运行配置支持导出。

### 集群管控

- `Cluster Center` 支持通过 `buihub://` 加入 Hub 域、查看域成员、同步成员快照、删除成员、查看节点详情。
- Hub 只保存域成员和资源只读视图；节点间命令由 B-UI 节点根据 Hub 快照直连对端，不经过 Hub 转发。
- 节点通信使用 `X-Cluster-Token` 认证，并支持 Ed25519 公钥用于消息验证。
- Mesh Ping 支持 ICMP/TCP/HTTP 多位置探测，结果用于节点详情延迟卡片。
- 域入站使用稳定 `group_id` 作为长期资源身份，本地入站 tag 按 `[prefix]-[tag seed]-[node display name]-[suffix]` 生成，空片段会被省略。
- 域用户可绑定稳定的域入站 group id，Hub 订阅生成会按 `domain_inbound_group_id` 选择对应节点配置。
- 域用户支持外部直连链接和外部订阅链接；面板会去重并上报给 Hub，用于合并到 raw、Clash、Sing-box 等订阅输出。
- 域用户协议密钥可以由各落地节点本地生成，例如 UUID、password、auth，不会被 Hub 统一覆盖。

## 快速安装

Linux 全新安装：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)
```

安装指定版本：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) v0.2.0
```

默认安装结果：

- 管理命令：`b-ui`
- systemd 服务：`b-ui`
- 安装根目录：`/usr/local/b-ui`
- 数据库路径：`/usr/local/b-ui/db/b-ui.db`

安装完成后检查：

```sh
systemctl status b-ui
b-ui
```

首次登录后建议先进入 `Settings` 检查面板地址、Base URI、订阅端口、订阅路径、TLS 证书路径和时区。

## Docker 引导部署

仓库内提供交互式 Docker 引导脚本：

```sh
bash ./scripts/release/install-docker.sh
```

脚本会生成：

- `deploy/docker-compose.yml`
- `deploy/db/`
- `deploy/cert/`

默认镜像为 `ghcr.io/beanya/b-ui:latest`。如需固定版本、私有 registry 或 digest：

```sh
IMAGE_REF=ghcr.io/beanya/b-ui:v0.2.0 bash ./scripts/release/install-docker.sh
```

Docker 引导支持跳过协议配置，也支持创建最小可用的 `VLESS + TLS`、`VLESS + Reality` 或 `Hysteria2` 对象。默认面板访问方式是 `http://<server-ip>:<panel-port><panel-path>`，脚本不会替宿主机配置 Nginx，也不会自动为面板申请 ACME 证书。

## 基本使用流程

### 创建单节点代理

1. 进入 `TLS Settings`，创建或确认 TLS/Reality 模板。
2. 进入 `Clients`，创建客户端并保存 UUID、password 或 auth 等凭据。
3. 进入 `Inbounds`，创建入站，绑定 TLS 模板和客户端。
4. 从客户端视图或订阅入口复制链接。
5. 用真实客户端做一次连通性验证。

推荐首次使用 `VLESS + TLS`，因为它最容易验证域名、证书、端口和客户端配置是否正确。

### 加入集群域

1. 在 `b-cluster-hub` 管理页创建域，并复制 `buihub://` Join URI。
2. 在 B-UI 打开 `Cluster Center`，点击 `Register`。
3. 粘贴 Join URI，确认 `Hub URL`、`Domain`、`Token` 自动解析正确。
4. 提交注册，等待操作状态完成。
5. 在域详情中查看成员、Mesh Ping 延迟、活动日志和资源上报状态。

### 管理域入站和域用户

1. 在域详情中创建域入站组，填写 tag seed、prefix、suffix、协议和可选 TLS 模板。
2. 每个节点会根据自己的 display name 生成本地入站 tag，例如 `edge-vless-shanghai-main`。
3. 创建域用户时绑定域入站 group id，而不是绑定某个节点的本地入站 id。
4. 如需把外部节点加入同一个域用户订阅，可添加外部 direct link 或外部 subscription link。
5. Hub 订阅会合并节点上报的域入站配置和用户外部链接，并按所选格式输出。

## 更新与迁移

常规更新：

```sh
b-ui update
```

强制重装目标版本：

```sh
b-ui update --force
```

直接调用安装脚本：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --update
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update
```

从上游安装迁移到当前 fork：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate
```

迁移会保留 `/usr/local/b-ui` 下的现有数据，并切换默认服务名和管理命令到 `b-ui`。

## 架构概览

后端采用分层结构：

- `src/backend/internal/http/`：面板 API、订阅 API、中间件。
- `src/backend/internal/domain/`：客户端、入站、出站、TLS、集群、更新、统计等领域服务。
- `src/backend/internal/infra/`：SQLite/GORM、日志、网络、嵌入式前端资源。
- `src/backend/internal/cli/`：`admin`、`setting`、`uri`、`migrate` 等命令。
- `src/backend/internal/app/`：应用启动与依赖装配。

前端采用 Vue 3、Vuetify、Vite、TypeScript 和 Pinia，功能按 dashboard、cluster、webterminal、panelUpdate、settings、theme 等模块组织。

发布流水线构建 Linux 多架构静态二进制、Windows amd64/arm64 包和 GHCR 多架构 Docker 镜像。

## 开发入口

常用命令：

```sh
bash ./scripts/dev/run-local.sh
cd src/frontend && npm install && npm run dev
```

发布版本声明位于：

- 后端：`src/backend/internal/domain/config/version`
- 前端：`src/frontend/package.json`

## 文档导航

- 用户手册：[`docs/manual.md`](./docs/manual.md)
- 迁移说明：[`MIGRATION.md`](./MIGRATION.md)
- 贡献与本地开发：[`CONTRIBUTING.md`](./CONTRIBUTING.md)
- 前端设计基线：[`DESIGN.md`](./DESIGN.md)
- Hub 服务：`BeanYa/b-cluster-hub`
- 协议参考：`BeanYa/b-protocol`

## Fork 说明

- 上游后端：[`alireza0/b-ui`](https://github.com/alireza0/b-ui)
- 上游前端：[`alireza0/b-ui-frontend`](https://github.com/alireza0/b-ui-frontend)
- 当前 fork 已将前端源码并入 `BeanYa/b-ui`，由同一仓库发布。
