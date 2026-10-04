# 架构

Ithiltir Dash 是单实例应用。根入口只启动一个 HTTP 进程，该进程装配 API、SPA、主题资源、安装脚本、节点资产分发和后台服务。

## 运行边界

| 组件                     | 责任                                                                                                 |
| ------------------------ | ---------------------------------------------------------------------------------------------------- |
| `cmd/dash`               | 进程入口、配置加载、依赖装配、迁移、更新、Redis 检查和关闭流程                                       |
| HTTP 服务                | 挂载 `/api`、`/theme`、`/deploy` 和 SPA                                                              |
| PostgreSQL + TimescaleDB | 持久化指标历史、流量事实及物化进度、节点元数据、告警规则、加密通知配置、通知 outbox 和系统设置       |
| Redis                    | 管理员会话；可丢弃的前台运行快照/元数据、SMART/thermal 字段、节点目录和游客可见目录                  |
| 进程内内存               | 解密后的通知配置、`--no-redis` 管理员会话、节点鉴权索引、告警队列/最新快照/运行态、MTProto 登录态和易失 Node 更新请求  |
| Dash home 文件系统       | 仅文件所有者可读的通知配置密钥（受管 Linux 安装由 root 持有）和可变安装/运行状态                  |
| Node                     | 上报指标和静态主机信息；接收更新 manifest                                                            |
| Linux 节点服务管理       | systemd 或 OpenRC/supervise-daemon 管理前台 Node 进程；显式 `none` 模式由运维者持有进程生命周期      |
| Linux root 侧缓存        | systemd timer 刷新 SMART、连接数和 LVM；Alpine/OpenRC 只用 BusyBox cron 刷新 SMART 和 LVM            |
| Web UI                   | 读取看板数据并提交管理操作                                                                           |

## PVE 采集与上报

PVE 监控使用同一种 node 二进制和现有节点身份。可选的 Go `pve-cache` 工具随 node 版本发布，由 Linux 安装器安装在 root 持有的 `/usr/local/libexec/ithiltir-node/pve-cache`。systemd 服务运行 `pve-cache --serve`，以 root 查询本机 `/nodes/{host}/qemu` 完整状态，并从 `/cluster/resources?type=vm` 补充 PVE 统计服务维护的 CPU 比例；只按本机名称和 QEMU VMID 匹配，不上报其他宿主机。每次结束后 30 秒再次采集，基础指标查询共用 20 秒超时。安装时不编译代码，也不配置 PVE 用户或 Token。

Guest Agent IP 通过可选 `ips` 数组返回，每台 VM 最多 128 个不重复的 IPv4/IPv6 地址，包含私网地址，排除回环、链路本地、未指定、多播及非法地址。需要在 PVE 启用 Guest Agent，并在来宾内运行代理。IP 缺失不使基础采集失败。 IP 由 `pve-cache --serve` 内的独立慢任务采集（手动单次采集使用 `--guest`）：调度器在任务结束 60 秒后唤醒，成功的 VM 间隔 5 分钟再查，失败按 5、10、20、30 分钟退避（上限 30 分钟）。只查询新鲜清单中运行、非模板、未暂停的 VM。最多四路并发，单次查询超时 5 秒，慢任务总预算 50 秒。每台 VM 使用由查询进程继承的文件锁；超时终止进程组并等待退出，仍存活的查询会阻止该 VM 再次启动查询。启动前将退避状态写入仅 root 可读的 `/run/ithiltir-node/pve-guest`，该状态为易失数据，重启后重建。热采集只读此缓存。

可选字段 `ips_collected_at`、`ips_ttl_seconds` 独立描述 IP 样本新鲜度。helper 使用 900 秒 IP TTL；复用时保留原采样时间，省略过期样本以及已知早于 VM 本次启动的样本。成功查询无地址时可以只有新鲜度字段而没有 `ips`。查询失败仅在原 IP 样本未过期时继续使用；读取方必须单独判断 IP 新鲜度，不能只看快照 `stale`。兼容不带这两个字段的报告，此时 IP 新鲜度未知；提供时，时间必须非零且不晚于 `collected_at`，TTL 为 1–3600。

工具通过临时文件和原子 rename 写入 `/run/ithiltir-node/virt.json`，文件 root 所有、运行组可读。采集失败保留最后成功的 VM 列表和时间，并记录本次失败。该缓存是易失观测，重启后由定时任务重建。root 任务不执行普通 node 可写的 release 树中的程序。

安装器通过 systemd drop-in 设置 `ITHILTIR_NODE_VIRT_CACHE`。未设置时 node 不启动 VM 推送任务；设置后每个上报目标具有独立的缓存轮询、传输超时和失败退避，沿用目标 URL 和密钥向同级 `/virt` 发送。只保留最新样本，不积累报告队列；缓存不可用报告错误，VM 推送失败不阻塞宿主采样和上报。旧 Dash 返回不支持该接口时，VM 任务退避重试，宿主上报继续。

