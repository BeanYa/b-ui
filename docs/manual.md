# B-UI 使用手册

## 1. 安装

### 全新安装

以 `root` 运行：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)
```

安装指定版本：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) v0.0.1
```

安装完成后的默认结果：

- 管理命令：`b-ui`
- systemd 服务名：`b-ui`
- 安装根目录：`/usr/local/s-ui`
- 主数据库路径：`/usr/local/s-ui/db/b-ui.db`

安装脚本的实际行为：

- 要求 root 权限。
- 检测 Linux 发行版并安装 `wget`、`curl`、`tar`、`tzdata` 等基础依赖。
- 未指定版本时解析最新 GitHub release。
- 下载 `b-ui-linux-<arch>.tar.gz`。
- 将程序安装到 `/usr/local/s-ui`。
- 将管理命令安装到 `/usr/bin/b-ui`。
- 安装并启用 `b-ui` systemd 服务。
- 首次启动前执行数据库迁移。
- 安装完成后打印面板访问地址。

首次安装时的交互行为：

- 脚本会提示设置面板端口、面板路径、订阅端口和订阅路径。
- 也可以在安装时修改管理员用户名和密码。
- 如果跳过凭据修改，脚本会生成随机管理员凭据并打印一次。

## 2. 首次登录与初始化

这个阶段的目标是确认面板可访问、凭据可用、以及基础参数处于可继续配置站点的状态。

凭据来源：

- 如果你在安装过程中主动修改过用户名和密码，使用你填写的值。
- 如果你在全新安装时跳过了凭据修改，使用安装脚本打印的随机管理员凭据。
- 如果这是已有 `b-ui` 的更新，现有凭据会被保留。
- 如果这是从上游 `s-ui` 迁移，现有凭据和数据库内容会被保留。

首次登录步骤：

1. 打开安装脚本输出的面板地址。
2. 用当前管理员用户名和密码登录。
3. 如果是全新安装且无法登录，重新检查安装输出中的随机凭据。

建议先检查这几项：

1. 面板是否能正常打开，没有错误的路径或证书问题。
2. 进入 `Settings` -> `Interface`，确认：
   - `Address`
   - `Port`
   - `Base URI`
   - `Domain`
   - 面板 HTTPS 证书和密钥路径（如果启用了面板 HTTPS）
3. 如果计划提供订阅，进入 `Settings` -> `Subscription`，确认：
   - `Address`
   - `Port`
   - `Path`
   - `Domain`
   - `Subscription URI`
4. 只有在你确实要修改时再保存。
5. 如果界面提示需要重启，执行 `Restart App`。

面板中后续会频繁用到的区域：

- `Inbounds`：创建和修改入站监听。
- `TLS Settings`：创建可复用的 TLS 模板和 Reality 预设。
- `Settings`：配置面板地址、面板路径、面板 TLS 文件、订阅暴露参数等。

面板外的第一轮检查：

- `systemctl status b-ui`
- 确认服务处于活动状态
- 确认 `/usr/local/s-ui/db/b-ui.db` 已存在

## 3. 最快创建一个代理站点

推荐首次使用：`VLESS + TLS`。

这是最快、最稳妥、最适合作为第一次成功路径的组合：一个 TLS 模板、一个 VLESS 入站、一次客户端验证。

前置条件：

- `b-ui` 已安装且 `b-ui` 服务正常运行。
- 你有一个已经解析到服务器的公网域名。
- `443` 端口已对外放行。
- 你已经准备好了证书和密钥，或者打算在 `TLS Settings` 中通过 ACME 申请证书。

操作顺序：

1. 打开面板并登录。
2. 进入 `TLS Settings`。
3. 点击 `Add`，或使用内置 `TLS` 预设按钮。
4. 创建一个 TLS 模板。
5. 进入 `Inbounds`。
6. 新建一个 `vless` 入站，并绑定刚刚创建的 TLS 模板。
7. 添加客户端。
8. 复制节点或订阅信息到客户端。
9. 做一次基础连通性验证。

如果你只想先跑通，请先完成本节，再去看后面的字段细节章节。

## 4. TLS 设置

`TLS Settings` 用来定义可复用的 TLS 模板。入站通过 `tls_id` 绑定这些模板。绑定后，客户端导出配置会继承模板中的 TLS 信息，包括：

