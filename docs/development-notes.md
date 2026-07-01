# 开发注意事项

本文档记录当前代码中已经暴露或容易重复踩坑的问题。新增功能、修复缺陷、做性能优化前，先对照本页检查。

## 2026-07-01 列表分页与推送索引

- 事件、日志、原始事件、绑定类列表接口必须服务端分页，返回结构统一为 `{ items, total, page, pageSize }`。
- 筛选条件必须下推到 SQL，不要先 `Find(&items)` 全量查出，再在内存里过滤 provider、capability、status、source 等字段。
- `/smart/bindings` 需要用 `smart_device_binding` 关联 provider/capability，并通过规则数聚合查询当前页数据。
- `/smart/raw-events` 需要用 `smart_raw_event` 关联 provider/capability，并使用 `LIMIT/OFFSET`，避免逐行查询 provider/capability。
- 推送限流和推送日志热点查询需要复合索引 `alarm_push_log(push_config_id, status, pushed_at, id)`，新增或调整时保持 `sql/init_database.sql`、启动期运行时索引和部署升级脚本同步。

## 1. 文件拆分与性能

- `internal/service/platform_service.go` 当前约 5000 行，文件过大主要影响维护、代码审查、冲突解决和误改风险，不会直接造成运行性能下降。
- 如果拆分，优先做同包分文件：仍保留 `PlatformService` 结构体，只把方法按领域拆到多个 `platform_*.go` 文件，避免一次性改动 handler、router 和依赖注入。
- 不要把“拆文件”当作性能优化。真正影响性能的通常是 SQL 查询次数、索引、分页、外部 HTTP/SDK 调用和长连接管理。

## 2. 推送限流逻辑

相关代码：

- `internal/service/push_delivery.go`
- `push_config.rate_limit_window_seconds`
- `push_config.rate_limit_max_count`
- `alarm_push_log.status`

注意事项：

- 限流统计只能统计真实发起过的推送结果，例如 `success`、`failed`。
- `rate_limited` 是跳过记录，只用于审计，不能再计入限流窗口。
- 如果把 `rate_limited` 也计入窗口，会出现“限流日志给自己续期”的问题：每次被限流都会新增一条日志，下一次又因为这条日志继续被限流。
- 修改限流逻辑后，必须用连续触发场景验证：成功发送超过窗口秒数后，应允许再次发送。
- 相关查询建议使用复合索引：`alarm_push_log(push_config_id, status, pushed_at, id)`。

建议测试场景：

1. 最近窗口内没有 `success/failed`，只有 `rate_limited`，应允许发送。
2. 最近窗口内有 1 条 `success`，且最大次数为 1，应限流。
3. 最近窗口外有 `success/failed`，应允许发送。
4. 多个推送配置互不影响，限流按 `push_config_id` 独立计算。

## 3. 移动侦测去重、冷却与推送

相关代码：

- `internal/service/hikvision_alarm_bridge_service.go`
- `smart_binding_rule.dedup_window_seconds`
- `smart_binding_rule.cooldown_seconds`
- `alarm_record.dedup_key`

注意事项：

- 移动侦测去重发生在告警生成前，推送限流发生在告警生成后，两者不是同一个概念。
- 同一设备会话 + 同一通道会先进入内存聚合窗口，窗口结束后再落库为智能事件/告警。
- 告警创建时还会按 `dedup_key` 和 `dedup_window_seconds` 查历史告警，决定新建还是合并。
- 只有新建告警且规则启用推送时，才会进入自动推送流程。
- 修改去重或冷却逻辑时，要同时验证：
  - 聚合窗口内多次移动侦测是否合并。
  - 窗口外触发是否能生成新告警。
  - 冷却期内是否被抑制。
  - 被合并的告警是否不会重复推送。

## 4. 数据库查询与 N+1

当前有些列表和 map 转换中存在 N+1 查询风险，尤其是智能接口、智能事件、provider/capability/sourceName 相关逻辑。

高风险位置：

- `PlatformService.smartProviderMap`
- `PlatformService.smartBindingMap`
- `PlatformService.smartEventMapWithRelations`
- `PlatformService.GetSmartEventDetail`
- `PlatformService.ListSmartRawEvents`

开发约束：

- 列表接口不要在循环中反复 `First` 查询关联对象。
- 优先使用 `JOIN`、`Preload` 或批量 `IN` 查询后构建 map。
- 分页列表的关联数据，只加载当前页需要的关联对象。
- 不要为了展示名称对每一行分别查 camera、recorder、channel、provider、capability。

检查方式：

- 新增列表接口时，估算单页 20 条数据会产生多少 SQL。
- 如果单页 20 条产生 40 条以上 SQL，通常就需要重构。
- 对事件、告警、日志类接口，使用真实数据量执行 `EXPLAIN`。

## 5. 分页与全量加载

事件、日志、告警、AI 任务、推送记录等会持续增长，不能默认全量加载。

开发约束：

- 新增列表接口必须支持 `page/pageSize` 或明确限制 `Limit`。
- 原始事件、推送日志、操作日志、设备状态日志等表，禁止无条件 `Find(&items)` 返回全量数据。
- 筛选条件应下推到 SQL，不要先全量查出再在内存中过滤。
- 导出接口必须有最大条数、时间范围或异步任务机制。

当前需重点关注：

- `ListSmartRawEvents` 已改为分页并将 provider/capability/source/status 筛选下推到 SQL；后续不要退回全量 `Find`。
- `ListSmartBindings` 已改为分页、JOIN provider/capability、聚合 ruleCount；后续新增筛选也要继续下推到 SQL。
- `ListSmartAITasks` 已改为分页并支持状态、流程编码、最近天数过滤；后续不要退回全量 `Scan`。
- `ListSmartEvents`、`ListSmartBindings`、`ListSmartProviders` 的列表路径已去掉主要 N+1 查询；后续列表展示字段应在当前页 SQL 中通过 JOIN、子查询聚合或批量查询取回。

