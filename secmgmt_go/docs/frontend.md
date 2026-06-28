# 前端开发文档

本文档说明前端项目结构、开发约定、权限与接口调用方式。前端位于 `frontend` 目录，使用 Vue 3、Vite、TypeScript、Pinia、Vue Router、Element Plus、Axios、ECharts 与视频播放相关组件。

## 1. 技术栈

| 类型 | 技术 |
| --- | --- |
| 构建工具 | Vite |
| 框架 | Vue 3 |
| 类型系统 | TypeScript |
| 路由 | Vue Router |
| 状态管理 | Pinia |
| UI 组件 | Element Plus |
| HTTP | Axios |
| 图表 | ECharts |
| 视频 | hls.js、hikvideoctrl、海康 WebControl 相关静态资源 |

## 2. 目录结构

| 目录 | 职责 |
| --- | --- |
| `src/api` | 后端接口封装。 |
| `src/types` | 前端 TypeScript 类型。 |
| `src/router` | 路由定义与路由守卫。 |
| `src/stores` | Pinia 状态。 |
| `src/views` | 页面级组件。 |
| `src/components` | 通用组件、布局组件、视频组件、告警组件。 |
| `src/layouts` | 应用主布局。 |
| `src/directives` | 自定义指令，例如权限指令。 |
| `src/utils` | 时间、权限、菜单图标、实时连接等工具函数。 |
| `src/styles` | 全局样式、组件样式、页面样式、设计 token。 |
| `public/codebase` | 海康 WebControl 与无插件播放所需静态资源。 |

## 3. 启动与构建

安装依赖：

```powershell
cd frontend
npm install
```

本地开发：

```powershell
npm run dev
```

生产构建：

```powershell
npm run build
```

预览构建结果：

```powershell
npm run preview
```

## 4. 环境变量

前端主要使用：

```env
VITE_API_BASE_URL=http://127.0.0.1:8000/api
```

默认值定义在 `src/api/http.ts`：

```ts
export const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8000/api"
```

部署到 Nginx 并由同域代理 `/api` 时，推荐设置：

```env
VITE_API_BASE_URL=/api
```

## 5. HTTP 封装

代码位置：`src/api/http.ts`

主要职责：

- 创建 Axios 实例。
- 设置基础地址。
- 请求时自动携带 JWT。
- 响应 401 时清除 token 并跳转登录页。
- 响应 403 时弹出无权限提示。
- 封装文件下载。

token 存储键：

```ts
steel-monitor-access-token
```

新增 API 文件建议：

1. 在 `src/types` 中定义类型。
2. 在 `src/api` 中新建接口文件。
3. 所有请求复用 `http` 实例。
4. 列表接口统一支持分页、关键词、状态等参数时，应复用现有页面模式。

## 6. 路由与页面

路由定义在 `src/router/routes.ts`。

当前主要页面：

| 路径 | 页面 |
| --- | --- |
| `/login` | 登录页 |
| `/dashboard` | 首页驾驶舱 |
| `/safety/realtime-alarms` | 实时告警 |
| `/safety/alarm-list` | 告警查询 |
| `/safety/alarm-stats` | 告警统计 |
| `/monitor/preview` | 监控预览 |
| `/monitor/playback` | 录像查看 |
| `/monitor/ai-api` | 智能接口 |
| `/device/cameras` | 摄像机管理 |
| `/device/recorders` | 录像机管理 |
| `/device/channels` | 通道管理 |
| `/device/status-logs` | 设备状态日志 |
| `/push/config` | 推送配置 |
| `/push/logs` | 推送日志 |
| `/system/users` | 用户管理 |
| `/system/roles` | 角色权限 |
| `/master-data/factories` | 厂区管理 |
| `/master-data/zones` | 区域管理 |
| `/master-data/depts` | 部门管理 |
| `/master-data/dicts` | 字典管理 |

注意：当前 `routes.ts` 中部分中文标题显示为乱码，通常是文件编码或读取方式导致。建议统一保存为 UTF-8。