Dash 在 PostgreSQL `server_virt` 中每宿主保存一份最新 JSONB 快照，独立于宿主指标、Uptime、Redis 前台缓存和计费。采样时间单调更新；接收时间不会因重复样本而刷新。鉴权复查与写入使用节点生命周期锁，节点软删除在同一数据库事务中移除 VM 快照。管理员通过独立读取接口获取快照及新鲜度；不向游客公开，没有 UI。VM 历史通过节点查询会话按需读取 PVE RRD。

helper 的安装和更新属于特权安装流程；node 自更新只更新普通 node，不替换 helper。版本化缓存 schema 保持跨 node 更新兼容。可选资产使用 `deploy/linux/pve-cache.env` 独立校验清单；`release.env` v1 保持不变，旧包可以不含 PVE 资产。支持新清单的 Dash 更新器和 PVE 安装器验证版本及 SHA-256；旧 Dash 更新器仍只验证其已知的七项资产，PVE 安装前仍会执行 helper 校验。

## HTTP 面

各路由模块返回 EiluneKit Blueprint，只汇总直属子模块。HTTP 根入口组合 API、主题和各平台安装脚本路由，再通过 `routes.NewHandler` 构造唯一 handler。应用不直接向 chi 注册路由；构造结果由 `http.Server` 在服务生命周期内持有。

请求 ID、安全响应头、访问日志、panic 恢复，以及 API 和方法边界通过 `HandlerOptions.Middleware` 执行，覆盖路由匹配失败的请求。应用决定 `/api` 范围及其缓存和请求体限制策略。已实现的方法由最终路由表和静态回退的 GET/HEAD 组成；方法检查区分大小写，服务未实现的方法在路由前返回 501。对于已实现但端点不允许的方法，Kit 在调用应用的 405 handler 前设置 `Allow`。API 错误使用 JSON，非 API 的 405 使用空响应体。响应头发送前的 panic 保持空响应体的 500，响应开始后的 panic 中止请求。

安装脚本声明为明确的 GET/HEAD 路由。未匹配的 `/deploy/...` 路径交给带鉴权的文件 handler。其余非 API 路径通过 not-found 回调交给 SPA 文件 handler。方法错误不会重新进入这些回退。这些回退不注册为通配路由。SPA 历史路由仅在文件不存在时回退；文件系统权限及 I/O 错误保留文件服务错误响应。

| 前缀              | 作用                                 |
| ----------------- | ------------------------------------ |
| `/api/auth`       | 登录、续期、登出、会话撤销           |
| `/api/version`    | Dash 版本和打包携带的节点版本        |
| `/api/front`      | 看板读取                             |
| `/api/metrics`    | 历史指标和在线率查询                 |
| `/api/statistics` | 统计访问策略和流量统计查询           |
| `/api/node`       | Node 上报和节点身份读取              |
| `/api/admin`      | 管理台写操作                         |
| `/theme`          | 当前主题 CSS、主题 manifest 和预览图 |
| `/deploy`         | 安装脚本和节点发布资产               |
| `/`               | SPA                                  |

## 数据流

1. Node 通过 `/api/node/*` 上报指标和静态主机信息。
   指标在鉴权和解码前记录服务端接收时间，入库截止时间为该时间后 5 秒；鉴权、请求体读取、节点锁等待和数据库事务共用这一预算。超时上报不会在后续 Uptime 窗口才进入指标存储。
2. 指标上报成功响应可包含更新 manifest。
3. PostgreSQL + TimescaleDB 保存持久化历史、流量事实、普通配置、加密后的通知渠道配置和通知 outbox。
4. 默认模式下 Redis 保存管理员会话和可丢弃的前台缓存；`--no-redis` 用进程内内存替代两者。告警运行态和 MTProto 登录握手始终留在单个 Dash 进程内。
5. 后台服务评估告警、发送队列通知并汇总流量数据。

节点 IP 是已鉴权 Node 请求的观察值：有 `X-Forwarded-For` 时 Dash 取其第一个 IP，否则回退到 `RemoteAddr`；不可解析的值不会被使用。该字段用于展示和运维，不作为鉴权边界。

## 状态和保留策略

- 节点本地生命周期锁的等待遵守调用方取消和截止时间。取消的等待释放锁引用，不执行变更；不同节点仍可独立运行。结构读写锁不支持取消。指标报告在写入关系表前限制磁盘 base IO、逻辑盘、网卡和 SMART 设备的提交项合计最多 1024 项。历史记录使用服务端接收时间，不按相同的 Node 时间戳去重。
- Uptime 的游客展示开关和每日 SLA 阈值持久化在 `system_settings`，默认开启游客展示，warning 为 99%、error 为 95%。数据库约束保证 `0 <= error < warning <= 100`，设置更新沿用同一事务，非法阈值不会留下部分更新。Uptime 接口校验游客展示权限，并返回供卡片着色的阈值；这些设置不影响采样。

