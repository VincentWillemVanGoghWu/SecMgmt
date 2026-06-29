<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import { Delete, EditPen, Plus, RefreshRight, Search, SwitchButton } from "@element-plus/icons-vue"

import { listCamerasApi } from "../../api/camera"
import { listFactoriesApi, listDeptsApi, listZonesApi } from "../../api/master-data"
import { listChannelsApi, listRecordersApi } from "../../api/recorder"
import {
  createRoleApi,
  deleteRoleApi,
  listRoleMenuTreeApi,
  listRolePermissionOptionsApi,
  listRolesApi,
  updateRoleApi,
  updateRoleDataScopeApi,
  updateRoleMenusApi,
  updateRolePermissionsApi,
  updateRoleStatusApi,
} from "../../api/role"
import AccessDeniedState from "../../components/common/AccessDeniedState.vue"
import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import type { CameraRecord } from "../../types/camera"
import type { FactoryRecord, DeptRecord, ZoneRecord } from "../../types/master-data"
import type { RecorderChannelRecord, RecorderRecord } from "../../types/recorder"
import type {
  CustomScopeValue,
  DataScopeType,
  DeviceScopeValue,
  RoleDataScopeRecord,
  RoleMenuTreeItem,
  RolePermissionOption,
  RoleStatus,
  RoleSubmitPayload,
} from "../../types/role"

const loading = ref(false)
const roleSubmitting = ref(false)
const saving = ref(false)
const authSaving = ref(false)
const roleDialogVisible = ref(false)
const dialogVisible = ref(false)
const authDialogVisible = ref(false)
const accessDenied = ref(false)
const deniedMessage = ref("请联系管理员为当前账号开通角色查看权限。")
const roleEditingId = ref<number | null>(null)
const roleFormRef = ref<FormInstance>()
const activeRole = ref<RoleDataScopeRecord | null>(null)
const activeAuthRole = ref<RoleDataScopeRecord | null>(null)
const menuTreeRef = ref<{
  setCheckedKeys: (keys: number[], leafOnly?: boolean) => void
  getCheckedKeys: (leafOnly?: boolean) => number[]
  getHalfCheckedKeys: () => number[]
} | null>(null)

const roles = ref<RoleDataScopeRecord[]>([])
const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const depts = ref<DeptRecord[]>([])
const cameras = ref<CameraRecord[]>([])
const recorders = ref<RecorderRecord[]>([])
const channels = ref<RecorderChannelRecord[]>([])
const roleMenuTree = ref<RoleMenuTreeItem[]>([])
const permissionOptions = ref<RolePermissionOption[]>([])
const selectedPermissionIds = ref<number[]>([])
const permissionKeyword = ref("")

const scopeTypeOptions: Array<{ label: string; value: DataScopeType }> = [
  { label: "全部数据", value: "all" },
  { label: "指定厂区", value: "factory" },
  { label: "指定区域", value: "zone" },
  { label: "指定设备", value: "device" },
  { label: "本部门", value: "dept" },
  { label: "本人", value: "self" },
  { label: "自定义", value: "custom" },
]

const formState = reactive({
  dataScopeType: "all" as DataScopeType,
  factoryIds: [] as number[],
  zoneIds: [] as number[],
  deptIds: [] as number[],
  cameraIds: [] as number[],
  recorderIds: [] as number[],
  channelIds: [] as number[],
  userIds: [] as number[],
})

const queryForm = reactive({
  keyword: "",
  status: "",
})

const statusOptions = [
  { label: "全部", value: "" },
  { label: "启用", value: "enabled" },
  { label: "停用", value: "disabled" },
]

const roleStatusOptions: Array<{ label: string; value: RoleStatus }> = [
  { label: "启用", value: "enabled" },
  { label: "停用", value: "disabled" },
]

const roleForm = reactive<RoleSubmitPayload>({
  roleCode: "",
  roleName: "",
  status: "enabled",
  remark: "",
})

const roleRules: FormRules = {
  roleCode: [{ required: true, message: "请输入角色编码", trigger: "blur" }],
  roleName: [{ required: true, message: "请输入角色名称", trigger: "blur" }],
}

const getStatusTone = (status: string) => (status === "enabled" ? "success" : "default")
const getStatusText = (status: string) => (status === "enabled" ? "启用" : "停用")
const isAdminRole = (record: RoleDataScopeRecord) => record.roleCode === "admin"

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const isForbidden = (error: unknown) => (error as { response?: { status?: number } })?.response?.status === 403

const parseScopeValue = (record: RoleDataScopeRecord) => {
  const emptyCustom: CustomScopeValue = {
    factoryIds: [],
    zoneIds: [],
    deptIds: [],
    cameraIds: [],
    recorderIds: [],
    channelIds: [],
    userIds: [],
  }
  const emptyDevice: DeviceScopeValue = {
    cameraIds: [],
    recorderIds: [],
    channelIds: [],
  }
  const rawValue = record.dataScopeValue
  if (!rawValue) {
    return { list: [] as number[], device: emptyDevice, custom: emptyCustom }
  }
  try {
    const parsed = JSON.parse(rawValue)
    if (Array.isArray(parsed)) {
      const list = parsed.filter((item) => Number.isInteger(item))
      return { list, device: emptyDevice, custom: { ...emptyCustom } }
    }
    if (parsed && typeof parsed === "object") {
      return {
        list: [],
        device: {
          cameraIds: Array.isArray(parsed.cameraIds) ? parsed.cameraIds : [],
          recorderIds: Array.isArray(parsed.recorderIds) ? parsed.recorderIds : [],
          channelIds: Array.isArray(parsed.channelIds) ? parsed.channelIds : [],
        },
        custom: {
          factoryIds: Array.isArray(parsed.factoryIds) ? parsed.factoryIds : [],
          zoneIds: Array.isArray(parsed.zoneIds) ? parsed.zoneIds : [],
          deptIds: Array.isArray(parsed.deptIds) ? parsed.deptIds : [],
          cameraIds: Array.isArray(parsed.cameraIds) ? parsed.cameraIds : [],
          recorderIds: Array.isArray(parsed.recorderIds) ? parsed.recorderIds : [],
          channelIds: Array.isArray(parsed.channelIds) ? parsed.channelIds : [],
          userIds: Array.isArray(parsed.userIds) ? parsed.userIds : [],
        },
      }
    }
  } catch {
    return { list: [], device: emptyDevice, custom: emptyCustom }
  }
  return { list: [], device: emptyDevice, custom: emptyCustom }
}

