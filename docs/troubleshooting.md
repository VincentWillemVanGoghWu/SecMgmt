# 常见问题排查手册

本文档面向开发、实施和运维，按现象给出排查路径。遇到问题时建议先确认：后端日志、浏览器控制台、网络请求、数据库状态、容器状态。

## 1. 后端启动失败

### 现象：提示 `MYSQL_DSN is required`

原因：

- 未配置 `MYSQL_DSN`。
- `.env` 不在后端识别的项目根目录。
- 环境变量名写错。

处理：

1. 检查 `secmgmt_go/.env` 是否存在。
2. 检查变量名是否为 `MYSQL_DSN`。
3. 用系统环境变量覆盖时，确认当前终端已生效。

### 现象：MySQL 连接失败

可能原因：

- MySQL 未启动。
- DSN 地址或端口错误。
- 用户名或密码错误。
- 数据库不存在。
- Docker 内部应使用服务名 `mysql`，但配置成了 `127.0.0.1`。

处理：

```powershell
mysql -h 127.0.0.1 -P 3306 -u <user> -p
```

Docker Compose 内部应使用：

```text
tcp(mysql:3306)
```

本机开发通常使用：

```text
tcp(127.0.0.1:3306)
```

## 2. 前端访问接口失败

### 现象：前端请求 `127.0.0.1:8000` 失败

原因：

- 生产构建时未设置 `VITE_API_BASE_URL=/api`。
- 浏览器中的 `127.0.0.1` 指向用户自己的电脑，而不是服务器。

处理：

- 本地开发使用 `http://127.0.0.1:8000/api`。
- 生产部署推荐使用 `/api` 并由 Nginx 代理到后端。

### 现象：接口 404

排查：

1. 确认前端请求路径是否带 `/api`。
2. 确认 Nginx 是否正确代理 `/api`。
3. 确认后端 `router.go` 中是否注册该路由。
4. 确认 HTTP 方法是否正确，例如 `GET`、`POST`、`PUT`、`PATCH`、`DELETE`。

### 现象：接口跨域错误

当前后端 CORS 允许所有来源：

```go
AllowOrigins: []string{"*"}
```

如果仍出现跨域错误，通常是：

- 请求没有到达后端，被 Nginx 或浏览器策略拦截。
- 预检请求 `OPTIONS` 未被代理。
- 前端请求地址和代理配置不一致。

生产建议：

- 通过 Nginx 做同域代理，减少跨域问题。
- 收敛 CORS 来源，不建议长期使用 `*`。

## 3. 登录和 token 问题

### 现象：登录成功后刷新又回登录页

可能原因：

- token 未写入 localStorage。
- 浏览器隐私策略或无痕模式限制。
- 后端返回结构变化，前端未正确解析。
- `JWT_SECRET_KEY` 改变导致旧 token 失效。

处理：

1. 打开浏览器 DevTools。
2. 查看 localStorage 是否存在 `steel-monitor-access-token`。
3. 查看 `/api/auth/me` 是否返回 401。

### 现象：所有保护接口返回 401

排查：

- 请求头是否存在 `Authorization: Bearer <token>`。
- token 是否过期。
- 后端 `JWT_SECRET_KEY` 是否与签发 token 时一致。
- 系统时间是否异常。

### 现象：前端无权限，但直接调接口成功

原因：

- 当前后端主要校验 JWT 登录态，接口级权限校验不足。

处理：

- 在后端增加权限点中间件。
- 将路由和权限点建立映射。
- 对写操作和敏感查询优先加固。

## 4. 数据库问题

### 现象：页面列表为空

排查：

- 表中是否有数据。
- 数据状态是否为启用。
- 当前用户数据权限是否限制了结果。
- 查询条件是否过窄。
- 前端是否传了错误分页参数。

### 现象：中文乱码

排查：

- 数据库字符集是否为 `utf8mb4`。
- 表和字段排序规则是否为 `utf8mb4_unicode_ci` 或兼容配置。
- SQL 文件是否以 UTF-8 保存。
- 前端源码是否以 UTF-8 保存。

当前注意：

- `frontend/src/router/routes.ts` 中部分中文标题显示乱码，建议统一检查文件编码。

### 现象：时间不一致

排查：

- MySQL 时区。
- Docker 容器时区。
- Go DSN 中 `loc=Local`。
- 前端展示是否做了时区转换。

建议：

- 服务器、数据库、容器统一 `Asia/Shanghai`。
- 日志中明确记录时区。

## 5. 海康 SDK 问题

### 现象：设备连接测试失败

排查：

1. 设备 IP 和端口是否可达。
2. 账号密码是否正确。
3. 设备是否允许当前服务器 IP 登录。
4. SDK 动态库是否加载成功。
5. `HIKVISION_SDK_PATH` 是否正确。
6. Linux 下目录大小写是否一致。