## 7. 状态管理

Pinia store 位于 `src/stores`：

| Store | 职责 |
| --- | --- |
| `auth.ts` | 登录态、用户信息、token。 |
| `permission.ts` | 权限、菜单、可访问页面。 |
| `realtime.ts` | 实时告警连接与数据。 |
| `app.ts` | 应用级 UI 状态。 |

推荐约定：

- 登录成功后统一写入 token 与用户信息。
- 退出登录时清理 token、用户信息、权限缓存。
- 页面组件不要直接操作 localStorage，尽量通过 store 或 HTTP 层处理。

## 8. 权限控制

相关代码：

- `src/directives/permission.ts`
- `src/utils/permission.ts`
- `src/stores/permission.ts`
- `src/components/common/AccessDeniedState.vue`

当前权限主要用于：

- 控制菜单显示。
- 控制按钮或操作入口显示。
- 显示无权限状态。

重要提醒：

- 前端权限只能改善体验，不能作为安全边界。
- 后端也需要补充接口级权限校验，尤其是用户、角色、设备、告警处理、推送配置等写操作。

## 9. 页面开发规范

推荐一个业务页面按以下结构组织：

```text
views/<module>/<FeatureView.vue>
api/<feature>.ts
types/<feature>.ts
components/<module>/<FeatureDialog.vue>
```

页面职责建议：

- `View.vue`：负责页面状态、列表查询、弹窗开关。
- `api/*.ts`：只负责请求后端。
- `types/*.ts`：只定义类型。
- `components/*.vue`：复用表单、详情、弹窗。

表格页常见结构：

1. 搜索条件。
2. 操作按钮。
3. 数据表格。
4. 分页。
5. 新增/编辑弹窗。
6. 删除、启停、测试等二次确认。

## 10. 实时告警

相关代码：

- `src/services/realtime/alarmRealtimeClient.ts`
- `src/stores/realtime.ts`
- `src/components/realtime/RealtimeAlarmNotifier.vue`
- 后端接口：`GET /api/sse/alarms?token=<JWT>`

说明：

- 浏览器 EventSource 不能设置 Authorization 头，因此 SSE 使用 query token。
- 前端应处理断线重连、重复消息、登录过期等情况。
- 生产环境必须使用 HTTPS，避免 token 在传输中泄露。

## 11. 视频组件

相关组件：

- `src/components/video/VideoPlayer.vue`
- `src/components/video/HikWebControlPlayer.vue`
- `src/components/video/HikWebControlPlaybackPlayer.vue`
- `src/components/video/HikWebControlGrid.vue`
- `src/components/video/hikProxyRouting.ts`

静态资源：

- `public/codebase/webVideoCtrl.js`
- `public/codebase/jsPlugin`
- `public/codebase/encryption`

开发注意：

- 视频播放通常依赖浏览器兼容性、插件/无插件资源、设备网络可达性。
- 如果本地开发正常、服务器异常，优先检查 Nginx 静态资源路径和后端返回的播放地址。
- 如果画面黑屏但接口成功，继续检查流协议、跨域、浏览器控制台和设备侧限制。

## 12. 新增前端页面步骤

1. 在 `src/types` 新增类型。
2. 在 `src/api` 新增接口封装。
3. 在 `src/views` 新增页面组件。
4. 在 `src/router/routes.ts` 注册路由。
5. 如需菜单展示，检查后端菜单数据与权限配置。
6. 如需按钮权限，增加权限点并使用权限指令。
7. 运行 `npm run build` 验证类型和构建。

## 13. 前端提交前检查

```powershell
cd frontend
npm run build
```

检查项：

- TypeScript 无类型错误。
- Vite 构建成功。
- 登录、菜单、路由跳转正常。
- 401 能回到登录页。
- 403 能给出提示。
- 文件下载能得到正确文件名。
- 视频、SSE 等特殊能力在目标浏览器验证通过。
