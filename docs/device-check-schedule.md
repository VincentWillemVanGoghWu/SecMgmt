# 设备定时巡检计划开发说明

## 设计边界

- 设备定时巡检必须运行在后端服务中，不能依赖前端页面定时器。
- 第一版只做全局巡检计划：用户配置“每天检测次数”，后端按 `24h / frequency_per_day` 计算下一次执行时间。
- 应用启动时由 `DeviceCheckScheduler` 扫描 `device_check_schedule.next_run_at`，到期后执行巡检。

## 状态写入

- 巡检执行复用 `runDeviceStatusCheck`，单次手动检测和计划检测必须走同一套设备状态写入逻辑。
- `device_status_log` 只记录状态变化，避免定时巡检导致日志表快速膨胀。
- 离线邮件不能只依赖状态日志，否则已经离线的设备会被漏掉；邮件必须以本次检测后的离线清单为准。

## 邮件推送

- 离线邮件只复用底层 `deliverEmailPush`，不要伪造 `alarm_record`。
- 设备巡检通知不要混入 `alarm_push_log`，巡检推送应写入 `device_check_push_log`。
- 默认推送策略使用 `offline_changed`，只在设备从非离线变为离线时推送。
- 如果业务要求每次巡检都发送离线清单，再选择 `offline_each_run`，并注意推送配置的限流窗口。

## 数据清理

- 删除巡检计划时要同步清理 `device_check_run` 和 `device_check_push_log`。
- 后续如增加按厂区、区域、设备类型巡检，过滤条件必须下推到 SQL，不要先全量加载后在内存过滤。