const scopeSummaryText = (record: RoleDataScopeRecord) => {
  const type = record.dataScopeType
  if (type === "all") return "全部数据"
  if (type === "self") return "本人"
  if (type === "factory" || type === "zone" || type === "dept") {
    const { list } = parseScopeValue(record)
    return `${scopeTypeOptions.find((item) => item.value === type)?.label || type} / ${list.length} 项`
  }
  if (type === "device") {
    const { device } = parseScopeValue(record)
    const count = device.cameraIds.length + device.recorderIds.length + device.channelIds.length
    return `指定设备 / ${count} 项`
  }
  if (type === "custom") {
    const { custom } = parseScopeValue(record)
    const count =
      custom.factoryIds.length +
      custom.zoneIds.length +
      custom.deptIds.length +
      custom.cameraIds.length +
      custom.recorderIds.length +
      custom.channelIds.length +
      custom.userIds.length
    return `自定义 / ${count} 项`
  }
  return type
}

const menuSummary = (record: RoleDataScopeRecord) =>
  `${record.menuCodes.length} 个菜单 / ${record.permissionCodes.length} 个权限点`

const selectedZoneOptions = computed(() => {
  if (!formState.factoryIds.length) {
    return zones.value
  }
  return zones.value.filter((item) => formState.factoryIds.includes(item.factoryId))
})

const selectedDeptOptions = computed(() => {
  if (!formState.factoryIds.length && !formState.zoneIds.length) {
    return depts.value
  }
  return depts.value.filter((item) => {
    const factoryMatched = !formState.factoryIds.length || (item.factoryId ? formState.factoryIds.includes(item.factoryId) : false)
    const zoneMatched = !formState.zoneIds.length || (item.zoneId ? formState.zoneIds.includes(item.zoneId) : false)
    return factoryMatched && zoneMatched
  })
})

const selectedCameraOptions = computed(() => {
  return cameras.value.filter((item) => {
    const factoryMatched = !formState.factoryIds.length || formState.factoryIds.includes(item.factoryId)
    const zoneMatched = !formState.zoneIds.length || formState.zoneIds.includes(item.zoneId)
    return factoryMatched && zoneMatched
  })
})

const selectedRecorderOptions = computed(() => {
  return recorders.value.filter((item) => !formState.factoryIds.length || formState.factoryIds.includes(item.factoryId))
})

const selectedChannelOptions = computed(() => {
  return channels.value.filter((item) => {
    const factoryMatched = !formState.factoryIds.length || formState.factoryIds.includes(item.factoryId)
    const zoneMatched = !formState.zoneIds.length || item.zoneId == null || formState.zoneIds.includes(item.zoneId)
    const recorderMatched = !formState.recorderIds.length || formState.recorderIds.includes(item.recorderId)
    return factoryMatched && zoneMatched && recorderMatched
  })
})

const filteredRoles = computed(() => roles.value)
const menuTreeProps = {
  label: "label",
  children: "children",
}
const menuPermissionBindingMap: Record<string, { codes?: string[]; prefixes?: string[] }> = {
  dashboard: {
    codes: ["dashboard:refresh", "dashboard:stats:view"],
  },
  "safety-realtime-alarms": {
    codes: ["alarm:realtime:view"],
  },
  "safety-alarm-list": {
    codes: ["alarm:view", "alarm:process", "alarm:repush"],
  },
  "safety-alarm-stats": {
    codes: ["report:alarm:view", "report:alarm:export"],
  },
  "safety-operation-logs": {
    codes: ["log:operation:view", "log:operation:export"],
  },
  "monitor-preview": {
    codes: ["video:webcontrol:view"],
    prefixes: ["video:live:"],
  },
  "monitor-playback": {
    prefixes: ["video:playback:"],
    codes: ["video:snapshot:create"],
  },
  "monitor-ai-api": {
    prefixes: ["smart:provider:", "smart:capability:", "smart:binding:", "smart:rule:", "smart:event:", "smart:ai-task:"],
    codes: ["ai:event:view"],
  },
  "device-cameras": {
    prefixes: ["device:camera:"],
  },
  "device-recorders": {
    prefixes: ["device:recorder:"],
  },
  "device-channels": {
    prefixes: ["device:channel:"],
  },
  "device-status-logs": {
    codes: ["device:status:log:view", "device:status:check"],
  },
  "push-config": {
    prefixes: ["push:config:"],
  },
  "push-logs": {
    codes: ["push:log:view", "push:log:retry", "report:push:view", "report:push:export"],
  },
  "system-users": {
    prefixes: ["system:user:"],
  },
  "system-roles": {
    prefixes: ["system:role:"],
  },
  "basic-data-factories": {
    prefixes: ["basic:factory:"],
  },
  "basic-data-zones": {
    prefixes: ["basic:zone:"],
  },
  "basic-data-depts": {
    prefixes: ["basic:dept:"],
  },
  "basic-data-dicts": {
    prefixes: ["basic:dict:"],
  },
}

