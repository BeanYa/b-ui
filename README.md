# B-UI

基于 [S-UI](https://github.com/alireza0/s-ui) 的定制化 fork，保留 Go 后端能力，并将前端界面按本仓库的设计基线持续重做。

当前仓库的重点不再是“同步上游说明文档”，而是明确下面三件事：

- 这是一个 fork，核心能力来源于上游 `S-UI`
- 前端代码位于 `frontend/` 子模块，当前仓库为 `b-ui-frontend`
- 所有新的 UI 改动默认遵循 [DESIGN.md](./DESIGN.md)

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

## 从已安装的 S-UI 迁移

对于已经安装了上游 `s-ui` 的 Linux 服务器，本仓库现在支持原地迁移。

迁移时会继续复用原有：

- `s-ui` systemd 服务名
- `/usr/local/s-ui` 安装目录
- `/usr/local/s-ui/db/s-ui.db` 数据库
- `s-ui` 管理命令

推荐直接执行：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/migrate-to-b-ui.sh)
```

如果要迁移到指定版本：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/migrate-to-b-ui.sh) v0.0.1
```

迁移脚本会自动完成以下操作：

1. 检测现有 `s-ui` 安装
2. 停止旧服务
3. 在 `/var/backups/s-ui/<timestamp>/` 创建回滚备份
4. 下载本仓库 release 并原地覆盖安装
5. 执行 `sui migrate`
6. 重新启动 `s-ui` 服务

如果新版本启动失败，安装脚本会自动回滚到迁移前的版本。

更多说明见 [MIGRATION.md](./MIGRATION.md)。

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
