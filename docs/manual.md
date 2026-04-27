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
- 只有在全新安装时拒绝继续后续交互式修改流程，脚本才会生成随机管理员凭据并打印一次。

### 非交互式安装（带参数）

支持通过命令行参数直接完成安装，无需交互式输入：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) \
  --user admin \
  --pwd mypassword \
  --panel-port 8080 \
  --panel-path /admin/ \
  --sub-port 8081 \
  --sub-path /sub/
```

**可用参数：**

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--user` | 管理员用户名 | 不提供则生成随机凭据 |
| `--pwd` | 管理员密码 | 不提供则生成随机凭据 |
| `--panel-port` | 面板端口 | `2095` |
| `--panel-path` | 面板安全入口路径 | `/app/` |
| `--sub-port` | 订阅端口 | `2096` |
| `--sub-path` | 订阅入口路径 | `/sub/` |
| `--domain` | 面板域名 | 不提供则使用 IP 模式 |
| `--cert-path` | SSL 证书文件路径 | 需与 `--key-path` 同时提供 |
| `--key-path` | SSL 私钥文件路径 | 需与 `--cert-path` 同时提供 |
| `--acme-port` | ACME 证书申请验证端口 | `80` |

**域名与证书行为：**

- **不提供 `--domain`**：使用 IP 模式，面板通过 `http://<IP>:<port><path>` 访问。
- **提供 `--domain` 但不提供 `--cert-path` / `--key-path`**：安装脚本自动使用 ACME（acme.sh + Let's Encrypt）申请证书并启用自动续期。证书安装到 `/root/cert/<domain>/`，面板配置自动回填证书路径。
- **同时提供 `--domain`、`--cert-path`、`--key-path`**：使用你已有的证书文件，直接写入面板配置。脚本会校验文件是否存在。
- **只提供 `--cert-path` 或只提供 `--key-path` 中的一个**：脚本报错退出，要求两者必须同时提供。

**BBR 优化：**

- 全新安装时默认自动启用 BBR TCP 拥塞控制优化（`net.core.default_qdisc=fq` + `net.ipv4.tcp_congestion_control=bbr`）。
- 如果系统已启用 BBR 则跳过。
- 不影响 `--update` / `--force-update` 流程。

**参数对 `--migrate` 也生效：**

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate \
  --domain example.com \
  --panel-port 443
```

迁移时现有设置会被保留，但提供的参数会作为覆盖应用到对应设置字段。

### Docker 部署引导

Docker 模式默认使用官方 GHCR 镜像 `ghcr.io/beanya/b-ui:latest`。入口脚本是仓库内的 `scripts/release/install-docker.sh`。

最小示例：

```sh
bash ./scripts/release/install-docker.sh
```

运行前要求：

- 宿主机已安装 `docker` 和 `docker compose`
- 当前 shell 能执行 `curl`

如果你需要使用指定版本、fork 镜像、私有 registry，或通过 digest 固定部署内容，可以用 `IMAGE_REF` 覆盖默认镜像：

```sh
IMAGE_REF=ghcr.io/beanya/b-ui:v0.1.14 bash ./scripts/release/install-docker.sh
```

也可以替换为你自己的镜像引用，例如：

```sh
IMAGE_REF=registry.example.com/ops/b-ui@sha256:<digest> bash ./scripts/release/install-docker.sh
```

交互式流程会按顺序收集：

1. 面板端口和面板路径
2. 订阅端口和订阅路径
3. 管理员用户名和密码
4. 可选协议引导类型：跳过、`VLESS + TLS`、`VLESS + Reality`、`Hysteria2`
5. 选中协议后对应的最小引导字段，例如客户端名称、UUID 或密码、监听端口，以及 TLS / Reality 所需参数

脚本完成后会依次执行：

1. 检查 `docker`、`docker compose`、`curl`
2. 生成部署文件
3. `docker compose up -d`
4. 等待面板就绪
5. 写入基础面板设置和管理员凭据
6. 登录面板 API
7. 如果你选择了协议引导，则自动创建 TLS 模板、客户端和入站
8. 输出最终面板地址和 compose 文件路径

默认访问模型：

- 面板默认是直接 `http://<server-ip>:<panel-port><panel-path>` 访问
- 订阅默认是直接 `http://<server-ip>:<sub-port><sub-path>` 暴露
- 这套引导不依赖宿主机 Nginx，也不包含额外反向代理层
- 脚本只用本机 `127.0.0.1` 轮询面板就绪，但给操作者的访问方式仍应理解为服务器 `IP:port`