const permissionCodeToIdMap = computed(() => {
  return new Map(permissionOptions.value.map((item) => [item.code, item.id]))
})

const matchMenuPermissions = (menu: RoleMenuTreeItem) => {
  const binding = menuPermissionBindingMap[menu.routeName ?? ""] ?? menuPermissionBindingMap[menu.key]
  if (!binding) {
    return []
  }
  const keyword = permissionKeyword.value.trim().toLowerCase()
  return permissionOptions.value
    .filter((item) => {
      const codeMatched = binding.codes?.includes(item.code) ?? false
      const prefixMatched = binding.prefixes?.some((prefix) => item.code.startsWith(prefix)) ?? false
      if (!codeMatched && !prefixMatched) {
        return false
      }
      if (!keyword) {
        return true
      }
      const text = `${item.name} ${item.code}`.toLowerCase()
      return text.includes(keyword)
    })
    .sort((a, b) => a.code.localeCompare(b.code, "en"))
}

const getMenuPermissionItems = (menu: RoleMenuTreeItem) => matchMenuPermissions(menu)

const getMenuPermissionSelectedCount = (menu: RoleMenuTreeItem) =>
  getMenuPermissionItems(menu).filter((item) => selectedPermissionIds.value.includes(item.id)).length

const resetFormState = () => {
  formState.dataScopeType = "all"
  formState.factoryIds = []
  formState.zoneIds = []
  formState.deptIds = []
  formState.cameraIds = []
  formState.recorderIds = []
  formState.channelIds = []
  formState.userIds = []
}

const applyRoleToForm = (record: RoleDataScopeRecord) => {
  resetFormState()
  formState.dataScopeType = (record.dataScopeType as DataScopeType) || "all"
  const parsed = parseScopeValue(record)
  if (formState.dataScopeType === "factory" || formState.dataScopeType === "zone" || formState.dataScopeType === "dept") {
    if (formState.dataScopeType === "factory") formState.factoryIds = parsed.list
    if (formState.dataScopeType === "zone") formState.zoneIds = parsed.list
    if (formState.dataScopeType === "dept") formState.deptIds = parsed.list
  }
  if (formState.dataScopeType === "device") {
    formState.cameraIds = parsed.device.cameraIds
    formState.recorderIds = parsed.device.recorderIds
    formState.channelIds = parsed.device.channelIds
  }
  if (formState.dataScopeType === "custom") {
    formState.factoryIds = parsed.custom.factoryIds
    formState.zoneIds = parsed.custom.zoneIds
    formState.deptIds = parsed.custom.deptIds
    formState.cameraIds = parsed.custom.cameraIds
    formState.recorderIds = parsed.custom.recorderIds
    formState.channelIds = parsed.custom.channelIds
    formState.userIds = parsed.custom.userIds
  }
}

const buildPayload = () => {
  if (formState.dataScopeType === "all" || formState.dataScopeType === "self") {
    return { dataScopeType: formState.dataScopeType, dataScopeValue: null }
  }
  if (formState.dataScopeType === "factory") {
    return { dataScopeType: "factory" as const, dataScopeValue: formState.factoryIds }
  }
  if (formState.dataScopeType === "zone") {
    return { dataScopeType: "zone" as const, dataScopeValue: formState.zoneIds }
  }
  if (formState.dataScopeType === "dept") {
    return { dataScopeType: "dept" as const, dataScopeValue: formState.deptIds }
  }
  if (formState.dataScopeType === "device") {
    return {
      dataScopeType: "device" as const,
      dataScopeValue: {
        cameraIds: formState.cameraIds,
        recorderIds: formState.recorderIds,
        channelIds: formState.channelIds,
      },
    }
  }
  return {
    dataScopeType: "custom" as const,
    dataScopeValue: {
      factoryIds: formState.factoryIds,
      zoneIds: formState.zoneIds,
      deptIds: formState.deptIds,
      cameraIds: formState.cameraIds,
      recorderIds: formState.recorderIds,
      channelIds: formState.channelIds,
      userIds: formState.userIds,
    },
  }
}

const upsertRole = (record: RoleDataScopeRecord) => {
  const index = roles.value.findIndex((item) => item.id === record.id)
  if (index >= 0) {
    roles.value[index] = record
    return
  }
  roles.value = [record, ...roles.value]
}

const resetRoleForm = () => {
  roleEditingId.value = null
  roleForm.roleCode = ""
  roleForm.roleName = ""
  roleForm.status = "enabled"
  roleForm.remark = ""
}

const loadRoleRecords = async () => {
  const roleList = await listRolesApi({
    keyword: queryForm.keyword || undefined,
    status: queryForm.status || undefined,
  })
  roles.value = roleList
}

const loadLookups = async () => {
  const [factoryList, zoneList, deptList, cameraList, recorderList, channelList] = await Promise.all([
    listFactoriesApi(),
    listZonesApi(),
    listDeptsApi(),
    listCamerasApi(),
    listRecordersApi(),
    listChannelsApi(),
  ])
  factories.value = factoryList
  zones.value = zoneList
  depts.value = deptList
  cameras.value = cameraList
  recorders.value = recorderList
  channels.value = channelList
}

const loadRoleMenuTree = async () => {
  roleMenuTree.value = await listRoleMenuTreeApi()
}

const loadRolePermissionOptions = async () => {
  permissionOptions.value = await listRolePermissionOptionsApi()
}

const loadPage = async () => {
  loading.value = true
  accessDenied.value = false
  try {
    await Promise.all([loadRoleRecords(), loadLookups(), loadRoleMenuTree(), loadRolePermissionOptions()])
  } catch (error) {
    if (isForbidden(error)) {
      accessDenied.value = true
      deniedMessage.value = resolveErrorMessage(error, deniedMessage.value)
      return
    }
    ElMessage.error(resolveErrorMessage(error, "加载角色权限页面失败"))
  } finally {
    loading.value = false
  }
}

