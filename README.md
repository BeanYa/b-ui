# B-UI

基于 [S-UI](https://github.com/alireza0/s-ui) 的定制化 fork，保留 Go 后端能力，并将前端界面按本仓库的设计基线持续重做。

当前仓库的重点不再是“同步上游说明文档”，而是明确下面三件事：

- 这是一个 fork，核心能力来源于上游 `S-UI`
- 前端代码位于 `frontend/` 子模块，当前仓库为 `b-ui-frontend`
- 所有新的 UI 改动默认遵循 [DESIGN.md](./DESIGN.md)

**想参与开发？** 见 [CONTRIBUTING.md](./CONTRIBUTING.md)。

## 仓库结构

- `frontend/`: Vue 3 + Vuetify 4 前端子模块
- `web/`: 编译后的前端静态资源会被打包到这里
- `api/`, `service/`, `database/`, `middleware/`: Go 后端主体
- `DESIGN.md`: 当前 UI 设计参考，现阶段采用 Raycast 风格的深色工具界面

## 初始化

```sh
git clone https://github.com/BeanYa/b-ui.git
cd b-ui
git submodule update --init --remote --recursive
```

说明：

- `frontend` 子模块默认跟踪 `b-ui-frontend` 的 `main` 分支
- 但 Git 子模块机制本身仍然会在父仓库里记录一个具体 commit，这是 Git 的正常行为
- 如果你想在本地显式刷新到前端最新 `main`，重复执行一次 `git submodule update --remote --recursive` 即可
- 当前 release / docker workflow 也会在 CI 中主动刷新 `frontend` 子模块到最新 `main` 后再构建

## 从已安装的上游版本迁移

对于已经安装了上游版本的 Linux 服务器，本仓库现在支持原地迁移，并在迁移完成后自动确认更新到最新的 `b-ui` release。

迁移时会继续复用原有：

- `/usr/local/s-ui` 安装目录
- `/usr/local/s-ui/db/s-ui.db` 数据库
- 现有配置与数据

推荐直接执行：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/migrate-to-b-ui.sh)
```

如果要迁移到指定版本：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/migrate-to-b-ui.sh) v0.0.1
```

如果你想直接调用安装脚本，也可以显式使用迁移模式：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate
```

迁移脚本会自动完成以下操作：

1. 检测现有旧版安装
2. 停止旧服务
3. 在 `/var/backups/s-ui/<timestamp>/` 创建回滚备份
4. 下载本仓库 release 并原地覆盖安装
5. 执行 `sui migrate`
6. 把 systemd 服务名从 `s-ui` 切换为 `b-ui`
7. 把管理命令切换为 `b-ui`
8. 未指定版本时，再执行一次最新 `b-ui` release 的更新检查
9. 重新启动新的 `b-ui` 服务

如果新版本启动失败，安装脚本会自动回滚到迁移前的版本。未显式指定版本时，迁移脚本的默认目标是最新发布的 `b-ui`。

更多说明见 [MIGRATION.md](./MIGRATION.md)。

## 更新与强制更新

迁移或安装完成后，管理命令为 `b-ui`，更新区分为两种：

```sh
b-ui update
b-ui update --force
```

- `b-ui update`：仅当 GitHub 最新 release 版本与当前安装版本不同的时候才执行更新
- `b-ui update --force`：即使当前版本已经相同，也会重新下载并覆盖安装
- `b-ui update v0.0.1`：更新到指定版本

如果你需要直接调用安装脚本，对应参数如下：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --update
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update
```

## 版本与发布

当前主线版本使用 `v0.0.x` 这一组 tag，例如：

```sh
git tag v0.0.2
git push origin v0.0.2
```

触发 tag 构建后：

- Linux release 资产命名为 `b-ui-linux-<arch>.tar.gz`
- Windows release 资产命名为 `b-ui-windows-<arch>.zip`
- Docker workflow 会向 `ghcr.io/beanya/b-ui` 推送对应 tag 的镜像
- 构建流程会把 Git tag 注入二进制版本号，因此 `sui -v` 会显示对应 release tag

目前安装脚本默认下载上述 `b-ui-*` release 资产；Linux 迁移后服务名与管理命令都会统一为 `b-ui`，安装目录仍保持 `/usr/local/s-ui` 兼容。

## 前端开发

```sh
cd frontend
npm install
npm run dev
```

前端修改约定：

- 先阅读 [DESIGN.md](./DESIGN.md)
- 优先改主题层、布局层和高频页面，不要只在单个页面上堆局部样式
- 设计方向保持深色、紧凑、桌面工具化，不回退到默认后台风格

## 前后端联调

根目录脚本会同时处理前后端开发流程，现有项目里可继续使用：

```sh
./runSUI.sh
```

如果只手动构建前端并同步到后端静态目录，可以按现有流程执行：

```sh
cd frontend
npm install
npm run build

# 回到仓库根目录
rm -fr web/html/*
cp -R frontend/dist/ web/html/
go build -o sui main.go
```

## Fork 说明

- 上游后端: [alireza0/s-ui](https://github.com/alireza0/s-ui)
- 上游前端: [alireza0/s-ui-frontend](https://github.com/alireza0/s-ui-frontend)
- 当前前端仓库: [BeanYa/b-ui-frontend](https://github.com/BeanYa/b-ui-frontend)

本仓库保留对上游的兼容基础，但文档、品牌名和前端视觉方向以 `B-UI` 为准。

## 设计基线

`DESIGN.md` 不是装饰性文件，而是前端重构时的实际约束：

- 使用近黑蓝底色而不是纯黑
- 控件层次依赖边框、内阴影和玻璃感表面
- 主色以信息蓝和点缀红为主，不使用普通后台模板色板
- 首页、登录页、导航框架应优先体现桌面工具感

如果你准备继续改 UI，请先从 `DESIGN.md` 和 `frontend/src/` 下的布局文件开始。
