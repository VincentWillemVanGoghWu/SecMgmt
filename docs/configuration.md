# 配置项说明

本文档集中说明后端、前端、Docker Compose 与部署脚本相关配置。配置项以当前代码为准。

## 1. 后端配置加载方式

后端配置位于 `internal/config/config.go`。

加载顺序：

1. 尝试读取项目根目录 `.env`。
2. 读取系统环境变量。
3. 未配置时使用代码默认值。

`MYSQL_DSN` 是必填项，缺失时服务会启动失败。

## 2. 后端环境变量

| 变量 | 默认值 | 是否生产必改 | 说明 |
| --- | --- | --- | --- |
| `APP_NAME` | `secmgmt-go` | 否 | 应用名称。 |
| `APP_ENV` | `development` | 建议 | 运行环境标识，可使用 `development`、`test`、`production`。 |
| `HTTP_PORT` | `8000` | 按需 | 后端 HTTP 端口。 |
| `MYSQL_DSN` | 无 | 是 | MySQL 连接串，缺失会启动失败。 |
| `REDIS_ADDR` | `127.0.0.1:6379` | 按需 | Redis 地址。当前代码有配置但未发现实际 Redis 客户端使用。 |
| `REDIS_DB` | `0` | 否 | Redis DB 编号。 |
| `JWT_SECRET_KEY` | `change-me` | 是 | JWT 签名密钥，生产必须替换。 |
| `DEVICE_SECRET_KEY` | 空 | 是 | 设备密码、推送密钥等敏感字段加密密钥。 |
| `JWT_EXPIRE_MINUTES` | `1440` | 建议 | JWT 有效期，单位分钟。 |
| `HIKVISION_SDK_PATH` | 根据系统自动推断 | 按需 | 海康 SDK 目录。 |
| `MEDIA_ROOT_DIR` | `<root>/media` | 按需 | 媒体文件、截图、下载文件等本地存储目录。 |
| `MEDIA_MOUNT_PATH` | `/media` | 按需 | 静态资源挂载路径。 |
| `BACKEND_PUBLIC_BASE_URL` | `http://127.0.0.1:8000` | 是 | 后端对外访问地址，用于生成可访问链接。 |
| `FFMPEG_PATH` | `ffmpeg` | 按需 | HLS 实时预览使用的 ffmpeg 可执行文件路径。 |
| `LIVE_HLS_SEGMENT_SECONDS` | `2` | 按需 | HLS 分片时长，单位秒。 |
| `LIVE_HLS_LIST_SIZE` | `6` | 按需 | HLS 播放列表保留的分片数量。 |
| `LIVE_HLS_START_TIMEOUT_SECONDS` | `30` | 按需 | 等待首个 HLS 播放列表生成的超时时间，H.265 起流或长 GOP 设备建议保留较大值。 |
| `LIVE_HLS_SESSION_TTL_SECONDS` | `300` | 按需 | HLS 会话复用过期时间，单位秒。 |
| `LIVE_HLS_MAX_SESSIONS` | `16` | 按需 | HLS 实时预览最大并发会话数。 |
| `LIVE_HLS_TRANSCODE` | `true` | 按需 | 是否将 RTSP 视频转码为兼容性更强的 H.264/yuv420p HLS；关闭时使用 `copy`，CPU 更低但浏览器兼容性取决于原始编码。 |
| `AI_CALLBACK_SECRET` | `change-ai-signature-secret` | 是 | AI 回调签名密钥。 |
| `PUSH_HTTP_TIMEOUT_SECONDS` | `10` | 按需 | HTTP 推送超时时间。 |

## 3. DSN 示例

本地 MySQL：

```env
MYSQL_DSN=secmgmt:secmgmt_password@tcp(127.0.0.1:3306)/secmgmt?charset=utf8mb4&parseTime=True&loc=Local
```

Docker Compose 内部网络：

```env
MYSQL_DSN=secmgmt:secmgmt_password@tcp(mysql:3306)/secmgmt?charset=utf8mb4&parseTime=True&loc=Local
```

注意：

- `parseTime=True` 对 Go 时间字段很重要。
- `loc=Local` 会受容器或系统时区影响，生产环境应统一时区。
- 用户名、密码、数据库名需要与 MySQL 初始化配置一致。

## 4. 推荐后端 `.env`

开发环境示例：

```env
APP_ENV=development
HTTP_PORT=8000
MYSQL_DSN=secmgmt:secmgmt_password@tcp(127.0.0.1:3306)/secmgmt?charset=utf8mb4&parseTime=True&loc=Local
JWT_SECRET_KEY=dev-only-change-me
DEVICE_SECRET_KEY=dev-device-secret-32-bytes-min
AI_CALLBACK_SECRET=dev-ai-callback-secret
BACKEND_PUBLIC_BASE_URL=http://127.0.0.1:8000
MEDIA_ROOT_DIR=./media
MEDIA_MOUNT_PATH=/media
FFMPEG_PATH=ffmpeg
LIVE_HLS_START_TIMEOUT_SECONDS=30
LIVE_HLS_TRANSCODE=true
PUSH_HTTP_TIMEOUT_SECONDS=10
```

生产环境至少应修改：

```env
APP_ENV=production
GIN_MODE=release
JWT_SECRET_KEY=<高强度随机值>
DEVICE_SECRET_KEY=<高强度随机值>
AI_CALLBACK_SECRET=<高强度随机值>
BACKEND_PUBLIC_BASE_URL=https://<生产域名>
MYSQL_DSN=<生产数据库连接串>
```

## 5. 海康 SDK 路径

默认路径：

