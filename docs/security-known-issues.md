# 安全与已知约束

## 1. 当前安全机制

- JWT Bearer 认证，有服务端签名和过期校验。
- 用户密码支持 bcrypt 和 PBKDF2-SHA256，不以明文返回。
- 设备、推送及 Provider secret 使用 Fernet 对称加密保存。
- 前端根据菜单与 permission code 控制页面和按钮显示。
- AI 回调配置了独立 secret，推送支持 secret/appSecret。
- 设备与告警等写操作集中在 service，可进行关联和状态校验。

## 2. 上线前必须处理

1. 替换 `JWT_SECRET_KEY`、`DEVICE_SECRET_KEY`、`AI_CALLBACK_SECRET` 的默认值，使用高熵随机值并进入密钥管理系统。
2. 修改/禁用初始化管理员凭据；确认 SQL seed 不会在发布时重置管理员。
3. 将 CORS 从 `AllowOrigins: ["*"]` 收紧到实际前端域名。
4. 增加后端接口级授权中间件。当前仅验证“已登录”，不能阻止已登录普通用户直接调用管理 API。
5. 为登录、SSE、回调、设备探测、全量状态检查和下载增加限流。
6. 生产必须启用 HTTPS；SSE query token 会进入浏览器历史、代理访问日志和监控系统，应改为短期 SSE ticket 或安全 cookie。
7. 审查设备浏览器登录接口，避免向非授权用户返回可复用凭据。
8. 对 Provider ingest/AI callback 设计无需普通用户 JWT 的专用鉴权路由，并验证时间戳、签名、防重放和来源范围。

## 3. 已知代码约束

| 项目 | 当前状态 | 影响/建议 |
|---|---|---|
| Linux SDK 路径大小写 | 实际目录 `HCNetSDK_linux64`，引用多为 `HCNetSDK_Linux64` | Linux 构建可能失败；统一命名 |
| Redis | 配置字段存在，未使用 | 不具备 token 黑名单、分布式限流或缓存能力 |
| Logout | 仅返回成功 | JWT 在过期前仍有效；需要黑名单或短 token + refresh token |
| 权限校验 | 前端可见性为主 | 必须补后端 permission/data-scope enforcement |
| 数据范围 | 登录响应包含 scope | 需逐项审计查询是否实际应用 scope |
| SQL 迁移 | 只有初始化 SQL | 引入 goose、golang-migrate 等版本化方案 |
| `PlatformService` | 单文件约 4000 行 | 拆为 user/device/alarm/push/smart/video services |
| 动态返回 | 多处 `map[string]any` | 难以静态检查和生成 OpenAPI；逐步 DTO 化 |
| 回调路由 | 位于 JWT protected group | 外部系统集成困难，且专用签名边界不清晰 |
| CORS | 允许任意 Origin | 生产收紧 |
| 媒体静态目录 | Gin 直接公开挂载 | 若媒体敏感，应改为鉴权下载或短期签名 URL |
| 旧设备密钥 | 兼容明文读取 | 制定扫描和重加密任务，消除数据库明文 |
| 日志 | Zap production + GORM warning | 确保不记录密码、token、secret、原始敏感事件 |

## 4. 代码质量观察

- 路由很多，但没有 OpenAPI/Swagger 契约；建议从明确 DTO 和 handler 注解生成规范。
- 设备 SDK、数据库、HTTP 推送耦合在 service 流程中，测试替身较难注入；建议抽象接口。
- 告警桥接启动失败不会使服务退出，这是可用性优先的设计，但健康检查目前无法反映 bridge degraded 状态。
- `/healthz` 更接近存活检查；应另增 readiness，检查 MySQL、媒体目录、SDK 和桥接状态。
- 当前自动测试覆盖率有限，尤其是告警状态机、外部回调和设备集成。

## 5. 推荐整改顺序

1. 密钥、初始化账号、HTTPS、CORS。
2. 后端权限与数据范围强制执行。
3. 外部回调专用认证、限流与防重放。
4. readiness/监控、日志脱敏、备份恢复演练。
5. 数据库迁移体系与关键路径自动测试。
6. service 拆分、DTO/OpenAPI、SDK 接口化。
