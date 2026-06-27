# 数据库设计

## 1. 基本信息

- 数据库：`steel_hikvision_monitor`
- 引擎：InnoDB
- 字符集：utf8mb4
- 初始化入口：`sql/init_database.sql`
- 应用访问：GORM + MySQL driver
- 迁移方式：当前没有 AutoMigrate 或版本化迁移工具；DDL 变更必须人工维护 SQL，并为存量环境准备独立升级脚本。

## 2. 表清单（30 张）

### 2.1 组织、字典与 RBAC

| 表 | 用途 |
|---|---|
| `factory_area` | 厂区主数据 |
| `factory_zone` | 厂区下的区域 |
| `sys_dept` | 树形部门，可关联厂区/区域 |
| `sys_dict_type` | 字典类型 |
| `sys_dict_item` | 字典项 |
| `sys_user` | 用户、密码哈希、部门、状态 |
| `sys_role` | 角色及数据范围 |
| `sys_user_role` | 用户角色多对多 |
| `sys_menu` | 菜单树、路由、图标和排序 |
| `sys_role_menu` | 角色菜单多对多 |
| `sys_permission` | 按钮/操作权限代码 |
| `sys_role_permission` | 角色权限多对多 |

主要关系：`factory_area 1:N factory_zone`；`sys_dept` 通过 `parent_id` 自关联；用户通过中间表关联角色，角色再关联菜单和权限。

### 2.2 设备与运行状态

| 表 | 用途 |
|---|---|
| `camera_device` | 摄像机网络、凭据密文、位置、AI 与在线状态 |
| `recorder_device` | NVR/DVR 网络、凭据密文、通道数和状态 |
| `recorder_channel` | 录像机通道，可映射摄像机及厂区/区域 |
| `device_status_log` | 设备状态变更历史 |
| `device_operation_log` | 设备操作审计数据结构 |

设备密码字段保存 Fernet token。`DEVICE_SECRET_KEY` 不可丢失，否则现有设备凭据无法解密。`ResolveDeviceSecret` 兼容历史明文数据，便于迁移但也意味着数据库中可能存在旧明文。

### 2.3 告警与推送

| 表 | 用途 |
|---|---|
| `alarm_record` | 统一告警、来源设备、媒体、去重和次数 |
| `alarm_process_log` | 告警状态流转与操作人记录 |
| `push_config` | 推送通道、过滤、限流、重试配置 |
| `alarm_push_log` | 每次推送请求、响应、错误和重试链 |

告警通过 `camera_id/recorder_id/channel_id` 定位来源，通过 `factory_id/zone_id` 支持组织过滤。`dedup_key + occurrence_count + last_event_time` 用于合并重复事件。

### 2.4 智能事件与 AI

| 表 | 用途 |
|---|---|
| `smart_interface_provider` | 外部智能平台/设备协议提供方 |
| `smart_interface_capability` | 能力元数据和默认规则 |
| `smart_device_binding` | 能力到设备数据源的绑定 |
| `smart_binding_rule` | 告警、去重、冷却、媒体、推送、AI 策略 |
| `smart_raw_event` | 完整原始载荷、headers、签名与解析状态 |
| `smart_event` | 标准化事件和业务状态 |
| `ai_review_task` | AI 复核请求、状态、重试和错误 |
| `ai_review_result` | AI 决策、标签、置信度和证据 |
| `ai_event` | 旧版 AI 回调事件，供兼容接口使用 |

关系主线：Provider + Capability → Binding → Rule；RawEvent → SmartEvent → AI Task → AI Result；SmartEvent 可进一步生成 AlarmRecord。

## 3. 关键字段约定

- 主键通常为无符号自增 `id`。
- 业务状态常用字符串，如 `enabled/disabled`、`online/offline`、`pending/processing/done/false_alarm`；新增状态前应检查前后端枚举与 SQL 字典。
- 可变结构使用 JSON 文本字段（字段名以 `_json` 结尾），应用层负责序列化与校验。
- 时间使用 MySQL datetime/timestamp，应用按本地时区解析。
- 软删除未统一实现；多数 DELETE 是物理删除，并依赖 service 做关联校验。
- 外键和唯一索引以 `init_database.sql` 为最终依据。删除设备时 service 会阻止仍有关联数据的操作。

## 4. 初始化数据

SQL 脚本包含：

- 厂区、区域、部门等演示/基础数据；
- 菜单树和权限代码；
- 告警类型、等级、状态等字典；
- `hikvision` Provider；
- `motion_detection` 与 `ai_analysis` Capability；
- `admin` 角色、初始化管理员及其全部菜单/权限关系。

脚本部分 seed 使用 `ON DUPLICATE KEY UPDATE`，但整份脚本不等于可重复执行的迁移系统。生产重跑前必须备份，并先审查 DDL、DELETE 和初始化账号部分。

## 5. 修改数据库的规范

1. 为新部署更新 `init_database.sql`。
2. 为已有部署另外编写带版本号的增量 SQL，禁止直接依赖重跑初始化脚本。
3. 同步修改 `entity/models.go`；返回字段变化时同步 DTO 和前端类型。
4. 为常用过滤、关联和时间范围增加联合索引，并用真实规模数据执行 `EXPLAIN`。
5. JSON 字段应在 service 层设置默认结构，避免 `NULL`、空字符串和 `{}`/`[]` 混用。
6. 先备份再执行；DDL 与应用发布顺序应保持向后兼容。
