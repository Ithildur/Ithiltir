# API

本文定义当前 HTTP 契约。

## 基础

- API 基础路径：`/api`
- Dash 只支持根路径部署，不支持在 `app.public_url` 中配置路径前缀
- `app.public_url` 接受 HTTP 和 HTTPS；主机必须是 IP 字面量或 ASCII DNS 名称，可选端口范围为 1～65535；国际化域名必须使用 IDNA/punycode 形式；裸 IP 默认使用 HTTP，裸域名默认使用 HTTPS
- JSON 错误包装：

```json
{ "code": "<string>", "message": "<string>" }
```

## 鉴权模型

| 方式                                   | 用途                                                |
| -------------------------------------- | --------------------------------------------------- |
| 管理员密码                             | `POST /api/auth/login`                              |
| refresh cookie + `X-CSRF-Token`        | `POST /api/auth/refresh`、`POST /api/auth/logout`   |
| `Authorization: Bearer <access_token>` | 管理 API 和可选鉴权读取                             |
| `X-Node-Secret`                        | Node 上报、节点身份读取和 deploy 资产下载           |
| `upgrade_token` query                  | 只给旧 Node 自动升级使用的临时 deploy 资产下载授权  |

Bearer 可选端点会把缺失、格式错误、过期、已撤销或其他非法 Bearer token 当作匿名请求处理。这是有意设计：它们是提供可选管理员视图的公开端点，不是带游客兜底的鉴权端点。Refresh cookie 使用 `SameSite=Strict`，refresh/logout 还必须提交匹配的 `X-CSRF-Token`。
管理员密码通过 `monitor_dash_pwd` 提供，至少包含 8 个可见 ASCII 字符，且不得包含空白字符。

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
| `/api/admin/alerts/events`                    | Bearer                                                                                 | `GET /`、`GET /summary`、`GET /servers`                                                                                                                                                 |
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

- `POST /api/auth/login` 请求体必须包含 `password` 和 `persistence`；`persistence` 只允许 `session` 或 `persistent`。兼容字段 `username` 可以省略，Dash 的固定密码鉴权不会使用它。
- 登录成功返回 `{ "access_token": "...", "expires_at": "<RFC3339>", "csrf_token": "..." }`，并写入 refresh/CSRF cookie。格式错误的登录 JSON 返回 `400 invalid_json`，非法 `persistence` 返回 `400 invalid_persistence`，凭据错误返回 `401 unauthorized`，登录限流返回 `429 rate_limited`。
- `POST /api/auth/refresh` 使用 refresh cookie 和 `X-CSRF-Token`，轮换会话并返回与登录相同的响应字段。`POST /api/auth/logout` 使用相同鉴权，成功返回 `204` 并清除会话 cookie。
- 默认 Redis 模式下，认证会话保存在 Redis 中，在过期或被撤销前可跨 Dash 重启和原地升级继续有效；使用 `--no-redis` 时，会话只存在于当前进程，并在 Dash 重启后失效。
- `GET /api/auth/sessions/` 返回当前 Bearer token 用户的 `{ "sessions": [...] }`。每项包含 `id`、`expires_at`、`session_only` 和 `current`。
- `DELETE /api/auth/sessions/current`、`DELETE /api/auth/sessions/` 和 `DELETE /api/auth/sessions/{sid}` 成功时返回 `204`。

## 管理节点

