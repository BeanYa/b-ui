# B-UI Frontend

`B-UI Frontend` 是当前主仓库 `BeanYa/b-ui` 内直接维护的前端源码目录，来源于 [上游前端仓库](https://github.com/alireza0/s-ui-frontend)，并在这个 fork 中继续做视觉和交互层的定制。

## 技术栈

- Vue 3
- Vue Router
- Pinia
- Vuetify 4
- Vite

## 安装

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

前端构建产物会被复制到根仓库的 `web/html/` 中，由 Go 后端统一提供服务。通常流程如下：

```sh
npm run build
```

然后在根仓库执行静态资源同步与后端构建。