- `node_online` 保存 `(server_id, minute, online_ms, observed_ms)`，每个节点每个已结束分钟至多一条时长结果。TimescaleDB 的 `sample_node_online` 任务在整分后 6 秒执行，给上报写入的 5 秒超时窗口留出余量，再读取上一分钟。每条已持久化的 `server_metrics.collected_at` 上报形成 `[接收时间, 接收时间 + app.node_offline_threshold)` 有效区间（阈值默认 17 秒），合并重叠区间并裁剪到目标分钟和节点统计起点。统计起点沿用首次创建当前指标行的 `server_current_metrics.created_at`，后续上报不改写。没有当前指标行和已软删除节点不生成记录；删除后的已有时长保留至过期。迁移和启动同步阈值，Go 不运行 uptime 采样循环。重复执行同一分钟会幂等重算，包括先前读取后才提交的报告。
- `node_online` 缺少记录表示未知，任务不补写错过的分钟或更早历史，并跳过跨越 PostgreSQL 重启的分钟。数据库可用时，节点停止上报或 Dash 停止，超过阈值的部分计离线。它衡量上报可用性，并非独立的网络可达性；没有留下故障记录的数据库或接收链路异常，无法仅凭上报时间与节点掉线区分。时序表按一天分块并保留 46 天，策略按整个分块清理。任务调度与失败可通过 `timescaledb_information.jobs`、`job_stats`、`job_errors` 查看。现有在线率 API 仍读取 `server_online_30m`；预留的 `services` 和 `service_checks` 表保持独立。
- `node_online_1h` 是启用实时合并的小时连续聚合，累加 `online_ms` 和 `observed_ms`，在当地整点后 10 秒执行，物化终点向前保留 1 小时。上一完整小时和当前小时由原表实时合并，分钟任务延迟时无需依赖它先于小时刷新完成；仍未结束的分钟不参与。分钟结果和小时聚合均保留 46 天。`/api/metrics/uptime` 将小时的时长汇总为 45 个日历日，`/api/metrics/uptime/day` 读取同一视图。比例按观测时长计算；普通完整小时有 3,600,000 观测毫秒，首次上报、不完整小时、缺失分钟和夏令时切换时可能不同。迁移和启动将小时桶对齐 `app.timezone`；修改时区只重建小时视图。未命名的本地时区须能从宿主配置解析，否则须显式配置 IANA `app.timezone`。
- 卡片日统计拥有独立的 60 秒轮询生命周期；小时明细只在浮层展开时读取，各卡片缓存至多 45 个日期、60 秒过期，离开页面或失去访问权限时清除。CPU、内存轮询不会重新请求或重绘未变化的 Uptime。迁移或启动时先执行一次尚未成功运行过的小时归集任务，在对外服务前物化历史；之后由数据库按小时调度。