- `GET /api/admin/nodes/` 包含 `traffic_p95_enabled`、`traffic_cycle_mode`、`traffic_billing_start_day`、`traffic_billing_anchor_date`、`traffic_billing_timezone`、`traffic_direction_mode`、`tags` 和 `version`。`tags` 始终是字符串数组。
- `version.version` 是 Node 最后上报版本；缺失、非法或低于受支持节点版本下限时，`version.is_outdated` 为 true。上报的 Node 版本支持自动更新协议时，`version.supports_auto_update` 为 true；平台支持和打包更新资产是否可用会在请求升级时继续校验。
- `PATCH /api/admin/nodes/{id}` 接受 `traffic_p95_enabled`、`tags` 和节点流量字段。非账期字段未提交时保持不变。节点账期字段是原子组：只要提交 `traffic_cycle_mode`、`traffic_billing_start_day`、`traffic_billing_anchor_date` 或 `traffic_billing_timezone` 中任意一个字段，就必须同时提交 `traffic_cycle_mode` 和该模式使用的全部字段，否则返回 `400 invalid_traffic_cycle_settings`。账期和统计方向变更立即生效；账期改变时，该节点受影响的月度派生数据会失效，并在后台从新旧当前账期较早的起点局部重算仍在保留期内的网卡 raw。局部重算期间只暂停该节点的 Lite 实时累计，不会回退全局进度或阻塞其他节点；受影响节点在追平前可能暂时没有当前账期统计或只显示部分覆盖。`calendar_month` 使用 `traffic_billing_timezone`；`clamp_to_month_end` 使用 `traffic_billing_start_day` 和 `traffic_billing_timezone`；`whmcs_compatible` 使用 `traffic_billing_anchor_date` 和 `traffic_billing_timezone`，`traffic_billing_start_day` 由锚点日期推导。兼容旧客户端的输入别名 `default` 仍可在不带账期字段时提交，但会保存为从 1 号开始的显式 `calendar_month`。`tags` 接受字符串数组；值会 trim，空值和重复值会被删除，`[]` 表示清空标签。响应中的 `traffic_cycle_mode` 只包含 `calendar_month`、`whmcs_compatible`、`clamp_to_month_end`；`traffic_direction_mode` 允许 `default`、`out`、`both`、`max`。
- `PATCH /api/admin/nodes/{id}` 会 trim `secret`；字段必须包含 8–128 个 Unicode 字符，否则返回 `400 invalid_secret`。提交的 `secret` 已属于其他节点时返回 `409 duplicate_secret`。
- 提交的节点 `name` 会 trim，必须包含 1 到 64 个 Unicode 字符且不得含控制字符；非法值返回 `400 invalid_name`。
- `GET /api/admin/nodes/deploy` 在 `scripts` 下返回各平台的 `url` 和 `command_prefix`；把节点 secret 追加到 `command_prefix` 后就是可直接执行的一行安装命令。
- `PATCH /api/admin/nodes/traffic-p95` 接受 `ids` 和 `enabled`。`enabled` 必填。`ids` 必须是非空正整数数组，不能重复，最多 10000 项。该命令先校验所有节点 ID，再在一个事务中更新全部选中节点。成功返回 `204`；任一节点不存在或已删除时返回 `404 not_found`，且不会更新任何节点。
- `GET /api/admin/nodes/traffic/rebuild` 返回当前进程内的单例重建状态。没有任务时返回 `status=idle`；运行中包含 `server_id`、`running=true` 和 `started_at`；结束后保持 `completed` 或 `failed`，直到下一次任务替换。Dash 重启会终止运行中任务并把该状态重置为 `idle`。失败状态只暴露稳定的 `code` 和 `error`，不会返回内部错误字符串。
- `POST /api/admin/nodes/{id}/traffic/rebuild` 启动 Billing 专用任务，根据保留期内的网卡 raw 重建该节点的 5 分钟流量事实。成功返回 `202` 和同样的状态体；节点不存在或已删除时返回 `404 not_found`；Lite 模式返回 `409 traffic_rebuild_requires_billing`；已有任意重建任务运行时返回 `409 traffic_rebuild_running`。重建范围取网卡 raw 保留期与 `database.traffic_retention_days` 的交集。任务按 6 小时串行分块重写事实，并在同一事务中让重叠月度快照失效；分块之间释放流量写入门。运行中切换到 Lite 时，已开始的分块先完成，后续分块停止，任务以 `traffic_rebuild_requires_billing` 失败。超出保留窗口的数据不会被恢复。
- 非法 `tags` 返回 `400 invalid_tags`。
- 每个节点都保存显式账期。新节点默认为 `calendar_month` 且 `traffic_billing_start_day=1`；非 `whmcs_compatible` 模式保存空 `traffic_billing_anchor_date`；空 `traffic_billing_timezone` 在读取时使用应用时区。
- 节点统计方向规范化语义稳定：`default` 继承全局统计方向；`out`、`both`、`max` 覆盖该节点。
- 非法节点流量字段返回 `400 invalid_traffic_cycle_mode`、`invalid_traffic_cycle_settings`、`invalid_traffic_billing_start_day`、`invalid_traffic_billing_anchor_date`、`invalid_traffic_billing_timezone` 或 `invalid_traffic_direction_mode`。
- `POST /api/admin/nodes/{id}/upgrade` 成功返回 `204`；节点无法接收自动下发更新时返回 `409 node_upgrade_unsupported`；打包版本、平台或资产不可用时返回 `409`；Dash 无法生成旧 Node 临时下载授权时返回 `503 node_upgrade_grant_error`。

