# User Documentation Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite B-UI user-facing documentation so installation is the primary entrypoint, a new full manual covers setup and operations, and protocol walkthroughs guide users through `VLESS + TLS`, `Hysteria2`, and `VLESS + Reality`.

**Architecture:** Keep `README.md` short and install-first, add a new long-form manual under `docs/`, move substantive migration content into that manual, and reduce `MIGRATION.md` to a compatibility redirect page. Source all behavioral details from the current install/update scripts and current backend/frontend paths so the docs match the real product rather than a guessed flow.

**Tech Stack:** Markdown, repository docs, shell install/update scripts, Go backend configuration concepts, Vue frontend panel concepts

---

## File Structure Map

### Create

- `docs/manual.md`

### Modify

- `README.md`
- `MIGRATION.md`
- `CONTRIBUTING.md`
- `src/frontend/README.md`

### Reference During Implementation

- `scripts/release/install.sh`
- `install.sh`
- `scripts/migration/migrate-to-b-ui.sh`
- `scripts/build/build-frontend.sh`
- `scripts/build/build-backend.sh`
- `src/backend/internal/shared/util/outJson.go`
- `src/backend/internal/shared/util/linkToJson.go`

## Task 1: Rewrite README As Install-First Entry Point

**Files:**
- Modify: `README.md`
- Test: `README.md` path/command accuracy by cross-checking against `scripts/release/install.sh`, `scripts/build/build-frontend.sh`, and current repository layout

- [ ] **Step 1: Write the failing README outline check**

Before editing, verify the current README still starts operationally with migration instead of installation.

Run:

```bash
rg -n "从已安装的上游版本迁移|## 安装|## 安装与快速开始" README.md
```

Expected: `README.md` contains a migration-first operational section and does not yet provide the desired install-first landing structure.

- [ ] **Step 2: Replace the top-level operational flow in README**

Rewrite `README.md` to use this section order and content shape:

```markdown
# B-UI

<1-2 short paragraphs describing the fork and what B-UI is for>

## 安装

### 全新安装

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)
```

### 安装指定版本

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) v0.0.1
```

安装完成后默认：
- 管理命令：`b-ui`
- systemd 服务名：`b-ui`
- 安装目录：`/usr/local/s-ui`
- 数据库路径：`/usr/local/s-ui/db/b-ui.db`

## 快速开始

1. 安装完成后访问面板
2. 完成首次登录和基础检查
3. 按完整手册中的“最快创建一个代理站点”完成 `VLESS + TLS`

## 文档

- [完整手册](./docs/manual.md)
- [迁移说明](./MIGRATION.md)
- [开发说明](./CONTRIBUTING.md)

## 开发相关

- `src/frontend/`
- `src/backend/`
- `src/services/`
- `scripts/`
- `packaging/`
```

The rewritten README must stop treating migration as the first operational path.

- [ ] **Step 3: Cross-check the new install/update wording against the actual installer**

Read `scripts/release/install.sh` and verify the README install commands and install result bullets match:

- `bash install.sh`
- `bash install.sh <version>`
- default command name `b-ui`
- default service name `b-ui`
- install root `/usr/local/s-ui`
- default database path `/usr/local/s-ui/db/b-ui.db`

Expected: README wording matches the real script behavior.

- [ ] **Step 4: Run a focused stale-content check**

Run:

```bash
rg -n "从已安装的上游版本迁移|--migrate" README.md
```

Expected: migration is no longer the first major operational section; any remaining migration mention is only a brief pointer into the dedicated migration section/manual.

- [ ] **Step 5: Commit the README rewrite**

```bash
git add README.md
git commit -m "docs: make README install-first"
```

## Task 2: Create The Full Operator Manual

**Files:**
- Create: `docs/manual.md`
- Test: `docs/manual.md` structure and examples against installer behavior and known panel concepts

- [ ] **Step 1: Write the new manual skeleton**

Create `docs/manual.md` with this section structure:

```markdown
# B-UI 使用手册

## 1. 安装
## 2. 首次登录与初始化
## 3. 最快创建一个代理站点
## 4. TLS 设置
## 5. 入站设置
## 6. 协议示例
### 6.1 VLESS + TLS
### 6.2 Hysteria2
### 6.3 VLESS + Reality
## 7. 更新与强制更新
## 8. 从上游迁移
## 9. 功能逻辑与基本使用
## 10. 基本排错
```

- [ ] **Step 2: Fill the installation and update sections from the real script behavior**

Document these exact command families from `scripts/release/install.sh`:

```markdown
### 全新安装

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)
```

### 安装指定版本

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) v0.0.1
```

### 更新

```sh
b-ui update
b-ui update v0.0.1
```

### 强制更新

```sh
b-ui update --force
b-ui update v0.0.1 --force
```
```

Explain when each mode should be used, matching the installer’s actual mode logic.

- [ ] **Step 3: Write the fast setup path using `VLESS + TLS` as the main example**

In `docs/manual.md`, write the “fastest way to create a proxy site” section using this operational order:

```markdown
## 最快创建一个代理站点

推荐首次使用：`VLESS + TLS`

前置条件：
- 服务器已安装 `b-ui`
- 已有可解析到服务器的域名
- 面板可以正常访问
- 对应端口已放行

步骤：
1. 创建 TLS 设置
2. 创建 VLESS 入站
3. 添加客户端/用户
4. 复制节点或订阅信息
5. 在客户端验证连通性
```

This section must explicitly point readers to the dedicated TLS and inbound sections for field-by-field explanation.

- [ ] **Step 4: Write TLS, inbound, and protocol walkthrough sections**

The manual must contain:

- a TLS section that explains what the panel’s TLS settings are for and which minimum fields matter
- an inbound section that explains the required fields to bring up a site
- a full `VLESS + TLS` walkthrough
- a full `Hysteria2` walkthrough
- a full `VLESS + Reality` walkthrough

Each protocol walkthrough must answer these five questions explicitly:

```markdown
- 什么时候用它
- 需要先准备什么
- 面板里先点哪里
- 哪些字段必须填
- 创建完后如何验证
```

Use the backend utility code as reference to ensure protocol names and concepts are not invented:

- `src/backend/internal/shared/util/outJson.go`
- `src/backend/internal/shared/util/linkToJson.go`

- [ ] **Step 5: Verify manual coverage against the approved spec and commit**

Run:

```bash
rg -n "^## |^### " docs/manual.md
```

Expected: all required sections and protocol walkthroughs exist.

Commit:

```bash
git add docs/manual.md
git commit -m "docs: add operator manual"
```

## Task 3: Merge Migration Content Into The New Manual

**Files:**
- Modify: `docs/manual.md`
- Modify: `MIGRATION.md`

- [ ] **Step 1: Move the substantive migration logic into the manual**

Take the current `MIGRATION.md` content and fold its substance into `docs/manual.md` section `## 8. 从上游迁移`.

The migrated section must preserve:

- one-line migration command
- version-specific migration example
- what `--migrate` does step-by-step
- preserved data and rollback behavior
- update-mode behavior after migration

- [ ] **Step 2: Reduce MIGRATION.md to a compatibility redirect page**

Replace `MIGRATION.md` with a short compatibility page like:

```markdown
# Migration From Upstream

迁移说明已合并到完整手册：

- [查看完整迁移章节](./docs/manual.md#8-从上游迁移)

如果你是从上游 `s-ui` 迁移，请优先阅读该章节中的：
- 迁移命令
- 数据保留说明
- 回滚说明
- 迁移后的更新方式
```

- [ ] **Step 3: Verify no migration details were lost**

Run:

```bash
rg -n "--migrate|--update|--force-update|rollback|b-ui-linux-<arch>.tar.gz" docs/manual.md MIGRATION.md
```

Expected: migration commands and behavioral notes still exist, but the long-form content now lives in `docs/manual.md`.

- [ ] **Step 4: Commit the migration consolidation**

```bash
git add docs/manual.md MIGRATION.md
git commit -m "docs: merge migration guide into manual"
```

## Task 4: Align Supporting Docs With The New User Journey

**Files:**
- Modify: `CONTRIBUTING.md`
- Modify: `src/frontend/README.md`

- [ ] **Step 1: Update contributor docs to link to the new manual**

Add or update short references in `CONTRIBUTING.md` so contributors can find the user-facing documentation entrypoints:

```markdown
- 用户安装与使用说明见 `README.md`
- 完整运维与配置说明见 `docs/manual.md`
- 迁移兼容入口见 `MIGRATION.md`
```

- [ ] **Step 2: Keep frontend README narrowly scoped**

Ensure `src/frontend/README.md` stays frontend-focused and does not try to duplicate the new user manual. It may link back to:

```markdown
完整安装与使用说明见仓库根目录 `README.md` 与 `docs/manual.md`。
```

- [ ] **Step 3: Verify the new doc link graph**

Run:

```bash
rg -n "docs/manual.md|MIGRATION.md|README.md" CONTRIBUTING.md src/frontend/README.md
```

Expected: both supporting docs point users and contributors to the new manual structure instead of duplicating the same operational content.

- [ ] **Step 4: Commit the supporting-doc alignment**

```bash
git add CONTRIBUTING.md src/frontend/README.md
git commit -m "docs: align supporting docs with manual"
```

## Self-Review Checklist

- Spec coverage:
  - install-first README is covered by Task 1
  - full manual with `VLESS + TLS`, `Hysteria2`, and `VLESS + Reality` is covered by Task 2
  - migration consolidation is covered by Task 3
  - supporting doc alignment is covered by Task 4
- Placeholder scan:
  - no `TBD`, `TODO`, or deferred implementation notes remain
- Consistency:
  - all user-facing command examples must match `scripts/release/install.sh`
  - protocol names must match current backend terminology from `src/backend/internal/shared/util/outJson.go`
  - README remains concise while `docs/manual.md` becomes the long-form source of truth