- 默认启动依赖 PostgreSQL 和 Redis `6.2.0+`，推荐 Redis `8.2.3+`。Dash 会通过 `PING` 和 `INFO server` 校验实际连接的服务端，因此配置的 Redis 账号必须允许这两个命令；服务不可用、版本无法识别或低于 6.2.0 时终止启动，低于 8.2.3 时仍可运行但会记录启动警告。Redis 保存管理员会话和可丢弃的前台缓存，单次 Redis 故障不会回退到内存。传 `--no-redis` 时会跳过 Redis 连接和版本校验，并从启动时把会话与前台缓存装配到进程内内存。 Redis 网络操作同时遵守调用方 context 截止时间和 socket 超时；前台缓存与认证会话操作各自保留原有超时预算。
- `app.timezone` 在启动时编译。空值使用本地时区；非空值必须是有效 IANA 时区名，否则配置加载失败，错误中会包含配置值。
- 前台缓存 v2 使用项目 namespace `ithiltir:dash:`，具体 key 为 `ithiltir:dash:front:v2:node:runtime:{id}`、`ithiltir:dash:front:v2:node:meta:{id}`、`ithiltir:dash:front:v2:node:smart:{id}`、`ithiltir:dash:front:v2:node:thermal:{id}`、`ithiltir:dash:front:v2:node:ids`、`ithiltir:dash:front:v2:node:catalog`、`ithiltir:dash:front:v2:guest:ids` 和 `ithiltir:dash:front:v2:guest:catalog`。旧 v1 和未加 namespace 的 v2 缓存 key 会被忽略，不做双写，也不会在启动时自动删除；冷缓存按需重建。管理员会话继续使用兼容前缀 `auth:jwt:*`，保证存量 session 在升级后仍然有效，该前缀不会在启动时迁移或删除。
- PostgreSQL 是节点元数据和当前指标的权威来源；对节点投影而言，Redis 或 `--no-redis` 的内存后端只是前台读取索引。运行指标与 PostgreSQL 派生元数据分开保存，读取时组合成公开契约不变的 `NodeView`。普通已接受指标只更新 runtime/SMART/thermal，不改变目录投影 generation。当前目录中不存在的 ID 上报 runtime 时会保留样本并让目录失效，但绝不自行加入目录；成员和元数据只能由 PostgreSQL 投影重建决定。全量重建会替换元数据和目录，但只在 runtime 不存在时补写，因此不会覆盖并发到达的新样本，持续上报也不会让重建饥饿。节点生命周期串行化由 node 领域持有：全局变更使用结构锁独占侧，单节点投影变更使用共享侧和节点本地锁，高频 runtime 写入只使用节点本地锁；不同节点仍可并发，任何单节点投影变更都不能与全局变更重叠。独立 generation gate 负责条件发布重建，数据库或缓存 I/O 期间不持有其互斥量。代表逻辑盘的路径、文件系统类型和容量是一组 last-known 观测：上报没有可用路径时整组保留旧观测；存在路径时整组替换，未上报的类型或容量记为未知。节点名称/排序/标签和影响前台的静态字段只失效元数据；游客可见性只失效游客目录；流量设置、密钥、分组、hostname/IP/Node 版本、CPU 厂商/频率、磁盘总量、RAID 能力和上报周期不写前台 Redis。批量排序和节点成员变化失效节点目录。PostgreSQL 元数据事务在提交前执行必要的 Redis 失效；Redis 失败会回滚 PostgreSQL，之后若 PostgreSQL 提交失败则只留下缓存 miss。节点删除在提交前只失效目录，成功提交后才删除运行快照。
- 在已鉴权节点上报边界，写入 PostgreSQL 的数值直接解码成对应的 Go 有符号宽度：JSON 解码负责可表示范围，语义校验负责非负计数器和数值领域约束，持久化过程不再执行窄化转换。定长协议标识在现有报告遍历中校验长度；hostname、操作系统路径、挂载点和硬件描述使用足以表达正常系统观察值的存储类型。Node 的普通正整数 JSON 编码不变，继续保持兼容。
- 默认模式下管理员会话保存在 Redis，可跨 Dash 进程重启和原地升级继续使用；`--no-redis` 模式下会话属于进程内存并在重启后失效。告警评估状态和 MTProto 登录握手在两种模式下都属于进程内存，重启后有意重置。开放中的 firing 告警从 PostgreSQL 事件恢复；pending 和 cooldown 不承诺跨重启保留。指标提交把最新评估快照放入内存队列，规则/全量协调缺少快照时直接读取 PostgreSQL 当前指标，绝不读取前台 Redis。快照缺失、非法、过期或可选运行字段缺失时保留已有 firing 状态；只有新鲜样本明确证明条件消失时才按恢复关闭告警。
- SPA 根级运行时负责浏览器资源版本一致性。手工提交更新时，目标版本作为不可变状态保存在当前浏览器 session；根级运行时随后用恢复覆盖层阻止继续操作旧界面，并每 10 秒请求一次本地 `/api/version`，单次超时 5 秒。已安装 SemVer 大于等于目标版本后，覆盖层等待用户明确确认再重新加载文档。迁移耗时不用于判断失败；只有后台任务明确进入 `failed` 终态时，session 目标才转为失败状态，覆盖层停止轮询并等待用户确认关闭后查看更新详情。显式更新流程之外，页面可见时仍会检查本地版本，版本变化就自动重新加载整个文档，不受当前页面影响；远端 release 是否可访问不会阻止加载本地已经安装的新前端。
- Dash 只有一个管理用户、一个运行实例和一个内置安装后执行器：`DASH_HOME/bin/dash update`。`install_dash_linux.sh` 只负责首次安装；`update_dash_linux.sh` 只是没有更新逻辑的兼容包装。Dash 与自动更新服务都是控制器：先从 GitHub Releases 解析目标，把目标版本、预期当前版本和预期安装修订号组成不可变计划，持久化到 `DASH_HOME/runtime/dash-update/jobs/<job-id>`，再通过 systemd transient unit 启动隐藏的 `dash update execute` 命令。执行器先取得 root 所有的跨进程锁，再读取已安装状态并执行 compare-and-swap；过期计划会失败，不会重新选择目标或把安装回退。手工更新使用同一个执行器和锁。管理台控制器仍只支持 systemd；执行器本身也支持显式手动安装。任务来源、阶段、状态、失败代码、恢复路径和日志都原子持久化；独立的任务 `current` 符号链接只选择管理 API 展示的任务。完成切换的事务会作为恢复证据保留到任务终态可靠落盘，执行器崩溃后也可由状态协调从该事务恢复真实终态。自动通知扫描所有未确认的终态自动任务，并以任务 ID 作为持久化 outbox 幂等键。
- 打包文件位于 `DASH_HOME/releases` 下的不可变目录，由原子 `current` 符号链接选择当前 release；旧平铺路径在迁移窗口内保留为兼容别名，可变运行时和配置仍位于 `DASH_HOME`。Linux 发布格式 v1 归档必须包含匹配的 `release.env`、`bin/dash`、`dist/index.html`、`deploy` 下覆盖五个受支持平台/架构目标的全部七个 node/runner 资产、`configs/config.example.yaml` 和 Linux 安装/更新脚本。manifest 以 SHA-256 绑定每个内置资产；停止线上进程前，候选 Dash 二进制必须同时报告匹配的 Dash 版本和打包节点版本。官方前端构建还要求非空的 `dist/theme-bootstrap.js`；任意自定义前端产物不属于发布契约。归档只允许单一根目录下的普通文件和目录，并限制为 1 GiB 压缩大小、4 GiB 解压大小和 20000 个条目。暂存目录和旧平铺布局恢复资产位于安装目录旁的同一文件系统。执行器保持现有 systemd/手动运行方式和受管服务更新前的运行状态。systemd 服务重启后必须连续五秒保持 `active/running` 且 `NRestarts` 不增加，更新才能结束；稳定检查失败时会恢复 `update.block` 并停止服务等待恢复。手工运行模式只停止受管安装目录中以服务模式启动的 Dash 命令行，不会终止维护子命令。
- `runtime/dash-update/transaction.env` 是持久化切换记录。停止 systemd 管理的 Dash 前，执行器会安装永久服务条件并创建 `update.block`；因此重启或执行器异常退出都不能在迁移前或部分迁移区间拉起 Dash。迁移开始前，恢复会还原之前的 release 和运行状态；迁移一旦开始，恢复只会激活候选版本并向前完成，绝不把旧二进制恢复到可能已经更新的 schema 上。迁移成功后先移除启动阻断，再启动服务；事务文件保留到启动和清理成功。完成结果依次持久化为终态事务和任务终态，最后才删除事务；执行器在两次写入之间退出时，状态协调会从终态事务恢复任务结果，而不是凭空合成失败。`dash update recover` 会继续或回滚记录中的事务。Goose 的 `goose_db_version` 仍是唯一 schema 版本来源：`dash migrate` 推进较旧 schema 并拒绝较新 schema，正常启动要求完全一致。GitHub Releases 元数据与资产是更新根信任源；GitHub 账号、token、仓库、workflow 或 release 权限被攻破，等价于更新源被攻破。
- Linux 节点安装器只负责安装和强制重装，不负责版本升级。它会在停止 systemd、OpenRC 和匹配的手动运行进程前暂存并执行下载的二进制，随后覆盖受管 release、上报配置、服务/采集器资产，并原子切换 `current` 符号链接。节点运行用户拥有数据和 release 树，因为非特权 Node 自更新协议需要创建和切换 release；root 所有的服务及采集器资产位于该树之外。安装器最多跟随五次重定向，目标必须保持初始主机；同协议跳转必须保持有效端口，只允许 HTTP 升级到 HTTPS。节点版本升级及其恢复语义归独立的节点自更新路径所有。
- 当前主题 ID 是持久化配置，主题解析结果只是可丢弃的展示状态。选中主题包缺失或非法时记录为 `missing` 或 `broken`，运行时使用前端内置默认皮肤，主题管理仍可用于修复或重新选择；数据库和主题根目录不可访问仍属于运行错误。主题包格式 v1 已冻结并弃用但继续兼容，不再扩展语法或能力；新增能力必须使用不同的格式版本。固定文件名 `/theme-bootstrap.js` 使用 `Cache-Control: no-store`，其他静态资源保持各自的缓存行为。
- SMART、thermal 和完整 RAID 详情属于运行时状态。SMART 缓存新鲜度、helper 可用性、设备健康结果、完整 thermal 传感器 payload 以及完整 RAID 阵列/成员 payload 保存在当前快照或热点缓存，不写入 PostgreSQL 历史指标行。确认是物理盘的 SMART 温度会归约写入 `disk_physical_metrics.temp_c`，用于按设备查询历史；虚拟盘和 RAID 设备会被忽略。同一套后端判定会生成 `disk.temperature_devices`，供前端进入硬盘温度历史。thermal 会归约写入 `cpu_temp_c` 作为主机历史；完整 thermal 详情拆成独立前台字段缓存，读取前台节点视图时再组合进 JSON。
- TCP/UDP 连接数是持久化数值指标，会写入 `tcp_conn` 和 `udp_conn`，并作为 `conn.tcp` 和 `conn.udp` 支持历史查询。systemd Linux 主机上的完整主机/netns 连接数来自 1 秒周期的 root 侧连接数缓存，因为 Node 以低权限运行；安装脚本会在存在 `cc`、`gcc` 或 `clang` 时本地编译该 helper。OpenRC 不运行该 helper，因为 BusyBox cron 无法保持 1 秒周期。缓存缺失、过期、helper 无法编译或使用 OpenRC 时，Node 使用自带连接数统计，可能缺失容器连接数据。
- Linux PSI pressure 指标是固定数值时序数据。PSI 的 `avg10`、`avg60`、`avg300` 和 `total` 会作为可空列保存到 `server_metrics` 和 `server_current_metrics`；缺失列表示不可用，不表示 0 压力。Dashboard 持久化会忽略采集原因/状态字符串。PSI 数据只进入历史链路，不参与告警评估。
- 告警评估读取进程内最新上报快照或 PostgreSQL 当前投影。内置离线、RAID、SMART 健康失败和 NVMe 关键告警规则来自快照新鲜度和上报磁盘状态。
- 告警服务启动后 1 分钟内不会新开告警事件。
- 指标提交把最新节点快照放入进程内告警脏队列；控制变更可以只放节点 ID。同一节点的重复标记合并为最新快照，执行期间再次变脏会在本轮结束后再执行一次。队列和运行态都不写 Redis；进程重启后的全量协调、PostgreSQL 开放事件和后续指标上报负责恢复评估。
- 告警和 Dash 更新消息共用一个 PostgreSQL 通知 outbox 和一个单进程投递 worker。投递不使用运行时租约；进程异常退出留下的 `sending` 行会在下一次轮询立即恢复。Outbox 会持久化在途行是否为 blocked 渠道探针，崩溃恢复后继续使用探针退避并保持积压合并。远端发送成功后，仍存活的 worker 只重试本地完成事务，不会再次发送该行；优雅停机时会保留最长 10 秒的本地完成窗口。若进程在远端接收与本地提交之间崩溃，或停机完成窗口超时，因为远端协议没有共享幂等回执，边界仍是至少一次投递（at-least-once）。告警事件及运行时状态仍不依赖投递；首次加载告警目标失败且没有 last-good 快照时，本次告警状态转换会延后重试，不会提交一个缺少 outbox 的终态；已有 last-good 快照时继续按该快照创建 outbox。发出远端请求前发生的数据库查询或 worker 本地故障只会释放当前任务以便再次尝试，不消耗远端投递预算，也不会改变渠道健康状态。瞬时远端错误先按 5–320 秒指数退避，之后进入 `blocked`；确定性的配置错误或远端拒绝会同时阻塞该渠道兼容的待发送/重试积压，只保留一个低频探针。该 blocked 探针发生任何失败都会继续合并整条渠道积压，并按 5、15、30、60 分钟安排下一次探测，同时遵守有上限的远端 `Retry-After`；Telegram Bot 的 `429` 还会读取 JSON `parameters.retry_after`。渠道停用时，未发送行进入 `paused`；只有 payload 损坏、目标已删除或渠道类型不匹配才进入 `discarded`。保存、重新启用或真实投递成功都会唤醒兼容任务并重置重试预算。渠道配置 revision 防止旧配置的在途结果污染当前健康状态，也防止 MTProto 登录完成时覆盖并发替换的新配置。投递健康状态持久化在 `notify_channels`，活跃队列数量及重试/探针时间由 outbox 派生。历史 `failed_permanent` 会迁移为 `blocked`，远端恢复不依赖管理员打开管理界面。
- 告警通知服务统一负责系统通知的默认目标解析和持久化 outbox 入队。调用方只接收 `queued` 或终态 `skipped_no_targets`；目标加载或 outbox 故障仍作为可重试错误返回。自动更新控制器只拥有任务生命周期：两种终态入队结果都会写入 `finish.handled`，同时继续识别旧 `finish.notified` marker 以兼容存量任务清理。
- 通知渠道配置只在 alert store 穿过一次加密边界。Dash 使用 AES-256-GCM 加密完整 JSON 文档，并把渠道 ID 和类型作为附加认证数据；当前逻辑行只保留密文，旧 `config` 列固定为 `{}`。单一安装密钥是 PostgreSQL 之外的原始 32 字节 `$DASH_HOME/configs/notify-config.key`，权限仅限所有者读取。`dash migrate` 只会在数据库尚无密文时创建密钥，并在一个数据库事务中转换全部旧行。正常启动绝不生成密钥或回退到明文：它要求密钥存在、数据库满足仅密文不变量，并能解密包括软删除渠道在内的每一行。数据库备份与密钥必须分开保存；密钥丢失后通知凭据不可恢复。该迁移不会从 MVCC 死元组、表空闲空间、WAL、副本、备份或存储快照中物理擦除旧明文。
- 告警控制任务在执行和等待重试时保持 `pending`，由单进程 worker 串行取得；不使用运行时租约。已完成任务会删除；失败任务行保留为运行历史，但不再占用去重键，因此后续协调仍可重新排入同一项逻辑修复。
- 服务器、磁盘 IO、磁盘容量和物理盘温度历史统一使用 1 小时 raw chunk。raw 在首日保持未压缩，之后无损压缩，达到 `database.metrics_raw_retention_days`（默认 `8`，最小 `2`）后删除。15 分钟聚合保留 16 天，1 小时聚合保留 32 天；`30m` 至 `24h` 查询 raw，`1w` 和 `15d` 查询 15 分钟聚合，`30d` 查询 1 小时聚合。历史曲线聚合包含 real-time raw 尾部，只有 `server_online_30m` 继续只统计已完成 bucket。
- 网卡 raw 继续使用 1 天 chunk、7 天压缩和 `database.retention_days`（默认 `45`）；5 分钟流量事实继续保持 rowstore 并使用 `database.traffic_retention_days`，不增加通用 NIC 聚合。Lite 不缩短这些窗口，Billing 重建仍可使用保留期内的 NIC 样本。`traffic_5m` 保持可写并滚动清理，历史 95 计费值保存在月度快照中。
- 默认生命周期的容量基线：100 节点约 `24–26 GiB` 核心时序存储、`6–8 GiB RAM` 和 `80–100 GiB SSD`；500 节点约 `120 GiB`、`16 GiB RAM` 和 `250 GiB SSD/NVMe`。
- 流量 `lite` 模式只在 `traffic_month_usage` 保存每网卡月度入/出总量、估算峰值、覆盖边界和逐行源进度，不写 5 分钟事实。一个持久化 `usage` 扫描水位只负责全局实时头部和进程重启后的追赶，不取代每行独立的 `last_collected_at`，也不承担局部账期修复。`billing` 保留同一月度累计作为影子校验和连续性来源，并额外使用独立 `facts` 水位维护 `traffic_5m`；Billing API 的总量和高级指标仍统一以 `traffic_5m` 为权威来源。Lite 切换为 Billing 时把 Facts 水位定位到最近 30 分钟，使当前统计先恢复；Lite 期间更早的节点历史不进入自动追赶主路径，只能在网卡 raw 仍在保留期内时通过 Billing 节点重建按需补齐。
- 一个后台 Service 拥有两个显式物化器；两者各自保留查询、算法、水位和事务。每个一小时分块把派生数据与对应水位原子提交；Usage 失败不推进 Usage 水位，Facts 失败不推进 Facts 水位，两者可独立追赶。每次五分钟调度中，每个实时物化器最多执行 12 个一小时分块；节点 Usage 修复另有 30 秒和 120 个六小时分块的双重预算。单个流量写入门只覆盖一个分块。Usage 追赶受网卡 raw 保留期约束，Facts 追赶受网卡 raw 与事实保留期的较短者约束。物化查询使用当前网卡投影和已有月累计作为小型接口目录，再为每个接口查询区间前后的相邻采样，因此相邻采样跨过没有采样的一小时分块时仍会完整计入；Facts 幂等覆盖重叠桶，Usage 用逐行 `last_collected_at` 防止重复累计，同一长间隔在每个账期只记录一次 gap。
- 日汇总、P95、覆盖率和 Billing 月度快照由 5 分钟流量事实派生。Billing 专用节点重建只有一个进程内状态，不持久化任务或游标，不引入任务队列。它按 6 小时分块使用同一写入门，在一个事务中重写事实并让重叠月度快照失效；Dash 重启会取消任务。每个分块重新检查 Billing 模式，因此切换到 Lite 后不会开始新的事实分块。
- 账期归单节点所有，并始终显式保存。新节点默认使用 `calendar_month`、1 号和应用时区；全局流量设置行只为响应兼容保留固定的账期形状字段，不能改变节点账期。升级时会先把旧 `default` 节点解析为迁移前实际继承的全局账期，因此现有账期边界不会移动。单节点账期变更立即生效，同一事务只删除该节点受影响的月度累计和快照，并在 `traffic_usage_repairs` 合并一条短期修复水位；全局实时 Usage 水位不回退。存在修复记录的节点暂时不参与实时 Usage 扫描，后台按六小时分块从该节点新旧当前账期较早的起点追到实时头部后删除记录；其他节点持续实时更新。实际补算仍受网卡 raw 保留期约束，覆盖不足通过累计行的 `covered_from` 和统计完整性字段表达。全局统计方向仍是可被节点覆盖的默认值。不保存账期规则历史或待生效配置。
- 流量方向模式只改变选中的计费视图；原始入站和出站计数仍分开保存。
- 历史指标默认不对游客公开。`history_guest_access_mode=by_node` 时，只对游客可见节点开放。
- 流量统计是否对游客公开由流量设置控制，并仍受节点可见性限制。

