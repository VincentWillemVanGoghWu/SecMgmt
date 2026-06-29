<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import { Delete, EditPen, Plus, RefreshRight, Search } from "@element-plus/icons-vue"

import { listDeptsApi } from "../../api/master-data"
import { listRolesApi } from "../../api/role"
import { createUserApi, deleteUserApi, listUsersApi, updateUserApi } from "../../api/user"
import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import type { DeptRecord } from "../../types/master-data"
import type { RoleDataScopeRecord } from "../../types/role"
import type { UserCreatePayload, UserRecord, UserStatus, UserSubmitPayload, UserUpdatePayload } from "../../types/user"

const PASSWORD_MESSAGE = "密码必须至少8位，且包含大写字母、小写字母和数字"
const PASSWORD_PATTERN = /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}$/

const loading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()

const records = ref<UserRecord[]>([])
const roles = ref<RoleDataScopeRecord[]>([])
const depts = ref<DeptRecord[]>([])

const statusOptions: Array<{ label: string; value: UserStatus | "" }> = [
  { label: "全部", value: "" },
  { label: "启用", value: "enabled" },
  { label: "停用", value: "disabled" },
]

const userStatusOptions: Array<{ label: string; value: UserStatus }> = [
  { label: "启用", value: "enabled" },
  { label: "停用", value: "disabled" },
]

const queryForm = reactive({
  keyword: "",
  status: "",
  deptId: "",
  roleId: "",
})

const formState = reactive<{
  username: string
  realName: string
  deptId: number | null
  status: UserStatus
  roleIds: number[]
  password: string
}>({
  username: "",
  realName: "",
  deptId: null,
  status: "enabled",
  roleIds: [],
  password: "",
})

const validatePassword = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  const password = value.trim()
  if (!editingId.value && !password) {
    callback(new Error("请输入密码"))
    return
  }
  if (password && !PASSWORD_PATTERN.test(password)) {
    callback(new Error(PASSWORD_MESSAGE))
    return
  }
  callback()
}

const rules: FormRules = {
  username: [{ required: true, message: "请输入用户名", trigger: "blur" }],
  realName: [{ required: true, message: "请输入姓名", trigger: "blur" }],
  password: [{ validator: validatePassword, trigger: "blur" }],
}

const getStatusTone = (status: string) => (status === "enabled" ? "success" : "default")
const getStatusText = (status: string) => (status === "enabled" ? "启用" : "停用")
const isAdminUser = (record: UserRecord) => record.username === "admin"

const roleOptions = computed(() => roles.value)

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, "0")
  const day = String(date.getDate()).padStart(2, "0")
  const hours = String(date.getHours()).padStart(2, "0")
  const minutes = String(date.getMinutes()).padStart(2, "0")
  const seconds = String(date.getSeconds()).padStart(2, "0")
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

const getRoleNames = (record: UserRecord) => {
  if (!record.roles.length) return "-"
  return record.roles.map((item) => item.roleName).join("、")
}

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const loadOptions = async () => {
  const [deptList, roleList] = await Promise.all([listDeptsApi(), listRolesApi()])
  depts.value = deptList
  roles.value = roleList
}

const loadRecords = async () => {
  loading.value = true
  try {
    records.value = await listUsersApi({
      keyword: queryForm.keyword || undefined,
      status: queryForm.status || undefined,
      dept_id: queryForm.deptId ? Number(queryForm.deptId) : undefined,
      role_id: queryForm.roleId ? Number(queryForm.roleId) : undefined,
    })
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "加载用户列表失败"))
  } finally {
    loading.value = false
  }
}

const resetFormState = () => {
  editingId.value = null
  formState.username = ""
  formState.realName = ""
  formState.deptId = null
  formState.status = "enabled"
  formState.roleIds = []
  formState.password = ""
}

const openCreateDialog = () => {
  resetFormState()
  dialogVisible.value = true
}

const openEditDialog = (record: UserRecord) => {
  editingId.value = record.id
  formState.username = record.username
  formState.realName = record.realName
  formState.deptId = record.deptId ?? null
  formState.status = (record.status === "disabled" ? "disabled" : "enabled") as UserStatus
  formState.roleIds = record.roles.map((item) => item.id)
  formState.password = ""
  dialogVisible.value = true
}

const upsertRecord = (record: UserRecord) => {
  const index = records.value.findIndex((item) => item.id === record.id)
  if (index >= 0) {
    records.value[index] = record
    return
  }
  records.value = [record, ...records.value]
}

