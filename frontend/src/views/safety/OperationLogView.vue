<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { Download, RefreshRight, Search, View } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import {
  exportOperationLogsApi,
  getOperationLogDetailApi,
  listOperationLogsApi,
} from "../../api/operation-log"
import type {
  OperationLogDetail,
  OperationLogRecord,
} from "../../types/operation-log"

const loading = ref(false)
const exporting = ref(false)
const detailLoading = ref(false)
const detailVisible = ref(false)
const records = ref<OperationLogRecord[]>([])
const detailRecord = ref<OperationLogDetail | null>(null)

const pagination = reactive({
  page: 1,
  pageSize: 30,
  total: 0,
})

const queryForm = reactive({
  username: "",
  keyword: "",
  operationType: "",
  resultStatus: "",
  range: [] as string[],
})

const operationTypeOptions = [
  { label: "全部类型", value: "" },
  { label: "登录", value: "登录" },
  { label: "退出", value: "退出" },
  { label: "浏览页面", value: "浏览页面" },
  { label: "打开页面", value: "打开页面" },
  { label: "菜单切换", value: "菜单切换" },
  { label: "标签页切换", value: "标签页切换" },
  { label: "页面刷新", value: "页面刷新" },
  { label: "查询", value: "查询" },
  { label: "新增", value: "新增" },
  { label: "编辑", value: "编辑" },
  { label: "删除", value: "删除" },
  { label: "导出", value: "导出" },
  { label: "预览", value: "预览" },
  { label: "回放", value: "回放" },
  { label: "设备配置", value: "设备配置" },
  { label: "按钮点击", value: "按钮点击" },
  { label: "分页切换", value: "分页切换" },
  { label: "筛选重置", value: "筛选重置" },
]

const resultStatusOptions = [
  { label: "全部结果", value: "" },
  { label: "成功", value: "success" },
  { label: "失败", value: "failed" },
]

const metrics = computed(() => {
  const successCount = records.value.filter((item) => item.resultStatus === "success").length
  const failedCount = records.value.filter((item) => item.resultStatus === "failed").length
  const avgDuration = records.value.length
    ? Math.round(records.value.reduce((sum, item) => sum + Number(item.durationMs || 0), 0) / records.value.length)
    : 0
  return [
    { label: "今日日志总数", value: pagination.total, accent: "primary" },
    { label: "当前页成功", value: successCount, accent: "success" },
    { label: "当前页失败", value: failedCount, accent: "danger" },
    { label: "平均耗时(ms)", value: avgDuration, accent: "info" },
  ]
})

const pad = (value: number) => String(value).padStart(2, "0")

const formatDateTimeInput = (date: Date) =>
  `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(
    date.getMinutes(),
  )}:${pad(date.getSeconds())}`

const buildTodayRange = () => {
  const now = new Date()
  const start = new Date(now)
  start.setHours(0, 0, 0, 0)
  return [formatDateTimeInput(start), formatDateTimeInput(now)]
}

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString("zh-CN", { hour12: false })
}

const getResultText = (value: string) => {
  if (value === "success") return "成功"
  if (value === "failed") return "失败"
  return value || "-"
}

const getResultTone = (value: string) => {
  if (value === "success") return "success"
  if (value === "failed") return "danger"
  return "default"
}

const buildParams = () => {
  const [startAt, endAt] = queryForm.range ?? []
  return {
    username: queryForm.username || undefined,
    keyword: queryForm.keyword || undefined,
    operation_type: queryForm.operationType || undefined,
    result_status: queryForm.resultStatus || undefined,
    start_at: startAt || undefined,
    end_at: endAt || undefined,
    page: pagination.page,
    page_size: pagination.pageSize,
  }
}

const loadRecords = async () => {
  loading.value = true
  try {
    const result = await listOperationLogsApi(buildParams())
    records.value = result.items
    pagination.total = result.total
    pagination.page = result.page
    pagination.pageSize = result.pageSize
  } finally {
    loading.value = false
  }
}