## 鉴权边界

Redis 和内存认证存储均使用 EiluneKit 缺省的每用户 255 个未过期会话上限。容量限制只拒绝新登录，已有会话和刷新轮换仍可使用。会话格式和 `auth:jwt:*` Redis 命名空间保持兼容。

| 区域                                                  | 鉴权                                                          |
| ----------------------------------------------------- | ------------------------------------------------------------- |
| `/api/auth/login`                                     | 管理员密码                                                    |
| `/api/auth/refresh`、`/api/auth/logout`               | `SameSite=Strict` refresh cookie + `X-CSRF-Token`             |
| `/api/front/*`、`/api/metrics/*`、`/api/statistics/*` | Bearer 可选；匿名请求按系统可见性设置过滤                     |
| `/api/node/*`                                         | `X-Node-Secret`                                               |
| `/api/admin/*`                                        | `Authorization: Bearer <access_token>`                        |
| `/deploy/*` 打包资产                                  | `X-Node-Secret` 或旧 Node 升级临时 token；安装脚本模板仍公开  |

## 前端和反向代理

访问日志以及登录、节点鉴权限流只信任来自 `http.trusted_proxies` 的客户端 IP 转发头。优先级沿用 EiluneKit 缺省值：`X-Forwarded-For`、`X-Real-IP`、`Forwarded`、`True-Client-IP`、`CF-Connecting-IP`。该选择不改变已认证节点的 IP 观测规则，节点 IP 仍使用 `X-Forwarded-For` 第一项或连接地址。

