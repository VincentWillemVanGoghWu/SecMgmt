# 安全管理平台 Go 版开发文档

> 文档基线：2026-06-27。内容根据 `secmgmt_go` 当前代码整理，适合作为开发交接、部署实施、运维排障和后续扩展的工程文档。

## 1. 项目概述

本项目是一套面向厂区视频安防管理的全栈系统，包含设备档案、海康摄像机/NVR 接入、实时预览、录像回放、设备状态、告警闭环、消息推送、智能事件、AI 复核、基础资料和 RBAC 管理。

项目由以下部分组成：

| 部分 | 技术 | 位置 |
| --- | --- | --- |
| 后端 API | Go、Gin、GORM、MySQL、JWT、Zap | `cmd/`、`internal/` |
| 管理前端 | Vue 3、TypeScript、Vite、Pinia、Element Plus、ECharts | `frontend/` |
| 数据库 | MySQL 8，DDL 与初始化数据 | `sql/init_database.sql` |
| 设备集成 | 海康 HCNetSDK，Windows DLL / Linux SO + CGO | `internal/integration/hikvision/`、`third_party/` |
| 部署 | Docker Compose、Nginx、Ubuntu 自动部署脚本 | `docker-compose.yml`、`deploy_ubuntu.sh` |

## 2. 文档导航

建议按角色阅读：

| 角色 | 推荐阅读 |
| --- | --- |
| 新接手开发 | `README.md`、`architecture.md`、`backend-modules.md`、`frontend.md` |
| 后端开发 | `backend-modules.md`、`api.md`、`database.md`、`configuration.md` |
| 前端开发 | `frontend.md`、`api.md`、`configuration.md` |
| 实施部署 | `development-deployment.md`、`configuration.md`、`troubleshooting.md` |
| 测试验收 | `testing.md`、`api.md`、`troubleshooting.md` |
| 安全整改 | `security-known-issues.md`、`configuration.md`、`backend-modules.md` |

完整文档列表：

- [系统架构与代码说明](architecture.md)：分层架构、启动生命周期、核心链路、前后端协作方式。
- [后端模块开发文档](backend-modules.md)：认证、基础资料、设备、海康 SDK、告警、推送、智能接口、视频和报表模块说明。
- [前端开发文档](frontend.md)：Vue 项目结构、路由、状态管理、权限、HTTP 封装、页面开发规范。
- [API 接口说明](api.md)：统一响应、认证方式、公开路由、保护路由、核心接口清单。
- [API 联调与调试指南](api-debugging.md)：curl、Apifox、Postman、SSE、文件下载和常见错误定位。
- [OpenAPI 草案](openapi.yaml)：可导入 Apifox、Postman 或 Swagger UI 的接口描述文件。
- [数据库设计](database.md)：表分组、核心关系、初始化数据、数据库变更规范。
- [RBAC 权限设计方案](rbac-design.md)：角色、菜单、按钮、接口、数据范围的一体化权限设计与实施路径。
- [配置项说明](configuration.md)：后端、前端、Docker Compose、SDK、媒体目录和生产配置检查。
- [开发、部署与运维](development-deployment.md)：本地启动、构建、Docker Compose、Ubuntu 部署和运维要点。
- [测试与验收文档](testing.md)：后端、前端、数据库、接口、设备、视频、告警、推送、AI 的验收清单。
- [告警 AI 复核开发建议](ai-alarm-review-design.md)：告警生成后自动排队调用视觉模型、Prompt/JSON 协议、结果回写与等级调整方案。
- [常见问题排查手册](troubleshooting.md)：登录、接口、数据库、SDK、视频、SSE、推送、Docker 等问题排查。
- [安全与已知约束](security-known-issues.md)：当前安全机制、上线前必须整改项和风险建议。

## 3. 快速开始

### 3.1 前置环境

- Go 版本与 `go.mod`、`Dockerfile.backend` 保持一致。
- Node.js 与 npm。
- MySQL 8.0。
- 如需设备能力，准备与操作系统匹配的海康 HCNetSDK 动态库。

### 3.2 初始化数据库

```powershell
mysql -u root -p < sql/init_database.sql
```

初始化脚本会创建数据库、表结构、菜单权限、字典、智能能力和初始管理数据。初始账号密码应以 SQL 脚本为准，首次登录后必须修改密码。

### 3.3 配置后端