- `enabled`
- `server_name`
- `alpn`
- `min_version`
- `max_version`
- 证书材料
- cipher suites
- 可选的 Reality 或 ECH 设置

面板路径：

- `TLS Settings` -> `Add`
- `TLS Settings` -> 内置预设按钮：`TLS`、`Hysteria2`、`Reality`
- 从入站内快速创建：`TLS` 卡片 -> `Quick Create`

最重要的字段：

- `Name`：面向运维的模板名称。
- `Domain candidates` / `server_name`：客户端 SNI 使用的域名。
- `ALPN`：协商的应用层协议。
- `Min Version` / `Max Version`：TLS 版本范围。
- 证书来源：证书路径/密钥路径，或直接粘贴证书和私钥。
- `disable_sni`、`insecure`、客户端指纹选项：面向客户端兼容性的设置。

Reality 相关字段：

- 握手目标 `server`
- 握手目标 `server_port`
- `private_key`
- `public_key`
- `short_id`
- 可选的 `max_time_difference`

ACME 说明：

- ACME 可以直接在 TLS 模板中启用。
- 你可以配置域名、provider、挑战方式、备用端口、EAB、DNS-01 provider 等。
- 当服务器能够完成挑战流程时，优先使用 ACME 自动签发。

运维建议：

- 标准 `VLESS + TLS` 优先使用普通 TLS 模板。
- `Hysteria2` 优先使用内置 `Hysteria2` 预设，让 `ALPN` 固定为 `h3`，TLS 固定为 `1.3`。
- `VLESS + Reality` 只在确定客户端支持 Reality 时使用。
- 不要删除仍被活动入站引用的 TLS 模板。

验证方法：

- 在 `TLS Settings` 里确认卡片显示正确的 `server_name`。
- 确认 `ACME`、`ECH`、`Reality` 标记符合预期。
- 确认模板卡片上的入站数量与实际绑定的入站一致。

## 5. 入站设置

`Inbounds` 用来定义监听器。服务器侧表单控制本地监听行为，客户端侧视图则由入站字段和绑定的 TLS 模板共同生成，并用于导出链接和订阅。

面板路径：

- `Inbounds` -> `Add`
- `Inbounds` -> 打开已有卡片 -> `Edit`

最常用的字段：

- `Type`：协议类型，如 `vless`、`hysteria2`
- `Tag`：全局唯一标识
- `Listen`：本地监听地址
- `Listen Port`：本地监听端口；它会成为导出客户端视图中的 `server_port`
- `Clients` / `客户端`：该协议使用的客户端绑定
- `TLS`：绑定的 TLS 模板
- `Transport`：协议支持时可选的传输封装
- `Multiplex`：可选的连接复用能力
- 客户端侧 `Addr` 列表：导出链接时使用的公网地址列表

关键逻辑：

- `vless`、`trojan`、`vmess` 会把 `transport` 复制到导出结果中。
- `hysteria2` 导出时会把带宽字段映射为客户端侧的 `up_mbps` / `down_mbps`，并可带上 `obfs`。
- 如果入站没有绑定 TLS 模板，导出结果中的 `tls` 字段会被删除。
- 对于基于 TLS 的协议，导出结果里的 `server_name`、`alpn`、证书信息和 Reality 参数全部来自绑定的 TLS 模板。

验证方法：

- 保存入站后重新打开它。
- 确认 `Tag`、`Listen`、`Listen Port` 已正确持久化。
- 查看客户端侧视图，确认 `type`、`server`、`server_port`、`tls`、`transport` 与你的预期一致。

## 6. 协议示例

### 6.1 VLESS + TLS

#### 什么时候用它

当你想要一条默认、通用、兼容性强的代理站点路径时，用 `VLESS + TLS`。

#### 需要先准备什么

- 一个可用的 TLS 模板
- 一个公网域名
- 证书和密钥，或可用的 ACME 签发条件

#### 面板里先点哪里

- `TLS Settings` -> 创建或确认标准 TLS 模板
- `Inbounds` -> `Add` -> 选择 `vless`

#### 哪些字段必须填

- TLS 模板中：
  - `Name`
  - `server_name`
  - 证书和密钥