const openCreateRoleDialog = () => {
  resetRoleForm()
  roleDialogVisible.value = true
}

const openEditRoleDialog = (record: RoleDataScopeRecord) => {
  roleEditingId.value = record.id
  roleForm.roleCode = record.roleCode
  roleForm.roleName = record.roleName
  roleForm.status = (record.status === "disabled" ? "disabled" : "enabled") as RoleStatus
  roleForm.remark = record.remark || ""
  roleDialogVisible.value = true
}

const openEditDialog = (record: RoleDataScopeRecord) => {
  activeRole.value = record
  applyRoleToForm(record)
  dialogVisible.value = true
}

const collectMenuIdsByCodes = (nodes: RoleMenuTreeItem[], selectedCodes: Set<string>, output: number[]) => {
  nodes.forEach((node) => {
    if (node.id && selectedCodes.has(node.key)) {
      output.push(node.id)
    }
    if (node.children?.length) {
      collectMenuIdsByCodes(node.children as RoleMenuTreeItem[], selectedCodes, output)
    }
  })
}

const resolveMenuIdsByCodes = (menuCodes: string[]) => {
  const output: number[] = []
  collectMenuIdsByCodes(roleMenuTree.value, new Set(menuCodes), output)
  return Array.from(new Set(output))
}

const openAuthDialog = async (record: RoleDataScopeRecord) => {
  activeAuthRole.value = record
  authDialogVisible.value = true
  permissionKeyword.value = ""
  const checkedKeys = resolveMenuIdsByCodes(record.menuCodes)
  selectedPermissionIds.value = resolvePermissionIdsByCodes(record.permissionCodes)
  await nextTick()
  menuTreeRef.value?.setCheckedKeys(checkedKeys)
}

const resolvePermissionIdsByCodes = (codes: string[]) => {
  const ids = codes
    .map((code) => permissionCodeToIdMap.value.get(code))
    .filter((item): item is number => typeof item === "number")
  return Array.from(new Set(ids)).sort((a, b) => a - b)
}

const handleSelectAllPermissions = () => {
  selectedPermissionIds.value = permissionOptions.value.map((item) => item.id)
}

const handleClearPermissions = () => {
  selectedPermissionIds.value = []
}

const handlePermissionItemChange = (permissionID: number, checked: string | number | boolean) => {
  const selected = new Set(selectedPermissionIds.value)
  if (checked) {
    selected.add(permissionID)
  } else {
    selected.delete(permissionID)
  }
  selectedPermissionIds.value = Array.from(selected).sort((a, b) => a - b)
}

const handleSubmitRole = async () => {
  const valid = await roleFormRef.value?.validate().catch(() => false)
  if (!valid) {
    return
  }

  const isEditing = roleEditingId.value !== null
  roleSubmitting.value = true
  try {
    const payload: RoleSubmitPayload = {
      roleCode: roleForm.roleCode,
      roleName: roleForm.roleName,
      status: roleForm.status,
      remark: roleForm.remark || null,
    }
    const saved = roleEditingId.value ? await updateRoleApi(roleEditingId.value, payload) : await createRoleApi(payload)
    upsertRole(saved)
    roleDialogVisible.value = false
    resetRoleForm()
    ElMessage.success(isEditing ? "角色更新成功" : "角色创建成功")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, isEditing ? "更新角色失败" : "创建角色失败"))
  } finally {
    roleSubmitting.value = false
  }
}

const handleDeleteRole = async (record: RoleDataScopeRecord) => {
  try {
    await ElMessageBox.confirm(`确认删除角色“${record.roleName}”吗？`, "删除确认", { type: "warning" })
    await deleteRoleApi(record.id)
    roles.value = roles.value.filter((item) => item.id !== record.id)
    ElMessage.success("角色删除成功")
  } catch (error) {
    if (error === "cancel" || error === "close") {
      return
    }
    ElMessage.error(resolveErrorMessage(error, "删除角色失败"))
  }
}

const handleToggleRoleStatus = async (record: RoleDataScopeRecord) => {
  const nextStatus: RoleStatus = record.status === "enabled" ? "disabled" : "enabled"
  try {
    const updated = await updateRoleStatusApi(record.id, { status: nextStatus })
    upsertRole(updated)
    ElMessage.success(`角色${nextStatus === "enabled" ? "启用" : "停用"}成功`)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新角色状态失败"))
  }
}

const handleSave = async () => {
  if (!activeRole.value) {
    return
  }
  saving.value = true
  try {
    const updated = await updateRoleDataScopeApi(activeRole.value.id, buildPayload())
    const index = roles.value.findIndex((item) => item.id === updated.id)
    if (index >= 0) {
      roles.value[index] = updated
    }
    dialogVisible.value = false
    ElMessage.success("角色数据范围更新成功")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新角色数据范围失败"))
  } finally {
    saving.value = false
  }
}

const handleSaveAuthorization = async () => {
  if (!activeAuthRole.value) {
    return
  }
  authSaving.value = true
  try {
    const checkedKeys = menuTreeRef.value?.getCheckedKeys(false) ?? []
    const halfCheckedKeys = menuTreeRef.value?.getHalfCheckedKeys() ?? []
    const menuIds = Array.from(new Set([...checkedKeys, ...halfCheckedKeys])).sort((a, b) => a - b)
    await updateRoleMenusApi(activeAuthRole.value.id, { menuIds })
    const updated = await updateRolePermissionsApi(activeAuthRole.value.id, {
      permissionIds: [...selectedPermissionIds.value].sort((a, b) => a - b),
    })
    upsertRole(updated)
    authDialogVisible.value = false
    ElMessage.success("角色菜单与按钮权限更新成功")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新角色授权失败"))
  } finally {
    authSaving.value = false
  }
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.status = ""
  await loadRoleRecords()
}

