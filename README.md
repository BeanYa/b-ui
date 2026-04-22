# B-UI

基于 [S-UI](https://github.com/alireza0/s-ui) 的定制化 fork。当前仓库保留上游后端兼容安装布局，并持续维护 `BeanYa/b-ui` 的发布、安装脚本、前端源码与文档。

## 安装与快速开始

Linux 默认通过仓库根目录的 `install.sh` 进入 `scripts/release/install.sh` 完成安装。全新安装后，默认名称和路径如下：

- 管理命令：`b-ui`
- systemd 服务名：`b-ui`
- 安装根目录：`/usr/local/s-ui`
- 数据库路径：`/usr/local/s-ui/db/b-ui.db`

### 全新安装

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)
```

### 安装指定版本

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) v0.0.1
```

### Docker 引导入口

Docker 模式使用 `scripts/release/install-docker.sh`，会交互式收集面板端口、路径、管理员凭据，以及可选的协议引导信息，然后在当前目录下生成 `deploy/docker-compose.yml`、`deploy/db/`、`deploy/cert/` 并启动容器。

```sh
IMAGE_REF=<your-image-ref> bash ./scripts/release/install-docker.sh
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

- **交互式 WebTerminal**：管理员可以在面板内直接打开 `/webterminal`，连接服务器本地 shell，进行实时键盘输入、光标交互、流式输出查看与终端窗口尺寸同步。
- **更安全的终端连接流程**：WebTerminal 页面默认不会自动连入；需要先点击 `Connect` 并确认。离开页面、刷新页面或关闭标签页前会再次提醒，并在确认后主动中断当前终端会话。
- **Docker 引导部署**：提供 `scripts/release/install-docker.sh` 作为交互式 Docker 引导入口，可直接生成 compose 文件、初始化面板并按需引导基础协议对象。
- **与上游兼容的安装布局**：默认安装根目录、数据库路径和服务运行方式继续兼容上游 `s-ui` 布局，便于迁移与运维。

## 文档导航

- 安装迁移上游 `s-ui`：[`MIGRATION.md`](./MIGRATION.md)
- 完整用户手册：[`docs/manual.md`](./docs/manual.md)
- 贡献与本地开发：[`CONTRIBUTING.md`](./CONTRIBUTING.md)
- 前端设计基线：[`DESIGN.md`](./DESIGN.md)

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

如果服务器已经安装上游 `s-ui`，请使用迁移模式，而不是普通更新模式：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate
```

迁移会保留现有安装根目录 `/usr/local/s-ui`，并把默认服务名和管理命令切换为 `b-ui`。如果只存在旧库 `/usr/local/s-ui/db/s-ui.db`，程序会在首次迁移/启动时自动迁移到 `/usr/local/s-ui/db/b-ui.db`。

更多迁移细节见 [`MIGRATION.md`](./MIGRATION.md)。

## 仓库结构

- `src/backend/cmd/b-ui/`: Go 后端可执行入口
- `src/backend/internal/`: Go 后端主体代码
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

- 上游后端：[`alireza0/s-ui`](https://github.com/alireza0/s-ui)
- 上游前端：[`alireza0/s-ui-frontend`](https://github.com/alireza0/s-ui-frontend)
- 当前 fork 已将前端源码直接并入 `BeanYa/b-ui`