## 管理分组

- `GET /api/admin/groups/` 返回分组列表；`GET /api/admin/groups/map` 返回节点管理客户端使用的分组查找表。
- `POST /api/admin/groups/` 创建分组；`PATCH /api/admin/groups/{id}` 更新已提交字段；`DELETE /api/admin/groups/{id}` 删除分组。
- 分组名会 trim，不能为空，最多 64 个 Unicode 字符，且不得含控制字符。备注会 trim，最多 255 个 Unicode 字符，且不得含控制字符。创建时分别返回 `400 invalid_name` 或 `invalid_remark`；更新时任一非法值返回 `400 invalid_fields`，两个字段都未提交时返回 `400 no_fields`。

## 管理告警事件

- `GET /api/admin/alerts/events` 返回活跃节点的告警事件 `{ "items": [...], "next_cursor": "...", "has_more": true }`，不包含已删除节点。`has_more=false` 时 `next_cursor` 为 `null`。每项包含 `id`、`rule_id`、`rule_generation`、`server_id`、`server_name`、`server_hostname`、`status`、`metric`、`rule_name`、`first_trigger_at` 和 `last_trigger_at`。`server_ip`、`closed_at`、`current_value`、`effective_threshold`、`close_reason`、`title` 和 `message` 在有值时返回。节点快照过期或暂时不可用不会关闭已有的非离线告警；只有新鲜样本明确证明条件消失后，事件才会报告为已恢复。
- 查询参数：`server_id` 可按节点筛选；`status` 允许 `open`、`closed`、`all`，省略时为 `open`；`metric` 按告警指标名筛选；`from` 和 `to` 为可选 RFC3339 时间，提供时按 `last_trigger_at` 过滤；`limit` 默认 200，最大 500；`cursor` 传入上一页响应的 `next_cursor` 后按 `last_trigger_at DESC, id DESC` 继续读取下一页。
- `GET /api/admin/alerts/events/summary` 返回每台存在未恢复告警的节点摘要 `{ "items": [...] }`。每项包含 `server_id`、`open_count`、`last_trigger_at`、`metric`、`rule_name` 和 `metrics`，用于节点概览显示当前告警状态。`metric` 和 `rule_name` 表示最近一条未恢复事件；`metrics` 按最近触发时间倒序列出未恢复事件指标。摘要不按时间过滤。
- `GET /api/admin/alerts/events/servers` 返回可用于筛选的活跃服务器选项 `{ "items": [{ "id": 1, "name": "..." }] }`。

## 管理告警挂载

- `GET /api/admin/alerts/mounts/` 返回规则、节点及每个节点的挂载状态。
- `PUT /api/admin/alerts/mounts/` 接受非空 `rule_ids`、非空正整数 `server_ids` 和必填布尔值 `mounted`；重复 ID 会合并，未知规则或节点返回 `400 invalid_fields`。数据库更新成功后返回 `204`，后续告警评估在进程内排队；持久化或校验查询失败返回 `503 db_error`，不会返回缓存或告警运行时专用错误。

## 管理告警通知渠道