- VLESS 入站中：
  - `Tag`
  - `Listen`
  - `Listen Port`
  - `UUID`
  - `TLS`

建议字段：

- `Flow` 除非客户端明确要求，否则留空
- `Transport` 只有在确实需要 HTTP、WebSocket、gRPC、HTTPUpgrade 时再启用
- 如果公网主机名和本地绑定地址不同，在客户端侧 `Addr` 中补充对外地址

#### 创建完后如何验证

- 确认入站出现在 `Inbounds` 列表里。
- 确认 TLS 模板卡片上显示该入站已绑定。
- 用客户端测试生成的 VLESS 链接。
- 检查导出的客户端链接或订阅内容，确认其中保留了 `uuid`、`flow`、`security=tls`、`transport` 等预期字段。

### 6.2 Hysteria2

#### 什么时候用它

当你希望使用基于 QUIC、适合高延迟或不稳定网络的协议时，用 `Hysteria2`。

#### 需要先准备什么

- 一个适合 `Hysteria2` 的 TLS 模板，优先使用内置 `Hysteria2` 预设
- 对外可达的 UDP 端口
- 客户端使用的密码

#### 面板里先点哪里

- `TLS Settings` -> 内置 `Hysteria2` 预设或 `Add`
- `Inbounds` -> `Add` -> 选择 `hysteria2`

#### 哪些字段必须填

- TLS 模板中：
  - `Name`
  - `server_name`
  - 证书和密钥，或等价的 ACME 配置
- Hysteria2 入站中：
  - `Tag`
  - `Listen`
  - `Listen Port`
  - `Password`
  - `TLS`
  - 带宽参数，除非启用了 `Ignore Client Bandwidth`

可选字段：

- `Obfs` 与 salamander 密码
- `Masquerade`
- 客户端侧多端口跳跃相关字段

#### 创建完后如何验证

- 确认入站保存后 `tls_id` 不为空。
- 确认绑定的 TLS 模板没有误开 Reality。
- 在 Hysteria2 客户端中测试导出的密码和主机名。
- 检查导出的 Hysteria2 客户端配置或链接内容，确认其中包含 `password`、`up_mbps`、`down_mbps`，以及你实际启用的 `obfs`、多端口或 `fastopen` 等字段。

### 6.3 VLESS + Reality

#### 什么时候用它

当你需要使用 Reality，而不是普通证书型 TLS，并且客户端已经支持 Reality 时，用 `VLESS + Reality`。

#### 需要先准备什么

- 一个握手目标域名和端口，通常是已知 TLS 站点的 `443`
- Reality-capable 客户端
- 已生成密钥材料的 Reality TLS 模板

#### 面板里先点哪里

- `TLS Settings` -> 内置 `Reality` 预设或 `Add`
- 在 TLS 表单中切换到 `Reality`
- `Inbounds` -> `Add` -> 选择 `vless`

#### 哪些字段必须填

- Reality 模板中：
  - `Name`
  - 握手 `server`
  - 握手 `server_port`
  - `private_key`
  - `public_key`
  - `short_id`
- VLESS 入站中：
  - `Tag`
  - `Listen`
  - `Listen Port`
  - `UUID`
  - `TLS` 指向 Reality 模板

#### 创建完后如何验证

- 确认 TLS 卡片显示 `Reality: yes`。
- 确认 VLESS 入站已绑定该模板。
- 用支持 Reality 的客户端测试生成的链接。
- 检查导出的客户端配置或链接内容，确认其中带有 `security=reality`，并包含预期的 TLS Reality 参数。

## 7. 更新与强制更新

### 普通更新

推荐命令：

```sh
b-ui update
```

脚本等价命令：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --update
```

行为规则：

- `--update` 只在已安装 `b-ui` 的情况下工作。
- 如果未安装 `b-ui`，脚本会退出并提示先执行安装。
- 如果系统里只有上游 `s-ui`，脚本会退出并提示先迁移。
- 如果当前版本已等于或高于目标版本，脚本会退出并提示改用 `--force-update`。
- 普通更新保留现有设置和凭据。
- 普通更新不会执行旧版 `s-ui` 迁移逻辑。

### 强制更新

推荐命令：

```sh
b-ui update --force
```

脚本等价命令：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update
```

当你需要在版本相同的情况下重新安装目标版本时，使用强制更新。

