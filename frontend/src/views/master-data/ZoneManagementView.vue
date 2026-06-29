<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import { Delete, EditPen, Plus, RefreshRight, Search, SwitchButton } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import {
  createZoneApi,
  deleteZoneApi,
  listFactoriesApi,
  listZonesApi,
  updateZoneApi,
  updateZoneStatusApi,
} from "../../api/master-data"
import type { FactoryRecord, ZoneRecord } from "../../types/master-data"

const loading = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const factories = ref<FactoryRecord[]>([])

const statusOptions = [
  { label: "启用", value: "enabled" },
  { label: "停用", value: "disabled" },
]

const queryForm = reactive({
  keyword: "",
  status: "",
  factoryId: "",
})

const formState = reactive<Omit<ZoneRecord, "id" | "factoryName">>({
  factoryId: 0,
  zoneCode: "",
  zoneName: "",
  status: "enabled",
  remark: "",
})

const rules: FormRules = {
  factoryId: [{ required: true, message: "请选择所属厂区", trigger: "change" }],
  zoneName: [{ required: true, message: "请输入区域名称", trigger: "blur" }],
}

const records = ref<ZoneRecord[]>([])
const factoryOptions = computed(() => factories.value)

const getStatusTone = (status: string) => (status === "enabled" ? "success" : "default")
const getStatusText = (status: string) => (status === "enabled" ? "启用" : "停用")

const loadFactoryOptions = async () => {
  factories.value = await listFactoriesApi()
}

const loadRecords = async () => {
  loading.value = true
  try {
    records.value = await listZonesApi({
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
  formState.factoryId = factoryOptions.value[0]?.id ?? 0
  formState.zoneCode = ""
  formState.zoneName = ""
  formState.status = "enabled"
  formState.remark = ""
}

const openCreateDialog = () => {
  resetFormState()
  dialogVisible.value = true
}

const openEditDialog = (record: ZoneRecord) => {
  editingId.value = record.id
  formState.factoryId = record.factoryId
  formState.zoneCode = record.zoneCode
  formState.zoneName = record.zoneName
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
    await updateZoneApi(editingId.value, { ...formState })
    ElMessage.success("区域更新成功")
  } else {
    await createZoneApi({ ...formState })
    ElMessage.success("区域创建成功")
  }

  dialogVisible.value = false
  resetFormState()
  await loadRecords()
}

const handleDelete = async (record: ZoneRecord) => {
  await ElMessageBox.confirm(`确认删除区域“${record.zoneName}”吗？`, "删除确认", { type: "warning" })
  await deleteZoneApi(record.id)
  ElMessage.success("区域删除成功")
  await loadRecords()
}

const handleToggleStatus = async (record: ZoneRecord) => {
  const nextStatus = record.status === "enabled" ? "disabled" : "enabled"
  await updateZoneStatusApi(record.id, { status: nextStatus })
  ElMessage.success("区域状态更新成功")
  await loadRecords()
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.status = ""
  queryForm.factoryId = ""
  await loadRecords()
}

onMounted(async () => {
  await loadFactoryOptions()
  resetFormState()
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
            <option v-for="item in factoryOptions" :key="item.id" :value="String(item.id)">{{ item.factoryName }}</option>
          </select>
        </div>
        <div class="app-field">
          <ClearableSearchInput v-model="queryForm.keyword" placeholder="输入区域名称或编码" @clear="loadRecords" />
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
            v-permission="'basic:zone:create'"
            class="app-button app-button--success master-data-page__button unified-list-page__button unified-list-page__search-button"
            @click="openCreateDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增区域</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <PageCard>
      <table class="app-table unified-list-page__table">
        <thead>
          <tr>
            <th>所属厂区</th>
            <th>区域编码</th>
            <th>区域名称</th>
            <th>状态</th>
            <th>备注</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="6" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>{{ record.factoryName }}</td>
            <td>{{ record.zoneCode }}</td>
            <td>
              <div class="unified-list-page__name-cell">
                <strong>{{ record.zoneName }}</strong>
                <span>{{ record.zoneCode }}</span>
              </div>
            </td>
            <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
            <td>{{ record.remark || "-" }}</td>
            <td>
              <div class="table-actions">
                <button
                  v-permission="'basic:zone:update'"
                  class="app-button app-button--secondary master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="openEditDialog(record)"
                >
                  <el-icon><EditPen /></el-icon>
                  <span>编辑</span>
                </button>
                <button
                  v-permission="'basic:zone:update'"
                  class="app-button app-button--warning master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="handleToggleStatus(record)"
                >
                  <el-icon><SwitchButton /></el-icon>
                  <span>{{ record.status === "enabled" ? "停用" : "启用" }}</span>
                </button>
                <button
                  v-permission="'basic:zone:delete'"
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

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑区域' : '新增区域'" width="560px">
      <el-form ref="formRef" :model="formState" :rules="rules" label-width="90px">
        <el-form-item label="所属厂区" prop="factoryId">
          <el-select v-model="formState.factoryId" style="width: 100%">
            <el-option v-for="item in factoryOptions" :key="item.id" :label="item.factoryName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="区域编码" prop="zoneCode">
          <el-input
            v-model="formState.zoneCode"
            :placeholder="editingId ? '' : '系统自动生成'"
            disabled
          />
        </el-form-item>
        <el-form-item label="区域名称" prop="zoneName">
          <el-input v-model="formState.zoneName" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
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
