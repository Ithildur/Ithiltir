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
| `X-Node-Secret`                        | Agent 上报和节点身份读取                          |

Bearer 可选端点会把缺失、格式错误、过期、已撤销或其他无法通过校验的 Bearer token 当作匿名请求处理。这是有意保留的兼容行为：需要管理视图的客户端必须自行区分响应是已鉴权视图还是游客过滤视图。

## 命名空间

| 前缀                                          | 鉴权                                                                                   | 资源                                                                                                                              |
| --------------------------------------------- | -------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `/api/auth`                                   | 登录用管理员密码；刷新和登出用 refresh cookie + `X-CSRF-Token`；`/sessions*` 用 Bearer | `POST /login`、`POST /refresh`、`POST /logout`、会话撤销                                                                          |
| `/api/version`                                | 无                                                                                     | `GET /`                                                                                                                           |
| `/api/front`                                  | Bearer 可选                                                                            | `GET /brand`、`GET /metrics`、`GET /groups`                                                                                       |
| `/api/metrics`                                | Bearer 可选；历史指标默认只对已授权用户开放                                            | `GET /online`、`GET /history`                                                                                                     |
| `/api/statistics`                             | Bearer 可选                                                                            | `GET /access`                                                                                                                     |
| `/api/statistics/traffic`                     | Bearer 可选；`PATCH /settings` 需要 Bearer                                             | `GET /settings`、`PATCH /settings`、`GET /ifaces`、`GET /summary`、`GET /daily`、`GET /monthly`                                   |
| `/api/node`                                   | `X-Node-Secret`                                                                        | `POST /identity`、`POST /metrics`、`POST /static`                                                                                 |
| `/api/admin/groups`                           | Bearer                                                                                 | `GET /`、`GET /map`、`POST /`、`PATCH /{id}`、`DELETE /{id}`                                                                      |
| `/api/admin/nodes`                            | Bearer                                                                                 | `GET /`、`GET /deploy`、`POST /`、`PUT /display-order`、`PATCH /traffic-p95`、`PATCH /{id}`、`POST /{id}/upgrade`、`DELETE /{id}` |
| `/api/admin/alerts/rules`                     | Bearer                                                                                 | `GET /`、`POST /`、`PATCH /{id}`、`DELETE /{id}`                                                                                  |
| `/api/admin/alerts/mounts`                    | Bearer                                                                                 | `GET /`、`PUT /`                                                                                                                  |
| `/api/admin/alerts/settings`                  | Bearer                                                                                 | `GET /`、`PUT /`                                                                                                                  |
| `/api/admin/alerts/channels`                  | Bearer                                                                                 | `GET /`、`POST /`、`GET /{id}`、`PUT /{id}`、`PUT /{id}/enabled`、`POST /{id}/test`、`DELETE /{id}`                               |
| `/api/admin/alerts/channels/telegram/mtproto` | Bearer                                                                                 | `POST /code`、`POST /verify`、`POST /password`、`POST /ping`                                                                      |
| `/api/admin/system/settings`                  | Bearer                                                                                 | `GET /`、`PUT /`、`PATCH /`                                                                                                       |
| `/api/admin/system/themes`                    | Bearer                                                                                 | `GET /`、`POST /upload`、`POST /{id}/apply`、`DELETE /{id}`                                                                       |

## 匿名读取

- `/api/front/brand` 可匿名读取。
- `/api/front/metrics` 和 `/api/front/groups` 允许匿名读取，但匿名结果只包含游客可见节点。
- `/api/metrics/online` 允许匿名读取游客可见节点。
- `/api/metrics/history` 默认需要 Bearer。只有 `history_guest_access_mode` 为 `by_node` 时，匿名读取才按游客可见节点放开。
- `/api/statistics/access` 可匿名读取。
- `/api/statistics/traffic/*` 的匿名读取由流量设置控制，并仍受节点游客可见性限制。
- `GET /api/front/metrics` 在节点配置了标签时，会在节点元数据中包含字符串数组 `node.tags`。

## 管理节点

- `GET /api/admin/nodes/` 包含 `traffic_p95_enabled`、`traffic_cycle_mode`、`traffic_billing_start_day`、`traffic_billing_anchor_date`、`traffic_billing_timezone`、`tags` 和 `version`。`tags` 始终是字符串数组。
- `version.version` 是 Agent 最后上报版本；缺失、非法或低于打包节点版本时，`version.is_outdated` 为 true。
- `PATCH /api/admin/nodes/{id}` 接受 `traffic_p95_enabled`、`tags` 和节点账期覆盖字段。未提交字段保持不变。`tags` 接受字符串数组；值会 trim，空值和重复值会被删除，`[]` 表示清空标签。`traffic_cycle_mode` 允许 `default`、`calendar_month`、`whmcs_compatible`、`clamp_to_month_end`。
- `PATCH /api/admin/nodes/traffic-p95` 接受 `ids` 和 `enabled`。`enabled` 必填。`ids` 必须是非空正整数数组，不能重复，最多 10000 项。该命令先校验所有节点 ID，再在一个事务中更新全部选中节点。成功返回 `204`；任一节点不存在或已删除时返回 `404 not_found`，且不会更新任何节点。
- 非法 `tags` 返回 `400 invalid_tags`。
- 节点账期规范化语义稳定：`default` 继承全局账期并清空节点账期字段；`calendar_month` 保存 `traffic_billing_start_day=1`；非 `whmcs_compatible` 模式保存空 `traffic_billing_anchor_date`；非默认模式下空 `traffic_billing_timezone` 在读取时使用应用时区。
- 非法节点账期字段返回 `400 invalid_traffic_cycle_mode`、`invalid_traffic_billing_start_day`、`invalid_traffic_billing_anchor_date` 或 `invalid_traffic_billing_timezone`。
- `POST /api/admin/nodes/{id}/upgrade` 成功返回 `204`；打包版本、平台或资产不可用时返回 `409`。

