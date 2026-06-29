# RBAC 权限设计方案

## 1. 设计目标

本文档为当前安全管理平台提供一套完整、可落地的 RBAC 设计，覆盖：

- 用户、角色、菜单、按钮、接口、数据范围的统一授权模型；
- 当前项目已有表结构的复用方式；
- 面向现有菜单、页面、接口的权限点拆分；
- 预置角色建议；
- 后端鉴权中间件与前端联动方案；
- 分阶段实施路径。

适用范围基于当前代码与数据结构：

- 后端：Gin + GORM + JWT
- 前端：Vue 3 + Pinia + `v-permission`
- 现有 RBAC 表：`sys_user`、`sys_role`、`sys_user_role`、`sys_menu`、`sys_role_menu`、`sys_permission`、`sys_role_permission`
- 现有数据范围字段：`sys_role.data_scope_type`、`sys_role.data_scope_value`

## 2. 当前现状与问题

当前项目已经具备 RBAC 的基础骨架，但仍不完整：

1. 登录后可以返回用户角色、菜单、按钮权限和数据范围。
2. 前端已经基于 `buttonPermissions` 做按钮显隐控制。
3. 数据库中已经有菜单、角色、权限、角色菜单、角色权限、多角色用户关系。
4. 角色管理页已经支持数据范围配置。

当前主要缺口如下：

1. 后端大部分接口只校验 JWT，未做接口级权限点拦截。
2. `sys_permission` 目前主要承载按钮权限，尚未形成“接口权限”和“资源动作权限”的统一映射。
3. 数据范围虽然已存储，但尚未系统性下沉到查询过滤、详情读取、导出下载、视频访问等后端逻辑。
4. 现有权限点未覆盖智能接口、视频、报表导出、误报处理、AI 任务等全部业务域。
5. 角色管理页面当前只能维护角色基本信息和数据范围，尚未支持菜单授权、权限点授权。

结论：当前系统属于“前端可见性控制 + 后端登录态校验”的半成品 RBAC，需要升级为“后端强制鉴权 + 前端按权限渲染 + 数据范围统一过滤”的完整模型。

## 3. 设计原则

### 3.1 总体原则

- 默认拒绝：未授权即不可见、不可调用、不可导出、不可下载。
- 后端为准：前端权限只负责体验优化，最终以后端鉴权结果为准。
- 菜单与动作分离：菜单决定“看得到页面”，动作权限决定“能做什么”。
- 数据权限独立：功能权限决定“能不能做”，数据范围决定“能看哪些数据”。
- 最小授权：默认仅授予完成岗位职责所需最小权限。
- 可审计：所有关键敏感操作都应记录操作人、时间、对象和结果。

### 3.2 鉴权层次

完整 RBAC 设计分四层：

1. 登录认证层：JWT 校验用户身份。
2. 菜单访问层：控制页面入口和路由可见性。
3. 动作权限层：控制新增、编辑、删除、测试、处理、导出、下载等行为。
4. 数据范围层：控制用户只能访问自己被授权的厂区、区域、部门、设备和本人数据。

## 4. 目标授权模型

### 4.1 核心实体

- 用户 `sys_user`
- 角色 `sys_role`
- 菜单 `sys_menu`
- 权限点 `sys_permission`
- 用户角色关系 `sys_user_role`
- 角色菜单关系 `sys_role_menu`
- 角色权限关系 `sys_role_permission`

### 4.2 授权关系

采用标准多对多模型：

- 用户 <-> 角色
- 角色 <-> 菜单
- 角色 <-> 权限点

用户登录后，系统聚合其全部角色，取并集：

- 菜单取并集
- 权限点取并集
- 数据范围按“最大可见集合”合并

### 4.3 数据范围模型

当前项目已具备以下数据范围类型，建议保留并标准化：

- `all`：全部数据
- `factory`：指定厂区
- `zone`：指定区域
- `device`：指定设备
- `dept`：本部门
- `self`：本人
- `custom`：自定义组合

推荐统一解释如下：

- `all`：忽略所有组织与设备过滤
- `factory`：按 `factory_id in (...)` 过滤
- `zone`：按 `zone_id in (...)` 过滤
- `device`：按 `camera_id` / `recorder_id` / `channel_id` 过滤
- `dept`：按用户所属部门及其可选子部门过滤
- `self`：仅允许访问 `created_by = 当前用户`、`operator_id = 当前用户`、`owner_id = 当前用户` 等本人关联数据
- `custom`：厂区、区域、部门、用户、设备多维组合

