# 破坏性变更

## 未发布

### 运行配置

- `monitor_dash_pwd` 至少包含 8 个可见 ASCII 字符，且不得包含空白字符；现有安装若使用更短的管理员密码，必须在启动升级后的 Dash 前完成替换。
- `app.public_url` 现在只接受 IP 字面量或 ASCII DNS 名称，端口范围为 1～65535；国际化域名必须配置为 IDNA/punycode 形式。
- 未知 YAML 字段和显式填写的非法时长会在启动时被拒绝，不再静默忽略或替换为默认值；省略时长字段仍使用文档定义的默认值。

### 通知渠道配置

- 通知渠道在列表、更新、测试和发送通知时，均按当前严格 schema 校验数据库中的配置。
- `api_id`、`smtp_port` 等整数字段不再接受数字字符串或值为整数的 JSON 浮点数。
- 未知配置字段会被拒绝。
- 不符合当前 schema 的存量配置必须重新创建，或直接在 PostgreSQL 中修正后，才能继续使用或通过 API 编辑。
- 通知投递不再使用静默的 `failed_permanent` 终态。升级时，已有该状态的行会迁移为 `blocked` 并恢复低频自动探测；因此远端恢复后，历史通知可能再次送达。
- 停用渠道会暂停未发送通知；重新启用或以兼容类型替换配置会唤醒这些通知。删除渠道或改变渠道类型会明确丢弃不兼容的未发送通知。
- 渠道列表和详情响应新增投递健康字段，以及 `unknown`、`healthy`、`degraded`、`disabled` 四种 `delivery_status`。
- 渠道列表和详情响应还会返回 `next_retry_at`；唤醒兼容任务时会先重置其重试次数。
- 渠道名最多允许 64 个 Unicode 字符，且不得含控制字符。
- Webhook URL 只允许 HTTP(S)，不得含用户信息或 fragment，且最多 4096 字节。通知 HTTP 客户端最多跟随五次指向初始主机名的重定向；同协议跳转必须保持有效端口，只允许 HTTP 升级到 HTTPS，并在转发凭据前拒绝不安全跳转。POST 通知只跟随保留方法和请求体的 `307`/`308`；会降级为 GET 的 `301`/`302`/`303` 以及其他重定向策略拒绝会被记为 `blocked`，不再误按瞬时发送失败重试。邮件地址改为严格解析，收件人最多 100 个；所有渠道类型都对字段大小设置上限。
- 密码、token 和 secret 占位值仅在字段省略或值严格为空字符串时继承存量值。非空凭据材料不再 trim，首尾空白会作为凭据数据保留。
- MTProto 登录状态故障改用 `503 login_state_error`，不再使用绑定 Redis 实现的 `redis_error`。目标渠道在登录期间被替换时，完成登录返回 `409 channel_changed`，不会覆盖较新的配置。

### 节点标签

- 每个节点最多允许 32 个标签。
- 每个标签最多允许 64 个 Unicode 字符，且不得包含控制字符。
- 新写入的标签必须满足这些限制。读取存量标签时，Dash 会记录警告并丢弃不符合限制或超过数量上限的标签；存量 JSON 无法解析时整组标签为空。节点和其他指标仍会进入前台快照。

### 节点和分组元数据

- 节点名最多允许 64 个 Unicode 字符，且不得含控制字符。
- 新提交的节点 secret 会 trim，且必须包含 8～128 个 Unicode 字符；存量更短 secret 在轮换前仍可继续使用。
- Node 上报标识超过文档约定的 PostgreSQL 边界时，会在持久化前被拒绝；超长指标/静态上报现在返回 `422`，不再落到数据库错误。hostname 和磁盘标识已扩宽；操作系统路径、挂载点和硬件描述改用 `TEXT`。
- 升级迁移会在修改这些列类型前临时解压保留期内的磁盘指标 chunk，并重建可丢弃的连续聚合。运维侧需要预留临时数据库空间；迁移会恢复压缩策略，符合条件的 chunk 会在之后重新压缩。
- 分组名不能为空且最多允许 64 个 Unicode 字符；分组备注最多允许 255 个 Unicode 字符；两者均不得含控制字符。
- 空字符串不再代表节点流量模式或全局 `usage_mode=lite`；客户端必须提交允许的明确模式值。

### 告警规则

- 告警规则名称最多允许 128 个 Unicode 字符，且不得包含控制字符。
- 阈值和阈值偏移必须是有限数值；持续时间只允许 0、60 或 300 秒；冷却时间最多为 525600 分钟。
- 不满足当前规则约束的存量规则会被标记为非法；启动后的全量协调会以 `rule_invalid` 关闭该规则仍处于开放状态的告警事件。

### 运行配置

- `auth.jwt_signing_key` 至少为 32 字节，且不得包含首尾空白。修改该密钥会使现有登录会话失效。
- 默认 Redis 模式要求 `redis.addr` 非空、实际服务端版本不低于 `8.2.3`，且配置账号允许执行 `PING` 和 `INFO server`。不满足条件时 Dash 停止启动；`--no-redis` 跳过这些 Redis 要求。
- Redis 默认继续保存管理员会话，并保存可丢弃的前台缓存；`--no-redis` 才把会话和前台缓存放入进程内存。告警评估运行态和 MTProto 登录握手改为进程内状态，重启后重置；开放中的 firing 告警会从 PostgreSQL 恢复，pending 和 cooldown 不会恢复。
- 前台缓存迁移到带项目 namespace 的 `ithiltir:dash:front:v2:*` key 布局。已有 v1、未加 namespace 的 v2、告警运行态和 MTProto 登录态 key 会被忽略、不做双写，也不会在启动时自动删除。`auth:jwt:*` 作为会话兼容前缀继续使用，升级不会主动清除现有登录凭证。

