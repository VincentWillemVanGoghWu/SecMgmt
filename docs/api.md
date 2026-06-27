# API 接口说明

## 1. 通用约定

- 后端默认地址：`http://127.0.0.1:8000`
- API 前缀：`/api`
- 请求/响应：`application/json`，下载接口除外
- 时间：查询通常使用 `start_at`、`end_at`，支持 RFC3339 等 handler 可解析格式；数据库 DSN 必须启用 `parseTime=True&loc=Local`
- 分页：`page` 从 1 开始，`page_size` 为每页条数
- 认证：除登录、SSE 和健康检查外均需 `Authorization: Bearer <JWT>`

标准成功响应：

```json
{"code": 0, "message": "ok", "data": {}}
```

标准错误响应使用 HTTP 状态码，同时 envelope 的 `code` 等于该状态码：

```json
{"code": 400, "message": "错误说明", "data": {}}
```

## 2. 公共接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查 |
| POST | `/api/auth/login` | 登录；body：`username`、`password` |
| GET | `/api/sse/alarms?token=<JWT>` | 告警 SSE；EventSource 无法设置 Authorization，故使用 query token |

登录返回 `access_token`、`token_type`、`expires_in`。

## 3. 认证、用户与角色

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/auth/logout` | 前端语义注销，无服务端 token 黑名单 |
| GET | `/api/auth/me` | 当前用户、角色、菜单、按钮权限、数据范围 |
| GET | `/api/menus` | 当前用户菜单树 |
| GET/POST | `/api/users` | 用户列表/新增 |
| PUT/DELETE | `/api/users/:id` | 修改/删除用户 |
| GET/POST | `/api/roles` | 角色列表/新增 |
| PUT/DELETE | `/api/roles/:id` | 修改/删除角色 |
| PATCH | `/api/roles/:id/status` | 修改状态 |
| PUT | `/api/roles/:id/data-scope` | 修改数据范围 |

用户 body：`username, realName, deptId?, status, roleIds[], password`。角色 body：`roleCode, roleName, status, remark?`。状态 body 为 `{"status":"enabled|disabled"}`；数据范围 body 为 `dataScopeType, dataScopeValue`。

## 4. 基础资料

| 资源 | 查询 | 新增 | 修改 | 状态 | 删除 |
|---|---|---|---|---|---|
| 厂区 | `GET /factories` | `POST /factories` | `PUT /factories/:id` | `PATCH /factories/:id/status` | `DELETE /factories/:id` |
| 区域 | `GET /zones` | `POST /zones` | `PUT /zones/:id` | `PATCH /zones/:id/status` | `DELETE /zones/:id` |
| 部门 | `GET /depts` | `POST /depts` | `PUT /depts/:id` | `PATCH /depts/:id/status` | `DELETE /depts/:id` |
| 字典类型 | `GET /dicts` | `POST /dicts/types` | `PUT /dicts/types/:id` | `PATCH /dicts/types/:id/status` | `DELETE /dicts/types/:id` |
| 字典项 | 随 `/dicts` 返回 | `POST /dicts/items` | `PUT /dicts/items/:id` | `PATCH /dicts/items/:id/status` | `DELETE /dicts/items/:id` |

以上路径均位于 `/api`。列表支持 `keyword`、`status`；区域/部门还支持 `factory_id`，部门支持 `zone_id`。请求字段与数据库字段采用 lowerCamelCase，例如厂区 `factoryCode, factoryName, status, remark`。

## 5. 设备与状态

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | `/api/cameras` | 摄像机列表/新增 |
| GET/PUT/DELETE | `/api/cameras/:id` | 详情/修改/删除 |
| PATCH | `/api/cameras/:id/status` | 修改启停状态 |
| POST | `/api/cameras/sdk-device-identity` | 按连接参数读取设备身份 |
| POST | `/api/cameras/:id/test` | 测试连接 |
| POST | `/api/cameras/:id/status/check` | 检查并记录状态 |
| GET | `/api/cameras/:id/browser-login` | 浏览器 WebControl 登录配置 |
| GET | `/api/cameras/:id/sdk-config` | 读取 SDK 配置 |
| PUT | `/api/cameras/:id/sdk-config/network` | 网络配置 |
| PUT | `/api/cameras/:id/sdk-config/image` | 图像配置 |
| PUT | `/api/cameras/:id/sdk-config/recording` | 录像配置 |
| PUT | `/api/cameras/:id/sdk-config/ptz` | PTZ 配置/控制 |
| PUT | `/api/cameras/:id/sdk-config/ptz/zoom/:action` | 变焦；action 由 handler/service 校验 |
| PUT | `/api/cameras/:id/sdk-config/ptz/presets` | 设置预置点 |
| PUT | `/api/cameras/:id/sdk-config/ptz/presets/:presetId/goto` | 转到预置点 |
| DELETE | `/api/cameras/:id/sdk-config/ptz/presets/:presetId` | 删除预置点 |
| PUT/DELETE | `/api/cameras/:id/sdk-config/users[/:userId]` | 新增修改/删除设备用户 |
| GET/POST | `/api/recorders` | NVR 列表/新增 |
| GET/PUT/DELETE | `/api/recorders/:id` | 详情/修改/删除 |
| POST | `/api/recorders/:id/test` | 测试连接 |
| POST | `/api/recorders/:id/status/check` | 检查状态 |
| POST | `/api/recorders/:id/sync-channels` | 从 NVR 同步通道 |
| GET | `/api/recorders/:id/channels` | NVR 通道列表 |
| GET | `/api/channels` | 全部通道 |
| PUT | `/api/channels/:id` | 修改通道业务属性 |
| GET | `/api/device-status/logs` | 状态日志 |
| POST | `/api/devices/status/check-all` | 检查全部设备 |

摄像机 body 主要字段：`deviceCode, name, ip, sdkPort, httpPort, rtspPort, username, password, factoryId, zoneId, installLocation, supportAi, status, remark`。NVR 类似，另有 `channelCount`。密码只在传入非空值时更新，并以 Fernet 加密保存。

## 6. 告警、首页、报表与导出

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/alarms/realtime` | 实时告警分页 |
| GET | `/api/alarms` | 历史告警分页 |
| GET | `/api/alarms/:id` | 告警详情及处理记录 |
| POST | `/api/alarms/:id/process` | 处理；body：`status, remark?` |
| POST | `/api/alarms/:id/false-alarm` | 标记误报；body 可带 `remark` |
| POST | `/api/alarms/:id/repush` | 重新推送 |
| GET | `/api/dashboard/summary` | 首页汇总 |
| GET | `/api/dashboard/alarm-trend` | 告警趋势 |
| GET | `/api/dashboard/alarm-types` | 类型分布 |
| GET | `/api/dashboard/zone-ranking` | 区域排名 |
| GET | `/api/dashboard/device-status` | 设备状态 |
| GET | `/api/reports/alarms` | 告警报表 |
| GET | `/api/reports/devices` | 设备报表 |
| GET | `/api/reports/push` | 推送报表 |
| GET | `/api/export/alarms` | 告警 CSV 下载 |
| GET | `/api/export/device-status` | 状态 CSV 下载 |
| GET | `/api/export/push-logs` | 推送日志 CSV 下载 |