onMounted(async () => {
  await loadPage()
})
</script>

<template>
  <div class="role-page">
    <PageCard v-if="accessDenied" title="角色权限" description="当前账号暂不具备角色数据范围查看权限。">
      <AccessDeniedState :description="deniedMessage">
        <template #actions>
          <button class="app-button app-button--secondary" @click="loadPage">
            <el-icon><RefreshRight /></el-icon>
            <span>重新加载</span>
          </button>
        </template>
      </AccessDeniedState>
    </PageCard>

    <template v-else>
      <PageCard class="role-page__filters-card unified-list-page__filters-card">
        <SearchForm class="unified-list-page__search-form">
          <div class="app-field role-page__keyword">
            <ClearableSearchInput v-model="queryForm.keyword" placeholder="输入角色编码、角色名称或备注" @clear="loadRoleRecords" />
          </div>
          <div class="app-field">
            <select v-model="queryForm.status" v-refresh-on-empty="loadRoleRecords">
              <option value="">状态</option>
              <option v-for="item in statusOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </div>
          <template #actions>
            <button class="app-button app-button--primary role-page__button unified-list-page__button unified-list-page__search-button" type="button" @click="loadRoleRecords">
              <el-icon><Search /></el-icon>
              <span>查询</span>
            </button>
            <button class="app-button app-button--secondary role-page__button unified-list-page__button unified-list-page__search-button" type="button" @click="handleResetQuery">
              <el-icon><RefreshRight /></el-icon>
              <span>重置</span>
            </button>
            <button
              v-permission="'system:role:create'"
              class="app-button app-button--success role-page__button unified-list-page__button unified-list-page__search-button"
              type="button"
              @click="openCreateRoleDialog"
            >
              <el-icon><Plus /></el-icon>
              <span>新增角色</span>
            </button>
          </template>
        </SearchForm>
      </PageCard>

      <PageCard>
        <table class="app-table unified-list-page__table">
          <thead>
            <tr>
              <th>角色编码</th>
              <th>角色名称</th>
              <th>状态</th>
              <th>权限摘要</th>
              <th>数据范围</th>
              <th>备注</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!filteredRoles.length">
              <td colspan="7" class="app-table__empty">{{ loading ? "加载中..." : "暂无角色数据" }}</td>
            </tr>
            <tr v-for="record in filteredRoles" :key="record.id">
              <td>{{ record.roleCode }}</td>
              <td>
                <div class="unified-list-page__name-cell">
                  <strong>{{ record.roleName }}</strong>
                  <span>{{ record.roleCode }}</span>
                </div>
              </td>
              <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
              <td>{{ menuSummary(record) }}</td>
              <td>{{ scopeSummaryText(record) }}</td>
              <td>{{ record.remark || "-" }}</td>
              <td>
                <div class="table-actions">
                  <button
                    v-permission="'system:role:update'"
                    class="app-button app-button--secondary role-page__button role-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="openEditRoleDialog(record)"
                  >
                    <el-icon><EditPen /></el-icon>
                    <span>编辑</span>
                  </button>
                  <button
                    v-if="!isAdminRole(record)"
                    v-permission="'system:role:update'"
                    class="app-button app-button--secondary role-page__button role-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="openAuthDialog(record)"
                  >
                    <el-icon><EditPen /></el-icon>
                    <span>配置授权</span>
                  </button>
                  <button
                    v-permission="'system:role:update'"
                    class="app-button app-button--secondary role-page__button role-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="openEditDialog(record)"
                  >
                    <el-icon><EditPen /></el-icon>
                    <span>配置范围</span>
                  </button>
                  <button
                    v-if="!isAdminRole(record) && record.status === 'enabled'"
                    v-permission="'system:role:disable'"
                    class="app-button app-button--warning role-page__button role-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="handleToggleRoleStatus(record)"
                  >
                    <el-icon><SwitchButton /></el-icon>
                    <span>停用</span>
                  </button>
                  <button
                    v-if="!isAdminRole(record) && record.status !== 'enabled'"
                    v-permission="'system:role:enable'"
                    class="app-button app-button--primary role-page__button role-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="handleToggleRoleStatus(record)"
                  >
                    <el-icon><SwitchButton /></el-icon>
                    <span>启用</span>
                  </button>
                  <button
                    v-if="!isAdminRole(record)"
                    v-permission="'system:role:delete'"
                    class="app-button app-button--danger role-page__button role-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="handleDeleteRole(record)"
                  >
                    <el-icon><Delete /></el-icon>
                    <span>删除</span>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </PageCard>
    </template>

    <el-dialog
      v-model="roleDialogVisible"
      :title="roleEditingId ? '编辑角色' : '新增角色'"
      width="560px"
      destroy-on-close
      @closed="resetRoleForm"
    >
      <el-form ref="roleFormRef" :model="roleForm" :rules="roleRules" label-width="90px">
        <el-form-item label="角色编码" prop="roleCode">
          <el-input v-model="roleForm.roleCode" :disabled="roleEditingId !== null && roleForm.roleCode === 'admin'" />
        </el-form-item>
        <el-form-item label="角色名称" prop="roleName">
          <el-input v-model="roleForm.roleName" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="roleForm.status" style="width: 100%" :disabled="roleEditingId !== null && roleForm.roleCode === 'admin'">
            <el-option v-for="item in roleStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="roleForm.remark" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <button class="app-button app-button--secondary" @click="roleDialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="roleSubmitting" @click="handleSubmitRole">
          {{ roleSubmitting ? "保存中..." : "保存" }}
        </button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="authDialogVisible"
      :title="activeAuthRole ? `配置授权 - ${activeAuthRole.roleName}` : '配置授权'"
      width="920px"
      class="role-page__panel-dialog role-page__auth-dialog"
      destroy-on-close
    >
      <div class="role-page__dialog">
        <el-alert
          class="role-page__menu-alert"
          type="info"
          :closable="false"
          title="菜单勾选控制导航可见范围，菜单后面的权限清单控制页面按钮和接口能力，二者会一起保存。"
        />
        <div class="role-page__permission-toolbar">
          <div class="app-field role-page__permission-keyword">
            <ClearableSearchInput v-model="permissionKeyword" placeholder="搜索菜单后的权限名称或权限码" />
          </div>
          <div class="role-page__permission-actions">
            <button class="app-button app-button--secondary" type="button" @click="handleSelectAllPermissions">全部勾选</button>
            <button class="app-button app-button--secondary" type="button" @click="handleClearPermissions">全部清空</button>
          </div>
        </div>
        <div class="role-page__menu-tree">
          <el-tree
            ref="menuTreeRef"
            :data="roleMenuTree"
            node-key="id"
            show-checkbox
            default-expand-all
            :props="menuTreeProps"
          >
            <template #default="{ data }">
              <div class="role-page__menu-node">
                <div class="role-page__menu-node-main">
                  <span class="role-page__menu-node-label">{{ data.label }}</span>
                  <span v-if="getMenuPermissionItems(data).length" class="role-page__menu-node-meta">
                    {{ getMenuPermissionSelectedCount(data) }} / {{ getMenuPermissionItems(data).length }}
                  </span>
                </div>
                <div v-if="getMenuPermissionItems(data).length" class="role-page__inline-permissions" @click.stop>
                  <div class="role-page__inline-permission-list">
                    <label
                      v-for="item in getMenuPermissionItems(data)"
                      :key="item.id"
                      :class="[
                        'role-page__inline-permission-item',
                        { 'role-page__inline-permission-item--selected': selectedPermissionIds.includes(item.id) },
                      ]"
                    >
                      <el-checkbox
                        :model-value="selectedPermissionIds.includes(item.id)"
                        @update:model-value="handlePermissionItemChange(item.id, $event)"
                      />
                      <span class="role-page__permission-name">{{ item.name }}</span>
                      <span class="role-page__permission-code">{{ item.code }}</span>
                    </label>
                  </div>
                </div>
              </div>
            </template>
          </el-tree>
        </div>
      </div>

      <template #footer>
        <button class="app-button app-button--secondary role-page__dialog-footer-button" @click="authDialogVisible = false">取消</button>
        <button class="app-button app-button--primary role-page__dialog-footer-button" :disabled="authSaving" @click="handleSaveAuthorization">
          {{ authSaving ? "保存中..." : "保存" }}
        </button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="dialogVisible"
      :title="activeRole ? `配置数据范围 - ${activeRole.roleName}` : '配置数据范围'"
      width="980px"
      class="role-page__panel-dialog role-page__scope-dialog"
      destroy-on-close
    >
      <div class="role-page__dialog">
        <el-form label-width="110px">
          <el-form-item label="数据范围类型">
            <el-radio-group v-model="formState.dataScopeType">
              <el-radio-button v-for="item in scopeTypeOptions" :key="item.value" :label="item.value">
                {{ item.label }}
              </el-radio-button>
            </el-radio-group>
          </el-form-item>

          <el-form-item v-if="formState.dataScopeType === 'factory' || formState.dataScopeType === 'custom'" label="厂区">
            <el-select v-model="formState.factoryIds" multiple collapse-tags filterable style="width: 100%">
              <el-option v-for="item in factories" :key="item.id" :label="item.factoryName" :value="item.id" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="formState.dataScopeType === 'zone' || formState.dataScopeType === 'custom'" label="区域">
            <el-select v-model="formState.zoneIds" multiple collapse-tags filterable style="width: 100%">
              <el-option v-for="item in selectedZoneOptions" :key="item.id" :label="`${item.factoryName} / ${item.zoneName}`" :value="item.id" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="formState.dataScopeType === 'dept' || formState.dataScopeType === 'custom'" label="部门">
            <el-select v-model="formState.deptIds" multiple collapse-tags filterable style="width: 100%">
              <el-option v-for="item in selectedDeptOptions" :key="item.id" :label="item.deptName" :value="item.id" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="formState.dataScopeType === 'device' || formState.dataScopeType === 'custom'" label="摄像机">
            <el-select v-model="formState.cameraIds" multiple collapse-tags filterable style="width: 100%">
              <el-option v-for="item in selectedCameraOptions" :key="item.id" :label="`${item.name} / ${item.deviceCode}`" :value="item.id" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="formState.dataScopeType === 'device' || formState.dataScopeType === 'custom'" label="录像机">
            <el-select v-model="formState.recorderIds" multiple collapse-tags filterable style="width: 100%">
              <el-option v-for="item in selectedRecorderOptions" :key="item.id" :label="`${item.name} / ${item.deviceCode}`" :value="item.id" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="formState.dataScopeType === 'device' || formState.dataScopeType === 'custom'" label="通道">
            <el-select v-model="formState.channelIds" multiple collapse-tags filterable style="width: 100%">
              <el-option
                v-for="item in selectedChannelOptions"
                :key="item.id"
                :label="`${item.recorderName} / CH${String(item.channelNo).padStart(2, '0')} / ${item.name}`"
                :value="item.id"
              />
            </el-select>
          </el-form-item>

          <el-alert
            v-if="formState.dataScopeType === 'self'"
            type="info"
            :closable="false"
            title="本人范围表示仅允许访问与当前登录人直接关联的数据。"
          />
          <el-alert
            v-if="formState.dataScopeType === 'all'"
            type="warning"
            :closable="false"
            title="全部数据将绕过厂区、区域和设备限制，请谨慎分配。"
          />
        </el-form>
      </div>

      <template #footer>
        <button class="app-button app-button--secondary role-page__dialog-footer-button" @click="dialogVisible = false">取消</button>
        <button class="app-button app-button--primary role-page__dialog-footer-button" :disabled="saving" @click="handleSave">
          {{ saving ? "保存中..." : "保存" }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.role-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.role-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.role-page__table-button {
  min-height: 30px;
  padding: 0 10px;
  font-size: 12px;
  gap: 4px;
  white-space: nowrap;
}

.role-page__table-button :deep(.el-icon) {
  font-size: 12px;
}

.role-page__dialog-footer-button {
  min-height: 34px;
  padding: 0 14px;
  border-radius: 12px;
  font-size: 12px;
}

.role-page :deep(.unified-list-page__table th:last-child),
.role-page :deep(.unified-list-page__table td:last-child) {
  width: 420px;
  min-width: 420px;
}

.role-page :deep(.unified-list-page__table .table-actions) {
  min-width: 400px;
  gap: 6px;
}

.role-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.role-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.role-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 180px minmax(320px, 1fr);
  gap: 10px;
  align-items: end;
}

