import type { ModulePageConfig } from '../types/navigation'

const status = (text: string, tone: 'success' | 'warning' | 'danger' | 'info' | 'default') => ({
  text,
  tone,
})

export const modulePageMap: Record<string, ModulePageConfig> = {
  'safety-alarm-list': {
    title: '告警查询',
    description: '提供告警查询表格骨架，后续阶段接入真实检索条件和分页能力。',
    filters: ['告警类型', '告警等级', '处理状态', '时间范围'],
    columns: [
      { key: 'alarmNo', label: '告警编号' },
      { key: 'alarmType', label: '告警类型' },
      { key: 'zone', label: '区域' },
      { key: 'processStatus', label: '处理状态' },
    ],
    rows: [
      { alarmNo: 'ALM20260515001', alarmType: '危险区域闯入', zone: '炼钢一区 / 转炉平台', processStatus: status('未处理', 'danger') },
      { alarmNo: 'ALM20260515002', alarmType: '未戴安全帽', zone: '炼钢一区 / 废钢区', processStatus: status('处理中', 'info') },
    ],
  },
  'safety-alarm-stats': {
    title: '告警统计',
    description: '用于承接后续 ECharts 图表和报表卡片，当前先使用统计列表和状态标签占位。',
    filters: ['统计周期', '厂区', '区域', '告警等级'],
    columns: [
      { key: 'metric', label: '指标' },
      { key: 'value', label: '数值' },
      { key: 'trend', label: '趋势' },
    ],
    rows: [
      { metric: '今日告警数', value: 18, trend: status('上升', 'warning') },
      { metric: '推送成功率', value: '99.2%', trend: status('稳定', 'success') },
    ],
  },
  linkage: {
    title: '模块联调',
    description: '前后端、海康接入层、AI 回调和推送模块的联调入口页面，当前展示模拟状态。',
    filters: ['模块名称', '接口路径', '联调状态'],
    columns: [
      { key: 'module', label: '模块' },
      { key: 'api', label: '接口' },
      { key: 'result', label: '联调结果' },
    ],
    rows: [
      { module: 'Health API', api: '/api/health', result: status('通过', 'success') },
      { module: 'Media Service', api: '/api/video/live/1', result: status('Mock', 'info') },
    ],
  },
  'monitor-preview': {
    title: '监控预览',
    description: '视频预览页骨架已准备完成，后续会接入播放器组件和流媒体地址。',
    filters: ['厂区', '区域', '摄像机', '播放协议'],
    columns: [
      { key: 'camera', label: '摄像机' },
      { key: 'streamType', label: '流类型' },
      { key: 'status', label: '状态' },
    ],
    rows: [
      { camera: '转炉平台-01', streamType: 'HTTP-FLV', status: status('播放中', 'success') },
      { camera: '废钢区-03', streamType: 'WebRTC', status: status('告警', 'warning') },
    ],
  },
  'monitor-playback': {
    title: '录像查看',
    description: '回放查询页预留时间轴和播放器区域，当前先展示筛选区和片段列表。',
    filters: ['录像机', '通道', '开始时间', '结束时间'],
    columns: [
      { key: 'segment', label: '录像片段' },
      { key: 'duration', label: '时长' },
      { key: 'playbackStatus', label: '状态' },
    ],
    rows: [
      { segment: '2026-05-15 08:00 - 08:15', duration: '15 分钟', playbackStatus: status('可回放', 'success') },
      { segment: '2026-05-15 09:00 - 09:10', duration: '10 分钟', playbackStatus: status('加载中', 'info') },
    ],
  },
  'monitor-ai-api': {
    title: '智能接口',
    description: '统一承接 AI 事件回调、签名校验和去重规则配置。',
    filters: ['事件来源', '最低置信度', '状态'],
    columns: [
      { key: 'source', label: '事件来源' },
      { key: 'callback', label: '回调地址' },
      { key: 'apiStatus', label: '接口状态' },
    ],
    rows: [
      { source: '海康 ISAPI 事件', callback: '/api/ai/events/callback', apiStatus: status('已启用', 'success') },
      { source: '第三方 AI 服务', callback: '/api/ai/events/callback', apiStatus: status('Mock', 'info') },
    ],
  },
  'device-cameras': {
    title: '摄像机管理',
    description: '展示摄像机台账、连接状态和智能能力状态，当前为演示数据。',
    filters: ['摄像机名称', '厂区', '区域', '在线状态'],
    columns: [
      { key: 'name', label: '摄像机名称' },
      { key: 'ip', label: 'IP 地址' },
      { key: 'zone', label: '区域' },
      { key: 'deviceStatus', label: '状态' },
    ],
    rows: [
      { name: '转炉平台-01', ip: '192.168.1.98', zone: '转炉平台', deviceStatus: status('在线', 'success') },
      { name: '天车作业区-01', ip: '192.168.1.102', zone: '天车区', deviceStatus: status('离线', 'danger') },
    ],
  },
  'device-recorders': {
    title: '录像机管理',
    description: '展示海康 NVR/DVR 清单、通道数和连接状态。',
    filters: ['录像机名称', '厂区', '设备编码'],
    columns: [
      { key: 'name', label: '录像机名称' },
      { key: 'channelCount', label: '通道数' },
      { key: 'factory', label: '厂区' },
      { key: 'deviceStatus', label: '状态' },
    ],
    rows: [
      { name: 'NVR-炼钢一区', channelCount: 32, factory: '炼钢一区', deviceStatus: status('在线', 'success') },
      { name: 'NVR-炼钢二区', channelCount: 16, factory: '炼钢二区', deviceStatus: status('待同步', 'warning') },
    ],
  },
  'device-channels': {
    title: '通道管理',
    description: '管理录像机通道与摄像机和区域的绑定关系。',
    filters: ['录像机', '通道号', '区域', '支持回放'],
    columns: [
      { key: 'recorder', label: '录像机' },
      { key: 'channel', label: '通道号' },
      { key: 'camera', label: '绑定摄像机' },
      { key: 'channelStatus', label: '状态' },
    ],
    rows: [
      { recorder: 'NVR-炼钢一区', channel: 101, camera: '转炉平台-01', channelStatus: status('启用', 'success') },
      { recorder: 'NVR-炼钢一区', channel: 102, camera: '废钢区-03', channelStatus: status('停用', 'default') },
    ],
  },
  'push-config': {
    title: '钉钉/微信配置',
    description: '承接推送渠道、Webhook、模板消息和推送规则的统一配置。',
    filters: ['渠道', '区域', '告警等级'],
    columns: [
      { key: 'channel', label: '渠道' },
      { key: 'receiver', label: '接收对象' },
      { key: 'scope', label: '推送范围' },
      { key: 'configStatus', label: '状态' },
    ],
    rows: [
      { channel: '钉钉', receiver: '炼钢一区安全群', scope: '炼钢一区 / 紧急', configStatus: status('启用', 'success') },
      { channel: '微信', receiver: '区域负责人', scope: '炼钢一区 / 重要及紧急', configStatus: status('待配置', 'warning') },
    ],
  },
  'push-logs': {
    title: '推送日志',
    description: '展示钉钉和微信推送执行结果、失败原因和重试情况。',
    filters: ['推送渠道', '推送状态', '时间范围'],
    columns: [
      { key: 'time', label: '时间' },
      { key: 'channel', label: '渠道' },
      { key: 'target', label: '接收对象' },
      { key: 'pushStatus', label: '状态' },
    ],
    rows: [
      { time: '2026-05-15 10:23:13', channel: '钉钉', target: '炼钢一区安全群', pushStatus: status('成功', 'success') },
      { time: '2026-05-15 10:19:45', channel: '微信', target: '区域负责人', pushStatus: status('失败', 'danger') },
    ],
  },
  'system-users': {
    title: '用户管理',
    description: '用户台账、角色分配和数据范围配置页面骨架。',
    filters: ['用户名', '部门', '角色', '状态'],
    columns: [
      { key: 'username', label: '用户名' },
      { key: 'role', label: '角色' },
      { key: 'scope', label: '数据范围' },
      { key: 'userStatus', label: '状态' },
    ],
    rows: [
      { username: 'admin', role: '超级管理员', scope: '全部数据', userStatus: status('启用', 'success') },
      { username: 'safe01', role: '安全管理员', scope: '炼钢一区', userStatus: status('启用', 'success') },
    ],
  },
  'system-roles': {
    title: '角色权限',
    description: '角色、菜单权限、按钮权限和数据权限模型的前端展示入口。',
    filters: ['角色编码', '角色名称', '状态'],
    columns: [
      { key: 'roleCode', label: '角色编码' },
      { key: 'roleName', label: '角色名称' },
      { key: 'permissionSummary', label: '权限摘要' },
      { key: 'roleStatus', label: '状态' },
    ],
    rows: [
      { roleCode: 'admin', roleName: '超级管理员', permissionSummary: '全部菜单 / 全部按钮', roleStatus: status('启用', 'success') },
      { roleCode: 'safety', roleName: '安全管理员', permissionSummary: '告警 / 监控 / 报表', roleStatus: status('启用', 'success') },
    ],
  },
}
