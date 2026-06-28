# API 联调与调试指南

本文档说明如何使用 curl、Apifox、Postman 或 Swagger UI 调试后端接口。更完整的路由说明见 `api.md`，可导入的 OpenAPI 草案见 `openapi.yaml`。

## 1. 基础约定

后端默认地址：

```text
http://127.0.0.1:8000
```

API 前缀：

```text
/api
```

统一响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

错误响应：

```json
{
  "code": 400,
  "message": "错误原因",
  "data": {}
}
```

## 2. 导入 OpenAPI

文件位置：

```text
docs/openapi.yaml
```

Apifox：

1. 新建项目。
2. 选择“导入数据”。
3. 选择 OpenAPI/Swagger。
4. 导入 `docs/openapi.yaml`。
5. 设置环境变量 `baseUrl=http://127.0.0.1:8000`。

Postman：

1. Import。
2. 选择 `docs/openapi.yaml`。
3. 导入为 Collection。
4. 在 Authorization 中选择 Bearer Token。

Swagger UI：

可以使用任意 Swagger UI 容器或本地工具加载 `openapi.yaml`。如果通过浏览器直接访问文件受限，建议使用本地静态服务。

## 3. 健康检查

```powershell
curl http://127.0.0.1:8000/healthz
```

预期：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

具体 `data` 内容以实际代码返回为准。

## 4. 登录并获取 token

```powershell
curl -X POST http://127.0.0.1:8000/api/auth/login `
  -H "Content-Type: application/json" `
  -d "{\"username\":\"admin\",\"password\":\"<password>\"}"
```

返回中的核心字段：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "access_token": "<JWT>",
    "token_type": "Bearer",
    "expires_in": 86400
  }
}
```

保存 token：

```powershell
$token = "<JWT>"
```

## 5. 调用保护接口

```powershell
curl http://127.0.0.1:8000/api/auth/me `
  -H "Authorization: Bearer $token"
```

查询摄像机：

```powershell
curl "http://127.0.0.1:8000/api/cameras?keyword=&status=enabled" `
  -H "Authorization: Bearer $token"
```

分页查询告警：

```powershell
curl "http://127.0.0.1:8000/api/alarms?page=1&page_size=20&status=pending" `
  -H "Authorization: Bearer $token"
```

## 6. 常见请求参数

列表接口常见参数：

| 参数 | 说明 |
| --- | --- |
| `page` | 页码，默认 1。 |
| `page_size` | 每页条数，部分接口兼容 `pageSize`。 |
| `keyword` | 关键词。 |
| `status` | 状态。 |
| `factory_id` | 厂区 ID。 |
| `zone_id` | 区域 ID。 |
| `start_at` / `end_at` | 告警查询时间范围。 |
| `start_time` / `end_time` | 视频回放时间范围。 |

路径参数：

| 参数 | 说明 |
| --- | --- |
| `{id}` | 资源 ID。 |
| `{providerCode}` | 智能接口厂商编码。 |
| `{taskId}` | AI 任务 ID。 |
| `{ruleId}` | 智能绑定规则 ID。 |

## 7. 新增、编辑、删除示例

新增厂区：

```powershell
curl -X POST http://127.0.0.1:8000/api/factories `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d "{\"factoryCode\":\"F001\",\"factoryName\":\"一号厂区\",\"status\":\"enabled\"}"
```

更新厂区：

```powershell
curl -X PUT http://127.0.0.1:8000/api/factories/1 `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d "{\"factoryName\":\"一号厂区-更新\"}"
```

修改状态：

```powershell
curl -X PATCH http://127.0.0.1:8000/api/factories/1/status `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d "{\"status\":\"disabled\"}"
```

删除：

```powershell
curl -X DELETE http://127.0.0.1:8000/api/factories/1 `
  -H "Authorization: Bearer $token"
```

## 8. SSE 调试

接口：

```text
GET /api/sse/alarms?token=<JWT>
```

浏览器 EventSource 无法设置 Authorization 头，所以当前实现使用 query token。

curl 调试：

```powershell
curl -N "http://127.0.0.1:8000/api/sse/alarms?token=$token"
```

如果没有事件返回，先确认：

- token 是否有效。
- 后端是否有新告警。
- 代理是否支持长连接。
- Nginx 是否关闭响应缓冲。

## 9. 文件下载调试

导出告警：

```powershell
curl "http://127.0.0.1:8000/api/export/alarms" `
  -H "Authorization: Bearer $token" `
  -o alarms.xlsx
```

录像下载：

```powershell
curl "http://127.0.0.1:8000/api/video/playback/download?channel_id=1&start_time=2026-06-27T10:00:00%2B08:00&end_time=2026-06-27T10:05:00%2B08:00" `
  -H "Authorization: Bearer $token" `
  -o playback.mp4
```

## 10. 第三方接入调试提醒

当前代码中这些接口注册在 JWT 保护分组下：

- `POST /api/smart/events/ingest/:providerCode`
- `POST /api/smart/ai/callback`
- `POST /api/ai/events/callback`

如果第三方系统无法携带平台 JWT，需要先调整为公开回调路由，并增加以下保护之一：

- HMAC 签名。
- 时间戳 + nonce 防重放。
- IP 白名单。
- mTLS。
- 独立回调 token。

## 11. 常见错误定位

| 状态码 | 常见原因 | 排查方式 |
| --- | --- | --- |
| 400 | 请求体字段缺失或格式错误 | 检查 JSON、字段名、时间格式。 |
| 401 | 未登录、token 过期、token 签名不一致 | 重新登录，检查 Authorization。 |
| 403 | 权限不足 | 检查角色、菜单、按钮权限和后端权限策略。 |
| 404 | 路径或 HTTP 方法错误 | 对照 `router.go`、`api.md`、`openapi.yaml`。 |
| 500 | 服务内部错误 | 查看后端日志和数据库状态。 |

## 12. 维护 OpenAPI 的建议

当新增或修改接口时，同步更新：

1. `internal/http/router/router.go`
2. `docs/api.md`
3. `docs/openapi.yaml`
4. 相关前端 `src/api/*.ts`
5. 测试用例或调试示例

如果接口请求体已经稳定，建议把 `openapi.yaml` 中的 `GenericJson` 替换为明确 schema，避免前后端对字段理解不一致。