告警过滤参数：`keyword, status, level, alarm_type, start_at, end_at, page, page_size`。报表/图表主要使用 `start_at, end_at`，部分排名接受独立分页参数。

## 7. 推送管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | `/api/push/configs` | 配置列表/新增 |
| PUT/DELETE | `/api/push/configs/:id` | 修改/删除 |
| PATCH | `/api/push/configs/:id/status` | body：`enabled` |
| POST | `/api/push/configs/:id/test` | 测试配置 |
| GET | `/api/push/logs` | 日志分页 |
| POST | `/api/push/logs/:id/retry` | 重试指定日志 |

配置支持 webhook、secret、appId/appSecret、templateId、接收人、厂区/区域/告警类型/等级过滤、生效时段、限流和重试参数。列表过滤为 `keyword, provider_type, enabled`；日志过滤为 `channel, status, alarm_type, start_at, end_at`。

## 8. 智能接口与 AI

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | `/api/smart/providers` | Provider 列表/新增 |
| PUT | `/api/smart/providers/:id` | 修改 Provider |
| POST | `/api/smart/providers/:id/test` | 连通性测试 |
| GET | `/api/smart/capabilities` | 能力列表 |
| GET/POST | `/api/smart/bindings` | 绑定列表/新增 |
| GET/PUT/DELETE | `/api/smart/bindings/:id` | 详情/修改/删除 |
| POST | `/api/smart/bindings/:id/rules` | 新增规则 |
| PUT/DELETE | `/api/smart/bindings/:id/rules/:ruleId` | 修改/删除规则 |
| POST | `/api/smart/events/ingest/:providerCode` | 写入任意 JSON 事件 |
| GET | `/api/smart/raw-events` | 原始事件 |
| GET | `/api/smart/events` | 标准事件分页 |
| GET | `/api/smart/events/:id` | 事件详情 |
| POST | `/api/smart/events/:id/submit-ai-review` | 提交 AI 复核 |
| GET | `/api/smart/ai-tasks` | AI 任务列表 |
| GET | `/api/smart/ai-tasks/:taskId` | AI 任务详情 |
| POST | `/api/smart/ai-tasks/:taskId/retry` | 重试任务 |
| POST | `/api/smart/ai/callback` | 智能任务回调 |
| GET | `/api/ai/config` | 旧版 AI 配置 |
| POST | `/api/ai/events/callback` | 旧版 AI 事件回调 |
| GET | `/api/ai/events` | 旧版 AI 事件列表 |
| GET | `/api/ai/events/:id` | 旧版 AI 事件详情 |

