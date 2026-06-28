# 开发、部署与运维

## 1. 环境变量

| 变量 | 默认值 | 必填/说明 |
|---|---|---|
| `APP_NAME` | `secmgmt-go` | 应用名，目前主要用于标识 |
| `APP_ENV` | `development` | 环境标识 |
| `HTTP_PORT` | `8000` | 后端监听端口 |
| `MYSQL_DSN` | 无 | 必填，GORM MySQL DSN |
| `REDIS_ADDR` | `127.0.0.1:6379` | 已读取但当前未建立 Redis 客户端 |
| `REDIS_DB` | `0` | 同上 |
| `JWT_SECRET_KEY` | `change-me` | 生产必须替换 |
| `JWT_EXPIRE_MINUTES` | `1440` | JWT 有效期 |
| `DEVICE_SECRET_KEY` | 空 | 设备/推送 secret 的 Fernet 密钥；生产必配并备份 |
| `HIKVISION_SDK_PATH` | 按 OS 推导 | HCNetSDK 根目录 |
| `MEDIA_ROOT_DIR` | `<root>/media` | 媒体物理目录，启动时自动创建 |
| `MEDIA_MOUNT_PATH` | `/media` | Gin 静态访问路径 |
| `BACKEND_PUBLIC_BASE_URL` | `http://127.0.0.1:8000` | 生成媒体绝对 URL |
| `AI_CALLBACK_SECRET` | `change-ai-signature-secret` | AI 回调签名 secret |
| `PUSH_HTTP_TIMEOUT_SECONDS` | `10` | 外部推送 HTTP 超时 |
| `VITE_API_BASE_URL` | `http://127.0.0.1:8000/api` | 前端构建/开发 API 地址 |

Fernet key 可用项目所采用的 fernet 库/安全随机源生成。轮换 `DEVICE_SECRET_KEY` 前应设计双密钥解密和重加密流程，不可直接覆盖。

## 2. 本地开发

### Windows

1. 安装 Go、Node/npm、MySQL。
2. 初始化数据库并创建 `.env`。
3. 确认 `third_party/HCNetSDK_Win64/Library` 中 DLL 完整。
4. 执行 `run.bat`；脚本在前端依赖缺失时自动 `npm install`，随后打开两个终端窗口。

手工启动后端：

```powershell
go mod download
go run ./cmd/server
```

手工启动前端：

```powershell
cd frontend
npm ci
npm run dev
```

### Linux

Linux SDK 使用 CGO 和 C++ bridge，需要 GCC/G++、SDK headers、`libhcnetsdk.so` 及其依赖。运行时设置：

```bash
export CGO_ENABLED=1
export LD_LIBRARY_PATH="$PWD/third_party/HCNetSDK_Linux64/Library:$PWD/third_party/HCNetSDK_Linux64/Library/HCNetSDKCom"
go run ./cmd/server
```

仓库当前目录名显示为 `HCNetSDK_linux64`，而 Go CGO、Dockerfile、Compose 与默认配置引用 `HCNetSDK_Linux64`。Linux 文件系统区分大小写，部署前必须统一目录名和所有引用，否则编译阶段 COPY/include 或运行期动态库加载会失败。

## 3. 构建与测试

```powershell
go test ./...
go vet ./...
go build -o build/secmgmt-go.exe ./cmd/server

cd frontend
npm ci
npm run build
```

现有 Go 自动化测试主要覆盖设备 secret 加解密；上线前至少增加：

- JWT、密码兼容和认证 handler 测试；
- Repository 过滤/分页的 MySQL 集成测试；
- 告警状态机、去重/冷却、推送重试、AI 回调测试；
- 前端路由守卫与关键 API 交互测试；
- Windows/Linux 各一套海康真机冒烟测试。

## 4. Docker Compose 部署

Compose 包含 MySQL、backend、frontend 三个服务。MySQL 数据和 media 均挂载宿主机目录；后端仅绑定 `127.0.0.1:${BACKEND_PORT}`，前端 Nginx 对外暴露 `${FRONTEND_PORT}` 并反代 API。

部署变量由 `deploy_ubuntu.sh` 生成到：

```text
/opt/secmgmt_go/runtime/generated/backend.env
/opt/secmgmt_go/runtime/generated/compose.env
/opt/secmgmt_go/runtime/generated/nginx.default.conf
```

典型部署：

```bash
chmod +x deploy_ubuntu.sh
sudo ./deploy_ubuntu.sh
```

常用覆盖变量：

```bash
sudo INSTALL_DIR=/opt/secmgmt_go \
  FRONTEND_PORT=80 \
  BACKEND_PORT=8000 \
  SERVER_NAME=example.internal \
  PUBLIC_BASE_URL=http://example.internal \
  MYSQL_APP_DB=steel_hikvision_monitor \
  MYSQL_APP_USER=secmgmt \
  ./deploy_ubuntu.sh
```

脚本支持强制安装/重建/重初始化等参数，具体以脚本 `usage` 输出为准。数据库重初始化属于破坏性操作，必须先备份 runtime/mysql。

## 5. 生产 Nginx 要点

- `/api` 反代 backend，SSE 路由关闭响应缓冲并放宽读取超时。
- `/media` 可由 backend 提供，也可由 Nginx 直接映射同一媒体卷。
- 海康 WebControl 需要代理 `/ISAPI`、`/SDK`、`/webSocketVideoCtrlProxy`。
- 返回 `Cross-Origin-Opener-Policy: same-origin`、`Cross-Origin-Embedder-Policy: require-corp` 和合适的 `Cross-Origin-Resource-Policy`。
- 上传/录像下载需调整 `client_max_body_size`、proxy timeout 和 buffering。

## 6. 运维与排障

### 服务与日志

```bash
docker compose --env-file runtime/generated/compose.env ps
docker compose --env-file runtime/generated/compose.env logs -f backend
docker compose --env-file runtime/generated/compose.env logs -f frontend
docker compose --env-file runtime/generated/compose.env logs -f mysql
```

后端应用日志使用结构化 Zap，GORM 只记录 warning 级别及超过 1 秒的慢 SQL。

### 常见问题

| 现象 | 检查 |
|---|---|
| 启动提示 `MYSQL_DSN is required` | `.env` 所在工作目录及 DSN |
| MySQL 时间扫描失败 | DSN 是否有 `parseTime=True&loc=Local` |
| SDK 编译/加载失败 | 目录大小写、CGO、动态库依赖、`LD_LIBRARY_PATH` |
| 设备登录失败 | IP、SDK 端口（常为 8000）、用户名密码、设备锁定策略 |
| 前端 401 循环 | JWT secret 是否在重启后变化、token 是否过期 |
| WebControl 黑屏 | Nginx/Vite ISAPI 与 WS 代理、跨源隔离 headers、浏览器控制台 |
| SSE 不刷新 | query token、反代 buffering/timeout、连接数 |
| 媒体 404 | `MEDIA_ROOT_DIR`、`MEDIA_MOUNT_PATH`、公网 base URL 与卷挂载 |
| 告警桥接未工作但 API 正常 | 启动只 warning；查看 backend 日志及智能绑定是否启用 |

## 7. 备份与恢复

至少备份 MySQL 和 media，两者应采用一致的时间点：

```bash
mysqldump --single-transaction steel_hikvision_monitor > secmgmt.sql
tar -czf secmgmt-media.tar.gz /opt/secmgmt_go/runtime/media
```

同时离线保存 `DEVICE_SECRET_KEY`、JWT/AI secrets 和部署配置。没有设备密钥的数据库备份无法恢复设备凭据。