const handleSearch = async () => {
  pagination.page = 1
  await loadRecords()
}

const handleResetQuery = async () => {
  queryForm.username = ""
  queryForm.keyword = ""
  queryForm.operationType = ""
  queryForm.resultStatus = ""
  queryForm.range = buildTodayRange()
  pagination.page = 1
  await loadRecords()
}

const handlePageChange = async (page: number) => {
  pagination.page = page
  await loadRecords()
}

const handleExport = async () => {
  exporting.value = true
  try {
    const [startAt, endAt] = queryForm.range ?? []
    await exportOperationLogsApi({
      username: queryForm.username || undefined,
      keyword: queryForm.keyword || undefined,
      operation_type: queryForm.operationType || undefined,
      result_status: queryForm.resultStatus || undefined,
      start_at: startAt || undefined,
      end_at: endAt || undefined,
    })
  } finally {
    exporting.value = false
  }
}

const openDetail = async (record: OperationLogRecord) => {
  detailVisible.value = true
  detailLoading.value = true
  try {
    detailRecord.value = await getOperationLogDetailApi(record.id)
  } finally {
    detailLoading.value = false
  }
}

const displayText = (value: unknown) => {
  if (Array.isArray(value)) return value.length ? value.join("、") : "-"
  if (value === null || value === undefined || value === "") return "-"
  return String(value)
}

onMounted(async () => {
  queryForm.range = buildTodayRange()
  await loadRecords()
})
</script>

