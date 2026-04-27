# B-UI Frontend

`B-UI Frontend` 是当前主仓库 `BeanYa/b-ui` 内直接维护的前端源码目录，位于 `src/frontend/`，来源于 [上游前端仓库](https://github.com/alireza0/b-ui-frontend)，并在这个 fork 中继续做视觉和交互层的定制。

## 技术栈

- Vue 3
- Vue Router
- Pinia
- Vuetify 4
- Vite

## 安装

如果你是在找仓库整体的安装、运行、升级或面板使用说明，请回到根目录查看 [`README.md`](../../README.md)、[`docs/manual.md`](../../docs/manual.md) 和 [`MIGRATION.md`](../../MIGRATION.md)。这个 README 只保留前端源码目录本身的开发说明。

```sh
npm install
```

## 本地开发

```sh
npm run dev
```

## 构建

```sh
npm run build
```

## Lint

```sh
npm run lint
```

## 设计约定

这里的前端代码不再通过子模块独立发布；所有 UI 改动直接跟随根仓库提交。改 UI 前请先看根仓库的 `DESIGN.md`。

当前方向：

- 深色桌面工具感
- 明确的层次、边框和阴影系统
- 更强的首页信息编排和登录页品牌感
- 避免通用管理台模板观感

## 与根仓库协作

前端源码目录内直接执行 `npm run build`，主要用于验证前端工程本身可以正常构建。仓库级构建、后端嵌入静态资源、CI 和发布流程所使用的资源刷新，应在仓库根目录执行集中构建脚本：

```sh
bash ./scripts/build/build-frontend.sh
```

按需要再执行 `bash ./scripts/build/build-backend.sh`，或直接运行 `bash ./scripts/dev/run-local.sh` 完成本地联调。
