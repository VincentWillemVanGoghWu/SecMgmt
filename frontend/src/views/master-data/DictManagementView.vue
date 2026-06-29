<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import { Delete, EditPen, Plus, RefreshRight, Search, SwitchButton } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import {
  createDictItemApi,
  createDictTypeApi,
  deleteDictItemApi,
  deleteDictTypeApi,
  listDictTypesApi,
  updateDictItemApi,
  updateDictItemStatusApi,
  updateDictTypeApi,
  updateDictTypeStatusApi,
} from "../../api/master-data"
import type { DictItemRecord, DictTypeRecord } from "../../types/master-data"

const loading = ref(false)
const dictTypeDialogVisible = ref(false)
const dictItemDialogVisible = ref(false)
const editingTypeId = ref<number | null>(null)
const editingItemId = ref<number | null>(null)
const selectedTypeId = ref<number | null>(null)
const typeFormRef = ref<FormInstance>()
const itemFormRef = ref<FormInstance>()

const statusOptions = [
  { label: "启用", value: "enabled" },
  { label: "停用", value: "disabled" },
]

const queryForm = reactive({
  keyword: "",
  status: "",
})

const typeForm = reactive<Omit<DictTypeRecord, "id" | "items">>({
  dictCode: "",
  dictName: "",
  status: "enabled",
  remark: "",
})

const itemForm = reactive<Omit<DictItemRecord, "id">>({
  dictTypeId: 0,
  itemLabel: "",
  itemValue: "",
  itemSort: 0,
  isDefault: false,
  status: "enabled",
  remark: "",
})

const typeRules: FormRules = {
  dictCode: [{ required: true, message: "请输入字典类型编码", trigger: "blur" }],
  dictName: [{ required: true, message: "请输入字典类型名称", trigger: "blur" }],
}

const itemRules: FormRules = {
  dictTypeId: [{ required: true, message: "请选择字典类型", trigger: "change" }],
  itemLabel: [{ required: true, message: "请输入字典项名称", trigger: "blur" }],
  itemValue: [{ required: true, message: "请输入字典项值", trigger: "blur" }],
}

const dictTypes = ref<DictTypeRecord[]>([])

const selectedType = computed(
  () => dictTypes.value.find((item) => item.id === selectedTypeId.value) ?? dictTypes.value[0] ?? null,
)

const getStatusTone = (status: string) => (status === "enabled" ? "success" : "default")
const getStatusText = (status: string) => (status === "enabled" ? "启用" : "停用")

const loadDictTypes = async () => {
  loading.value = true
  try {
    dictTypes.value = await listDictTypesApi({
      keyword: queryForm.keyword || undefined,
      status: queryForm.status || undefined,
    })
    if (!selectedType.value && dictTypes.value.length > 0) {
      selectedTypeId.value = dictTypes.value[0].id
    }
  } finally {
    loading.value = false
  }
}

const resetTypeForm = () => {
  editingTypeId.value = null
  typeForm.dictCode = ""
  typeForm.dictName = ""
  typeForm.status = "enabled"
  typeForm.remark = ""
}

const resetItemForm = () => {
  editingItemId.value = null
  itemForm.dictTypeId = selectedType.value?.id ?? 0
  itemForm.itemLabel = ""
  itemForm.itemValue = ""
  itemForm.itemSort = 0
  itemForm.isDefault = false
  itemForm.status = "enabled"
  itemForm.remark = ""
}

const openCreateTypeDialog = () => {
  resetTypeForm()
  dictTypeDialogVisible.value = true
}

const openEditTypeDialog = (record: DictTypeRecord) => {
  editingTypeId.value = record.id
  typeForm.dictCode = record.dictCode
  typeForm.dictName = record.dictName
  typeForm.status = record.status
  typeForm.remark = record.remark ?? ""
  dictTypeDialogVisible.value = true
}

const openCreateItemDialog = () => {
  resetItemForm()
  dictItemDialogVisible.value = true
}

const openEditItemDialog = (record: DictItemRecord) => {
  editingItemId.value = record.id
  itemForm.dictTypeId = record.dictTypeId
  itemForm.itemLabel = record.itemLabel
  itemForm.itemValue = record.itemValue
  itemForm.itemSort = record.itemSort
  itemForm.isDefault = record.isDefault
  itemForm.status = record.status
  itemForm.remark = record.remark ?? ""
  dictItemDialogVisible.value = true
}