| 系统 | 默认路径 |
| --- | --- |
| Windows | `third_party/HCNetSDK_Win64` |
| Linux | `third_party/HCNetSDK_Linux64` |

风险提醒：

- 仓库中 Linux SDK 目录名可能是 `HCNetSDK_linux64`。
- Linux 文件系统大小写敏感，必须确保代码、Dockerfile、目录名、`LD_LIBRARY_PATH` 完全一致。
- 如果不一致，常见表现是 SDK 加载失败、设备连接测试失败、容器启动后视频相关功能不可用。

可通过环境变量显式指定：

```env
HIKVISION_SDK_PATH=/app/third_party/HCNetSDK_Linux64
```

## 6. 媒体文件配置

| 配置 | 说明 |
| --- | --- |
| `MEDIA_ROOT_DIR` | 实际文件存储目录。 |
| `MEDIA_MOUNT_PATH` | HTTP 静态访问路径。 |

示例：

```env
MEDIA_ROOT_DIR=/data/secmgmt/media
MEDIA_MOUNT_PATH=/media
BACKEND_PUBLIC_BASE_URL=https://security.example.com
```

HLS 实时预览会在 `MEDIA_ROOT_DIR/live/...` 下生成临时 m3u8 和 ts 分片，并通过 `MEDIA_MOUNT_PATH` 对外访问。生产环境需要确保后端进程能执行 `FFMPEG_PATH`，且浏览器能访问 `BACKEND_PUBLIC_BASE_URL + MEDIA_MOUNT_PATH`。

生成访问地址时通常应组合为：

```text
https://security.example.com/media/xxx
```

运维建议：

- 将媒体目录挂载到独立磁盘或持久化卷。
- 定期清理临时截图、录像下载文件。
- 给媒体目录设置备份和容量告警。

## 7. 前端配置

前端配置主要来自 Vite 环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VITE_API_BASE_URL` | `http://127.0.0.1:8000/api` | 前端请求后端 API 的基础路径。 |

本地开发：

```env
VITE_API_BASE_URL=http://127.0.0.1:8000/api
```

生产同域代理：

```env
VITE_API_BASE_URL=/api
```

Dockerfile 中构建参数会传入：

```text
VITE_API_BASE_URL=/api
```

## 8. Docker Compose 变量

`docker-compose.yml` 使用较多外部变量，通常应由 `.env` 或部署脚本提供。

| 变量 | 说明 |
| --- | --- |
| `DOCKER_LIBRARY_MIRROR` | Docker 基础镜像前缀或镜像源。 |
| `DEBIAN_APT_MIRROR_URL` | Debian apt 源。 |
| `DEBIAN_APT_SECURITY_MIRROR_URL` | Debian security apt 源。 |
| `GOPROXY` | Go 模块代理。 |
| `NPM_REGISTRY` | npm 镜像源。 |
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码。 |
| `MYSQL_APP_DB` | 应用数据库名。 |
| `MYSQL_APP_USER` | 应用数据库用户。 |
| `MYSQL_APP_PASSWORD` | 应用数据库密码。 |
| `MYSQL_DATA_DIR` | MySQL 数据持久化目录。 |
| `BACKEND_ENV_FILE` | 后端环境变量文件路径。 |
| `MEDIA_DIR` | 后端媒体目录挂载路径。 |
| `BACKEND_PORT` | 宿主机后端端口，绑定到 `127.0.0.1`。 |
| `NGINX_CONF_FILE` | 前端 Nginx 配置文件。 |
| `FRONTEND_PORT` | 前端宿主机端口。 |

Compose 中后端端口绑定为：

```yaml
127.0.0.1:${BACKEND_PORT}:8000
```

这表示后端默认只暴露给本机，通常由前端 Nginx 或外部反向代理转发。

## 9. 配错后的常见表现

| 表现 | 可能原因 |
| --- | --- |
| 后端启动时报 `MYSQL_DSN is required` | 未配置 `MYSQL_DSN`。 |
| 登录接口 500 或连接失败 | DSN 用户、密码、地址、数据库名错误。 |
| 前端一直请求 `127.0.0.1:8000` | 生产构建时未设置 `VITE_API_BASE_URL=/api`。 |
| token 经常失效 | `JWT_EXPIRE_MINUTES` 设置过短，或服务重启后密钥变化。 |
| 设备密码解密失败 | `DEVICE_SECRET_KEY` 变更或历史明文/密文混用。 |
| AI 回调验签失败 | `AI_CALLBACK_SECRET` 与调用方不一致。 |
| 视频或 SDK 功能异常 | `HIKVISION_SDK_PATH`、SDK 动态库路径或目录大小写错误。 |
| 媒体文件链接打不开 | `BACKEND_PUBLIC_BASE_URL`、`MEDIA_ROOT_DIR`、`MEDIA_MOUNT_PATH` 配置不一致。 |

## 10. 生产配置检查清单

- [ ] `MYSQL_DSN` 使用独立应用账号，不使用 root。
- [ ] `JWT_SECRET_KEY` 已替换为高强度随机值。
- [ ] `DEVICE_SECRET_KEY` 已配置并妥善备份。
- [ ] `AI_CALLBACK_SECRET` 已替换为高强度随机值。
- [ ] `BACKEND_PUBLIC_BASE_URL` 是真实 HTTPS 域名。
- [ ] `GIN_MODE=release`。
- [ ] CORS 不再使用全开放 `*`，或通过反向代理收敛访问入口。
- [ ] 媒体目录已持久化并纳入备份/清理策略。
- [ ] MySQL 数据目录已持久化。
- [ ] 海康 SDK 路径和动态库路径已在目标系统验证。
