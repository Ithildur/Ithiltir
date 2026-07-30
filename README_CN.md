# Ithiltir Dash [![release](https://img.shields.io/github/v/release/Ithildur/Ithiltir?label=release)](https://github.com/Ithildur/Ithiltir/releases) [![downloads](https://img.shields.io/github/downloads/Ithildur/Ithiltir/total?label=downloads)](https://github.com/Ithildur/Ithiltir/releases) [![license](https://img.shields.io/github/license/Ithildur/Ithiltir)](LICENSE)

Ithiltir Dash 是单实例、自托管的服务器监控面板。一个 Dash 进程同时提供 Web UI、HTTP API、主题资源、安装脚本和节点二进制分发入口。

**资源：** [English](README.md) · [在线文档](https://www.ithiltir.dev/) · [架构](docs/architecture_CN.md) · [API](docs/api_CN.md) · [破坏性变更](docs/breaking-changes_CN.md) · [节点端](https://github.com/Ithildur/Ithiltir-node)

## 界面预览

![Ithiltir Control 看板](screenshot/zh.jpg)

## 功能范围

- 实时节点看板、在线率和历史指标
- 节点搜索、分组过滤和游客可见范围控制
- PVE / Proxmox VE 宿主机适配
- RAID 状态嗅探和失效告警
- SMART 状态、NVMe critical warning、SMART 温度和普通温度传感器运行时字段
- 流量统计、月度周期和 95 计费数据
- 节点、分组、告警、主题和系统设置管理
- Agent 指标、静态信息和打包更新下发
- 内置 Linux / macOS / Windows Agent 安装脚本
- 单进程交付 SPA、API、管理台和部署资产

## 运行要求

- PostgreSQL 16+ 和按相同 PostgreSQL 主版本构建的 TimescaleDB
- Redis 默认持久化管理员会话，并保存可丢弃的前台缓存；`--no-redis` 把两者都放入进程内存。告警运行态和 MTProto 登录握手始终在内存中，重启后重置
- 从源码运行或打包需要 Go 1.26+
- 构建前端需要 Bun 1.3.11

## 快速启动

1. 复制 `configs/config.example.yaml` 为 `config.local.yaml`。
2. 替换 `config.local.yaml` 里的 `__...__` 占位符。
3. 设置管理员密码环境变量 `monitor_dash_pwd`。
4. 执行数据库迁移。
5. 启动服务。

```bash
cp configs/config.example.yaml config.local.yaml
export monitor_dash_pwd='<password>'
go run ./cmd/dash migrate -config config.local.yaml
go run ./cmd/dash -debug
```

启动后，访问 `app.public_url` 打开看板，访问 `app.public_url + "/login"` 进入管理台。`app.public_url` 必须是根路径 URL，不支持 `/dash` 这类路径前缀。IP 部署允许使用 HTTP；网络边界支持时仍建议使用 HTTPS。

## 配置

最小运行字段：

- `app.listen`
- `app.public_url`
- `database.user`
- `database.name`
- `redis.addr`
- `auth.jwt_signing_key`

管理员登录密码只从环境变量 `monitor_dash_pwd` 读取，不写入配置文件。
`auth.jwt_signing_key` 至少为 32 字节，且不能带首尾空白。Linux 安装器会生成 32 字节的随机字母数字密钥。
受支持的环境变量只要存在，就会覆盖 YAML，即使其值为空。必填字段会在后续校验中拒绝空值；可选字段按各自的空值/默认语义处理；凭据字段可以被显式清空。整数环境变量为空时表示 0；非空但无法解析为整数时属于配置错误。
数据库连接池数值必须非负；`database.max_open_conns` 为正数时不得小于 `database.max_idle_conns`，零值保留既有默认/不限语义。`database.conn_max_lifetime` 不得为负，零表示不按连接年龄淘汰。
Redis 配置和 `REDIS_*` 覆盖只在启用 Redis 模式时加载。连接池数值必须非负；`redis.pool_size=0` 使用 go-redis 默认值，此时要求 `redis.min_idle_conns=0`；pool size 为正数时不得小于最小空闲连接数。`--no-redis` 和迁移命令不会读取或校验 Redis 专属配置。
`redis.username` 和 `redis.password` 是可选鉴权字段。Linux 安装器会询问可选 Redis 密码，用该密码校验端点且不把密码放入进程参数，并只将其写入 root 可读的本地配置。

`app.timezone` 可选。空值使用本地时区；非空值必须是有效的 IANA 时区名，例如 `Asia/Shanghai` 或 `UTC`，否则启动会因配置错误停止。

配置查找顺序：

- `config.local.yaml`
- `config.yaml`
- `configs/config.local.yaml`
- `configs/config.yaml`
- `$DASH_HOME/configs/config.local.yaml`
- `$DASH_HOME/configs/config.yaml`

`database.retention_days` 可选，省略时默认 `45` 天。流量 5 分钟事实表使用独立的 `database.traffic_retention_days`，省略时取 `max(database.retention_days, 45)`。它保持可写并通过滚动保留删除；历史 95 计费值保存在月度快照中。如果需要 95 计费历史，建议设置为 `90` 或更高。

## 部署基线

- 默认部署形态：`PostgreSQL 16+ + 主版本匹配的 TimescaleDB + Redis`
- `install_dash_linux.sh` 使用带签名的 PostgreSQL/TimescaleDB 仓库，并优先通过系统包管理器安装 Redis。系统仓库没有 Redis 或版本低于 `8.2.3` 时，可选择从源码安装或升级（默认 `8.2.5`）。覆盖现有 Redis 配置或服务、停止 6379 端口监听进程前，安装器会明确确认且默认继续，并备份被覆盖的文件。切换安装前，安装器使用包内 Dash 二进制按实际配置的 `redis.addr` 和可选 `redis.password` 执行 `PING`、`INFO server` 和 `8.2.3+` 版本校验，不再以本机 `redis-server` 可执行文件推断远端服务状态；正常启动会重复同一端点校验。`--no-redis` 会跳过 Redis 连接和版本校验。
- 节点安装脚本提示语言跟随 `app.language`。
- Dash Linux 安装器只在 systemd 确实运行时注册服务。安装器要求 `pgrep`，以便切换时停止仍从已安装二进制手动启动的进程。非 systemd 主机必须显式使用 `--service-manager=none`；该模式只安装文件和手动启动脚本，不声称服务已启动或已开机自启。其受支持生命周期是在全新主机上执行一次首次安装：它不是重装、修复、回滚或版本更新入口，也不会与更新器并发执行。安装器只安装 `install_dash_linux.sh` 旁的软件包内容并校验其中的 `dash` 二进制，不会选择线上 release。首次安装后，所有版本变更都由 `/opt/Ithiltir-dash/bin/dash update` 执行；`update_dash_linux.sh` 只保留为兼容包装。受管安装包写入 `/opt/Ithiltir-dash/releases`，由单一原子 `current` 符号链接选择当前版本；`/opt/Ithiltir-dash/bin/dash` 等旧路径作为兼容别名保留，`configs`、`runtime`、`logs`、`themes` 和 `install_id` 等可变数据位于 release 目录之外。安装器中的备份和恢复路径只负责约束这一次首次安装的失败范围，不构成受支持的重复执行或降级契约。更新器用 root 所有的跨进程锁覆盖已安装状态校验到切换的完整事务。数据库迁移开始前失败时恢复切换前状态；迁移开始后绝不恢复旧二进制。持久化事务和 systemd 启动保护会让中断的迁移保持停止，直到 `dash update recover` 向前完成。安装包暂存使用 `/opt/Ithiltir-dash` 旁的 sibling 目录；首次安装的原子切换仍要求 GNU coreutils `mv`。Alpine 上还必须预先安装并启动 PostgreSQL 16+、按该 PostgreSQL 主版本构建的 TimescaleDB 和 Redis 8.2.3+。
- Linux 节点安装器正式支持 systemd 和 Alpine/OpenRC；其他 OpenRC 发行版仅尽力兼容。Alpine 执行 Bash 安装器前必须已有 `bash`、`ca-certificates`、`curl`、`coreutils`。该安装器始终覆盖受管节点 release、上报配置、服务定义和采集器；节点版本升级由独立的节点自更新路径负责。安装器先在 `/var/lib/ithiltir-node/releases` 下暂存下载的二进制并执行 `--version`，再停止已有运行方式、强制替换目标 release，并原子切换 `current`。运行用户拥有数据和 release 树，使非特权自更新器可以创建和切换 release。下载最多跟随五次指向初始主机的重定向；同协议跳转保持有效端口，只允许 HTTP 升级到 HTTPS。systemd 使用 service/timer 调度 SMART、连接数和检测到的 LVM thin-pool 采集器；OpenRC 用 `supervise-daemon` 管理节点进程，并暂用 BusyBox `crond` 每 5 分钟刷新 SMART、每分钟刷新 LVM。1 秒周期的 root 连接数 helper 不会错误降级为分钟级 cron，OpenRC 下改用节点自带统计，可能缺失容器连接数据。
- systemd 下，Linux 节点安装器会在存在 `cc`、`gcc` 或 `clang` 时编译 root 侧连接数 helper，用于完整统计主机和容器网络命名空间的 TCP/UDP 连接数。安装编译器后重新运行安装脚本即可启用。
- 推荐最小配置：`1 vCPU / 2 GB RAM / 40 GB SSD/NVMe`
- `4 GB RAM` 以下推荐启用 `SWAP`
- 反向代理必须保留同源路径：`/api`、`/theme`、`/deploy` 转发到 Dash 后端，`/` 交给 Dash SPA
- Dash 暴露到不可信网络时建议使用 HTTPS；节点安装命令和资产下载会携带节点密钥。
- 跨域后端地址需要同时配置 CORS、cookie 和 CSRF 策略

## 开发

前端单独开发：

```bash
cd web
FRONT_TEST_API=http://127.0.0.1:8080 bun run dev
```

`FRONT_TEST_API` 指向正在运行的 Dash 后端。Vite 开发服务器只代理 `/api` 和 `/theme`，前端代码仍使用同源相对路径。

常用检查：

```bash
go test ./...
cd web && bun run lint
cd web && bun run typecheck
```

依赖 PostgreSQL 或 TimescaleDB 的后端集成测试需要设置 `TEST_DATABASE_URL`。没有该环境变量时，这些测试会被跳过。完整后端测试需要使用 PostgreSQL 超级用户，或具备创建和删除临时数据库权限的角色：

```bash
TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable' go test ./...
```

## 构建和打包

构建前端：

```bash
bash scripts/build_frontend.sh --version 0.0.0-dev -o build/frontend/dist
```

前端构建会嵌入传入的 Dash 版本，使已经打开的浏览器能够发现后端升级并刷新到匹配的资源。默认值为 `0.0.0-dev`；发布打包始终传入其解析出的 Dash 版本。官方构建还要求 `dist/index.html` 和 `dist/theme-bootstrap.js` 均存在且非空，否则直接失败。CI 发布打包始终经过该构建入口；任意自定义前端产物不属于受支持的 release 输入。

构建 Linux 发布包：

```bash
bash scripts/package.sh --version 1.2.3-alpha.1 --node-version 1.2.3-alpha.1 -o release -t linux/amd64 --tar-gz
```

Dash 主控端发布包当前只面向 Linux amd64 和 Linux arm64。macOS 与 Windows 的 deploy 资产只用于 Agent。

PowerShell：

```powershell
powershell -File scripts/build_frontend.ps1 -Version 0.0.0-dev -OutDir build/frontend/dist
powershell -File scripts/package.ps1 -Version 1.2.3-alpha.1 -NodeVersion 1.2.3-alpha.1 -OutDir release -Targets linux/amd64 -Zip
```

节点版本解析：

- 省略 `--node-version` / `-NodeVersion`：从 `https://github.com/Ithildur/Ithiltir-node.git` 取最新兼容 tag。Dash 预发布构建会在 node 最新预发布和最新发布中选择更新的一个；没有 node 预发布时回退到最新发布。
- 传 `--node-local` / `-NodeLocal`：从 `deploy/node` 读取本地二进制。

更新已安装的 Linux 服务：

```bash
/opt/Ithiltir-dash/bin/dash update --check
sudo /opt/Ithiltir-dash/bin/dash update
sudo /opt/Ithiltir-dash/bin/dash update --test
sudo /opt/Ithiltir-dash/bin/dash update reinstall --test
```

默认更新到最新 release。加 `--test` 时更新到最新 prerelease。如果当前部署的是高于最新 release 的 prerelease，默认 release 更新会警告并停止。
`reinstall` 会在 Dash 版本号不变时重新安装所选通道的最新包，适合测试发布包只更新了内置 node 资产的情况。
内置手工更新器支持 systemd 和显式手动安装，不再依赖 Git、tar、curl/wget，也不再用 Shell 实现更新事务。切换前会停止所选受管服务，并停止仍从已安装二进制手动启动的 Dash 进程；手动安装更新后不会自行启动进程。管理台后台控制器仍要求 `systemd-run`。打包的 `update_dash_linux.sh` 继续接受旧参数，但只负责转交给 `dash update`。
每个 Linux release 包都使用发布格式 v1，并携带 `release.env`，其中包含 `format_version=1`、Dash 版本、打包节点版本、目标系统/架构，以及覆盖五个受支持平台/架构目标的全部七个 node/runner 资产 SHA-256；包内必须包含版本一致的 `bin/dash`、`dist/index.html`、`deploy` 下这些资产、`configs/config.example.yaml` 以及 Linux 安装/更新脚本，且必需文件不能为空。内置更新器会在停止线上服务前校验运行目录与归档边界、逐个核对内置资产摘要，并要求候选 Dash 二进制同时报告 manifest 中的 Dash 版本和打包节点版本。旧部署先通过其现有更新器升级到本代；完成该过渡后，缺少 manifest 的归档会被拒绝。暂存和恢复目录创建在 `/opt/Ithiltir-dash` 旁，与安装目录处于同一文件系统。迁移或迁移后的服务启动失败时，执行 `sudo /opt/Ithiltir-dash/bin/dash update recover`。数据库迁移只支持前向演进：`goose_db_version` 是唯一 schema 版本来源，正常启动要求它与二进制内置迁移完全一致，升级后手工启动旧二进制不受支持。
Linux 更新器把官方 GitHub release 源作为根信任源。GitHub 账号、token、仓库、workflow 或 release 权限被攻破，等价于更新源被攻破。

## 仓库结构

| 路径 | 内容 |
| --- | --- |
| `cmd/dash` | 服务、迁移、更新和主题打包入口 |
| `internal` | 后端应用代码 |
| `web` | 随应用一起打包的 SPA 前端源码 |
| `configs` | 示例配置 |
| `db/migrations` | 数据库结构变更 |
| `scripts` | 前端构建和发布打包入口 |
| `deploy/node` | 离线打包用本地节点二进制 |

## 许可证

AGPL-3.0-only。见 [LICENSE](LICENSE)。