### 4.4 数据范围合并规则

一个用户可挂多个角色，建议采用“并集放宽”策略：

1. 若任一角色为 `all`，最终范围即为 `all`。
2. `factory`、`zone`、`device`、`custom` 按 JSON 集合求并集。
3. `dept` 追加当前用户所属部门范围。
4. `self` 只补充本人范围，不覆盖其他范围。

这样更符合多角色叠加的业务直觉，也便于实现。

## 5. 表结构设计

## 5.1 可直接复用的现有表

当前库表已经足以支撑第一阶段完整落地：

- `sys_menu`：页面菜单
- `sys_permission`：动作权限编码
- `sys_role_menu`：页面授权
- `sys_role_permission`：动作授权
- `sys_role.data_scope_type/value`：数据范围授权

这意味着第一阶段无需强制新增 RBAC 表，即可完成后端权限拦截与数据范围过滤。

## 5.2 推荐扩展字段

为了让权限体系更“完整、可维护、可审计”，建议扩展 `sys_permission`：

| 字段 | 说明 |
| --- | --- |
| `permission_type` | `menu` / `action` / `api` / `export` / `data` |
| `module_code` | 所属模块，如 `alarm`、`device.camera` |
| `resource_code` | 所属资源，如 `camera`、`push_config` |
| `action_code` | 动作，如 `view/create/update/delete/test/export` |
| `api_method` | HTTP 方法，可选 |
| `api_pattern` | 接口路径模板，可选 |
| `sort` | 排序 |
| `builtin` | 是否内置权限，不允许随意删除 |

如果暂时不想改表，可以先约定：

- `sys_permission.code` 仍作为唯一权限码；
- 代码侧维护 `接口 -> 权限码` 映射；
- 后续再平滑升级结构。

## 6. 权限编码规范

建议统一采用：

```text
<模块>:<资源>:<动作>
```

示例：

- `device:camera:view`
- `device:camera:create`
- `alarm:process`
- `push:config:test`
- `smart:binding:update`
- `video:playback:download`
- `report:alarm:export`

补充规范：

- 页面按钮使用资源动作式编码。
- 页面访问本身以菜单授权为主，不再重复创建大量 `*:menu:view` 权限。
- 只有脱离页面按钮、需要直接保护接口的动作，才要求单独权限码。

## 7. 本项目完整权限点设计

以下按当前项目实际模块拆分建议权限点。已存在的权限保留，不足部分补齐。

### 7.1 首页驾驶舱

- `dashboard:refresh`
- `dashboard:stats:view`
- `dashboard:trend:view`

说明：

- `dashboard:refresh` 已存在。
- 图表明细如果后续分接口隔离，建议拆成查看权限。

### 7.2 安全日志与告警

- `alarm:view`
- `alarm:detail:view`
- `alarm:process`
- `alarm:false`
- `alarm:repush`
- `alarm:stats:view`
- `alarm:realtime:view`
- `alarm:export`

说明：

- 当前已存在 `alarm:view`、`alarm:process`、`alarm:repush`。
- 建议补充误报标记、详情查看、导出权限。

### 7.3 监控与视频

- `video:live:view`
- `video:live:control`
- `video:playback:view`
- `video:playback:search`
- `video:playback:download`
- `video:snapshot:create`
- `video:webcontrol:view`

说明：

- 监控预览、录像查看虽然已有菜单，但后端 `/api/video/*` 仍需动作权限。
- 下载录像和抓图属于敏感行为，必须单独授权。

### 7.4 智能接口与 AI

- `smart:provider:view`
- `smart:provider:create`
- `smart:provider:update`
- `smart:provider:test`
- `smart:binding:view`
- `smart:binding:create`
- `smart:binding:update`
- `smart:binding:delete`
- `smart:binding:test`
- `smart:rule:create`
- `smart:rule:update`
- `smart:rule:delete`
- `smart:event:view`
- `smart:event:detail:view`
- `smart:event:submit-ai-review`
- `smart:ai-task:view`
- `smart:ai-task:retry`
- `ai:event:view`
- `ai:event:callback`

说明：

- 当前 SQL 里几乎未覆盖这一域的权限点，属于必须补齐的核心缺口。

### 7.5 设备管理

摄像机：

