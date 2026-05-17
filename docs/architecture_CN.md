# 架构

Ithiltir Dash 是单实例应用。根入口只启动一个 HTTP 进程，该进程装配 API、SPA、主题资源、安装脚本、节点资产分发和后台服务。

## 运行边界

| 组件                     | 责任                                                                                           |
| ------------------------ | ---------------------------------------------------------------------------------------------- |
| `cmd/dash`               | 进程入口、配置加载、依赖装配、迁移入口和关闭流程                                               |
| HTTP 服务                | 挂载 `/api`、`/theme`、`/deploy` 和 SPA                                                        |
| PostgreSQL + TimescaleDB | 持久化指标历史、流量事实、节点元数据、告警规则、告警通知 outbox 和系统设置                     |
| Redis                    | 默认保存会话、热点快照和告警运行时状态                                                         |
| 进程内内存               | 节点鉴权索引和易失 Agent 更新请求；`--no-redis` 时承接会话和热点运行时状态                     |
| Agent                    | 上报指标和静态主机信息；接收更新 manifest                                                      |
| Linux SMART 缓存 timer   | root 侧 `smartctl` helper 写入 `/run/ithiltir-node/smart.json`；Agent 只读缓存，不以 root 运行 |
| Web UI                   | 读取看板数据并提交管理操作                                                                     |

## HTTP 面

| 前缀              | 作用                                 |
| ----------------- | ------------------------------------ |
| `/api/auth`       | 登录、续期、登出、会话撤销           |
| `/api/version`    | Dash 版本和打包携带的节点版本        |
| `/api/front`      | 看板读取                             |
| `/api/metrics`    | 历史指标和在线率查询                 |
| `/api/statistics` | 统计访问策略和流量统计查询           |
| `/api/node`       | Agent 上报和节点身份读取             |
| `/api/admin`      | 管理台写操作                         |
| `/theme`          | 当前主题 CSS、主题 manifest 和预览图 |
| `/deploy`         | 安装脚本和节点发布资产               |
| `/`               | SPA                                  |

## 数据流

1. Agent 通过 `/api/node/*` 上报指标和静态主机信息。
2. 指标上报成功响应可包含更新 manifest。
3. PostgreSQL + TimescaleDB 保存持久化历史、流量事实、配置和告警通知 outbox。
4. Redis 或进程内内存保存热点快照、会话和告警运行时状态。
5. 后台服务评估告警、发送队列通知并汇总流量数据。

节点 IP 是已鉴权 Agent 请求的观察值：有 `X-Forwarded-For` 时 Dash 取其第一个 IP，否则回退到 `RemoteAddr`；不可解析的值不会被使用。该字段用于展示和运维，不作为鉴权边界。

## 状态和保留策略

- 默认启动依赖 PostgreSQL 和 Redis；传 `--no-redis` 时，Redis 承载的运行时状态改用进程内内存。
- 节点鉴权和待下发 Agent 更新请求使用进程内内存，不走 Redis。
- SMART 和 thermal 指标属于运行时状态。SMART 缓存新鲜度、helper 可用性和设备健康结果保存在独立热点缓存，不写入 PostgreSQL 指标快照。确认是物理盘的 SMART 温度会归约写入 `disk_physical_metrics.temp_c`，用于按设备查询历史；虚拟盘和 RAID 设备会被忽略。同一套后端判定会生成 `disk.temperature_devices`，供前端进入硬盘温度历史。thermal 随指标快照保存，并归约写入 `cpu_temp_c`，同时在前台缓存中拆成独立字段缓存；读取前台节点视图时再组合进 JSON。
- 告警评估读取热点快照。内置离线、RAID、SMART 健康失败和 NVMe 关键告警规则来自快照新鲜度和上报磁盘状态。
- 告警服务启动后 1 分钟内不会新开告警事件。
- 告警事件和运行时状态提交不依赖通知发送。可加载通知目标时，通知 payload 存入 PostgreSQL outbox，并由带租约的重试 worker 发送；通知目标不可用时，状态变更仍会提交，但不会新增通知 outbox。
- 数据库历史保留默认 `45 days`；普通监控用 `database.retention_days`，流量 5 分钟事实表用 `database.traffic_retention_days`。
- 流量 `lite` 模式保存月度入/出总量和估算峰值；`billing` 模式额外维护 5 分钟事实、日汇总、P95、覆盖率和月度快照。
- 全局账期配置是默认值；节点可覆盖账期模式、月度起始日、账期锚点和账期时区。流量查询、月度用量回填和月度快照都按节点有效账期切分。
- 流量方向模式只改变选中的计费视图；原始入站和出站计数仍分开保存。
- 历史指标默认不对游客公开。`history_guest_access_mode=by_node` 时，只对游客可见节点开放。
- 流量统计是否对游客公开由流量设置控制，并仍受节点可见性限制。

## 鉴权边界

| 区域                                                  | 鉴权                                      |
| ----------------------------------------------------- | ----------------------------------------- |
| `/api/auth/login`                                     | 管理员密码                                |
| `/api/auth/refresh`、`/api/auth/logout`               | refresh cookie + `X-CSRF-Token`           |
| `/api/front/*`、`/api/metrics/*`、`/api/statistics/*` | Bearer 可选；匿名请求按系统可见性设置过滤 |
| `/api/node/*`                                         | `X-Node-Secret`                           |
| `/api/admin/*`                                        | `Authorization: Bearer <access_token>`    |

## 前端和反向代理

前端可以单独启动开发服务器，但运行边界仍是同源路径。开发代理或生产反向代理应把 `/api`、`/theme`、`/deploy` 转给后端，`/` 保持为 Dash SPA。跨域后端地址需要同时配置 CORS、cookie 和 CSRF 策略。

## 目录

| 路径                          | 内容                                     |
| ----------------------------- | ---------------------------------------- |
| `cmd/dash`                    | 服务、迁移和主题打包入口                 |
| `internal/config`             | 配置加载、默认值、校验和运行目录         |
| `internal/transport/http`     | HTTP 服务、静态资源、主题资源和 API 挂载 |
| `internal/transport/http/api` | `/api` 路由树                            |
| `internal/store`              | 持久化和缓存访问层                       |
| `internal/alert`              | 告警编译、运行时和发送编排               |
| `internal/traffic`            | 流量统计后台服务                         |
| `web`                         | SPA 前端源码                             |
| `configs`                     | 示例配置                                 |
| `db/migrations`               | 数据库结构变更                           |
| `scripts`                     | 构建和打包入口                           |