.role-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.role-page__filters-card :deep(.app-field select),
.role-page__filters-card :deep(.app-field input) {
  height: 36px;
  font-size: 13px;
}

.role-page__filters-card .role-page__button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.role-page__keyword {
  grid-column: auto;
}

.role-page__dialog {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.role-page :deep(.role-page__panel-dialog) {
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background:
    radial-gradient(circle at top right, rgba(96, 165, 250, 0.18), transparent 36%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(241, 245, 249, 0.98) 100%);
  box-shadow:
    0 24px 64px rgba(15, 23, 42, 0.16),
    inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.role-page :deep(.role-page__panel-dialog .el-dialog__header) {
  margin: 0;
  padding: 18px 22px 14px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.18);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(226, 232, 240, 0.72) 100%);
  border-radius: 18px 18px 0 0;
}

.role-page :deep(.role-page__scope-dialog .el-dialog__header) {
  margin: 10px 10px 0;
  border-radius: 14px 14px 0 0;
}

.role-page :deep(.role-page__panel-dialog .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.role-page :deep(.role-page__panel-dialog .el-dialog__headerbtn .el-dialog__close) {
  color: #64748b;
}

.role-page :deep(.role-page__panel-dialog .el-dialog__body) {
  padding: 18px 22px 12px;
  background: transparent;
}

.role-page :deep(.role-page__panel-dialog .el-dialog__footer) {
  padding: 14px 22px 20px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.88) 0%, rgba(226, 232, 240, 0.68) 100%);
  border-radius: 0 0 18px 18px;
}

