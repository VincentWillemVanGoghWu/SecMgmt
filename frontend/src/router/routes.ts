import type { RouteRecordRaw } from 'vue-router'

const AppLayout = () => import('../layouts/AppLayout.vue')
const LoginView = () => import('../views/auth/LoginView.vue')
const DashboardView = () => import('../views/dashboard/DashboardView.vue')
const ModulePageView = () => import('../views/shared/ModulePageView.vue')
const RealtimeAlarmView = () => import('../views/safety/RealtimeAlarmView.vue')
const OperationLogView = () => import('../views/safety/OperationLogView.vue')
const FactoryManagementView = () => import('../views/master-data/FactoryManagementView.vue')
const ZoneManagementView = () => import('../views/master-data/ZoneManagementView.vue')
const DeptManagementView = () => import('../views/master-data/DeptManagementView.vue')
const DictManagementView = () => import('../views/master-data/DictManagementView.vue')
const CameraManagementView = () => import('../views/device/CameraManagementView.vue')
const RecorderManagementView = () => import('../views/device/RecorderManagementView.vue')
const ChannelManagementView = () => import('../views/device/ChannelManagementView.vue')
const DeviceStatusLogView = () => import('../views/device/DeviceStatusLogView.vue')
const DeviceCheckScheduleView = () => import('../views/device/DeviceCheckScheduleView.vue')
const MonitorPreviewView = () => import('../views/monitor/MonitorPreviewView.vue')
const PlaybackView = () => import('../views/monitor/PlaybackView.vue')
const AiIntegrationView = () => import('../views/monitor/AiIntegrationView.vue')
const AlarmQueryView = () => import('../views/safety/AlarmQueryView.vue')
const AlarmStatsView = () => import('../views/safety/AlarmStatsView.vue')
const PushConfigView = () => import('../views/push/PushConfigView.vue')
const PushLogView = () => import('../views/push/PushLogView.vue')
const UserManagementView = () => import('../views/system/UserManagementView.vue')
const RoleManagementView = () => import('../views/system/RoleManagementView.vue')

export const appRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: LoginView,
    meta: { title: '登录', guestOnly: true },
  },
  {
    path: '/',
    component: AppLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: { name: 'dashboard' },
      },
      {
        path: 'dashboard',
        name: 'dashboard',
        component: DashboardView,
        meta: { title: '首页驾驶舱', requiresAuth: true },
      },
      {
        path: 'safety/realtime-alarms',
        name: 'safety-realtime-alarms',
        component: RealtimeAlarmView,
        meta: { title: '安全日志 / 实时告警', requiresAuth: true },
      },
      {
        path: 'safety/alarm-list',
        name: 'safety-alarm-list',
        component: AlarmQueryView,
        meta: { title: '安全日志 / 告警查询', requiresAuth: true },
      },
      {
        path: 'safety/alarm-stats',
        name: 'safety-alarm-stats',
        component: AlarmStatsView,
        meta: { title: '安全日志 / 告警统计', requiresAuth: true },
      },
      {
        path: 'safety/operation-logs',
        name: 'safety-operation-logs',
        component: OperationLogView,
        meta: { title: '安全日志 / 操作日志', requiresAuth: true },
      },
      {
        path: 'linkage',
        name: 'linkage',
        component: ModulePageView,
        meta: { title: '模块联调', pageKey: 'linkage', requiresAuth: true },
      },
      {
        path: 'monitor/preview',
        name: 'monitor-preview',
        component: MonitorPreviewView,
        meta: { title: '监控管理 / 监控预览', requiresAuth: true },
      },
      {
        path: 'monitor/playback',
        name: 'monitor-playback',
        component: PlaybackView,
        meta: { title: '监控管理 / 录像查看', requiresAuth: true },
      },
      {
        path: 'monitor/ai-api',
        name: 'monitor-ai-api',
        component: AiIntegrationView,
        meta: { title: '监控管理 / 智能接口', requiresAuth: true },
      },
      {
        path: 'device/cameras',
        name: 'device-cameras',
        component: CameraManagementView,
        meta: { title: '设备管理 / 摄像机管理', requiresAuth: true },
      },
      {
        path: 'device/recorders',
        name: 'device-recorders',
        component: RecorderManagementView,
        meta: { title: '设备管理 / 录像机管理', requiresAuth: true },
      },
      {
        path: 'device/channels',
        name: 'device-channels',
        component: ChannelManagementView,
        meta: { title: '设备管理 / 通道管理', requiresAuth: true },
      },
      {
        path: 'device/status-logs',
        name: 'device-status-logs',
        component: DeviceStatusLogView,
        meta: { title: '设备管理 / 设备状态日志', requiresAuth: true },
      },
      {
        path: 'device/check-schedules',
        name: 'device-check-schedules',
        component: DeviceCheckScheduleView,
        meta: { title: '设备管理 / 巡检计划', requiresAuth: true },
      },
      {
        path: 'push/config',
        name: 'push-config',
        component: PushConfigView,
        meta: { title: '推送管理 / 推送配置', requiresAuth: true },
      },
      {
        path: 'push/logs',
        name: 'push-logs',
        component: PushLogView,
        meta: { title: '推送管理 / 推送日志', requiresAuth: true },
      },
      {
        path: 'system/users',
        name: 'system-users',
        component: UserManagementView,
        meta: { title: '系统管理 / 用户管理', requiresAuth: true },
      },
      {
        path: 'system/roles',
        name: 'system-roles',
        component: RoleManagementView,
        meta: { title: '系统管理 / 角色权限', requiresAuth: true },
      },
      {
        path: 'master-data/factories',
        name: 'basic-data-factories',
        component: FactoryManagementView,
        meta: { title: '基础资料 / 厂区管理', requiresAuth: true },
      },
      {
        path: 'master-data/zones',
        name: 'basic-data-zones',
        component: ZoneManagementView,
        meta: { title: '基础资料 / 区域管理', requiresAuth: true },
      },
      {
        path: 'master-data/depts',
        name: 'basic-data-depts',
        component: DeptManagementView,
        meta: { title: '基础资料 / 部门管理', requiresAuth: true },
      },
      {
        path: 'master-data/dicts',
        name: 'basic-data-dicts',
        component: DictManagementView,
        meta: { title: '基础资料 / 字典管理', requiresAuth: true },
      },
    ],
  },
]
