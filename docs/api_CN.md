# API

本文档汇总稳定 HTTP 契约。公开路径、方法和字段语义属于兼容边界：既有语义不在原路径上硬改，新行为通过新增端点或追加字段提供。

## 基础

- API 基础路径：`/api`
- Dash 只支持根路径部署，不支持在 `app.public_url` 中配置路径前缀
- JSON 错误包装：

```json
{ "code": "<string>", "message": "<string>" }
```

## 鉴权模型

| 方式                                   | 用途                                              |
| -------------------------------------- | ------------------------------------------------- |
| 管理员密码                             | `POST /api/auth/login`                            |
| refresh cookie + `X-CSRF-Token`        | `POST /api/auth/refresh`、`POST /api/auth/logout` |
| `Authorization: Bearer <access_token>` | 管理 API 和可选鉴权读取                           |
| `X-Node-Secret`                        | Agent 上报、节点身份读取和 deploy 资产下载        |
| `upgrade_token` query                  | 只给旧 Agent 自动升级使用的临时 deploy 资产下载授权 |

Bearer 可选端点会把缺失、格式错误、过期、已撤销或其他无法通过校验的 Bearer token 当作匿名请求处理。这是有意保留的兼容行为：需要管理视图的客户端必须自行区分响应是已鉴权视图还是游客过滤视图。

## 命名空间