生成的文件和目录：

- `deploy/docker-compose.yml`：最终 compose 文件
- `deploy/db/`：数据库持久化目录，会挂载到容器内 `/app/db`
- `deploy/cert/`：证书持久化目录，会挂载到容器内 `/app/cert`
- `deploy/panel-cookie.jar`：脚本在引导阶段临时创建的 cookie jar，结束后会自动清理，不应依赖它作为持久化产物

证书行为边界：

- Docker 引导不会给面板自身自动申请 ACME 证书
- `VLESS + TLS` 和 `Hysteria2` 都属于证书型 TLS 引导
- 当交互里把 TLS `server_name` 留空时，引导会走面板当前可生成的自签名材料路径，适合先把面板和协议跑通
- 如果你已经有证书和私钥，可以把文件放到宿主机 `deploy/cert/`，它们会出现在容器内 `/app/cert/`，随后到面板 `TLS Settings` 中把模板切换到这些挂载路径
- 为了兼容首次自签名或证书尚未完全就绪的场景，`VLESS + TLS` 和 `Hysteria2` 自动生成的客户端侧 TLS 会先保留 `insecure: true`；当你确认已经换成可信证书后，应该回到面板里把对应客户端或模板收紧
- `VLESS + Reality` 不使用普通证书链，而是通过 Reality 握手目标、密钥和 `short_id` 建立模板

协议引导选项：

- `VLESS + TLS`：创建标准 TLS 模板、VLESS 客户端和 VLESS 入站；需要客户端名称、UUID、监听端口，可选 `server_name`
- `VLESS + Reality`：创建 Reality 模板、VLESS 客户端和 VLESS 入站；需要客户端名称、UUID、监听端口、握手目标域名、握手目标端口、`short_id`
- `Hysteria2`：创建 Hysteria2 TLS 模板、Hysteria2 客户端和 Hysteria2 入站；需要客户端名称、密码、监听端口，可选 `server_name`
- `跳过`：只完成面板和 compose 部署，不自动创建协议对象

部署后的第一轮检查：

1. 打开脚本输出的 `Panel URL`
2. 用交互阶段设置的管理员用户名和密码登录
3. 确认 `deploy/docker-compose.yml` 已生成且 `docker compose -f deploy/docker-compose.yml ps` 显示容器在运行
4. 如果引导了协议，进入 `TLS Settings`、`Clients`、`Inbounds` 检查自动创建的对象是否存在
5. 如果后续改成可信证书，再回到 `TLS Settings` 更新模板并重测客户端

## 2. 首次登录与初始化

这个阶段的目标是确认面板可访问、凭据可用、以及基础参数处于可继续配置站点的状态。

凭据来源：

- 如果你在安装过程中主动修改过用户名和密码，使用你填写的值。
- 如果你在全新安装时直接拒绝继续后续交互式修改流程，使用安装脚本打印的随机管理员凭据。
- 如果这是已有 `b-ui` 的更新，现有凭据会被保留。
- 如果这是从上游 `s-ui` 迁移，现有凭据和数据库内容会被保留。

首次登录步骤：

1. 打开安装脚本输出的面板地址。
2. 用当前管理员用户名和密码登录。
3. 如果是全新安装且当时拒绝了后续交互式修改流程，无法登录时重新检查安装输出中的随机凭据。

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

这是最快、最稳妥、最适合作为第一次成功路径的组合：一个 TLS 模板、一个客户端、一个 VLESS 入站、一次客户端验证。

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
5. 进入 `Clients`。
6. 新建一个客户端，先保存客户端的 `UUID`，如有需要再在客户端编辑器里设置 `Flow`。
7. 进入 `Inbounds`。
8. 新建一个 `vless` 入站，并绑定刚刚创建的 TLS 模板与已有客户端。
9. 复制节点或订阅信息到客户端。
10. 做一次基础连通性验证。