<template>
  <div class="operation-log-page unified-list-page">
    <PageCard class="operation-log-page__filters-card">
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <ClearableSearchInput v-model="queryForm.username" placeholder="操作账号 / 操作人" @clear="handleSearch" />
        </div>
        <div class="app-field">
          <ClearableSearchInput v-model="queryForm.keyword" placeholder="菜单 / 按钮 / 对象 / IP 关键字" @clear="handleSearch" />
        </div>
        <div class="app-field">
          <select v-model="queryForm.operationType" v-refresh-on-empty="handleSearch">
            <option v-for="item in operationTypeOptions" :key="item.value || 'all-type'" :value="item.value">
              {{ item.label }}
            </option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.resultStatus" v-refresh-on-empty="handleSearch">
            <option v-for="item in resultStatusOptions" :key="item.value || 'all-result'" :value="item.value">
              {{ item.label }}
            </option>
          </select>
        </div>
        <div class="app-field operation-log-page__range">
          <el-date-picker
            v-model="queryForm.range"
            type="datetimerange"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DDTHH:mm:ss"
            style="width: 100%"
          />
        </div>
        <template #actions>
          <button class="app-button app-button--primary operation-log-page__button" @click="handleSearch">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary operation-log-page__button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'log:operation:export'"
            class="app-button app-button--success operation-log-page__button"
            :disabled="exporting"
            @click="handleExport"
          >
            <el-icon><Download /></el-icon>
            <span>{{ exporting ? "导出中..." : "导出日志" }}</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <section class="operation-log-page__summary unified-list-page__summary">
      <article
        v-for="card in metrics"
        :key="card.label"
        class="operation-log-page__metric unified-list-page__metric"
        :class="[`operation-log-page__metric--${card.accent}`, `unified-list-page__metric--${card.accent}`]"
      >
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </section>

    <PageCard :description="`默认展示今日全部操作日志，当前共 ${pagination.total} 条。`">
      <table class="app-table unified-list-page__table">
        <thead>
          <tr>
            <th style="width: 168px">操作时间</th>
            <th style="width: 140px">操作人</th>
            <th style="width: 220px">菜单页面</th>
            <th style="width: 120px">按钮操作</th>
            <th style="width: 200px">操作对象</th>
            <th style="width: 100px">操作类型</th>
            <th style="width: 120px">IP</th>
            <th style="width: 80px">结果</th>
            <th style="width: 80px">耗时</th>
            <th style="width: 80px">详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="10" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>{{ formatDateTime(record.operationTime) }}</td>
            <td>
              <div class="operation-log-page__stack">
                <strong>{{ record.operatorName || "-" }}</strong>
                <span>{{ record.operatorUsername || "-" }}</span>
              </div>
            </td>
            <td>
              <div class="operation-log-page__stack">
                <strong>{{ record.menuName || "-" }}</strong>
                <span>{{ record.pageTitle || "-" }}</span>
              </div>
            </td>
            <td>{{ record.actionName || "-" }}</td>
            <td>{{ record.objectName || "-" }}</td>
            <td>{{ record.operationType || "-" }}</td>
            <td>{{ record.clientIp || "-" }}</td>
            <td><StatusTag :text="getResultText(record.resultStatus)" :tone="getResultTone(record.resultStatus)" /></td>
            <td>{{ record.durationMs || 0 }}ms</td>
            <td>
              <button class="app-button app-button--secondary operation-log-page__detail-button" @click="openDetail(record)">
                <el-icon><View /></el-icon>
                <span>详情</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="operation-log-page__pagination">
        <el-pagination
          layout="total, prev, pager, next"
          :current-page="pagination.page"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          @current-change="handlePageChange"
        />
      </div>
    </PageCard>

    <el-dialog v-model="detailVisible" title="操作日志详情" width="980px" class="operation-log-page__dialog">
      <div v-loading="detailLoading" class="operation-log-page__detail" element-loading-text="详情加载中...">
        <template v-if="detailRecord">
          <section class="operation-log-page__detail-grid">
            <div><label>日志ID</label><span>{{ detailRecord.id }}</span></div>
            <div><label>操作时间</label><span>{{ formatDateTime(detailRecord.operationTime) }}</span></div>
            <div><label>操作账号</label><span>{{ displayText(detailRecord.operatorUsername) }}</span></div>
            <div><label>操作人姓名</label><span>{{ displayText(detailRecord.operatorRealName) }}</span></div>
            <div><label>所属角色</label><span>{{ displayText(detailRecord.roleNames) }}</span></div>
            <div><label>客户端IP</label><span>{{ displayText(detailRecord.clientIp) }}</span></div>
            <div><label>IP归属地</label><span>{{ displayText(detailRecord.ipLocation) }}</span></div>
            <div><label>操作系统</label><span>{{ displayText(detailRecord.osName) }}</span></div>
            <div><label>浏览器UA</label><span>{{ displayText(detailRecord.userAgent) }}</span></div>
            <div><label>菜单编码</label><span>{{ displayText(detailRecord.menuCode) }}</span></div>
            <div><label>菜单名称</label><span>{{ displayText(detailRecord.menuName) }}</span></div>
            <div><label>页面路由</label><span>{{ displayText(detailRecord.routePath) }}</span></div>
            <div><label>页面标题</label><span>{{ displayText(detailRecord.pageTitle) }}</span></div>
            <div><label>组件标识</label><span>{{ displayText(detailRecord.pageComponent) }}</span></div>
            <div><label>按钮标识</label><span>{{ displayText(detailRecord.actionCode) }}</span></div>
            <div><label>按钮名称</label><span>{{ displayText(detailRecord.actionName) }}</span></div>
            <div><label>操作类型</label><span>{{ displayText(detailRecord.operationType) }}</span></div>
            <div><label>对象类型</label><span>{{ displayText(detailRecord.objectType) }}</span></div>
            <div><label>对象ID</label><span>{{ displayText(detailRecord.objectId) }}</span></div>
            <div><label>对象名称</label><span>{{ displayText(detailRecord.objectName) }}</span></div>
            <div><label>点位地址</label><span>{{ displayText(detailRecord.objectLocation) }}</span></div>
            <div><label>请求方法</label><span>{{ displayText(detailRecord.requestMethod) }}</span></div>
            <div><label>请求路径</label><span>{{ displayText(detailRecord.requestPath) }}</span></div>
            <div><label>请求查询</label><span>{{ displayText(detailRecord.requestQuery) }}</span></div>
            <div><label>操作结果</label><span>{{ getResultText(detailRecord.resultStatus) }}</span></div>
            <div><label>响应状态</label><span>{{ detailRecord.responseStatus }}</span></div>
            <div><label>耗时</label><span>{{ detailRecord.durationMs }}ms</span></div>
            <div><label>存储分区</label><span>{{ displayText(detailRecord.storagePartition) }}</span></div>
            <div><label>留存周期</label><span>{{ detailRecord.retentionDays }}天</span></div>
          </section>

          <section class="operation-log-page__detail-block">
            <h4>原始请求参数</h4>
            <pre>{{ displayText(detailRecord.requestParams) }}</pre>
          </section>

          <section class="operation-log-page__detail-block">
            <h4>设备点位信息</h4>
            <pre>{{ displayText(detailRecord.devicePointInfo) }}</pre>
          </section>

          <section class="operation-log-page__detail-block">
            <h4>修改前对比</h4>
            <pre>{{ displayText(detailRecord.beforeSnapshot) }}</pre>
          </section>

          <section class="operation-log-page__detail-block">
            <h4>修改后对比</h4>
            <pre>{{ displayText(detailRecord.afterSnapshot) }}</pre>
          </section>

          <section class="operation-log-page__detail-block">
            <h4>错误堆栈</h4>
            <pre>{{ displayText(detailRecord.errorStack) }}</pre>
          </section>

          <section class="operation-log-page__detail-block">
            <h4>扩展字段</h4>
            <pre>{{ displayText(detailRecord.extraJson) }}</pre>
          </section>
        </template>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.operation-log-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.operation-log-page__summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.operation-log-page__metric {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border-radius: 12px;
  border: 1px solid #dbe6f0;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.operation-log-page__metric span {
  color: #667b91;
  font-size: 13px;
}