前端可以单独启动开发服务器，但运行边界仍是同源路径。开发代理或生产反向代理应把 `/api`、`/theme`、`/deploy` 转给后端，`/` 保持为 Dash SPA。IP 部署允许使用 HTTP；经过不可信网络时应使用 HTTPS，否则管理员凭据和节点密钥会以明文传输。跨域后端地址需要同时配置 CORS、cookie 和 CSRF 策略。

## 目录

`cmd/dash` 通过 `internal/http` 构造 HTTP 服务。HTTP 根入口组合 API、主题和安装脚本 Blueprint，并提供静态文件回退 handler；路由父模块只汇总直属子模块。MTProto 登录状态的序列化和过期由 `store/mtlogin` 负责，渠道版本检查和会话持久化由 `store/alert` 负责。

| 路径                          | 内容                                     |
| ----------------------------- | ---------------------------------------- |
| `cmd/dash`                    | 服务、迁移、更新、Redis 检查和主题打包入口 |
| `internal/config`             | 配置加载、默认值、校验和运行目录         |
| `internal/http`               | HTTP 服务、静态资源、主题资源和 API 挂载 |
| `internal/http/api`           | `/api` 路由树                            |
| `internal/http/deploy`        | 各平台安装脚本路由树                    |
| `internal/store`              | 持久化和缓存访问层                       |
| `internal/model`              | 同包内按领域分文件的数据库模型           |
| `internal/alert`              | 告警编译、运行时和发送编排               |
| `internal/traffic`            | 流量统计后台服务                         |
| `web`                         | SPA 前端源码                             |
| `configs`                     | 示例配置                                 |
| `db/migrations`               | 数据库结构变更                           |
| `scripts`                     | 构建和打包入口                           |


