<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import { Delete, EditPen, Plus, RefreshRight, Search, SwitchButton } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import {
  createDeptApi,
  deleteDeptApi,
  listDeptsApi,
  listFactoriesApi,
  listZonesApi,
  updateDeptApi,
  updateDeptStatusApi,
} from "../../api/master-data"
import type { DeptRecord, FactoryRecord, ZoneRecord } from "../../types/master-data"

const loading = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()

const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const records = ref<DeptRecord[]>([])

const statusOptions = [
  { label: "启用", value: "enabled" },
  { label: "停用", value: "disabled" },
]

const queryForm = reactive({
  keyword: "",
  status: "",
  factoryId: "",
})

const formState = reactive<Omit<DeptRecord, "id" | "parentName" | "factoryName" | "zoneName">>({
  deptCode: "",
  deptName: "",
  parentId: null,
  factoryId: null,
  zoneId: null,
  leader: "",
  phone: "",
  sort: 0,
  status: "enabled",
  remark: "",
})

const rules: FormRules = {
  deptName: [{ required: true, message: "请输入部门名称", trigger: "blur" }],
}

const zoneOptions = computed(() =>
  zones.value.filter((item) => !formState.factoryId || item.factoryId === formState.factoryId),
)

const parentOptions = computed(() =>
  records.value.filter((item) => !editingId.value || item.id !== editingId.value),
)

const getStatusTone = (status: string) => (status === "enabled" ? "success" : "default")
const getStatusText = (status: string) => (status === "enabled" ? "启用" : "停用")

const loadOptions = async () => {
  const [factoryList, zoneList] = await Promise.all([listFactoriesApi(), listZonesApi()])
  factories.value = factoryList
  zones.value = zoneList
}

const loadRecords = async () => {
  loading.value = true
  try {
    records.value = await listDeptsApi({
      keyword: queryForm.keyword || undefined,
      status: queryForm.status || undefined,
      factory_id: queryForm.factoryId ? Number(queryForm.factoryId) : undefined,
    })
  } finally {
    loading.value = false
  }
}

const resetFormState = () => {
  editingId.value = null
  formState.deptCode = ""
  formState.deptName = ""
  formState.parentId = null
  formState.factoryId = null
  formState.zoneId = null
  formState.leader = ""
  formState.phone = ""
  formState.sort = 0
  formState.status = "enabled"
  formState.remark = ""
}

watch(
  () => formState.factoryId,
  () => {
    if (!formState.zoneId) {
      return
    }
    const matched = zoneOptions.value.some((item) => item.id === formState.zoneId)
    if (!matched) {
      formState.zoneId = null
    }
  },
)

const openCreateDialog = () => {
  resetFormState()
  dialogVisible.value = true
}

const openEditDialog = (record: DeptRecord) => {
  editingId.value = record.id
  formState.deptCode = record.deptCode
  formState.deptName = record.deptName
  formState.parentId = record.parentId ?? null
  formState.factoryId = record.factoryId ?? null
  formState.zoneId = record.zoneId ?? null
  formState.leader = record.leader ?? ""
  formState.phone = record.phone ?? ""
  formState.sort = record.sort
  formState.status = record.status
  formState.remark = record.remark ?? ""
  dialogVisible.value = true
}

const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) {
    return
  }

  if (editingId.value) {
    await updateDeptApi(editingId.value, { ...formState })
    ElMessage.success("部门更新成功")
  } else {
    await createDeptApi({ ...formState })
    ElMessage.success("部门创建成功")
  }

  dialogVisible.value = false
  resetFormState()
  await loadRecords()
}

const handleDelete = async (record: DeptRecord) => {
  await ElMessageBox.confirm(`确认删除部门“${record.deptName}”吗？`, "删除确认", { type: "warning" })
  await deleteDeptApi(record.id)
  ElMessage.success("部门删除成功")
  await loadRecords()
}

const handleToggleStatus = async (record: DeptRecord) => {
  const nextStatus = record.status === "enabled" ? "disabled" : "enabled"
  await updateDeptStatusApi(record.id, { status: nextStatus })
  ElMessage.success("部门状态更新成功")
  await loadRecords()
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.status = ""
  queryForm.factoryId = ""
  await loadRecords()
}

onMounted(async () => {
  await loadOptions()
  await loadRecords()
})
</script>