如果你只想先跑通，请先完成本节，再去看后面的字段细节章节。

## 4. TLS 设置

`TLS Settings` 用来定义可复用的 TLS 模板。入站通过 `tls_id` 绑定这些模板。绑定后，客户端导出 JSON 会组合入站字段和模板中的 TLS 信息；生成的客户端链接只会带上该协议链接格式实际支持的那部分字段。

- JSON 视图中常见的 TLS 字段包括 `enabled`、`server_name`、`alpn`、`min_version`、`max_version`、证书相关字段，以及可选的 Reality 或 ECH 设置。
- 生成链接时通常只会携带协议链接本身支持的字段；不要假设链接一定会包含 `min_version`、`max_version`、证书材料、`cipher suites` 或 `ECH`。

面板路径：

- `TLS Settings` -> `Add`
- `TLS Settings` -> 内置预设按钮：`TLS`、`Hysteria2`、`Reality`
- 在入站编辑界面中选择一个已存在的 TLS 模板

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
- `hysteria2` 在导出到客户端 JSON 时会把服务端带宽字段转换成客户端侧的 `up_mbps` / `down_mbps`；生成链接时则映射为 `upmbps` / `downmbps`，并可带上 `obfs`。
- 如果入站没有绑定 TLS 模板，导出结果中的 `tls` 字段会被删除。
- 对于基于 TLS 的协议，客户端导出 JSON 中的 `server_name`、`alpn` 以及可用的 Reality 等 TLS 参数来自绑定的 TLS 模板；链接里只会出现协议链接格式实际支持的那部分字段。

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
- 一个已经在 `Clients` 中创建好的客户端

#### 面板里先点哪里

- `TLS Settings` -> 创建或确认标准 TLS 模板
- `Clients` -> 创建或确认一个客户端
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
  - `TLS`
  - 绑定已有客户端

建议字段：

- `UUID` 不在入站表单中填写；先到 `Clients` 页面或客户端编辑器中创建客户端
- `Flow` 也在客户端编辑器中配置；当前新建 VLESS 客户端通常默认会带上 `xtls-rprx-vision`，只有在客户端或部署目标明确不需要时再手动调整
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
- 一个已经在 `Clients` 中创建好的客户端，密码在该客户端里配置

#### 面板里先点哪里

- `TLS Settings` -> 内置 `Hysteria2` 预设或 `Add`
- `Clients` -> 创建或确认一个客户端
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
  - `TLS`
  - 带宽参数，除非启用了 `Ignore Client Bandwidth`
  - 绑定已有客户端

可选字段：

- `Password` 不在服务端入站表单中填写；先在客户端编辑器里为该客户端设置密码
- `Obfs` 与 salamander 密码
- `Masquerade`
- 客户端侧多端口跳跃相关字段

#### 创建完后如何验证

- 确认入站保存后 `tls_id` 不为空。
- 确认绑定的 TLS 模板没有误开 Reality。
- 在 Hysteria2 客户端中测试导出的密码和主机名。
- 检查导出的 Hysteria2 客户端配置或链接内容：JSON 里确认 `password`、`up_mbps`、`down_mbps`；链接里确认 `password`、`upmbps`、`downmbps`，并注意它们是由服务端带宽字段换位映射过来的，同时确认你实际启用的 `obfs`、多端口或 `fastopen` 等字段。

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
  - `TLS` 指向 Reality 模板
  - 绑定已有客户端

补充说明：

- `UUID` 不在 VLESS 入站表单中填写；请先在 `Clients` 页面或客户端编辑器中创建客户端。
- 如果需要 `Flow`，同样在客户端编辑器中配置，而不是在入站表单中配置。

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

一行迁移命令：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate
```

指定版本迁移：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate v0.0.1
```

`--migrate` 的实际步骤：