在 `secmgmt_go/.env` 创建配置，最小示例：

```dotenv
APP_ENV=development
HTTP_PORT=8000
MYSQL_DSN=secmgmt:your_password@tcp(127.0.0.1:3306)/steel_hikvision_monitor?charset=utf8mb4&parseTime=True&loc=Local
JWT_SECRET_KEY=replace-with-a-long-random-secret
DEVICE_SECRET_KEY=replace-with-a-device-secret
HIKVISION_SDK_PATH=third_party/HCNetSDK_Win64
MEDIA_ROOT_DIR=media
MEDIA_MOUNT_PATH=/media
BACKEND_PUBLIC_BASE_URL=http://127.0.0.1:8000
AI_CALLBACK_SECRET=replace-with-an-ai-callback-secret
```

完整配置见 [配置项说明](configuration.md)。

### 3.4 启动服务

Windows 可直接运行：

```powershell
.\run.bat
```

也可以分别启动：

```powershell
go run ./cmd/server
```

```powershell
cd frontend
npm install
npm run dev
```

默认访问地址：

- 后端：`http://127.0.0.1:8000`
- 健康检查：`http://127.0.0.1:8000/healthz`
- 前端：`http://127.0.0.1:5173`

## 4. 目录结构

```text
secmgmt_go/
├── cmd/server/                 # 后端启动入口
├── internal/
│   ├── bootstrap/              # 依赖组装
│   ├── config/                 # 环境变量配置
│   ├── database/               # MySQL/GORM 初始化
│   ├── domain/entity/          # 数据库实体
│   ├── domain/dto/             # API DTO
│   ├── http/                   # Router、Handler、中间件、统一响应
│   ├── integration/hikvision/  # HCNetSDK 跨平台适配
│   ├── repository/             # 数据访问
│   ├── service/                # 业务逻辑
│   └── util/                   # JWT、密码、设备密钥
├── frontend/                   # Vue 管理端
├── sql/                        # 数据库初始化脚本
├── third_party/                # 海康 SDK 与 WebSDK
├── media/                      # 告警图片、录像等运行数据
├── docs/                       # 开发文档
├── Dockerfile.backend
├── docker-compose.yml
├── deploy_ubuntu.sh
└── run.bat
```

## 5. 开发入口速查

| 修改目标 | 首要文件 |
| --- | --- |
| 新增 API | `internal/http/router/router.go`、`internal/http/handler/` |
| 新增业务规则 | `internal/service/` |
| 新增数据访问 | `internal/repository/repository.go` |
| 新增表 | `sql/init_database.sql`、`internal/domain/entity/models.go` |
| 修改认证/菜单 | `internal/service/auth_service.go`、`internal/http/middleware/auth.go` |
| 修改设备 SDK | `internal/integration/hikvision/` |
| 新增前端页面 | `frontend/src/views/`、`frontend/src/router/routes.ts`、`frontend/src/api/` |
| 修改部署 | `deploy_ubuntu.sh`、`docker-compose.yml`、`Dockerfile.backend`、`frontend/Dockerfile` |

## 6. 基本验证

后端：

```powershell
go test ./...
```

前端：

```powershell
cd frontend
npm run build
```

涉及设备能力时，还需要使用真实摄像机或 NVR 验证登录、布防、预览、抓图、通道同步、录像检索和录像下载。单元测试无法覆盖厂商动态库和真实设备协议行为。

## 7. 当前重点风险

- 后端目前主要做 JWT 登录态校验，接口级权限点校验仍需加强。
- CORS 当前为全开放，生产环境建议收敛来源或通过同域反向代理访问。
- Linux 海康 SDK 目录名需要与代码、Dockerfile、`LD_LIBRARY_PATH` 保持大小写一致。
- 智能事件接入和 AI 回调接口当前部分注册在 JWT 保护分组下，第三方接入前需要确认鉴权方案。
- 生产环境必须替换 `JWT_SECRET_KEY`、`DEVICE_SECRET_KEY`、`AI_CALLBACK_SECRET` 等默认密钥。

## 8. 开发注意事项

新增功能或修复线上问题前，建议先阅读 [development-notes.md](development-notes.md)。该文档记录当前代码中需要特别注意的问题，包括推送限流统计、移动侦测去重/冷却、N+1 查询、全量列表、报表聚合、复合索引、设备状态检查、前端大组件和编码文案等，避免后续开发重复踩坑。