| 前缀                                          | 鉴权                                                                                   | 资源                                                                                                                                                                                    |
| --------------------------------------------- | -------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/api/auth`                                   | 登录用管理员密码；刷新和登出用 refresh cookie + `X-CSRF-Token`；`/sessions*` 用 Bearer | `POST /login`、`POST /refresh`、`POST /logout`、会话列表和撤销                                                                                                                          |
| `/api/version`                                | 无                                                                                     | `GET /`                                                                                                                                                                                 |
| `/api/front`                                  | Bearer 可选                                                                            | `GET /brand`、`GET /metrics`、`GET /groups`                                                                                                                                             |
| `/api/metrics`                                | Bearer 可选；历史指标默认只对已授权用户开放                                            | `GET /online`、`GET /history`                                                                                                                                                           |
| `/api/statistics`                             | Bearer 可选                                                                            | `GET /access`                                                                                                                                                                           |
| `/api/statistics/traffic`                     | Bearer 可选；`PATCH /settings` 需要 Bearer                                             | `GET /settings`、`PATCH /settings`、`GET /ifaces`、`GET /summary`、`GET /daily`、`GET /monthly`                                                                                         |
| `/api/node`                                   | `X-Node-Secret`                                                                        | `POST /identity`、`POST /metrics`、`POST /static`                                                                                                                                       |
| `/api/admin/groups`                           | Bearer                                                                                 | `GET /`、`GET /map`、`POST /`、`PATCH /{id}`、`DELETE /{id}`                                                                                                                            |
| `/api/admin/nodes`                            | Bearer                                                                                 | `GET /`、`GET /deploy`、`POST /`、`PUT /display-order`、`PATCH /traffic-p95`、`PATCH /{id}`、`POST /{id}/upgrade`、`GET /traffic/rebuild`、`POST /{id}/traffic/rebuild`、`DELETE /{id}` |
| `/api/admin/alerts/rules`                     | Bearer                                                                                 | `GET /`、`POST /`、`PATCH /{id}`、`DELETE /{id}`                                                                                                                                        |
| `/api/admin/alerts/mounts`                    | Bearer                                                                                 | `GET /`、`PUT /`                                                                                                                                                                        |
| `/api/admin/alerts/settings`                  | Bearer                                                                                 | `GET /`、`PUT /`                                                                                                                                                                        |
| `/api/admin/alerts/channels`                  | Bearer                                                                                 | `GET /`、`POST /`、`GET /{id}`、`PUT /{id}`、`PUT /{id}/enabled`、`POST /{id}/test`、`DELETE /{id}`                                                                                     |
| `/api/admin/alerts/channels/telegram/mtproto` | Bearer                                                                                 | `POST /code`、`POST /verify`、`POST /password`、`POST /ping`                                                                                                                            |
| `/api/admin/system/dash-update`               | Bearer                                                                                 | `GET /status`、`GET /check`、`POST /run`、`GET /release-notes`                                                                                                                          |
| `/api/admin/system/settings`                  | Bearer                                                                                 | `GET /`、`PUT /`、`PATCH /`                                                                                                                                                             |
| `/api/admin/system/themes`                    | Bearer                                                                                 | `GET /`、`POST /upload`、`POST /{id}/apply`、`DELETE /{id}`                                                                                                                             |

## 匿名读取

- `/api/front/brand` 可匿名读取。
- `/api/front/metrics` 和 `/api/front/groups` 允许匿名读取，但匿名结果只包含游客可见节点。匿名 `/api/front/groups` 会省略没有游客可见节点的分组。
- `/api/metrics/online` 允许匿名读取游客可见节点。
- `/api/metrics/history` 默认需要 Bearer。只有 `history_guest_access_mode` 为 `by_node` 时，匿名读取才按游客可见节点放开。
- `/api/statistics/access` 可匿名读取。
- `/api/statistics/traffic/*` 的匿名读取由流量设置控制，并仍受节点游客可见性限制。
- `GET /api/front/metrics` 在节点配置了标签时，会在节点元数据中包含字符串数组 `node.tags`。

## 认证会话

- `GET /api/auth/sessions/` 返回当前 Bearer token 用户的 `{ "sessions": [...] }`。每项包含 `id`、`expires_at`、`session_only` 和 `current`。
- `DELETE /api/auth/sessions/current`、`DELETE /api/auth/sessions/` 和 `DELETE /api/auth/sessions/{sid}` 成功时返回 `204`。

## 管理节点

- `GET /api/admin/nodes/` 包含 `traffic_p95_enabled`、`traffic_cycle_mode`、`traffic_billing_start_day`、`traffic_billing_anchor_date`、`traffic_billing_timezone`、`traffic_direction_mode`、`tags` 和 `version`。`tags` 始终是字符串数组。
- `version.version` 是 Agent 最后上报版本；缺失、非法或低于受支持节点版本下限时，`version.is_outdated` 为 true。上报的 Agent 版本支持自动更新协议时，`version.supports_auto_update` 为 true；平台支持和打包更新资产是否可用会在请求升级时继续校验。
- `PATCH /api/admin/nodes/{id}` 接受 `traffic_p95_enabled`、`tags` 和节点流量覆盖字段。非账期字段未提交时保持不变。节点账期覆盖字段是原子组：只要提交 `traffic_cycle_mode`、`traffic_billing_start_day`、`traffic_billing_anchor_date` 或 `traffic_billing_timezone` 中任意一个字段，就必须同时提交 `traffic_cycle_mode` 和该模式使用的全部字段，否则返回 `400 invalid_traffic_cycle_settings`。`default` 不使用账期字段；`calendar_month` 使用 `traffic_billing_timezone`；`clamp_to_month_end` 使用 `traffic_billing_start_day` 和 `traffic_billing_timezone`；`whmcs_compatible` 使用 `traffic_billing_anchor_date` 和 `traffic_billing_timezone`，`traffic_billing_start_day` 由锚点日期推导。`tags` 接受字符串数组；值会 trim，空值和重复值会被删除，`[]` 表示清空标签。`traffic_cycle_mode` 允许 `default`、`calendar_month`、`whmcs_compatible`、`clamp_to_month_end`；`traffic_direction_mode` 允许 `default`、`out`、`both`、`max`。
- `PATCH /api/admin/nodes/{id}` 提交空 `secret` 时返回 `400 invalid_secret`；提交的 `secret` 已属于其他节点时返回 `409 duplicate_secret`。
- `PATCH /api/admin/nodes/traffic-p95` 接受 `ids` 和 `enabled`。`enabled` 必填。`ids` 必须是非空正整数数组，不能重复，最多 10000 项。该命令先校验所有节点 ID，再在一个事务中更新全部选中节点。成功返回 `204`；任一节点不存在或已删除时返回 `404 not_found`，且不会更新任何节点。
- `GET /api/admin/nodes/traffic/rebuild` 返回最近一次进程内重建任务状态。还没有任务时返回 `status=idle`；运行中时包含 `server_id`、`running=true` 和 `started_at`；任务结束后可能继续返回 `completed` 或 `failed`，直到下一次任务替换。该状态不会跨进程重启持久化。失败状态只暴露稳定的 `code` 和 `error`，不会返回内部错误字符串。
- `POST /api/admin/nodes/{id}/traffic/rebuild` 启动进程内任务，根据该节点当前保存的网卡原始指标，重建 `database.traffic_retention_days` 窗口内的 5 分钟流量事实。成功返回 `202` 和同样的状态体；节点不存在或已删除时返回 `404 not_found`；已有任意重建任务运行时返回 `409 traffic_rebuild_running`。该任务只重写 5 分钟事实，并在启动时让该节点保留窗口内重叠周期的月度快照失效；如果任务失败，受影响的月度历史会由后续常规快照维护重新生成，在没有可复用快照时由读取路径按保留事实计算，或通过重试重建恢复；超出保留窗口的数据不会由数据库自动补回。
- 非法 `tags` 返回 `400 invalid_tags`。
- 节点账期规范化语义稳定：`default` 继承全局账期并清空节点账期字段；`calendar_month` 保存 `traffic_billing_start_day=1`；非 `whmcs_compatible` 模式保存空 `traffic_billing_anchor_date`；非默认模式下空 `traffic_billing_timezone` 在读取时使用应用时区。
- 节点统计方向规范化语义稳定：`default` 继承全局统计方向；`out`、`both`、`max` 覆盖该节点。
- 非法节点流量字段返回 `400 invalid_traffic_cycle_mode`、`invalid_traffic_cycle_settings`、`invalid_traffic_billing_start_day`、`invalid_traffic_billing_anchor_date`、`invalid_traffic_billing_timezone` 或 `invalid_traffic_direction_mode`。
- `POST /api/admin/nodes/{id}/upgrade` 成功返回 `204`；节点无法接收自动下发更新时返回 `409 node_upgrade_unsupported`；打包版本、平台或资产不可用时返回 `409`；Dash 无法生成旧 Agent 临时下载授权时返回 `503 node_upgrade_grant_error`。

## 管理系统设置

- `GET /api/admin/system/settings` 返回 `history_guest_access_mode`、`dash_update_channel`、`dash_update_mode`、`logo_url`、`page_title` 和 `topbar_text`。`dash_update_channel` 为 `release` 或 `prerelease`；`dash_update_mode` 为 `manual`、`notify` 或 `auto`。
- `PATCH /api/admin/system/settings` 接受这些字段的局部更新。空更新返回 `400 no_fields`；非法值返回 `400 invalid_fields`。
- `PUT /api/admin/system/settings` 全量替换设置文档，必须提交 `history_guest_access_mode`、`dash_update_channel`、`dash_update_mode`、`logo_url`、`page_title` 和 `topbar_text`。

## 管理 Dash 更新

- `GET /api/admin/system/dash-update/status` 返回最近一次 Dash 更新任务状态。`status` 为 `idle`、`running`、`completed` 或 `failed`。响应包含 `available`，并可能包含 `id`、`action`、`channel`、`started_at`、`finished_at`、`exit_code`、`log_tail` 和 `unavailable_reason`。
- `GET /api/admin/system/dash-update/check?channel=release|prerelease` 返回 `current_version`、`current_channel`、`target_channel`、`latest_version`、`version_status` 和 `bundled_node_version`。`version_status` 为 `available`、`current`、`ahead` 或 `unknown`。省略 `channel` 时默认使用 `release`；`prerelease` 只检查 prerelease tag；非法 `channel` 返回 `400 invalid_fields`；本机缺少 `git` 返回 `503 dash_update_unavailable`；远端 tag 拉取失败返回 `502 dash_update_check_failed`。
- `POST /api/admin/system/dash-update/run` 必须提交 `action=update|reinstall`、`channel=release|prerelease` 和 `lang=zh|en`。它会启动后台 Dash 更新任务，成功返回 `202` 和状态体。`prerelease` 只针对 prerelease tag 运行。已有任务运行时返回 `409` 和当前状态体。字段缺失或非法返回 `400 invalid_fields`；更新器不可用返回 `503 dash_update_unavailable`。
- `dash_update_mode=notify` 会让 Dash 按配置通道定期检查更新，并通过已启用通知渠道发送可用更新提醒。`dash_update_mode=auto` 会定期检查、发现更高版本时自动启动更新，并发送开始和结果通知。
- 已部署的 Dash 更新器基于 Linux/systemd，并通过打包内置的 `update_dash_linux.sh` 运行。更新任务可能重启 Dash，调用方收到 `202` 后必须容忍短暂断连。
- `GET /api/admin/system/dash-update/release-notes?lang=zh|en` 从文档站返回 `{ "source_url": "...", "html": "..." }`；抓取失败返回 `502 release_notes_fetch_failed`。

## Agent 更新

- `POST /api/node/metrics` 成功响应包含 `update`。
- 无待升级任务时，`update` 为 `null`。
- 有待升级任务时，`update` 包含 `id`、`version`、`url`、`sha256` 和 `size`。
- `url` 可能包含短期有效的 `upgrade_token`，让旧 Agent 不发送 `X-Node-Secret` 也能下载本次升级的精确资产。客户端必须按原样使用返回的 URL。
- 待升级任务是易失状态，Agent 上报完全相同的目标版本或 SemVer 优先级更高的版本后清除。同一 SemVer 优先级但 build metadata 不同的版本视为不同节点二进制，仍可下发。

## 节点运行时指标字段

- `POST /api/node/metrics` 接受可选的 `metrics.disk.smart`、`metrics.thermal` 和 `metrics.pressure`。旧 Agent 可以不带这些字段。
- `metrics.disk.smart` 是磁盘 SMART 运行时状态，进入独立热点缓存，不写入 PostgreSQL 指标快照。确认是物理盘的 SMART 温度可归约成按设备区分的 `disk.temp_c` 历史值。`metrics.thermal` 保存硬件温度传感器，位置在 metrics 根级；thermal 会写入 PostgreSQL 指标快照，但在前台缓存中作为独立字段缓存保存。
- `metrics.pressure` 是 Linux PSI（Pressure Stall Information）。它可以包含 `cpu`、`memory`、`io`，每项可带 `some` 和 `full` 数值组。每个组包含 `avg10`、`avg60`、`avg300` 百分比和累计 `total` 微秒。Dashboard 会把这些值保存成固定数值时序列；采集状态/原因字符串不持久化。缺失的组保持 `NULL`，表示不可用，不会当成 0 压力。
- `disk.smart.devices` 和 `thermal.sensors` 是数组。字段存在但结果为空时使用 `[]`，不是 `null`。
- `temp_c`、`power_on_hours`、`lifetime_used_percent`、`critical_warning`、`high_c`、`critical_c` 等可选数值读不到时省略，不转换成 `0`。
- `disk.smart.devices[].critical_warning` 是 NVMe 的原始 critical warning bitset。`disk.smart.devices[].failing_attrs[]` 只包含当前 `FAILING_NOW` 的 ATA SMART 属性。
- SMART 和 thermal 的 `status` 是开放字符串。已知值包括 `ok`、`partial`、`unsupported`、`not_found`、`no_permission`、`timeout`、`error`、`no_cache`、`stale`、`no_tool`、`standby`。
- `disk.smart.status` 是采集状态，`disk.smart.devices[].health` 是磁盘健康结果。`status=ok` 且 `health=failed` 表示采集成功但磁盘健康失败。
- `status=no_cache`、`no_tool` 或 `unsupported` 不表示磁盘故障。`status=stale` 会保留最后一次 `devices[]`，同时标记缓存过期。
- `GET /api/front/metrics` 会把节点最新热点快照、SMART 字段缓存和 thermal 字段缓存组合成节点视图，返回 `disk.smart`、`disk.temperature_devices`、顶层 `thermal` 和顶层 `pressure`。`disk.temperature_devices` 是后端推导出的物理盘名称列表，可作为 `disk.temp_c` 历史查询的 `device`。
- `/api/metrics/history` 支持 `cpu.temp_c`、`disk.temp_c` 和 PSI 平均值指标：`pressure.cpu.some_avg10|avg60|avg300`、`pressure.memory.some_avg10|avg60|avg300`、`pressure.memory.full_avg10|avg60|avg300`、`pressure.io.some_avg10|avg60|avg300`、`pressure.io.full_avg10|avg60|avg300`。CPU 温度来自 thermal 的 CPU 传感器。硬盘温度来自确认是物理盘的 SMART 设备；虚拟盘和 RAID 设备不会持久化。带 `device` 时查询 `disk.temperature_devices` 中的单块物理盘；不带 `device` 时聚合已持久化的物理盘记录。

## 告警指标

- 内置 SMART 健康失败和 NVMe 关键告警规则和内置 RAID 失效规则一样默认挂载。
- 用户自定义规则可使用 `disk.smart.failed`、`disk.smart.nvme.critical_warning`、`disk.smart.attribute_failing`、`disk.smart.max_temp_c` 和 `thermal.max_temp_c`。
- `disk.smart.failed` 只统计 SMART 健康结果为 `failed` 的设备，不把 `no_cache`、`no_tool`、`unsupported` 或其他采集状态计为磁盘故障。
- `disk.smart.nvme.critical_warning` 统计 `critical_warning` bitset 非 0 的设备数。`disk.smart.attribute_failing` 统计当前 `FAILING_NOW` 的 SMART 属性数。
- 缺失 `disk.smart` 不表示 SMART 故障。节点没有 SMART 上报时，内置 SMART 规则不会触发。
- PSI pressure 数据当前只保存并用于历史查询；它暂时不是告警指标，也没有启用内置 PSI 告警。

## 流量统计

- `GET /api/statistics/traffic/settings` 返回 `guest_access_mode`、`usage_mode`、`cycle_mode`、`billing_start_day`、`billing_anchor_date`、`billing_timezone` 和 `direction_mode`。
- `PATCH /api/statistics/traffic/settings` 接受局部更新，未知值返回 `400 invalid_fields`。
- 允许值：`guest_access_mode`: `disabled`、`by_node`；`usage_mode`: `lite`、`billing`；`cycle_mode`: `calendar_month`、`whmcs_compatible`、`clamp_to_month_end`；`direction_mode`: `out`、`both`、`max`。
- 流量查询使用节点有效流量配置：节点 `traffic_cycle_mode=default` 时继承全局账期模式、月度起始日、账期锚点和账期时区；否则使用节点自己的 `traffic_*` 账期字段。节点 `traffic_direction_mode=default` 时继承全局统计方向；否则使用节点自己的方向覆盖。
- `lite` 和 `billing` 模式都会使用有效账期作为月度边界；`billing` 额外启用日统计、P95、覆盖率、5 分钟事实和月度计费快照。
- `GET /daily` 要求 `usage_mode=billing`，否则返回 `409 traffic_daily_requires_billing`。`period` 可选，允许 `current`、`previous`，省略时为 `current`。
- `GET /monthly` 支持 `months` 和 `period`。`months` 最大 24；`period=current` 从本账期开始，`period=previous` 从上账期开始，省略时为 `current`。响应字段 `includes_current` 在 `period=current` 时为 `true`，在 `period=previous` 时为 `false`。
- 统计方向用于选择计费视图：出站、入站加出站，或每项指标取入站/出站较大值。
- 流量 summary、daily、monthly 响应保留原始 `in_*` 和 `out_*` 字段，并通过 `selected_bytes`、`selected_p95_bytes_per_sec`、`selected_peak_bytes_per_sec` 及其方向字段暴露当前计费视图。
- 客户端应使用 `coverage_ratio` 展示样本覆盖率和准确性提示。`partial` 仅为兼容保留，新的展示逻辑不应依赖该字段。
- 只有 `p95_status` 为 `available` 时，P95 字段才不是 `null`。

## 非 API HTTP 路径

| 路径                          | 作用                                    |
| ----------------------------- | --------------------------------------- |
| `/theme/active.css`           | 当前主题 CSS                            |
| `/theme/active.json`          | 当前主题 manifest；默认主题可能返回 404 |
| `/theme/preview/{id}.png`     | 主题预览图                              |
| `/deploy/linux/install.sh`    | Linux Agent 安装脚本                    |
| `/deploy/macos/install.sh`    | macOS Agent 安装脚本                    |
| `/deploy/windows/install.ps1` | Windows Agent 安装脚本                  |
| `/deploy/*`                   | 打包携带的节点发布资产；需要 `X-Node-Secret` 或临时 `upgrade_token` |
| `/`                           | SPA                                     |

## 兼容性规则

- 既有路径、方法和字段语义保持稳定。
- 新行为通过新端点或追加字段提供。
- 需要废弃时先保留旧入口，再新增替代入口。