- `GET /api/admin/alerts/channels` 和 `GET /api/admin/alerts/channels/{id}` 返回脱敏后的渠道配置及投递健康状态。存量配置无法按当前 schema 解码时，对应渠道仍保留在成功响应中，但 `config` 为 `null`；单个非法渠道不会导致整个列表失败。管理界面会继续显示该渠道以供删除，但不允许编辑、启用、重新选作通知目标或测试。`delivery_status` 在首次成功投递前为 `unknown`；成功投递且当前无阻塞任务时为 `healthy`；存在投递失败或阻塞通知时为 `degraded`；渠道停用时为 `disabled`。
- 投递健康字段包括 `last_success_at`、`last_failure_at`、`consecutive_failures`、`last_error_code`、`last_error`、`next_retry_at`、`next_probe_at`、`pending_count` 和 `blocked_count`。没有值的可选时间及错误字段为 `null`。`next_retry_at` 是最早的瞬时失败重试时间，管理端按本地时间显示为 `YYYYMMDD HH:mm:ss`；`next_probe_at` 是最早的阻塞恢复探测时间。`pending_count` 包含待发送、发送中、重试、阻塞和暂停通知；`blocked_count` 是其中等待低频恢复探测的通知数。
- `updated_at` 表示最近一次渠道配置或启停状态变更；后台健康状态更新不会改变 API 返回的该时间。
- 渠道名会 trim，不能为空，最多 64 个 Unicode 字符，且不得含控制字符。非法创建或全量替换请求返回 `400 invalid_fields`。
- Telegram、邮件和 Webhook 的 `config.language` 均接受 `system`、`zh` 或 `en`。`system` 在消息入队时跟随后端 `app.language`；创建渠道或读取存量配置时，缺少该字段等同于 `system`。同类型渠道全量替换时，省略该字段会保留已有显式语言，以兼容旧客户端；改变渠道类型且省略时恢复为 `system`。告警、渠道测试消息和 Dash 更新通知都使用渠道语言。
- 通知标题和正文按渠道语言渲染后写入 outbox。修改渠道语言只影响之后入队的通知；已经入队的通知保留原有文本和语言。
- password、token、hash、session 和 Webhook secret 都是不透明凭据。兼容更新时，字段省略或严格为空字符串才继承存量值；强类型配置字段不接受 JSON `null`。通过校验的非空值不会 trim 或做其他规范化，首尾空白会保留。
- Webhook `url` 必须是包含非空主机名的绝对 HTTP 或 HTTPS URL，不允许用户信息或 fragment。
- `PUT /api/admin/alerts/channels/{id}` 全量替换渠道配置。保存同类型渠道后，处于重试、阻塞或暂停状态的通知会立即唤醒并重置重试次数预算；改变渠道类型会丢弃按旧类型创建的通知。如果请求读取当前 secret/session 后、提交前渠道已发生变化，请求会返回 `409 channel_changed`，不会覆盖更新的 revision。
- `PUT /api/admin/alerts/channels/{id}/enabled` 接受 `{ "enabled": true|false }`。停用会暂停尚未发送的通知；重新启用会立即唤醒这些通知，并保留此前的降级状态直到真实投递成功；单纯重新启用不会把渠道标为健康。
- `POST /api/admin/alerts/channels/{id}/test` 的远端发送窗口最长 10 秒。成功后会把本次受测配置标为健康并立即唤醒阻塞通知；测试失败会记录与后台 worker 相同的结构化投递错误。如果成功恢复状态或失败测试错误中的任一结果持久化失败，API 返回 `503 db_error`，不会返回尚未写入健康状态的测试结论。并发保存新配置时以新配置为准，旧 revision 的测试结果不会修改新配置的健康状态。
- Telegram Bot 收到 HTTP `429` 时，会采用 HTTP `Retry-After` 响应头和 Bot API JSON `parameters.retry_after` 中较长的等待时间，并受 worker 的重试上限约束。
- 通知 HTTP 客户端最多跟随五次重定向。每一跳都必须保持初始主机名和请求方法；同协议跳转必须保持有效端口，HTTP 可以升级到 HTTPS，HTTPS 降级、跨主机、含用户信息及同协议换端口都会被拒绝。通知请求是 POST，因此只跟随保留方法和请求体的 `307`/`308`，会把 POST 改成 GET 的 `301`/`302`/`303` 会被拒绝。Webhook 签名和渠道凭据只会在该跳通过检查后发送。重定向策略拒绝是确定性投递错误，会将渠道任务置为 `blocked` 并进入低频恢复探测，而不是按瞬时网络故障密集重试。
- `/telegram/mtproto/code`、`/verify` 和 `/password` 的 MTProto 登录状态故障返回 `503 login_state_error`。完成登录时只修改发起流程时对应渠道 revision 的 session；渠道被并发替换时，`/verify` 或 `/password` 返回 `409 channel_changed`，必须重新开始登录。
- 删除渠道会同时将其从告警设置中移除，并丢弃该渠道尚未发送的通知。

## 管理系统设置

- `GET /api/admin/system/settings` 返回 `history_guest_access_mode`、`dash_update_channel`、`dash_update_mode`、`logo_url`、`page_title` 和 `topbar_text`。`dash_update_channel` 为 `release` 或 `prerelease`；`dash_update_mode` 为 `manual`、`notify` 或 `auto`。
- `PATCH /api/admin/system/settings` 只校验并更新请求实际提交的字段，因此并发修改不同字段不会互相覆盖，未改动的旧 HTTP Logo 也不会阻断其他字段修改。空更新返回 `400 no_fields`；提交字段非法时返回 `400 invalid_fields`。
- `PUT /api/admin/system/settings` 全量替换设置文档，必须提交 `history_guest_access_mode`、`dash_update_channel`、`dash_update_mode`、`logo_url`、`page_title` 和 `topbar_text`。
- `logo_url` 可以是内置路径、同源绝对路径、base64 SVG、PNG、JPEG、GIF、WebP 或 ICO data URL，或外部 HTTPS URL；外部 HTTP URL 会被拒绝。
- 旧版本已保存的外部 HTTP Logo 会为兼容继续读取。在 HTTPS 页面上，浏览器仍可能按混合内容（mixed content）规则拦截；此时 Logo 和 favicon 会回退到内置 Logo。