- `device:camera:view`
- `device:camera:create`
- `device:camera:update`
- `device:camera:delete`
- `device:camera:test`
- `device:camera:check`
- `device:camera:sdk:view`
- `device:camera:sdk:update`
- `device:camera:ptz`
- `device:camera:user:update`

录像机：

- `device:recorder:view`
- `device:recorder:create`
- `device:recorder:update`
- `device:recorder:delete`
- `device:recorder:test`
- `device:recorder:check`
- `device:recorder:sync`

通道：

- `device:channel:view`
- `device:channel:update`

设备状态：

- `device:status:log:view`
- `device:status:check`
- `device:status:export`

说明：

- 当前已有一部分权限。
- 建议将 SDK 配置、PTZ、设备账号管理从普通编辑权限里拆出，避免设备管理员权限过大。

### 7.6 推送管理

- `push:config:view`
- `push:config:create`
- `push:config:update`
- `push:config:delete`
- `push:config:test`
- `push:log:view`
- `push:log:retry`
- `push:log:export`

### 7.7 基础资料

厂区：

- `basic:factory:view`
- `basic:factory:create`
- `basic:factory:update`
- `basic:factory:delete`

区域：

- `basic:zone:view`
- `basic:zone:create`
- `basic:zone:update`
- `basic:zone:delete`

部门：

- `basic:dept:view`
- `basic:dept:create`
- `basic:dept:update`
- `basic:dept:delete`

字典：

- `basic:dict:view`
- `basic:dict:create`
- `basic:dict:update`
- `basic:dict:delete`

### 7.8 系统管理

用户：

- `system:user:view`
- `system:user:create`
- `system:user:update`
- `system:user:delete`
- `system:user:reset-password`
- `system:user:assign-role`

角色：

- `system:role:view`
- `system:role:create`
- `system:role:update`
- `system:role:delete`
- `system:role:enable`
- `system:role:disable`
- `system:role:grant-menu`
- `system:role:grant-permission`
- `system:role:grant-data-scope`

### 7.9 报表与导出

- `report:alarm:view`
- `report:alarm:export`
- `report:device:view`
- `report:device:export`
- `report:push:view`
- `report:push:export`

### 7.10 联调与运维类

- `linkage:view`
- `linkage:test`
- `system:audit:view`

说明：

- 当前“模块联调”菜单存在，但没有独立权限点设计，建议纳入。

## 8. 菜单授权设计

### 8.1 菜单的职责

菜单只负责：

- 是否在侧边栏展示
- 是否允许进入页面路由

菜单不直接决定后端 API 是否可用。

### 8.2 菜单与动作的关系

建议保持如下关系：

1. 页面要能进入，必须有对应菜单。
2. 页面中的按钮、操作栏、弹窗动作，再由 `sys_permission` 控制。
3. 即使用户通过接口工具手动调用 API，后端仍需校验动作权限。

## 9. 预置角色设计

建议系统内置以下角色。

### 9.1 超级管理员

- 角色编码：`admin`
- 职责：系统初始化、全域管理、紧急处置
- 菜单：全部
- 权限：全部
- 数据范围：`all`

### 9.2 安环平台主管

- 角色编码：`safety_manager`
- 职责：查看全局告警、处理告警、查看统计、导出报表
- 菜单：驾驶舱、安全日志、监控预览、录像查看、推送日志、报表
- 权限：
  - 告警查看、详情、处理、误报、重推、导出
  - 驾驶舱查看
  - 视频查看、回放检索、录像下载
  - 推送日志查看
- 数据范围：`factory` 或 `all`

### 9.3 值班操作员

- 角色编码：`alarm_operator`
- 职责：实时值守、处理本厂区或本区域告警
- 菜单：驾驶舱、安全日志、监控预览、录像查看
- 权限：
  - 告警查看、详情、处理
  - 视频查看、抓图
- 数据范围：`zone` / `factory`

### 9.4 设备管理员

- 角色编码：`device_admin`
- 职责：维护摄像机、录像机、通道和设备状态
- 菜单：监控管理、设备管理、基础资料中的厂区/区域只读
- 权限：
  - 摄像机/录像机/通道查看、增改删、测试、状态检测、通道同步
  - 设备状态日志查看
  - 必要时开放设备 SDK 配置
- 数据范围：`factory` / `device`

### 9.5 智能接口管理员