.operation-log-page__metric strong {
  color: #0e2b4b;
  font-size: 30px;
}

.operation-log-page__metric--success strong {
  color: #1d9b52;
}

.operation-log-page__metric--danger strong {
  color: #d64f5a;
}

.operation-log-page__metric--info strong {
  color: #1d7ad9;
}

.operation-log-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.operation-log-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.operation-log-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 180px minmax(240px, 1fr) 150px 140px minmax(340px, 1fr);
  gap: 10px;
  align-items: end;
}

.operation-log-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  align-items: end;
}

.operation-log-page__button,
.operation-log-page__detail-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.operation-log-page__detail-button {
  min-height: 34px;
  padding: 0 12px;
  font-size: 12px;
}

.operation-log-page__stack {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.operation-log-page__stack strong {
  color: #102941;
  font-size: 13px;
}

.operation-log-page__stack span {
  color: #74889d;
  font-size: 12px;
}

.operation-log-page__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

.operation-log-page__detail {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 240px;
}

.operation-log-page__detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.operation-log-page__detail-grid div,
.operation-log-page__detail-block {
  border-radius: 14px;
  border: 1px solid #dfe7f0;
  background: #f8fbff;
}

.operation-log-page__detail-grid div {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
}

.operation-log-page__detail-grid label {
  color: #6f8499;
  font-size: 12px;
}

.operation-log-page__detail-grid span {
  color: #13304b;
  font-size: 13px;
  line-height: 1.5;
  word-break: break-all;
}

.operation-log-page__detail-block {
  padding: 12px 14px;
}

.operation-log-page__detail-block h4 {
  margin: 0 0 10px;
  color: #12324e;
  font-size: 14px;
}

.operation-log-page__detail-block pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  color: #27425d;
  font-size: 12px;
  line-height: 1.7;
}

@media (max-width: 1280px) {
  .operation-log-page__summary {
    grid-template-columns: 1fr 1fr;
  }

  .operation-log-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 960px) {
  .operation-log-page__summary,
  .operation-log-page__detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