- 检测兼容的上游安装。
- 停止旧 `s-ui` 服务。
- 在 `/var/backups/s-ui/<timestamp>/` 创建回滚备份。
- 从 `BeanYa/b-ui` 下载目标 release；当前 Linux 资源名为 `b-ui-linux-<arch>.tar.gz`。
- 原地替换已安装的二进制和 shell 脚本。
- 执行 `sui migrate`；如果系统里只有旧 `s-ui.db`，会先把它迁移到 `b-ui.db`。
- 把 systemd 服务名从 `s-ui` 切换为 `b-ui`。
- 把管理命令从 `s-ui` 切换为 `b-ui`。
- 启动并启用 `b-ui` 服务。

迁移过程中会保留的内容：

- 现有面板设置和管理员凭据会被保留。
- 现有入站、出站、端口和其他持久化面板数据会继续保留。
- 当只有旧数据库存在时，这些数据会自动从 `s-ui.db` 迁移到 `b-ui.db`。

回滚行为：

- 如果新版本启动失败，安装脚本会自动从回滚备份恢复之前的安装。
- 自动回滚后会尝试重新拉起旧服务，避免迁移后服务直接中断。

迁移后的更新模式：

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --update
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update
```

- 迁移完成后，后续更新会指向这个 fork，而不是上游仓库。
- `--update` 只在已安装 `b-ui` 且当前版本低于目标版本时执行。
- `--force-update` 会在版本相同的情况下也重新安装目标版本。
- 两种模式都可以附带版本号，例如 `--update v0.0.1`。
- 如果系统里还没有 `b-ui`，更新模式会提示先安装。
- 如果系统里只有上游 `s-ui`，更新模式会提示先迁移。
- 如果当前版本已是目标版本或更高版本，普通更新会提示改用 `--force-update`。
- 普通 `b-ui` 更新不会再执行旧版 `s-ui` 的迁移流程。

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

### 9.1 WebTerminal

`WebTerminal` 是一个面向管理员的浏览器内交互式终端，用来直接连接服务器本地 shell，做运行状态检查、快速诊断和只读/轻量命令操作。

入口与权限：

- 路径是 `/webterminal`
- 只有管理员可用；后端会再次校验权限，前端展示不是唯一安全边界
- 页面默认不会自动连接终端

连接行为：

1. 打开 `WebTerminal` 页面后，下方终端卡片会先显示半透明遮罩。
2. 点击遮罩中的 `Connect`，或工具栏中的 `Connect` 按钮。
3. 确认后，浏览器才会真正发起 WebSocket 连接并打开终端会话。
4. 连接建立后，可以直接在终端区域进行实时键盘输入，而不是通过单独的命令输入框逐条提交。

当前支持的交互能力：

- 光标显示与实时键盘输入
- 流式终端输出
- 终端窗口尺寸变化同步到后端
- 常见 ANSI / TTY 输出渲染
- 在同一管理界面内完成诊断，不需要额外 SSH 客户端

离开页面时的行为：

- 如果当前终端仍处于连接或激活状态，切换路由时会弹出确认框
- 刷新页面、关闭标签页、后退离开时，浏览器也会给出离开提醒
- 确认离开后，当前 WebTerminal 会话会被主动中断，正在执行的命令或交互任务可能被打断

使用建议：

- 适合执行 `pwd`、`ls`、`systemctl status b-ui`、日志观察、配置文件快速检查等轻量运维操作
- 如果你要执行长时间任务、交互式全屏程序或需要会话持久化的操作，优先使用 SSH、tmux、screen 等传统方式
- WebTerminal 更适合作为面板内诊断入口，而不是长期驻留终端

做完任何改动后的基础验证：

- 保存表单
- 重新打开对象
- 确认列表页卡片值符合预期
- 在批量推广前，先测试一条导出的客户端链接或一次订阅导入

### 9.2 面板自更新

Dashboard 面板提供内置自更新检测入口，可直接在面板界面中检查更新并触发升级流程：

- 进入 Dashboard 后，如果面板检测到有新的可用版本，会显示更新提示
- 点击更新按钮后，面板会通过后端触发标准更新流程，等价于执行 `b-ui update`
- 更新过程包括下载新版本、替换二进制、重启服务等步骤
- 更新完成后面板页面会自动刷新

自更新覆盖以下场景：

- 版本低于目标版本时的普通更新
- 如果已是最新版本，面板不会触发更新
- 面板内更新依赖于后端更新能力，确保与命令行 `b-ui update` 行为一致

## 10. 集群管控（Cluster Center）

`Cluster Center` 是面向多节点集群场景的管理页面，用来在单一面板内注册 Hub 域、查看集群成员和执行同步操作。

### 10.1 入口与前提

- 面板侧栏导航中的 `Cluster Center` 入口
- 当前节点需要能够访问目标 Hub 的面板 API

### 10.2 注册新域

点击 `Register` 打开注册对话框，支持两种输入方式：

**方式一：Join URI（推荐）**

在 `Join URI` 字段中粘贴 `buihub://` 格式的统一注册链接：