## 管理主题

- `GET /api/admin/system/themes/` 返回可读取的内置和自定义主题。每项包含 `id`、`name`、`version`、`author`、`description`、`skin`、`format_version`、`deprecated`、`built_in`、`active`、`deletable`、`missing`、`broken`、`has_preview`、`created_at` 和 `updated_at`。当前主题包均返回 `format_version=1` 和 `deprecated=true`。
- 主题包格式 v1 已冻结并弃用，但没有移除日期；现有 v1 包继续支持上传、应用和运行。v1 不再增加新的 CSS 语法、文件类型或皮肤能力；新增能力必须使用不同的格式版本。
- 主题 manifest 必须包含 `skin.admin.shell`、`skin.admin.frame`、`skin.dashboard.summary` 和 `skin.dashboard.density`；缺失或未知值会拒绝整个主题包。
- 主题 CSS 只允许自定义属性声明。单文件不是有效 UTF-8、超过 1 MiB、声明总数超过 1024、自定义属性名超过 128 字节、值超过 4096 个 Unicode 字符，或值中含 `!important`、`url()`、`image-set()`、`src()`、`expression()` 等可加载资源的函数时，主题包会被拒绝。校验按 CSS 字符串、转义和函数 token 解析；这些单词在引号文本中仍合法，转义函数名也不能绕过限制。
- `POST /api/admin/system/themes/upload` 接受一个 `file` part，其中 ZIP 内容最大 20 MiB；整个 multipart 请求最大 21 MiB，额外 1 MiB 用于边界、头部和其他 framing 开销。Dash 为该上传路由提供最长 5 分钟的请求读取和响应写入窗口。
- 配置的当前主题 ID 即使对应主题包缺失或损坏也继续保留。运行时回退到前端内置默认皮肤；列表返回合成的 `missing` 或 `broken` 项，并通过 `X-Dash-Warning: theme_active_missing` 或 `X-Dash-Warning: theme_active_broken` 暴露状态，管理员仍可重新上传该主题包或选择其他皮肤。

## 管理 Dash 更新

