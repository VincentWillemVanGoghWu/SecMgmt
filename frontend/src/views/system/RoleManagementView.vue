<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import { Delete, EditPen, Plus, RefreshRight, Search, SwitchButton } from "@element-plus/icons-vue"

import { listCamerasApi } from "../../api/camera"
import { listFactoriesApi, listDeptsApi, listZonesApi } from "../../api/master-data"
import { listChannelsApi, listRecordersApi } from "../../api/recorder"
import { createRoleApi, deleteRoleApi, listRolesApi, updateRoleApi, updateRoleDataScopeApi, updateRoleStatusApi } from "../../api/role"
import AccessDeniedState from "../../components/common/AccessDeniedState.vue"
import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import type { CameraRecord } from "../../types/camera"
import type { FactoryRecord, DeptRecord, ZoneRecord } from "../../types/master-data"
import type { RecorderChannelRecord, RecorderRecord } from "../../types/recorder"
import type { CustomScopeValue, DataScopeType, DeviceScopeValue, RoleDataScopeRecord, RoleStatus, RoleSubmitPayload } from "../../types/role"

const loading = ref(false)
const roleSubmitting = ref(false)
const saving = ref(false)
const roleDialogVisible = ref(false)
const dialogVisible = ref(false)
const accessDenied = ref(false)
const deniedMessage = ref("请联系管理员为当前账号开通角色查看权限。")
const roleEditingId = ref<number | null>(null)
const roleFormRef = ref<FormInstance>()
const activeRole = ref<RoleDataScopeRecord | null>(null)

const roles = ref<RoleDataScopeRecord[]>([])
const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const depts = ref<DeptRecord[]>([])
const cameras = ref<CameraRecord[]>([])
const recorders = ref<RecorderRecord[]>([])
const channels = ref<RecorderChannelRecord[]>([])

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
  `${record.menuCodes.length} 个菜单 / ${record.permissionCodes.length} 个按钮`

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
    const zoneMatched = !formState.zoneIds.length || (item.zoneId ? formState.zoneIds.includes(item.zoneId) : false)
    return factoryMatched && zoneMatched
  })
})

const filteredRoles = computed(() => roles.value)

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

const loadPage = async () => {
  loading.value = true
  accessDenied.value = false
  try {
    await Promise.all([loadRoleRecords(), loadLookups()])
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
            <input v-model="queryForm.keyword" type="text" placeholder="输入角色编码、角色名称或备注" />
          </div>
          <div class="app-field">
            <select v-model="queryForm.status">
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
      v-model="dialogVisible"
      :title="activeRole ? `配置数据范围 - ${activeRole.roleName}` : '配置数据范围'"
      width="860px"
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
        <button class="app-button app-button--secondary" @click="dialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="saving" @click="handleSave">
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

.role-page :deep(.el-radio-group) {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
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
}
</style>