const submitType = async () => {
  const valid = await typeFormRef.value?.validate().catch(() => false)
  if (!valid) {
    return
  }

  if (editingTypeId.value) {
    await updateDictTypeApi(editingTypeId.value, { ...typeForm })
    ElMessage.success("字典类型更新成功")
  } else {
    await createDictTypeApi({ ...typeForm })
    ElMessage.success("字典类型创建成功")
  }
  dictTypeDialogVisible.value = false
  await loadDictTypes()
}

const submitItem = async () => {
  const valid = await itemFormRef.value?.validate().catch(() => false)
  if (!valid) {
    return
  }

  if (editingItemId.value) {
    await updateDictItemApi(editingItemId.value, { ...itemForm })
    ElMessage.success("字典项更新成功")
  } else {
    await createDictItemApi({ ...itemForm })
    ElMessage.success("字典项创建成功")
  }
  dictItemDialogVisible.value = false
  await loadDictTypes()
  selectedTypeId.value = itemForm.dictTypeId
}

const handleDeleteType = async (record: DictTypeRecord) => {
  await ElMessageBox.confirm(`确认删除字典类型“${record.dictName}”吗？`, "删除确认", { type: "warning" })
  await deleteDictTypeApi(record.id)
  ElMessage.success("字典类型删除成功")
  if (selectedTypeId.value === record.id) {
    selectedTypeId.value = null
  }
  await loadDictTypes()
}

const handleDeleteItem = async (record: DictItemRecord) => {
  await ElMessageBox.confirm(`确认删除字典项“${record.itemLabel}”吗？`, "删除确认", { type: "warning" })
  await deleteDictItemApi(record.id)
  ElMessage.success("字典项删除成功")
  await loadDictTypes()
}

const handleToggleTypeStatus = async (record: DictTypeRecord) => {
  const nextStatus = record.status === "enabled" ? "disabled" : "enabled"
  await updateDictTypeStatusApi(record.id, { status: nextStatus })
  ElMessage.success("字典类型状态更新成功")
  await loadDictTypes()
}

const handleToggleItemStatus = async (record: DictItemRecord) => {
  const nextStatus = record.status === "enabled" ? "disabled" : "enabled"
  await updateDictItemStatusApi(record.id, { status: nextStatus })
  ElMessage.success("字典项状态更新成功")
  await loadDictTypes()
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.status = ""
  await loadDictTypes()
}

onMounted(async () => {
  await loadDictTypes()
})
</script>