绑定过滤：`source_type, provider_code, capability_code, enabled`。事件过滤：`keyword, provider_code, capability_code, status, source_stage, recent_days`。注意：当前 ingest 和两个 callback 路由都注册在 JWT 保护组内，外部系统调用时也必须提供有效 JWT；`AI_CALLBACK_SECRET` 的业务签名不能替代路由 JWT。

## 9. 视频接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/video/live/:id` | 摄像机实时流 |
| POST | `/api/video/live/:id/stop` | 停止摄像机实时流 |
| GET | `/api/video/live/:id/webcontrol-config` | 摄像机 WebControl 配置 |
| GET | `/api/video/live/channel/:id` | 通道实时流 |
| POST | `/api/video/live/channel/:id/stop` | 停止通道实时流 |
| GET | `/api/video/live/channel/:id/webcontrol-config` | 通道 WebControl 配置 |
| POST | `/api/video/snapshot` | 抓图；body：`cameraId?` 或 `channelId?` |
| GET | `/api/video/playback/search` | 录像检索 |
| GET | `/api/video/playback/url` | 回放地址 |
| POST | `/api/video/playback/seek` | 当前实现返回回放地址语义 |
| GET | `/api/video/playback/download` | 下载录像文件 |
| POST | `/api/video/playback/stop` | 停止回放 |

实时流参数：`stream_type, stream_profile`；录像接口核心参数为 `channel_id, start_at/start_time, end_at/end_time`，并可带 `playback_mode, alarm_no`。精确兼容别名以 `platform_handler.go` 的读取逻辑为准。

## 10. curl 示例

```bash
curl -X POST http://127.0.0.1:8000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your-password"}'

curl http://127.0.0.1:8000/api/cameras \
  -H "Authorization: Bearer YOUR_TOKEN"
```