- 角色编码：`integration_admin`
- 职责：维护智能接口提供方、绑定关系、规则和 AI 任务
- 菜单：模块联调、监控管理中的智能接口、部分报表
- 权限：
  - `smart:*`
  - `ai:*`
  - 联调测试
- 数据范围：`factory` / `device`

### 9.6 推送管理员

- 角色编码：`push_admin`
- 职责：维护推送配置和失败重试
- 菜单：推送管理、推送报表
- 权限：
  - 推送配置查看、增改删、测试
  - 推送日志查看、重试、导出
- 数据范围：`factory` / `all`

### 9.7 审计查看员

- 角色编码：`auditor`
- 职责：只读审计
- 菜单：驾驶舱、安全日志、设备状态、推送日志、报表
- 权限：
  - 全部查询类权限
  - 不含新增、编辑、删除、测试、处理、重试
- 数据范围：`all` 或 `factory`

## 10. 后端鉴权设计

### 10.1 中间件职责拆分

建议将后端中间件拆成两层：

1. `Auth`：只做 JWT 身份认证。
2. `RequirePermission(permissionCode)`：做接口动作权限校验。

并新增“数据范围解析器”：

3. `BuildAccessScope()`：把用户多角色的数据范围合并后写入上下文。

### 10.2 上下文字段建议

在 Gin Context 中统一注入：

- `currentUserID`
- `currentUsername`
- `currentRoleCodes`
- `currentPermissionCodes`
- `currentAccessScope`

### 10.3 接口权限映射方式

推荐在路由注册时显式绑定权限码，例如：

```go
protected.POST(
  "/cameras",
  middleware.RequirePermission("device:camera:create"),
  handlers.Platform.CreateCamera,
)
```

优点：

- 可读性高
- 不依赖反射
- 权限变更容易审查

### 10.4 权限拦截规则

- 无 token：返回 `401`
- token 无效：返回 `401`
- 已登录但无权限：返回 `403`
- 有功能权限但超出数据范围：返回 `403`
- 查询类接口若范围为空：返回空列表或 `403`，按业务决定

推荐规则：

- 列表查询：返回过滤后结果
- 详情读取：对象不在范围内时返回 `404` 或 `403`
- 修改/删除/处理：对象不在范围内直接 `403`

## 11. 数据范围落地设计

### 11.1 统一过滤维度

当前项目数据天然围绕以下维度组织：

- `factory_id`
- `zone_id`
- `dept_id`
- `camera_id`
- `recorder_id`
- `channel_id`
- `operator_id` / `created_by`

因此建议建立统一的 `AccessScope` 结构：

```text
all
factoryIds[]
zoneIds[]
deptIds[]
cameraIds[]
recorderIds[]
channelIds[]
userIds[]
selfOnly
```

### 11.2 查询过滤规则

建议按资源表进行标准过滤：

- 厂区：按 `factory.id`
- 区域：按 `zone.factory_id/zone.id`
- 部门：按 `dept.id/dept.factory_id/dept.zone_id`
- 摄像机：按 `factory_id/zone_id/camera_id`
- 录像机：按 `factory_id/recorder_id`
- 通道：按 `factory_id/zone_id/channel_id/recorder_id`
- 告警：按 `factory_id/zone_id/camera_id/recorder_id/channel_id`
- 推送日志：按 `factory_id/zone_id`
- 智能事件：按 `factory_id/zone_id/camera_id/recorder_id/channel_id`
- 视频能力：先校验视频对象归属是否在范围内，再返回地址

### 11.3 高风险对象

以下对象必须强制校验数据范围，不能仅依赖页面过滤：

- 告警详情、处理、误报、重推
- 视频实时预览、回放、下载、抓图
- 摄像机/NVR 详情与配置
- 智能绑定、智能事件、AI 任务
- 导出接口

## 12. 前端联动设计

### 12.1 前端保留职责

前端继续负责：

- 登录后缓存菜单、按钮权限、数据范围摘要
- 路由守卫时基于菜单树拦截未授权页面
- 使用 `v-permission` 控制按钮显示
- 在页面顶部展示当前数据范围摘要

### 12.2 前端新增职责

建议补充：

1. 路由与菜单编码强绑定，未出现在菜单树中的页面禁止进入。
2. 403 页面区分两类：
   - 无功能权限
   - 超出数据范围
3. 角色管理页增加：
   - 菜单授权树
   - 权限点授权树
   - 数据范围配置