const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) {
    return
  }

  const isEditing = editingId.value !== null
  submitting.value = true
  try {
    const basePayload: UserSubmitPayload = {
      username: formState.username.trim(),
      realName: formState.realName.trim(),
      deptId: formState.deptId,
      status: formState.status,
      roleIds: [...formState.roleIds],
    }

    const saved = isEditing
      ? await updateUserApi(editingId.value as number, {
          ...(basePayload satisfies UserSubmitPayload),
          password: formState.password.trim() || undefined,
        } satisfies UserUpdatePayload)
      : await createUserApi({
          ...(basePayload satisfies UserSubmitPayload),
          password: formState.password.trim(),
        } satisfies UserCreatePayload)

    upsertRecord(saved)
    dialogVisible.value = false
    resetFormState()
    ElMessage.success(isEditing ? "用户更新成功" : "用户创建成功")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, isEditing ? "更新用户失败" : "创建用户失败"))
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (record: UserRecord) => {
  try {
    await ElMessageBox.confirm(`确认删除用户“${record.username}”吗？`, "删除确认", { type: "warning" })
    await deleteUserApi(record.id)
    records.value = records.value.filter((item) => item.id !== record.id)
    ElMessage.success("用户删除成功")
  } catch (error) {
    if (error === "cancel" || error === "close") {
      return
    }
    ElMessage.error(resolveErrorMessage(error, "删除用户失败"))
  }
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.status = ""
  queryForm.deptId = ""
  queryForm.roleId = ""
  await loadRecords()
}

onMounted(async () => {
  await loadOptions()
  await loadRecords()
})
</script>

<template>
  <div class="user-page unified-list-page">
    <PageCard class="user-page__filters-card">
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.deptId" v-refresh-on-empty="loadRecords">
            <option value="">部门</option>
            <option v-for="item in depts" :key="item.id" :value="String(item.id)">{{ item.deptName }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.roleId" v-refresh-on-empty="loadRecords">
            <option value="">角色</option>
            <option v-for="item in roleOptions" :key="item.id" :value="String(item.id)">{{ item.roleName }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.status" v-refresh-on-empty="loadRecords">
            <option value="">状态</option>
            <option v-for="item in statusOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <ClearableSearchInput v-model="queryForm.keyword" placeholder="输入用户名或姓名" @clear="loadRecords" />
        </div>
        <template #actions>
          <button class="app-button app-button--primary user-page__button unified-list-page__button unified-list-page__search-button" type="button" @click="loadRecords">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary user-page__button unified-list-page__button unified-list-page__search-button" type="button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'system:user:create'"
            class="app-button app-button--success user-page__button unified-list-page__button unified-list-page__search-button"
            type="button"
            @click="openCreateDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增用户</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <PageCard>
      <table class="app-table unified-list-page__table">
        <thead>
          <tr>
            <th>用户名</th>
            <th>姓名</th>
            <th>部门</th>
            <th>角色</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="7" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>
              <div class="unified-list-page__name-cell">
                <strong>{{ record.username }}</strong>
              </div>
            </td>
            <td>{{ record.realName }}</td>
            <td>{{ record.deptName || "-" }}</td>
            <td>{{ getRoleNames(record) }}</td>
            <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
            <td>{{ formatDateTime(record.createdAt) }}</td>
            <td>
              <div class="table-actions">
                <button
                  v-permission="'system:user:update'"
                  class="app-button app-button--secondary user-page__button user-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="openEditDialog(record)"
                >
                  <el-icon><EditPen /></el-icon>
                  <span>编辑</span>
                </button>
                <button
                  v-if="!isAdminUser(record)"
                  v-permission="'system:user:delete'"
                  class="app-button app-button--danger user-page__button user-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="handleDelete(record)"
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

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑用户' : '新增用户'" width="620px" destroy-on-close @closed="resetFormState">
      <el-form ref="formRef" :model="formState" :rules="rules" label-width="90px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="formState.username" :disabled="editingId !== null && formState.username === 'admin'" />
        </el-form-item>
        <el-form-item label="姓名" prop="realName">
          <el-input v-model="formState.realName" />
        </el-form-item>
        <el-form-item label="所属部门">
          <el-select v-model="formState.deptId" clearable style="width: 100%">
            <el-option v-for="item in depts" :key="item.id" :label="item.deptName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="formState.roleIds" multiple collapse-tags filterable style="width: 100%">
            <el-option
              v-for="item in roleOptions"
              :key="item.id"
              :label="item.status === 'enabled' ? item.roleName : `${item.roleName}（停用）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="formState.status" style="width: 100%" :disabled="editingId !== null && formState.username === 'admin'">
            <el-option v-for="item in userStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="formState.password"
            type="password"
            show-password
            :placeholder="editingId ? '留空表示保持原密码；修改时需满足复杂度要求' : PASSWORD_MESSAGE"
          />
        </el-form-item>
        <el-form-item>
          <div class="user-page__password-tip">{{ PASSWORD_MESSAGE }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <button class="app-button app-button--secondary" @click="dialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? "保存中..." : "保存" }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.user-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.user-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.user-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.user-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.user-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 180px 180px 180px minmax(280px, 1fr);
  gap: 10px;
  align-items: end;
}

.user-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.user-page__filters-card :deep(.app-field select),
.user-page__filters-card :deep(.app-field input) {
  height: 36px;
  font-size: 13px;
}

.user-page__filters-card .user-page__button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.user-page__table-button {
  min-height: 30px;
  padding: 0 10px;
  font-size: 12px;
  gap: 4px;
  white-space: nowrap;
}

.user-page__table-button :deep(.el-icon) {
  font-size: 12px;
}

.user-page__password-tip {
  font-size: 12px;
  color: var(--app-text-secondary, #6b7280);
}

@media (max-width: 1200px) {
  .user-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .user-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 768px) {
  .user-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
