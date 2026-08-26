# Ithiltir Dash [![release](https://img.shields.io/github/v/release/Ithildur/Ithiltir?label=release)](https://github.com/Ithildur/Ithiltir/releases) [![downloads](https://img.shields.io/github/downloads/Ithildur/Ithiltir/total?label=downloads)](https://github.com/Ithildur/Ithiltir/releases) [![license](https://img.shields.io/github/license/Ithildur/Ithiltir)](LICENSE)

Ithiltir 是一套自托管服务器监控系统。Dash 集中接收节点上报，并在一个进程内提供看板、历史指标、流量统计、告警和管理功能。

[English](README.md) · [在线文档](https://www.ithiltir.dev/) · [快速开始](https://www.ithiltir.dev/docs/QuickStart) · [Demo](https://demo.ithiltir.dev/) · [Ithiltir-node](https://github.com/Ithildur/Ithiltir-node)

## 界面预览

![Ithiltir Dash 看板](screenshot/zh.jpg)

## 功能

- CPU、内存、磁盘、网络、温度、进程和连接数实时指标
- 基于 PostgreSQL 和 TimescaleDB 的在线率数据与指标历史
- RAID、SMART、NVMe critical warning 和 Proxmox VE 宿主机支持
- 月度流量、可配置账期和 95 计费数据
- 告警规则、事件记录，以及 Telegram、SMTP 或 Webhook 通知
- 节点分组、搜索、游客可见范围、主题和系统管理
- Linux、macOS 和 Windows Node 安装与更新

## 安装

Dash 发布包支持 Linux amd64 和 arm64。安装最新稳定版：

```bash
ARCH=amd64
curl -fL -o "Ithiltir_dash_linux_${ARCH}.tar.gz" \
  "https://github.com/Ithildur/Ithiltir/releases/latest/download/Ithiltir_dash_linux_${ARCH}.tar.gz"
tar -xzf "Ithiltir_dash_linux_${ARCH}.tar.gz"
cd Ithiltir-dash
sudo bash install_dash_linux.sh --lang zh --service-manager=systemd
```

AArch64 机器使用 `ARCH=arm64`。在受支持的 Debian/Ubuntu 系统上，安装器可以准备 PostgreSQL、TimescaleDB、Redis、数据库迁移和 systemd 服务；其他部署方式见[安装文档](https://www.ithiltir.dev/docs/Install)。

安装完成后访问配置的 `app.public_url`，在地址后加 `/login` 进入管理台。创建节点后，按管理台生成的命令安装 Node。

## 源码运行

源码开发需要 Go 1.26.6+、Bun 1.3.11、PostgreSQL 16+ 及对应主版本的 TimescaleDB。除非用 `--no-redis` 启动，否则还需要 Redis。

```bash
cp configs/config.example.yaml config.local.yaml
# 继续前请替换所有 __...__ 占位符。
export monitor_dash_pwd='<password>'
go run ./cmd/dash migrate -config config.local.yaml
go run ./cmd/dash -debug
```

管理员密码至少包含 8 个可见 ASCII 字符，且不能有空白。完整源码配置见[快速开始](https://www.ithiltir.dev/docs/QuickStart)。

## 运行注意事项

- Dash 是单实例应用，不能让多个 Dash 进程同时写同一套状态。
- `app.public_url` 必须是根路径 URL，不支持 `/dash` 这类路径前缀。
- 不可信网络使用 HTTPS；浏览器、API、主题和部署资产应保持同源。
- Redis 默认保存管理员会话和可丢弃的前台状态；`--no-redis` 模式改用进程内存，重启后失效。
- `$DASH_HOME/configs/notify-config.key` 必须与 PostgreSQL 分开备份；丢失匹配密钥后无法恢复通知渠道凭据。

数据保留、容量规划、备份、反向代理和安全说明见[运维文档](https://www.ithiltir.dev/docs/Operations)。

## 更新

```bash
/opt/Ithiltir-dash/bin/dash update --check
sudo /opt/Ithiltir-dash/bin/dash update
sudo /opt/Ithiltir-dash/bin/dash update --test
```

默认通道安装稳定版，`--test` 选择预发布版。如果迁移中断后需要恢复：

```bash
sudo /opt/Ithiltir-dash/bin/dash update recover
```

跨越带迁移说明的版本前，先阅读[破坏性变更](docs/breaking-changes_CN.md)。

## 开发

```bash
go test ./...
cd web
bun run lint
bun run typecheck
```

数据库集成测试需要设置 `TEST_DATABASE_URL`。前端开发可以代理一个正在运行的 Dash：

```bash
cd web
FRONT_TEST_API=http://127.0.0.1:8080 bun run dev
```

构建前端和 Linux 发布包：

```bash
bash scripts/build_frontend.sh --version 0.0.0-dev -o build/frontend/dist
bash scripts/package.sh --version 0.0.0-dev.0 --node-version 0.0.0-dev.0 \
  -o release -t linux/amd64 --tar-gz
```

本地 Node 资产布局见 [deploy/node/README.md](deploy/node/README.md)。

## 文档

| 文档                                                 | 内容                               |
| ---------------------------------------------------- | ---------------------------------- |
| [快速开始](https://www.ithiltir.dev/docs/QuickStart) | 安装 Dash 并接入第一个节点         |
| [安装部署](https://www.ithiltir.dev/docs/Install)    | 生产部署、反向代理、平台和升级     |
| [运维](https://www.ithiltir.dev/docs/Operations)     | 备份、保留期、容量、安全和排错     |
| [架构](docs/architecture_CN.md)                      | 运行边界、数据流、存储、告警和更新 |
| [API](docs/api_CN.md)                                | HTTP 路径和上报契约                |
| [破坏性变更](docs/breaking-changes_CN.md)            | 迁移和兼容说明                     |

## 许可证

AGPL-3.0-only。见 [LICENSE](LICENSE)。