### 更新过程中的备份与回滚

当系统里已存在安装时，脚本会创建回滚备份：

```text
/var/backups/s-ui/<timestamp>/
```

备份内容可能包括：

- 安装根目录归档 `install-root.tar.gz`
- 当前 `b-ui` CLI
- 仍然存在时的旧 `s-ui` CLI
- 当前 systemd 单元文件
- `/usr/local/s-ui/db/` 下的数据库文件

如果新服务启动失败并无法保持活动状态，安装脚本会自动回滚并尝试拉起旧服务。

## 8. 从上游迁移

当服务器已经安装了上游 `s-ui`，并且你希望在保留数据的前提下原地替换为 `b-ui` 时，使用迁移。

运行：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate
```

指定版本迁移：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate v0.0.1
```

迁移会做什么：

- 检测兼容的上游安装
- 停止旧 `s-ui` 服务
- 在 `/var/backups/s-ui/<timestamp>/` 创建回滚备份
- 用 `b-ui` 替换 `/usr/local/s-ui` 下的程序文件
- 执行 `sui migrate`
- 在需要时把 `s-ui.db` 迁移为 `b-ui.db`
- 把服务名切换为 `b-ui`
- 把管理命令切换为 `b-ui`

验证方法：

- `systemctl status b-ui`
- 确认 `/usr/local/s-ui/db/b-ui.db` 已存在
- 确认面板中的原设置、凭据、入站和其他持久化数据都仍然存在

## 9. 功能逻辑与基本使用

日常运维流程通常是：

1. 如果协议需要 TLS，先在 `TLS Settings` 中定义或复用 TLS 模板。
2. 在 `Inbounds` 中创建入站。
3. 绑定用户，以及可选的传输或复用设置。
4. 查看客户端侧导出视图和导出链接。
5. 在 `Settings` 中调整面板 URI、面板 TLS 文件、订阅地址、订阅路径和订阅暴露方式。

需要理解的核心逻辑：

- `TLS Settings` 是可复用基础设施；多个入站可以指向同一个模板。
- 客户端导出 JSON 是由入站服务器字段和绑定的 TLS 字段共同生成的，不是凭空拼接。
- 导出结果中的 `server_port` 来自入站的 `listen_port`。
- `VLESS`、`Trojan`、`VMess` 会把 `transport` 保留到客户端导出结果中。
- `Hysteria2` 会把 `obfs` 和客户端带宽字段保留到导出结果中。
- Reality 模板在存在多个 `short_id` 时，会为导出客户端结果随机选择一个。
- `Settings` 管的是面板和订阅的暴露方式，不等于协议入站监听本身。

`Settings` 中最常用的区域：

- `Interface`：面板监听地址、端口、Base URI、域名、面板 TLS 文件、时区、TLS 域名提示
- `Subscription`：订阅地址、端口、路径、域名、URI、可选证书文件
- 修改后根据提示执行 `Restart App`

做完任何改动后的基础验证：

- 保存表单
- 重新打开对象
- 确认列表页卡片值符合预期
- 在批量推广前，先测试一条导出的客户端链接或一次订阅导入

## 10. 基本排错

- 安装命令在下载前失败：检查 root 权限和到 GitHub 的出站连通性。
- 更新拒绝执行：确认系统里安装的是 `b-ui`；如果只有上游 `s-ui`，请先迁移。
- 服务启动后又回滚：检查 `systemctl status b-ui`、`journalctl -u b-ui` 和 `/var/backups/s-ui/` 下的回滚备份。
- 全新安装后不知道登录凭据：如果跳过了交互式设置，安装脚本会打印随机管理员凭据。
- 修改接口设置后面板地址不对：重新检查 `Settings` -> `Interface`，保存后重启应用。
- TLS 握手失败：检查 `server_name`、证书路径或证书内容，以及入站是否绑定了正确的 TLS 模板。
- `VLESS + TLS` 失败：检查端口开放、域名解析、`UUID` 和可选 `transport`。
- `Hysteria2` 失败：检查 UDP 放行、TLS 预设、密码或 `obfs`，以及是否错误省略了带宽参数。
- `VLESS + Reality` 失败：检查握手目标、`public_key`、`short_id` 和客户端的 Reality 支持。