- `GET /api/admin/system/dash-update/status` 返回最近一次 Dash 更新任务状态。`status` 为 `idle`、`running`、`completed` 或 `failed`。响应包含 `available`，并可能包含 `id`、`action`、`channel`、`target_version`、`phase`、`failure_code`、`recovery_path`、`started_at`、`finished_at`、`exit_code`、`log_tail` 和 `unavailable_reason`。必须继续恢复的失败事务使用 `failure_code=recovery_required`；`recovery_path` 指向本机保留的恢复文件。成功向前恢复后任务改为 `completed`；迁移前成功回滚后任务保持 `failed`，使用 `failure_code=rolled_back` 并清空恢复路径。
- `GET /api/admin/system/dash-update/check?channel=release|prerelease` 返回 `current_version`、`current_channel`、`target_channel`、`latest_version`、`install_revision`、`version_status` 和 `bundled_node_version`。`install_revision` 是用于固定后续执行计划的不透明 SHA-256 安装身份。`version_status` 为 `available`、`current`、`ahead` 或 `unknown`。省略 `channel` 时默认使用 `release`；`prerelease` 只检查 prerelease 发布。本机可用性、安装状态和 GitHub 查询在同一个有界响应预算内并行执行；非法 `channel` 返回 `400 invalid_fields`；本机更新器缺失或被恢复事务阻断时返回 `503 dash_update_unavailable`；GitHub Releases 查询失败或超时返回 `502 dash_update_check_failed`。
- `POST /api/admin/system/dash-update/run` 必须提交 `action=update|reinstall`、`channel=release|prerelease` 和 `lang=zh|en`。当前客户端还会把前一次检查中的 `target_version`、`expected_current_version` 和 `expected_install_revision` 原样提交；这三个字段必须同时存在。为兼容旧客户端，三个字段都省略时，服务端会在入队前执行有界且无副作用的新检查；任务提交使用独立的有界窗口。任务一旦持久预留，即使 transient unit 的提交结果暂时不确定，也以其状态资源为权威结果。不可变计划在执行器启动前持久化；执行器取得跨进程锁后校验当前版本和安装修订号。过期计划仍作为任务接收，但最终以 `failure_code=install_changed` 失败，绝不会另选目标。普通 `update` 要求目标版本更高；目标相同时会在任务持久化前返回 `409 dash_update_current`，调用方必须显式使用 `reinstall` 才能重新应用同一 release。成功返回 `202` 和状态体；已有任务返回 `409`；字段非法返回 `400 invalid_fields`；更新器不可用返回 `503 dash_update_unavailable`。
- `dash_update_mode=notify` 会让 Dash 按配置通道定期检查更新，并为每个已启用通知渠道入队可用更新提醒。`dash_update_mode=auto` 会定期检查并在发现更高版本时自动启动更新；无法启动更新器时入队失败通知，已启动任务进入终态后入队结果通知，成功启动本身不再单独通知。各渠道通过持久化通知 outbox 独立投递和重试。
- 管理端控制器基于 Linux/systemd，通过 transient unit 启动打包 Dash 二进制中的 `dash update execute`；`update_dash_linux.sh` 只保留为手工兼容包装。Release 包必须使用格式 v1，包含匹配的 `release.env`、`bin/dash`、`dist/index.html`、`deploy` 下覆盖五个受支持平台/架构目标的全部七个 node/runner 资产、`configs/config.example.yaml` 和 Linux 安装/更新脚本，且这些必需文件不能为空、`Ithiltir-dash/` 下只能有普通文件和目录。`release.env` 以 SHA-256 绑定每个内置资产，候选 Dash 二进制必须同时报告 manifest 中的 Dash 版本和打包节点版本。压缩大小、解压大小和条目数都有硬限制。更新任务可能重启 Dash，调用方收到 `202` 后必须容忍短暂断连。迁移或服务启动需要恢复时，以 root 执行 `DASH_HOME/bin/dash update recover`。
- `GET /api/admin/system/dash-update/release-notes?lang=zh|en` 从文档站返回 `{ "source_url": "...", "html": "..." }`；非法 `lang` 返回 `400 invalid_fields`；抓取失败返回 `502 release_notes_fetch_failed`。

## Node 更新

- `POST /api/node/metrics` 成功响应包含 `update`。
- 无待升级任务时，`update` 为 `null`。
- 有待升级任务时，`update` 包含 `id`、`version`、`url`、`sha256` 和 `size`。
- `url` 可能包含短期有效的 `upgrade_token`，让旧 Node 不发送 `X-Node-Secret` 也能下载本次升级的精确资产。客户端必须按原样使用返回的 URL。
- 待升级任务是易失状态，Node 上报完全相同的目标版本或 SemVer 优先级更高的版本后清除。同一 SemVer 优先级但 build metadata 不同的版本视为不同节点二进制，仍可下发。

## 节点运行时指标字段

