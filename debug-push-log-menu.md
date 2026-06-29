# Debug Session: push-log-menu

- Status: OPEN
- Created: 2026-06-29
- Symptom: docker 部署后“推送日志”菜单打不开

## Hypotheses

1. 前端路由中“推送日志”对应的 `path` 或组件懒加载路径在容器构建后失效，导致点击菜单时报前端路由错误。
2. 当前账号在 docker 环境下缺少“推送日志”菜单或接口权限码，菜单点击后触发 `403`，表现为页面打不开。
3. 前端生产环境的接口基地址、反向代理或 `history` 路由回退配置异常，导致“推送日志”相关资源请求 `404`。
4. 后端 docker 环境未加载最新权限/菜单数据，`/api/auth/me` 返回的菜单树缺少或错误配置了“推送日志”节点。
5. “推送日志”页面依赖的接口或字段在生产数据库中异常，导致页面初始化时报错中断渲染。

## Evidence Log

- `frontend/src/router/routes.ts` 中已存在 `push-logs` 路由，前端页面组件 `frontend/src/views/push/PushLogView.vue` 也存在，静态路由定义无缺失。
- `internal/http/router/router.go` 已注册 `GET /api/push/logs`，并绑定权限码 `push:log:view`，说明菜单页初始化依赖后端权限。
- `sql/init_database.sql` 已包含 `sys_menu.code = push-logs`、`sys_permission.code = push:log:view`、`push:log:retry` 以及内置角色的关联关系，说明最新初始化脚本是完整的。
- `internal/database/database.go` 当前仅对 `operation_log` 和 `system_setting` 做 `AutoMigrate`，不会把新增的菜单/权限/角色关系同步到已存在的 MySQL 数据卷。
- `internal/service/operation_log_service.go` 的 `EnsureBootstrapData()` 此前只会补“操作日志”菜单和权限，不会补“推送日志”菜单和权限；docker 复用旧库时，新增的 `push-logs` / `push:log:view` 很容易缺失。

## Analysis

- Confirmed Hypothesis 2: docker 环境中极可能缺少“推送日志”页面依赖的菜单/权限数据，导致菜单打不开或页面初始化请求被 `403` 拦截。
- Rejected Hypothesis 1: 代码仓库中的前端路由和页面组件定义正常，没有发现 `push-logs` 的命名或懒加载路径错误。
- Rejected Hypothesis 3: 部署脚本中的 nginx 已配置 `/api` 代理和 `try_files ... /index.html` 回退，这不是当前最可疑路径。

## Next Step

- Backend startup now auto-heals `push` / `push-logs` menus and `push:log:view` / `push:log:retry` permissions for built-in roles `admin` and `User`.
- Redeploy backend container and verify whether the “推送日志” menu can open normally.