<template>
  <div class="master-data-page unified-list-page">
    <PageCard class="master-data-page__filters-card">
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.factoryId" v-refresh-on-empty="loadRecords">
            <option value="">所属厂区</option>
            <option v-for="item in factories" :key="item.id" :value="String(item.id)">{{ item.factoryName }}</option>
          </select>
        </div>
        <div class="app-field">
          <ClearableSearchInput v-model="queryForm.keyword" placeholder="输入部门名称或编码" @clear="loadRecords" />
        </div>
        <div class="app-field">
          <select v-model="queryForm.status" v-refresh-on-empty="loadRecords">
            <option value="">状态</option>
            <option v-for="item in statusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <template #actions>
          <button class="app-button app-button--primary master-data-page__button unified-list-page__button unified-list-page__search-button" @click="loadRecords">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary master-data-page__button unified-list-page__button unified-list-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'basic:dept:create'"
            class="app-button app-button--success master-data-page__button unified-list-page__button unified-list-page__search-button"
            @click="openCreateDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增部门</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <PageCard>
      <table class="app-table unified-list-page__table">
        <thead>
          <tr>
            <th>部门编码</th>
            <th>部门名称</th>
            <th>厂区</th>
            <th>区域</th>
            <th>负责人</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="7" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>{{ record.deptCode }}</td>
            <td>
              <div class="unified-list-page__name-cell">
                <strong>{{ record.deptName }}</strong>
                <span>{{ record.deptCode }}</span>
              </div>
            </td>
            <td>{{ record.factoryName || "-" }}</td>
            <td>{{ record.zoneName || "-" }}</td>
            <td>{{ record.leader || "-" }}</td>
            <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
            <td>
              <div class="table-actions">
                <button
                  v-permission="'basic:dept:update'"
                  class="app-button app-button--secondary master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="openEditDialog(record)"
                >
                  <el-icon><EditPen /></el-icon>
                  <span>编辑</span>
                </button>
                <button
                  v-permission="'basic:dept:update'"
                  class="app-button app-button--warning master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="handleToggleStatus(record)"
                >
                  <el-icon><SwitchButton /></el-icon>
                  <span>{{ record.status === "enabled" ? "停用" : "启用" }}</span>
                </button>
                <button
                  v-permission="'basic:dept:delete'"
                  class="app-button app-button--danger master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
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

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑部门' : '新增部门'" width="620px">
      <el-form ref="formRef" :model="formState" :rules="rules" label-width="90px">
        <el-form-item label="部门编码" prop="deptCode">
          <el-input
            v-model="formState.deptCode"
            :placeholder="editingId ? '' : '系统自动生成'"
            disabled
          />
        </el-form-item>
        <el-form-item label="部门名称" prop="deptName">
          <el-input v-model="formState.deptName" />
        </el-form-item>
        <el-form-item label="上级部门">
          <el-select v-model="formState.parentId" clearable style="width: 100%">
            <el-option v-for="item in parentOptions" :key="item.id" :label="item.deptName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属厂区">
          <el-select v-model="formState.factoryId" clearable style="width: 100%">
            <el-option v-for="item in factories" :key="item.id" :label="item.factoryName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属区域">
          <el-select v-model="formState.zoneId" clearable style="width: 100%">
            <el-option v-for="item in zoneOptions" :key="item.id" :label="item.zoneName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="负责人">
          <el-input v-model="formState.leader" />
        </el-form-item>
        <el-form-item label="联系电话">
          <el-input v-model="formState.phone" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="formState.sort" :min="0" style="width: 100%" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="formState.status" style="width: 100%">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formState.remark" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <button class="app-button app-button--secondary" @click="dialogVisible = false">取消</button>
        <button class="app-button app-button--primary" @click="handleSubmit">保存</button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.master-data-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.master-data-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.master-data-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.master-data-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.master-data-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 180px 180px minmax(260px, 1fr);
  gap: 10px;
  align-items: end;
}

.master-data-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.master-data-page__filters-card :deep(.app-field select),
.master-data-page__filters-card :deep(.app-field input) {
  height: 36px;
  font-size: 13px;
}

.master-data-page__filters-card .master-data-page__button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.master-data-page__table-button {
  min-height: 30px;
  padding: 0 10px;
  font-size: 12px;
  gap: 4px;
  white-space: nowrap;
}

.master-data-page__table-button :deep(.el-icon) {
  font-size: 12px;
}

@media (max-width: 1100px) {
  .master-data-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .master-data-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr 1fr;
  }
}

@media (max-width: 768px) {
  .master-data-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