```
buihub://hub.example.com/domain/example.com?token=xxxx
```

输入后前端会自动解析并填充以下字段：

- `Domain`：解析自 URI 路径
- `Hub URL`：解析自 URI 主机部分，默认使用 https 协议
- `Token`：解析自查询参数

解析完成后 Join URI 字段会自动清空，表示已成功解析。

**方式二：手动填写**

如果只有分散信息，也可以逐一填写：

| 字段 | 说明 |
|------|------|
| `Domain` | 要注册的 Hub 域名称 |
| `Hub URL` | 协议选择下拉（https/http）+ 主机地址 |
| `Token` | Hub 域注册令牌（密码输入框） |

两个方式都需要的额外参数：

- `Base URL`：当前面板的地址，由前端自动基于浏览器当前 URL 解析，无需手动填写

### 10.3 域列表与成员查看

注册成功后：

- 左侧域列表会显示所有已注册的 Hub 域，每张卡片展示域名、版本号、成员数量
- 点击某个域，右侧会显示该域下的成员节点表格
- 表格列：`Node ID`、`Name`、`Base URL`、`Version`、操作

### 10.4 成员管理

- **查看成员**：选中域后自动显示其成员列表
- **删除成员**：点击成员行末的删除按钮，确认后从集群中移除该节点
- **手动同步**：点击 `Manual Sync` 按钮强制同步当前节点与 Hub 的数据，适用于自动同步未及时触发时的手动补充

### 10.5 注册流程状态

注册请求提交后，前端会轮询操作状态（`api/cluster/operations/:id`），直到操作完成：

- 轮询间隔递增：0ms → 300ms → 700ms → 1500ms → 3000ms
- 状态为 `completed` 时结束轮询并刷新数据
- 注册成功后自动关闭对话框并清空表单

### 10.6 使用建议

- 一个节点可以注册到多个 Hub 域
- `buihub://` URI 是最快捷的注册方式，优先使用
- 注册令牌具有时效性，请在使用有效期内完成注册
- 手动同步在自动同步正常时通常不需要执行，仅在数据不一致时使用

## 11. 基本排错

- 安装命令在下载前失败：检查 root 权限和到 GitHub 的出站连通性。
- 更新拒绝执行：确认系统里安装的是 `b-ui`；如果只有上游 `s-ui`，请先迁移。
- 服务启动后又回滚：检查 `systemctl status b-ui`、`journalctl -u b-ui` 和 `/var/backups/s-ui/` 下的回滚备份。
- 全新安装后不知道登录凭据：只有在拒绝继续后续交互式修改流程时，安装脚本才会打印随机管理员凭据。
- 修改接口设置后面板地址不对：重新检查 `Settings` -> `Interface`，保存后重启应用。
- TLS 握手失败：检查 `server_name`、证书路径或证书内容，以及入站是否绑定了正确的 TLS 模板。
- `VLESS + TLS` 失败：检查端口开放、域名解析、`UUID` 和可选 `transport`。
- `Hysteria2` 失败：检查 UDP 放行、TLS 预设、密码或 `obfs`，以及是否错误省略了带宽参数。
- `VLESS + Reality` 失败：检查握手目标、`public_key`、`short_id` 和客户端的 Reality 支持。
- `WebTerminal` 页面无法进入：先确认当前登录用户是管理员；如果只是前端菜单可见性异常，后端权限校验仍会拒绝非管理员连接。
- `WebTerminal` 一直停在 disconnected：先确认已经点击 `Connect` 并完成确认；如果仍无法连接，再检查浏览器到面板的 WebSocket 路径、反向代理升级头以及面板服务状态。