- `POST /api/node/metrics` 接受可选的 `metrics.disk.smart`、`metrics.thermal` 和 `metrics.pressure`。旧 Node 可以不带这些字段。
- 持久化的字节数、容量、计数器和 uptime 必须是有符号 64 位范围内的非负 JSON 整数；进程数和连接数使用有符号 32 位范围；`/api/node/static` 的上报间隔使用有符号 32 位范围，CPU 拓扑计数使用有符号 16 位范围。普通正整数的 JSON 编码不变，因此现有 Node 继续兼容。整数超出接收类型范围时返回 `400 invalid_request`；负数、非法比例或非法速率返回 `422 invalid_metrics` 或 `422 invalid_static_payload`。
- 写入 PostgreSQL 定长标识列的文本会在持久化前校验：Node 版本 64 字符，hostname 和磁盘名称 255，磁盘 ref 320，磁盘 kind/role 与 RAID health 16，网卡名称 64，文件系统类型及逻辑盘 health/level 32。静态 OS/platform/arch 限 32 字符，platform/kernel 版本限 255。路径、挂载点和硬件描述使用不定长 TEXT。超长值返回 `422 invalid_metrics` 或 `422 invalid_static_payload`，不会静默截断。
- 并发上报按服务端接收时间决定当前投影。因执行顺序倒置而较晚完成的旧接收样本仍写入指标历史，但不会覆盖当前指标、前台热点快照或触发新的告警评估；请求中的 `timestamp` 只作为 Node 上报时间保存，不决定当前投影顺序。
- `metrics.disk.smart` 是磁盘 SMART 运行时状态，进入独立热点缓存，不写入 PostgreSQL 指标快照。确认是物理盘的 SMART 温度可归约成按设备区分的 `disk.temp_c` 历史值。`metrics.thermal` 保存硬件温度传感器，位置在 metrics 根级；thermal 会写入 PostgreSQL 指标快照，但在前台缓存中作为独立字段缓存保存。
- `metrics.pressure` 是 Linux PSI（Pressure Stall Information）。它可以包含 `cpu`、`memory`、`io`，每项可带 `some` 和 `full` 数值组。每个组包含 `avg10`、`avg60`、`avg300` 百分比和累计 `total` 微秒。Dashboard 会把这些值保存成固定数值时序列；采集状态/原因字符串不持久化。缺失的组保持 `NULL`，表示不可用，不会当成 0 压力。
- `disk.smart.devices` 和 `thermal.sensors` 是数组。字段存在但结果为空时使用 `[]`，不是 `null`。
- `temp_c`、`power_on_hours`、`lifetime_used_percent`、`critical_warning`、`media_errors`、`high_c`、`critical_c` 等可选数值读不到时省略，不转换成 `0`。
- `disk.smart.devices[].critical_warning` 是 NVMe 的原始 critical warning bitset。`disk.smart.devices[].media_errors` 是可读到时上报的 NVMe SMART `media_errors` 计数；告警文案会用 SMART UI 条目号 `0E` 标识。`disk.smart.devices[].failing_attrs[]` 只包含当前 `FAILING_NOW` 的 ATA SMART 属性。
- SMART 和 thermal 的 `status` 是开放字符串。已知值包括 `ok`、`partial`、`unsupported`、`not_found`、`no_permission`、`timeout`、`error`、`no_cache`、`stale`、`no_tool`、`standby`。
- `disk.smart.status` 是采集状态，`disk.smart.devices[].health` 是磁盘健康结果。`status=ok` 且 `health=failed` 表示采集成功但磁盘健康失败。
- `status=no_cache`、`no_tool` 或 `unsupported` 不表示磁盘故障。`status=stale` 会保留最后一次 `devices[]`，同时标记缓存过期。
- `GET /api/front/metrics` 会把节点最新热点快照、SMART 字段缓存和 thermal 字段缓存组合成节点视图，返回 `disk.smart`、`disk.temperature_devices`、顶层 `thermal` 和顶层 `pressure`。`disk.temperature_devices` 是后端推导出的物理盘名称列表，可作为 `disk.temp_c` 历史查询的 `device`。
- `/api/metrics/history` 支持 `cpu.temp_c`、`disk.temp_c` 和 PSI 平均值指标：`pressure.cpu.some_avg10|avg60|avg300`、`pressure.memory.some_avg10|avg60|avg300`、`pressure.memory.full_avg10|avg60|avg300`、`pressure.io.some_avg10|avg60|avg300`、`pressure.io.full_avg10|avg60|avg300`。CPU 温度来自 thermal 的 CPU 传感器。硬盘温度来自确认是物理盘的 SMART 设备；虚拟盘和 RAID 设备不会持久化。带 `device` 时查询 `disk.temperature_devices` 中的单块物理盘；不带 `device` 时聚合已持久化的物理盘记录。`30m`、`1h`、`12h`、`24h` 查询 raw；`1w` 查询 15 分钟聚合；`15d` 通过加权的 15 分钟聚合生成 30 分钟点；`30d` 查询 1 小时聚合。温度和公开 PSI 平均值使用相同聚合路径，聚合查询包含最新 real-time raw 尾部。

## 告警指标

- 内置 SMART 健康失败和 NVMe 关键告警规则和内置 RAID 失效规则一样默认挂载。
- 用户自定义规则可使用 `disk.smart.failed`、`disk.smart.nvme.critical_warning`、`disk.smart.attribute_failing`、`disk.smart.max_temp_c` 和 `thermal.max_temp_c`。
- `disk.smart.failed` 只统计 SMART 健康结果为 `failed` 的设备，不把 `no_cache`、`no_tool`、`unsupported` 或其他采集状态计为磁盘故障。
- `disk.smart.nvme.critical_warning` 统计 `critical_warning` bitset 非 0 的设备数。`disk.smart.attribute_failing` 统计当前 `FAILING_NOW` 的 SMART 属性数。
- 缺失 `disk.smart` 不表示 SMART 故障。节点没有 SMART 上报时，内置 SMART 规则不会触发。
- PSI pressure 数据只保存并用于历史查询。