### 现象：Linux/Docker 正常编译但运行失败

常见原因：

- 动态库路径不在 `LD_LIBRARY_PATH`。
- Dockerfile 拷贝目录名与实际目录名不一致。
- 代码、镜像配置或服务器实际目录名大小写不一致。
- 缺少 SDK 依赖的系统库。

处理：

- 统一目录名。
- 检查容器环境变量：

```powershell
docker compose exec backend printenv LD_LIBRARY_PATH
```

- 进入容器查看 SDK 文件是否存在。

## 6. 视频播放问题

### 现象：接口成功但画面黑屏

排查：

- 播放地址是否可直接访问。
- 浏览器控制台是否有跨域错误。
- 视频流协议是否被浏览器支持。
- 设备是否达到最大预览连接数。
- WebControl 或无插件资源是否加载成功。
- HTTPS 页面是否加载了 HTTP 视频资源，导致混合内容拦截。

### 现象：WebControl 静态资源 404

排查：

- `frontend/public/codebase` 是否被打包到前端静态目录。
- Nginx 是否正确托管前端 `dist`。
- 请求路径是否大小写一致。

### 现象：录像下载失败

排查：

- 后端媒体目录是否可写。
- 磁盘空间是否充足。
- 录像时间范围是否有文件。
- 下载接口是否携带 token。
- 浏览器是否拦截下载。

## 7. SSE 实时告警问题

### 现象：实时告警页面没有消息

排查：

1. 浏览器 Network 中是否有 `/api/sse/alarms?token=...` 连接。
2. token 是否有效。
3. 后端是否有新告警进入。
4. 反向代理是否支持长连接。
5. Nginx 是否关闭了响应缓冲。

Nginx 代理 SSE 建议：

```nginx
proxy_http_version 1.1;
proxy_set_header Connection "";
proxy_buffering off;
proxy_read_timeout 3600s;
```

### 现象：SSE 经常断开

可能原因：

- 代理超时过短。
- 网络不稳定。
- token 过期。
- 后端连接管理异常。

处理：

- 延长代理超时。
- 前端增加重连策略。
- 缩短页面空闲连接数量。

## 8. 推送失败

排查：

- 推送配置是否启用。
- 目标 URL 是否可达。
- Header、签名、密钥是否正确。
- 目标系统是否要求 HTTPS。
- 超时时间是否过短。
- 失败日志是否记录响应码和响应体。

处理：

- 先使用“测试推送配置”验证。
- 再触发真实告警推送。
- 对失败日志执行重试并观察错误是否一致。

## 9. 智能接口和 AI 回调失败

### 现象：第三方事件推不进来

可能原因：

- 接口当前需要 JWT。
- providerCode 不存在或被禁用。
- 请求体格式不符合处理逻辑。
- 签名或密钥不匹配。

处理：

- 确认路由：`POST /api/smart/events/ingest/:providerCode`。
- 若第三方无法携带 JWT，应调整为公开回调路由，并使用签名验权。

### 现象：AI 回调失败

排查：

- `AI_CALLBACK_SECRET` 是否一致。
- 任务 ID 是否存在。
- 回调地址是否正确。
- 回调接口是否要求 JWT。
- 请求时间戳是否超过允许窗口。

## 10. Docker Compose 问题

### 现象：MySQL 一直 unhealthy

排查：

- `MYSQL_ROOT_PASSWORD` 是否为空。
- 数据目录是否有旧数据且密码不一致。
- 磁盘空间是否不足。
- 容器日志是否有初始化错误。

```powershell
docker compose logs mysql
```

### 现象：backend 起不来

排查：

```powershell
docker compose logs backend
```

重点看：

- DSN。
- SDK 路径。
- 媒体目录权限。
- 数据库是否健康。

### 现象：frontend 能打开但接口不通

排查：

- Nginx 配置是否代理 `/api`。
- Compose 中 backend 只绑定 `127.0.0.1:${BACKEND_PORT}:8000` 是否符合部署方式。
- 前端构建参数是否为 `VITE_API_BASE_URL=/api`。

## 11. 处理问题时建议收集的信息

提交问题给开发时，建议提供：

- 操作账号和角色。
- 操作页面。
- 复现步骤。
- 发生时间。
- 浏览器控制台错误截图。
- Network 请求 URL、状态码、响应内容。
- 后端日志。
- Docker 日志。
- 数据库相关记录。
- 是否 Windows、本机 Linux、Docker Linux 环境。

信息越完整，排障越快。这个系统牵涉前端、Go 服务、MySQL、设备 SDK、Nginx 和真实设备网络，很多问题不是单点故障，最好按链路一段段切开。