## Agent 更新

- `POST /api/node/metrics` 成功响应包含 `update`。
- 无待升级任务时，`update` 为 `null`。
- 有待升级任务时，`update` 包含 `id`、`version`、`url`、`sha256` 和 `size`。
- 待升级任务是易失状态，Agent 上报目标版本或更新版本后清除。

## 节点运行时指标字段

- `POST /api/node/metrics` 接受可选的 `metrics.disk.smart` 和 `metrics.thermal`。旧 Agent 可以不带这两个字段。
- `metrics.disk.smart` 是磁盘 SMART 运行时状态，进入独立热点缓存，不写入 PostgreSQL 指标快照。确认是物理盘的 SMART 温度可归约成按设备区分的 `disk.temp_c` 历史值。`metrics.thermal` 保存硬件温度传感器，位置在 metrics 根级；thermal 会写入 PostgreSQL 指标快照，但在前台缓存中作为独立字段缓存保存。
- `disk.smart.devices` 和 `thermal.sensors` 是数组。字段存在但结果为空时使用 `[]`，不是 `null`。
- `temp_c`、`power_on_hours`、`lifetime_used_percent`、`critical_warning`、`high_c`、`critical_c` 等可选数值读不到时省略，不转换成 `0`。
- `disk.smart.devices[].critical_warning` 是 NVMe 的原始 critical warning bitset。`disk.smart.devices[].failing_attrs[]` 只包含当前 `FAILING_NOW` 的 ATA SMART 属性。
- SMART 和 thermal 的 `status` 是开放字符串。已知值包括 `ok`、`partial`、`unsupported`、`not_found`、`no_permission`、`timeout`、`error`、`no_cache`、`stale`、`no_tool`、`standby`。
- `disk.smart.status` 是采集状态，`disk.smart.devices[].health` 是磁盘健康结果。`status=ok` 且 `health=failed` 表示采集成功但磁盘健康失败。
- `status=no_cache`、`no_tool` 或 `unsupported` 不表示磁盘故障。`status=stale` 会保留最后一次 `devices[]`，同时标记缓存过期。
- `GET /api/front/metrics` 会把节点最新热点快照、SMART 字段缓存和 thermal 字段缓存组合成节点视图，返回 `disk.smart`、`disk.temperature_devices` 和顶层 `thermal`。`disk.temperature_devices` 是后端推导出的物理盘名称列表，可作为 `disk.temp_c` 历史查询的 `device`。
- `/api/metrics/history` 支持 `cpu.temp_c` 和 `disk.temp_c`。CPU 温度来自 thermal 的 CPU 传感器。硬盘温度来自确认是物理盘的 SMART 设备；虚拟盘和 RAID 设备不会持久化。带 `device` 时查询 `disk.temperature_devices` 中的单块物理盘；不带 `device` 时聚合已持久化的物理盘记录。

## 告警指标

- 内置 SMART 健康失败和 NVMe 关键告警规则和内置 RAID 失效规则一样默认挂载。
- 用户自定义规则可使用 `disk.smart.failed`、`disk.smart.nvme.critical_warning`、`disk.smart.attribute_failing`、`disk.smart.max_temp_c` 和 `thermal.max_temp_c`。
- `disk.smart.failed` 只统计 SMART 健康结果为 `failed` 的设备，不把 `no_cache`、`no_tool`、`unsupported` 或其他采集状态计为磁盘故障。
- `disk.smart.nvme.critical_warning` 统计 `critical_warning` bitset 非 0 的设备数。`disk.smart.attribute_failing` 统计当前 `FAILING_NOW` 的 SMART 属性数。
- 缺失 `disk.smart` 不表示 SMART 故障。节点没有 SMART 上报时，内置 SMART 规则不会触发。

## 流量统计

- `GET /api/statistics/traffic/settings` 返回 `guest_access_mode`、`usage_mode`、`cycle_mode`、`billing_start_day`、`billing_anchor_date`、`billing_timezone` 和 `direction_mode`。
- `PATCH /api/statistics/traffic/settings` 接受局部更新，未知值返回 `400 invalid_fields`。
- 允许值：`guest_access_mode`: `disabled`、`by_node`；`usage_mode`: `lite`、`billing`；`cycle_mode`: `calendar_month`、`whmcs_compatible`、`clamp_to_month_end`；`direction_mode`: `out`、`both`、`max`。
- 流量查询使用节点有效账期配置：节点 `traffic_cycle_mode=default` 时继承全局账期模式、月度起始日、账期锚点和账期时区；否则使用节点自己的 `traffic_*` 账期字段。
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
| `/deploy/*`                   | 打包携带的节点发布资产                  |
| `/`                           | SPA                                     |

## 兼容性规则

- 既有路径、方法和字段语义保持稳定。
- 新行为通过新端点或追加字段提供。
- 需要废弃时先保留旧入口，再新增替代入口。