## 流量统计

- `GET /api/statistics/traffic/settings` 返回 `guest_access_mode`、`usage_mode`、`cycle_mode`、`billing_start_day`、`billing_anchor_date`、`billing_timezone` 和 `direction_mode`。其中账期字段仅用于兼容旧响应结构，固定表示应用时区中从 1 号开始的自然月，不再是可修改的全局默认值。
- `PATCH /api/statistics/traffic/settings` 只接受 `guest_access_mode`、`usage_mode` 和 `direction_mode` 的局部更新。提交任意账期字段返回 `400 billing_cycle_is_per_node`；账期必须通过节点接口修改。Lite 切换为 Billing 时，后台 Facts 物化器从最近 30 分钟开始，`204` 响应不会等待物化完成；Lite 期间更早且仍在保留期内的节点事实可通过节点重建接口按需补齐。Billing 切换为 Lite 不等待运行中的重建任务；该任务会在下一分块检查时停止。成功返回 `204`。
- 可修改字段的允许值：`guest_access_mode`: `disabled`、`by_node`；`usage_mode`: `lite`、`billing`；`direction_mode`: `out`、`both`、`max`。
- 流量查询使用每个节点显式保存的账期字段。只有 `traffic_direction_mode=default` 会继承全局统计方向；`out`、`both`、`max` 仍是节点自己的方向覆盖。
- `lite` 和 `billing` 模式都会使用有效账期作为月度边界；`billing` 额外启用日统计、P95、覆盖率、5 分钟事实和月度计费快照。
- `GET /daily` 要求 `usage_mode=billing`，否则返回 `409 traffic_daily_requires_billing`。`period` 可选，允许 `current`、`previous`，省略时为 `current`。
- `GET /monthly` 支持 `months` 和 `period`。`months` 必须在 1 到 24 之间，非法值返回 `400 invalid_request`；`period=current` 从本账期开始，`period=previous` 从上账期开始，省略时为 `current`。响应字段 `includes_current` 在 `period=current` 时为 `true`，在 `period=previous` 时为 `false`。
- 统计方向用于选择计费视图：出站、入站加出站，或每项指标取入站/出站较大值。
- 流量 summary、daily、monthly 响应保留原始 `in_*` 和 `out_*` 字段，并通过 `selected_bytes`、`selected_p95_bytes_per_sec`、`selected_peak_bytes_per_sec` 及其方向字段暴露当前计费视图。
- 客户端使用 `coverage_ratio`、`data_complete`、`gap_count` 和 `reset_count` 展示样本覆盖率和准确性提示。
- 只有 `p95_status` 为 `available` 时，P95 字段才不是 `null`。

## 非 API HTTP 路径

| 路径                          | 作用                                                                       |
| ----------------------------- | -------------------------------------------------------------------------- |
| `/theme/active.css`           | 实际解析出的主题 CSS；配置主题缺失或损坏时返回用于前端默认皮肤的空覆盖 CSS |
| `/theme/active.json`          | 实际解析出的主题 manifest；默认主题及回退到默认主题时返回 404              |
| `/theme/preview/{id}.png`     | 主题预览图                                                                 |
| `/theme-bootstrap.js`         | 启动主题引导脚本；固定使用 `Cache-Control: no-store`                       |
| `/deploy/linux/install.sh`    | Linux Node 安装脚本                                                        |
| `/deploy/macos/install.sh`    | macOS Node 安装脚本                                                        |
| `/deploy/windows/install.ps1` | Windows Node 安装脚本                                                      |
| `/deploy/*`                   | 打包携带的节点发布资产；需要 `X-Node-Secret` 或临时 `upgrade_token`        |
| `/`                           | SPA                                                                        |

## 契约规则

- 未知或格式错误的值在边界直接拒绝，不会静默归一化成另一个合法请求。
- JSON 请求超过路由 body 上限时返回 `413 body_too_large`；JSON 格式错误通常返回 `400 invalid_request`，`POST /api/auth/login` 使用鉴权契约的 `400 invalid_json`。
- 核心持久化存储和必需依赖失败会明确返回错误。文档声明的可选边界保留降级语义：Bearer 可选读取转为匿名视图，当前主题不可用时使用前端默认皮肤并暴露 `missing` 或 `broken` 状态。