### 皮肤清单

- 皮肤清单不允许未知字段。
- `skin.admin.shell`、`skin.admin.frame`、`skin.dashboard.summary` 和 `skin.dashboard.density` 均为必填字段，省略时不再补默认值。
- 不符合当前清单 schema 的存量自定义皮肤会被标记为损坏；替换皮肤包前，运行时使用默认皮肤。
- 主题 CSS 必须是有效 UTF-8，并新增明确的文件、声明数、属性名和值长度上限。可加载资源的函数和 `!important` 会在 CSS 转义解码后被拒绝；引号文本按语法结构解析，不再因为包含禁用单词而被误杀。

### 站点品牌

- 新写入的 `logo_url` 只接受同源绝对路径、base64 SVG/PNG/JPEG/GIF/WebP/ICO data URL 或外部 HTTPS URL。外部 HTTP URL、URL 凭据、非法 data URL 和其他图片媒体类型会被拒绝。
- 存量 HTTP Logo 为兼容继续读取，但 HTTPS 页面可能按混合内容规则拦截；此时 UI 回退到内置 Logo。

### 流量统计 API

- 账期改为归单节点所有。`PATCH /api/statistics/traffic/settings` 收到 `cycle_mode`、`billing_start_day`、`billing_anchor_date` 或 `billing_timezone` 时返回 `400 billing_cycle_is_per_node`；管理界面不再提供全局账期控件。
- 升级迁移会把所有存量 `traffic_cycle_mode=default` 节点解析为迁移前实际继承的全局账期，不排入重算，也不改变已有账期边界。新节点默认显式使用从 1 号开始的自然月。节点 PATCH 仍兼容旧输入 `traffic_cycle_mode=default`，但会规范化为该显式自然月，后续读取不再返回 `default`。
- 流量统计响应不再包含已废弃的 `stats.partial` 字段。客户端必须使用 `data_complete`、`coverage_ratio`、`gap_count` 和 `reset_count`。
- `GET /api/statistics/traffic/monthly` 只接受 1 到 24 的 `months`；超出范围返回 `400 invalid_request`，不再自动截断到 24。
- 节点流量重建只在 Billing 模式可用；Lite 模式返回 `409 traffic_rebuild_requires_billing`。运行中切换为 Lite 时，重建在下一分块检查停止。
- 升级迁移会把已有 `traffic_month_usage` 的 `covered_from` 初始化为对应 `cycle_start`，以保持旧版本“完整账期”的展示语义；这个值是兼容性假设，不是由历史原始采样证明的覆盖范围。
- 新的流量物化进度从升级时最近 30 分钟开始。升级前更早、但仍在原始指标保留期内的积压不会自动重放；Billing 模式可按节点重建 5 分钟事实，Lite 的既有累计行则继续按上一条兼容性假设展示。

### 请求错误

- JSON 请求 body 超过上限时返回 `413 body_too_large`，不再与格式错误 JSON 一样返回 `400 invalid_request`。

### Dash 更新归档

- Linux 更新器现为原生 `dash update` 子命令；`update_dash_linux.sh` 只保留为参数兼容包装。新的 Linux 归档必须包含版本一致的 `bin/dash`，以及为每个内置 node/runner 资产记录 SHA-256 的 v1 `release.env`；候选二进制必须报告 manifest 中的 Dash 版本和打包节点版本。缺少这套 manifest 契约的历史归档会被内置更新器拒绝。
- 归档只允许单一根目录下的普通文件和目录，并限制为 1 GiB 压缩大小、4 GiB 解压大小和 20000 个条目。迁移失败时不再把旧二进制恢复到可能已经前移的数据库上；Dash 会保持停止并保留恢复文件。
- `goose_db_version` 是唯一数据库 schema 版本。服务启动现在要求它与二进制内置迁移完全一致；`dash migrate` 只会推进较旧 schema，并拒绝较新 schema。schema 升级后启动旧二进制不受支持。

### Dash 手动安装

- `install_dash_linux.sh` 只支持在全新主机上执行一次首次安装，不是重装或更新命令；后续所有版本变更都使用打包的 `dash update` 执行器。原有 `update_dash_linux.sh` 命令通过兼容包装继续可用。
- 手动依赖模式在收集配置前，要求本机已安装 PostgreSQL 16+，以及与其 PostgreSQL 主版本匹配的 TimescaleDB；不要求本机存在 `redis-server` 二进制。
- 收集 Redis 配置后，安装器会使用包内 Dash 二进制校验实际配置的端点，包括连通性、`PING`、`INFO server` 和 Redis 8.2.3+ 版本下限；支持仅使用远程 Redis。

### 节点安装器重定向

- Linux、macOS 和 Windows 节点安装器不再在携带 `X-Node-Secret` 时跟随任意重定向。最多允许五次同主机跳转；同协议跳转必须保持有效端口，允许 HTTP 升级到 HTTPS，并拒绝 HTTPS 降级和跨主机跳转。