.role-page__menu-alert :deep(.el-alert) {
  border: 1px solid rgba(125, 211, 252, 0.32);
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.98) 0%, rgba(224, 242, 254, 0.9) 100%);
  border-radius: 14px;
}

.role-page__menu-alert :deep(.el-alert__title) {
  color: #0f172a;
  font-weight: 500;
}

.role-page__menu-alert :deep(.el-alert__icon) {
  color: #0284c7;
}

.role-page__menu-tree {
  max-height: 420px;
  overflow: auto;
  padding: 14px 16px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(241, 245, 249, 0.94) 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    inset 0 0 0 1px rgba(226, 232, 240, 0.72),
    0 16px 32px rgba(148, 163, 184, 0.12);
}

.role-page__menu-tree::-webkit-scrollbar {
  width: 10px;
}

.role-page__menu-tree::-webkit-scrollbar-track {
  background: rgba(226, 232, 240, 0.86);
  border-radius: 999px;
}

.role-page__menu-tree::-webkit-scrollbar-thumb {
  background: linear-gradient(180deg, rgba(148, 163, 184, 0.9) 0%, rgba(100, 116, 139, 0.92) 100%);
  border-radius: 999px;
  border: 2px solid transparent;
  background-clip: padding-box;
}

.role-page__permission-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.role-page__permission-keyword {
  flex: 1;
}

.role-page__permission-keyword input {
  width: 100%;
  border-radius: 12px;
}

.role-page__permission-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.role-page__menu-node {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  padding: 8px 0;
}

.role-page__menu-node-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.role-page__menu-node-label {
  color: #0f172a;
  font-size: 14px;
  font-weight: 600;
}

.role-page__menu-node-meta {
  color: #475569;
  font-size: 12px;
  font-family: Consolas, "Courier New", monospace;
}

.role-page__inline-permissions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-left: 4px;
  padding: 10px 12px 12px;
  border-radius: 12px;
  border: 1px solid rgba(191, 219, 254, 0.52);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96) 0%, rgba(241, 245, 249, 0.94) 100%);
}

.role-page__inline-permission-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.role-page__inline-permission-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 6px 10px;
  border-radius: 10px;
  border: 1px solid rgba(203, 213, 225, 0.9);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(248, 250, 252, 0.94) 100%);
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.role-page__inline-permission-item:hover {
  border-color: rgba(96, 165, 250, 0.46);
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.96) 0%, rgba(224, 242, 254, 0.9) 100%);
}

.role-page__inline-permission-item--selected {
  border-color: rgba(30, 41, 59, 0.92);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98) 0%, rgba(15, 23, 42, 0.96) 100%);
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.18),
    0 0 0 1px rgba(15, 23, 42, 0.16);
}