## 6. 报表与驾驶舱聚合

驾驶舱和报表接口容易频繁刷新，不能用大量独立 `COUNT` 放大数据库压力。

开发约束：

- 多个状态计数优先用条件聚合一次查询完成，例如 `SUM(status = 'success')`。
- 按天趋势优先用 `GROUP BY DATE(...)`，不要按天循环逐个 `COUNT`。
- 大表报表必须带时间范围，默认范围不应过大。
- 报表类接口建议为常用查询补复合索引。

当前可优化位置：

- `GetDashboardAlarmTrend`
- `GetPushReport`
- `Repository.GetDashboardSummary`
- `OperationLogService.GetStats`

## 7. 索引与迁移

当前项目没有完整版本化迁移工具，DDL 变更依赖 `sql/init_database.sql` 和部署脚本中的运行时补丁。

开发约束：

- 新增字段或索引时，同步更新：
  - `sql/init_database.sql`
  - `internal/domain/entity/models.go`
  - 存量环境升级 SQL 或部署脚本
- 不要在服务启动路径为大表自动 `CREATE INDEX`；大表索引应通过部署脚本或维护窗口执行，启动期只做轻量 AutoMigrate 或显式检查。
- 只加单列索引不一定够。对热点查询，要按 `WHERE + ORDER BY` 设计复合索引。
- 典型热点索引：
  - 告警去重：`alarm_record(dedup_key, alarm_time, id)`
  - 告警冷却：`alarm_record(dedup_key, last_event_time, id)`
  - 智能事件去重：`smart_event(dedup_key, event_time, id)`
  - 推送限流：`alarm_push_log(push_config_id, status, pushed_at, id)`
- 发布前用真实数据量跑 `EXPLAIN`，不要只在空库判断性能。

## 8. 设备状态与长任务

相关代码：

- `CheckAllDevicesStatus`
- `HikvisionAlarmBridgeService`
- `internal/integration/hikvision`

注意事项：

- 设备状态检查不能无限制全量串行处理。设备量上来后，应分页、限制并发，并设置超时。
- 状态日志只应在状态变化或关键检查结果变化时写入，避免定时任务导致日志表快速膨胀；`CheckAllDevicesStatus` 已改为批处理、TCP 端口探测，并仅记录状态变化。
- 单设备页面只保留“测试”操作；测试应完成连接探测并写入设备状态。“检测”语义保留给全部检测、后台巡检或兼容接口。
- SDK 登录、布防、抓图、录像下载等外部调用必须有超时、重试上限和清理逻辑。
- 移动侦测接收链路是长连接/回调型逻辑，后续如果拆成独立 agent 或 Windows 服务，应保持后端规则、去重、告警、推送逻辑仍在主服务统一处理。

## 9. 前端大组件

当前较大的前端文件包括：

- `frontend/src/views/monitor/PlaybackView.vue`
- `frontend/src/views/device/CameraManagementView.vue`
- `frontend/src/views/monitor/AiIntegrationView.vue`
- `frontend/src/components/video/HikWebControlPlaybackPlayer.vue`

注意事项：

- 大组件主要影响维护、HMR 和局部修改风险。由于路由已懒加载，它们不一定直接影响首屏。
- 新增复杂页面时，优先拆 composable、子组件、类型文件和 API 文件。
- 涉及定时器、播放器、WebControl、SSE 的组件必须在卸载时清理 `setTimeout/setInterval`、事件监听和播放器实例。
- 大表格或树形列表数据量增长后，需要考虑分页、虚拟列表或后端搜索。

## 10. 编码与文案

当前自有 Go/Vue/Markdown 文件应统一按 UTF-8 读取和保存。若在 PowerShell 或脚本输出中看到 `鍛婅`、`娴峰悍` 等 mojibake，先用 UTF-8 方式重新读取文件确认，避免把终端显示问题误判为源码损坏。

开发时注意：

- 源码、SQL、Markdown、前端文件统一使用 UTF-8。
- PowerShell 查看中文文件时优先使用 `Get-Content -Encoding UTF8`。
- 不要把终端乱码复制回源码、SQL 或文档。
- 新增错误消息、日志、页面文案时，在浏览器、终端和日志文件里各检查一次。
- 第三方压缩 SDK/WebControl 文件不纳入文案修复范围，除非确认会直接展示给用户。
- 重要日志建议使用稳定英文 code + 中文 message，方便检索。

## 11. 权限与安全

注意事项：

- 前端按钮隐藏不等于权限控制，新增敏感接口必须检查后端权限中间件或 access scope。
- 数据范围过滤要下推到 SQL，不能只在前端隐藏。
- 设备密码、推送 secret、AI callback secret 不得明文返回前端。
- 新增第三方回调接口时，要明确鉴权方式，避免误放在 JWT-only 分组导致第三方无法调用，或误开放导致伪造请求。

## 12. 提交前检查清单

- [ ] 是否引入了全量查询或循环内查询关联对象。
- [ ] 列表接口是否支持分页和服务端筛选。
- [ ] 热点查询是否有匹配复合索引。
- [ ] 新字段/索引是否同步 SQL、entity 和存量升级脚本。
- [ ] 推送、去重、冷却、重试等窗口类逻辑是否排除了“日志给自己续期”这类副作用。
- [ ] 外部 HTTP/SDK 调用是否有超时、错误日志和资源清理。
- [ ] 前端定时器、播放器、SSE/WebSocket 是否在卸载时清理。
- [ ] 是否运行 `go test ./...` 或至少相关包测试。
- [ ] 前端改动是否运行 `npm run build`。