## Node 传输与 PVE 查询归属

HTTP 服务根入口持有一个共享 `nodeingest.Receiver`、一个 `nodesession.Hub` 和 `noderpc` 适配器。HTTP 和 gRPC 调用相同受理方法，保留既有宿主指标、流量、在线率、告警和缓存行为。同一监听地址支持 HTTP/1、HTTP/2，以及可信 TLS 代理后的明文 HTTP/2；预留的 `grpc_port` 不作为第二监听端口。关闭时并行停止 RPC 会话、等待 HTTP 请求退出，共用 10 秒预算。到期后由 HTTP 服务强制关闭剩余连接，再等待 RPC 清理结束。

Dash 每个查询请求持有该流的读写任务，并等待两者退出。写任务直接消费会话命令队列，在发送时计算查询剩余预算。单次流写入限时 5 秒，空闲期间不设 HTTP 写入期限。会话结束后，最终状态最多等待 1 秒发送，超时重置该 HTTP/2 流。会话替换、凭据撤销和流控阻塞只回收对应流，不关闭普通上报复用的连接。

Node 每目标持有一个复用 gRPC 连接，用于普通 RPC 上报和独立的反向查询流，目标之间互不阻塞。默认保持 HTTP，通过 `ITHILTIR_NODE_TRANSPORT` 启用 gRPC 或自动协商。会话、待完成查询及重连退避是易失状态，不构成持久化工作队列。心跳不影响按指标上报判定的在线状态。派发查询前及长流存续期间每秒复查身份。会话替换和服务关闭取消查询流，允许重新连接；只有鉴权或授权拒绝才停止该目标的查询重连循环。