.role-page__inline-permission-item--selected:hover {
  border-color: rgba(15, 23, 42, 0.96);
  background: linear-gradient(180deg, rgba(51, 65, 85, 0.98) 0%, rgba(15, 23, 42, 0.98) 100%);
}

.role-page__inline-permission-item--selected .role-page__permission-name {
  color: #f8fafc;
}

.role-page__inline-permission-item--selected .role-page__permission-code {
  color: #7dd3fc;
}

.role-page__permission-name {
  color: #0f172a;
  font-size: 13px;
  font-weight: 600;
}

.role-page__permission-code {
  color: #64748b;
  font-size: 11px;
  font-family: Consolas, "Courier New", monospace;
}

.role-page :deep(.el-radio-group) {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.role-page :deep(.role-page__scope-dialog .el-form) {
  padding: 18px 20px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(248, 250, 252, 0.96) 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    0 12px 24px rgba(148, 163, 184, 0.08);
}

.role-page :deep(.role-page__scope-dialog .el-form-item) {
  margin-bottom: 18px;
}

.role-page :deep(.role-page__scope-dialog .el-form-item__label) {
  color: #334155;
  font-weight: 600;
}

.role-page :deep(.role-page__scope-dialog .el-radio-group) {
  flex-wrap: nowrap;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
}

.role-page :deep(.role-page__scope-dialog .el-radio-button) {
  margin-right: 0;
  flex: 0 0 auto;
}

.role-page :deep(.role-page__scope-dialog .el-radio-button__inner) {
  min-width: 86px;
  min-height: 36px;
  padding: 0 12px;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(241, 245, 249, 0.96) 100%);
  color: #334155;
  font-size: 13px;
  font-weight: 600;
  line-height: 34px;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    0 8px 18px rgba(148, 163, 184, 0.08);
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.role-page :deep(.role-page__scope-dialog .el-radio-button__inner:hover) {
  border-color: rgba(96, 165, 250, 0.42);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98) 0%, rgba(219, 234, 254, 0.78) 100%);
  color: #0f172a;
  transform: translateY(-1px);
}

.role-page :deep(.role-page__scope-dialog .el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: #0f172a;
  background: linear-gradient(180deg, #334155 0%, #0f172a 100%);
  color: #f8fafc;
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.22),
    0 12px 24px rgba(15, 23, 42, 0.22);
}

.role-page :deep(.role-page__scope-dialog .el-select__wrapper),
.role-page :deep(.role-page__scope-dialog .el-input__wrapper) {
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.22) inset;
  border-radius: 12px;
}

.role-page :deep(.role-page__scope-dialog .el-select__placeholder),
.role-page :deep(.role-page__scope-dialog .el-input__inner) {
  color: #475569;
}

.role-page :deep(.role-page__scope-dialog .el-tag) {
  border-color: rgba(30, 41, 59, 0.12);
  background: rgba(226, 232, 240, 0.88);
  color: #0f172a;
  border-radius: 999px;
}

.role-page :deep(.role-page__scope-dialog .el-alert) {
  border: 1px solid rgba(191, 219, 254, 0.68);
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.98) 0%, rgba(241, 245, 249, 0.94) 100%);
  border-radius: 14px;
}

.role-page :deep(.role-page__scope-dialog .el-alert__title) {
  color: #0f172a;
}

.role-page :deep(.role-page__scope-dialog .el-alert__icon) {
  color: #0284c7;
}

.role-page :deep(.el-tree) {
  background: transparent;
  color: #334155;
}

.role-page :deep(.el-tree-node__content) {
  min-height: 36px;
  height: auto;
  align-items: flex-start;
  border-radius: 12px;
  padding: 0 10px 0 6px;
  transition:
    background-color 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.role-page :deep(.el-tree-node__label) {
  color: #334155;
  font-size: 14px;
  font-weight: 500;
}

.role-page :deep(.el-tree-node__content:hover) {
  background: linear-gradient(90deg, rgba(219, 234, 254, 0.86) 0%, rgba(239, 246, 255, 0.9) 100%);
  box-shadow: inset 0 0 0 1px rgba(147, 197, 253, 0.34);
  transform: translateX(2px);
}

.role-page :deep(.el-tree-node:focus > .el-tree-node__content) {
  background: linear-gradient(90deg, rgba(219, 234, 254, 0.92) 0%, rgba(239, 246, 255, 0.96) 100%);
}

.role-page :deep(.el-tree-node.is-current > .el-tree-node__content) {
  background: linear-gradient(90deg, rgba(191, 219, 254, 0.96) 0%, rgba(224, 242, 254, 0.92) 100%);
}

.role-page :deep(.el-checkbox__input.is-checked + .el-checkbox__label),
.role-page :deep(.el-checkbox__input.is-indeterminate + .el-checkbox__label) {
  color: #0f172a;
  font-weight: 600;
}

.role-page :deep(.el-checkbox__inner) {
  background: rgba(255, 255, 255, 0.96);
  border-color: rgba(148, 163, 184, 0.72);
  border-radius: 6px;
}

.role-page :deep(.el-checkbox__input.is-checked .el-checkbox__inner),
.role-page :deep(.el-checkbox__input.is-indeterminate .el-checkbox__inner) {
  border-color: #0f172a;
  background: linear-gradient(180deg, #334155 0%, #0f172a 100%);
  box-shadow: 0 0 10px rgba(15, 23, 42, 0.18);
}

.role-page :deep(.el-checkbox__input.is-checked .el-checkbox__inner::after),
.role-page :deep(.el-checkbox__input.is-indeterminate .el-checkbox__inner::after) {
  border-color: #f8fafc;
}

@media (max-width: 960px) {
  .role-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .role-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 768px) {
  .role-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }

  .role-page__permission-toolbar,
  .role-page__menu-node-main {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
