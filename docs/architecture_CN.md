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

## HTTP 面

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
2. 指标上报成功响应可包含更新 manifest。
3. PostgreSQL + TimescaleDB 保存持久化历史、流量事实、普通配置、加密后的通知渠道配置和通知 outbox。
4. 默认模式下 Redis 保存管理员会话和可丢弃的前台缓存；`--no-redis` 用进程内内存替代两者。告警运行态和 MTProto 登录握手始终留在单个 Dash 进程内。
5. 后台服务评估告警、发送队列通知并汇总流量数据。

节点 IP 是已鉴权 Node 请求的观察值：有 `X-Forwarded-For` 时 Dash 取其第一个 IP，否则回退到 `RemoteAddr`；不可解析的值不会被使用。该字段用于展示和运维，不作为鉴权边界。

## 状态和保留策略

- 默认启动依赖 PostgreSQL 和 Redis `6.2.0+`，推荐 Redis `8.2.3+`。Dash 会通过 `PING` 和 `INFO server` 校验实际连接的服务端，因此配置的 Redis 账号必须允许这两个命令；服务不可用、版本无法识别或低于 6.2.0 时终止启动，低于 8.2.3 时仍可运行但会记录启动警告。Redis 保存管理员会话和可丢弃的前台缓存，单次 Redis 故障不会回退到内存。传 `--no-redis` 时会跳过 Redis 连接和版本校验，并从启动时把会话与前台缓存装配到进程内内存。
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

| 区域                                                  | 鉴权                                                          |
| ----------------------------------------------------- | ------------------------------------------------------------- |
| `/api/auth/login`                                     | 管理员密码                                                    |
| `/api/auth/refresh`、`/api/auth/logout`               | `SameSite=Strict` refresh cookie + `X-CSRF-Token`             |
| `/api/front/*`、`/api/metrics/*`、`/api/statistics/*` | Bearer 可选；匿名请求按系统可见性设置过滤                     |
| `/api/node/*`                                         | `X-Node-Secret`                                               |
| `/api/admin/*`                                        | `Authorization: Bearer <access_token>`                        |
| `/deploy/*` 打包资产                                  | `X-Node-Secret` 或旧 Node 升级临时 token；安装脚本模板仍公开  |

## 前端和反向代理

前端可以单独启动开发服务器，但运行边界仍是同源路径。开发代理或生产反向代理应把 `/api`、`/theme`、`/deploy` 转给后端，`/` 保持为 Dash SPA。IP 部署允许使用 HTTP；经过不可信网络时应使用 HTTPS，否则管理员凭据和节点密钥会以明文传输。跨域后端地址需要同时配置 CORS、cookie 和 CSRF 策略。

## 目录

| 路径                          | 内容                                     |
| ----------------------------- | ---------------------------------------- |
| `cmd/dash`                    | 服务、迁移、更新、Redis 检查和主题打包入口 |
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