4. 用户管理页增加：
   - 重置密码
   - 分配角色

## 13. 角色管理功能设计

目标中的角色管理不应只维护名称和数据范围，而应完整维护四类内容：

1. 角色基本信息
2. 菜单授权
3. 动作权限授权
4. 数据范围授权

建议角色编辑页结构：

- 基本信息页签
- 菜单权限页签
- 动作权限页签
- 数据范围页签

保存时建议采用事务：

1. 更新 `sys_role`
2. 替换 `sys_role_menu`
3. 替换 `sys_role_permission`
4. 更新 `data_scope_type/value`

## 14. 用户管理功能设计

用户管理应支持：

- 新增用户
- 编辑用户
- 启停用户
- 重置密码
- 分配多个角色
- 查看用户最终生效权限摘要

建议增加“权限预览”能力：

- 展示用户所有角色
- 展示合并后的菜单数量
- 展示合并后的权限点数量
- 展示最终数据范围

## 15. 审计与安全要求

以下操作建议纳入审计日志：

- 登录成功/失败
- 用户创建、修改、删除、重置密码
- 角色创建、修改、删除、授权变更
- 设备新增、编辑、删除、测试、状态检测
- 告警处理、误报、重推
- 推送配置测试、日志重试
- 智能绑定测试、AI 任务重试
- 视频下载、抓图

最少记录：

- 操作人 ID
- 操作人用户名
- 操作时间
- 资源类型
- 资源 ID
- 动作
- 请求摘要
- 执行结果

## 16. 分阶段实施建议

### 阶段一：补齐后端强制鉴权

目标：

- 所有写接口与敏感读接口加 `RequirePermission`
- 前后端 403 表现一致

范围：

- 用户、角色
- 基础资料
- 设备
- 告警
- 推送
- 智能接口
- 视频下载/抓图
- 导出

### 阶段二：落地统一数据范围过滤

目标：

- 查询、详情、操作、导出、视频全部按范围过滤

重点：

- 告警
- 设备
- 智能事件
- 视频
- 推送日志

### 阶段三：补齐角色授权界面

目标：

- 角色页支持菜单树和权限树分配
- 用户页支持权限预览

### 阶段四：结构优化与审计增强

目标：

- 扩展 `sys_permission`
- 建立接口权限映射配置
- 增加权限变更审计

## 17. 与当前项目的最小改造方案

如果希望最快落地，而不是一次性重构全部权限模型，推荐采用以下最小方案：

1. 保留现有 7 张 RBAC 表，不新增核心关系表。
2. 在 `sys_permission` 中继续维护全部动作权限码。
3. 后端通过中间件在路由层绑定权限码。
4. 在 service/repository 层统一接入数据范围过滤。
5. 前端角色页补充菜单授权、权限授权功能。
6. SQL 初始化脚本补齐缺失权限点与预置角色。

这个方案与当前工程兼容性最好，且能快速从“前端假拦截”升级到“后端真鉴权”。

## 18. 推荐的初始化角色矩阵

| 角色 | 菜单范围 | 动作范围 | 数据范围 |
| --- | --- | --- | --- |
| `admin` | 全部 | 全部 | `all` |
| `safety_manager` | 驾驶舱/告警/监控/推送日志/报表 | 告警全操作 + 视频查看/下载 + 报表导出 | `factory` 或 `all` |
| `alarm_operator` | 驾驶舱/告警/监控 | 告警查看处理 + 视频查看 | `zone` |
| `device_admin` | 监控/设备/部分基础资料 | 设备 CRUD/测试/同步/状态检测 | `factory` 或 `device` |
| `integration_admin` | 联调/智能接口 | `smart:*`、`ai:*` | `factory` 或 `device` |
| `push_admin` | 推送管理/推送报表 | 推送配置与日志处理 | `factory` |
| `auditor` | 只读菜单 | 全部查询类，不含写操作 | `all` 或 `factory` |

## 19. 最终结论

本项目最合适的 RBAC 形态不是重新推翻设计，而是在现有模型上完成四件事：

1. 补齐权限点全集；
2. 把后端接口鉴权真正做起来；
3. 把数据范围真正落到查询与对象访问；
4. 让角色管理页具备菜单、动作、范围三位一体的授权能力。

这样可以在不大改表结构的前提下，把当前系统升级为一套完整的、可维护的、适合厂区视频安防业务的 RBAC 权限体系。