<template>
  <div class="master-data-page unified-list-page">
    <PageCard class="master-data-page__filters-card">
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <ClearableSearchInput v-model="queryForm.keyword" placeholder="输入字典名称或编码" @clear="loadDictTypes" />
        </div>
        <div class="app-field">
          <select v-model="queryForm.status" v-refresh-on-empty="loadDictTypes">
            <option value="">状态</option>
            <option v-for="item in statusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <template #actions>
          <button class="app-button app-button--primary master-data-page__button unified-list-page__button unified-list-page__search-button" @click="loadDictTypes">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary master-data-page__button unified-list-page__button unified-list-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'basic:dict:create'"
            class="app-button app-button--success master-data-page__button unified-list-page__button unified-list-page__search-button"
            @click="openCreateTypeDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增类型</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <div class="master-data-grid">
      <PageCard title="字典类型列表" :description="`当前共 ${dictTypes.length} 个字典类型`">
        <table class="app-table unified-list-page__table">
          <thead>
            <tr>
              <th width="120">字典编码</th>
              <th width="180">字典名称</th>
              <th width="100">状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!dictTypes.length">
              <td colspan="4" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
            </tr>
            <tr
              v-for="record in dictTypes"
              :key="record.id"
              :class="{ 'app-table__row--active': selectedType?.id === record.id }"
              @click="selectedTypeId = record.id"
            >
              <td>{{ record.dictCode }}</td>
              <td>
                <div class="unified-list-page__name-cell">
                  <strong>{{ record.dictName }}</strong>
                  <span>{{ record.dictCode }}</span>
                </div>
              </td>
              <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
              <td>
                <div class="table-actions">
                  <button
                    v-permission="'basic:dict:update'"
                    class="app-button app-button--secondary master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click.stop="openEditTypeDialog(record)"
                  >
                    <el-icon><EditPen /></el-icon>
                    <span>编辑</span>
                  </button>
                  <button
                    v-permission="'basic:dict:update'"
                    class="app-button app-button--warning master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click.stop="handleToggleTypeStatus(record)"
                  >
                    <el-icon><SwitchButton /></el-icon>
                    <span>{{ record.status === "enabled" ? "停用" : "启用" }}</span>
                  </button>
                  <button
                    v-permission="'basic:dict:delete'"
                    class="app-button app-button--danger master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click.stop="handleDeleteType(record)"
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

      <PageCard
        :title="selectedType ? `${selectedType.dictName} / 字典项` : '字典项列表'"
        :description="selectedType ? `当前共 ${selectedType.items.length} 个字典项` : '请先选择左侧字典类型'"
      >
        <template #headerActions>
          <button
            v-permission="'basic:dict:create'"
            class="app-button app-button--primary master-data-page__button unified-list-page__button unified-list-page__search-button"
            :disabled="!selectedType"
            @click="openCreateItemDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增字典项</span>
          </button>
        </template>
        <table class="app-table unified-list-page__table">
          <thead>
            <tr>
              <th width="30">字典项</th>
              <th width="50">值</th>
              <th width="30">默认</th>
              <th width="30">状态</th>    
              <th width="30">排序</th>        
              <th width="100">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!selectedType?.items.length">
              <td colspan="6" class="app-table__empty">暂无字典项</td>
            </tr>
            <tr v-for="record in selectedType?.items ?? []" :key="record.id">
              <td>
                <div class="unified-list-page__name-cell">
                  <strong>{{ record.itemLabel }}</strong>
                  <span>{{ record.itemValue }}</span>
                </div>
              </td>
              <td>{{ record.itemValue }}</td>
              <td>{{ record.isDefault ? "是" : "否" }}</td>
              <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
              <td>{{ record.itemSort }}</td>
              <td>
                <div class="table-actions">
                  <button
                    v-permission="'basic:dict:update'"
                  class="app-button app-button--secondary master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="openEditItemDialog(record)"
                  >
                    <el-icon><EditPen /></el-icon>
                    <span>编辑</span>
                  </button>
                  <button
                    v-permission="'basic:dict:update'"
                  class="app-button app-button--warning master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="handleToggleItemStatus(record)"
                  >
                    <el-icon><SwitchButton /></el-icon>
                    <span>{{ record.status === "enabled" ? "停用" : "启用" }}</span>
                  </button>
                  <button
                    v-permission="'basic:dict:delete'"
                  class="app-button app-button--danger master-data-page__button master-data-page__table-button unified-list-page__button unified-list-page__table-button"
                    @click="handleDeleteItem(record)"
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
    </div>

    <el-dialog v-model="dictTypeDialogVisible" :title="editingTypeId ? '编辑字典类型' : '新增字典类型'" width="560px">
      <el-form ref="typeFormRef" :model="typeForm" :rules="typeRules" label-width="100px">
        <el-form-item label="类型编码" prop="dictCode">
          <el-input v-model="typeForm.dictCode" />
        </el-form-item>
        <el-form-item label="类型名称" prop="dictName">
          <el-input v-model="typeForm.dictName" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="typeForm.status" style="width: 100%">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="typeForm.remark" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <button class="app-button app-button--secondary" @click="dictTypeDialogVisible = false">取消</button>
        <button class="app-button app-button--primary" @click="submitType">保存</button>
      </template>
    </el-dialog>

    <el-dialog v-model="dictItemDialogVisible" :title="editingItemId ? '编辑字典项' : '新增字典项'" width="560px">
      <el-form ref="itemFormRef" :model="itemForm" :rules="itemRules" label-width="100px">
        <el-form-item label="所属类型" prop="dictTypeId">
          <el-select v-model="itemForm.dictTypeId" style="width: 100%">
            <el-option v-for="item in dictTypes" :key="item.id" :label="item.dictName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="字典项名称" prop="itemLabel">
          <el-input v-model="itemForm.itemLabel" />
        </el-form-item>
        <el-form-item label="字典项值" prop="itemValue">
          <el-input v-model="itemForm.itemValue" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="itemForm.itemSort" :min="0" style="width: 100%" />
        </el-form-item>
        <el-form-item label="默认项">
          <el-switch v-model="itemForm.isDefault" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="itemForm.status" style="width: 100%">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="itemForm.remark" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <button class="app-button app-button--secondary" @click="dictItemDialogVisible = false">取消</button>
        <button class="app-button app-button--primary" @click="submitItem">保存</button>
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

.master-data-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr);
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
  grid-template-columns: 180px minmax(260px, 1fr);
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

.master-data-page__filters-card .master-data-page__button,
.master-data-grid .master-data-page__button {
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
  .master-data-grid,
  .master-data-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .master-data-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 768px) {
  .master-data-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
