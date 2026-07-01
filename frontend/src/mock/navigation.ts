import type { MenuItem } from '../types/navigation'

export const menuGroups: MenuItem[] = [
  {
    key: 'dashboard',
    label: '首页驾驶舱',
    icon: 'House',
    routeName: 'dashboard',
  },
  {
    key: 'safety',
    label: '安全日志',
    icon: 'Warning',
    children: [
      { key: 'safety-alarm-list', label: '告警查询', icon: 'Warning', routeName: 'safety-alarm-list' },
      { key: 'safety-alarm-stats', label: '告警统计', icon: 'Histogram', routeName: 'safety-alarm-stats' },
    ],
  },
  {
    key: 'linkage',
    label: '模块联调',
    icon: 'Share',
    routeName: 'linkage',
  },
  {
    key: 'monitor',
    label: '监控管理',
    icon: 'Monitor',
    children: [
      { key: 'monitor-preview', label: '监控预览', icon: 'VideoCamera', routeName: 'monitor-preview' },
      { key: 'monitor-playback', label: '录像查看', icon: 'Operation', routeName: 'monitor-playback' },
      { key: 'monitor-ai-api', label: '智能接口', icon: 'Connection', routeName: 'monitor-ai-api' },
    ],
  },
  {
    key: 'device',
    label: '设备管理',
    icon: 'Grid',
    children: [
      { key: 'device-cameras', label: '摄像机管理', icon: 'VideoCamera', routeName: 'device-cameras' },
      { key: 'device-recorders', label: '录像机管理', icon: 'Monitor', routeName: 'device-recorders' },
      { key: 'device-channels', label: '通道管理', icon: 'Connection', routeName: 'device-channels' },
      { key: 'device-check-schedules', label: '巡检计划', icon: 'Timer', routeName: 'device-check-schedules' },
    ],
  },
  {
    key: 'push',
    label: '推送管理',
    icon: 'Bell',
    children: [
      { key: 'push-config', label: '推送配置', icon: 'Setting', routeName: 'push-config' },
      { key: 'push-logs', label: '推送日志', icon: 'Files', routeName: 'push-logs' },
    ],
  },
  {
    key: 'system',
    label: '系统管理',
    icon: 'Lock',
    children: [
      { key: 'system-users', label: '用户管理', icon: 'User', routeName: 'system-users' },
      { key: 'system-roles', label: '角色权限', icon: 'CircleCheck', routeName: 'system-roles' },
    ],
  },
]