Node 会话循环统一管理最多 32 个待完成查询的受理、取消和完成。查询任务只向循环返回结果，不修改共享会话状态。本机 helper 客户端以四条历史查询连接限制执行并发，等待连接同样消耗查询预算。会话退出时取消并等待全部查询及流读写任务结束，再进入重连。客户端为能力探测保留独立连接；探测失败时保留上次声明的能力，不关闭查询会话，成功探测到能力变化时重新建立会话。

PVE root helper 以 `ithiltir-node-pve-cache.service` 运行 `--serve`，持有两套采集调度和 `/run/ithiltir-node/pve.sock`。socket 为 root 所有、0660 权限，校验对端 root UID 或运行主 GID。仅提供固定能力查询和经过参数校验的历史查询，不接受任意命令或 PVE 路径。安装时先停用旧 PVE 定时器，再启用常驻服务；手动单次模式保持相同 VM 锁及缓存格式。

历史查询要求本地新鲜的成功 VM 清单。相同进行中请求共享一次执行，最后一个等待者离开时取消任务。Guest Agent 与历史查询共用四个执行槽及子进程继承的 VM 文件锁，进程退出后才释放 VM 锁。helper 最多受理 32 个不同的进行中历史查询，结果最多 2 MiB、4096 点；成功缓存 30 秒，失败缓存 10 秒，历史失败还对该 VM 的其他窗口施加 10 秒冷却。缓存最多 128 项、16 MiB 结果字节；正在返回的响应可独立持有受限结果。共享执行由缓存持有独立的 8 秒预算，各远端等待者的预算最多 10 秒；某个等待者超时不会取消其他等待者仍需要的执行。Dash 不增加 VM 历史表，也不增加前端适配。
