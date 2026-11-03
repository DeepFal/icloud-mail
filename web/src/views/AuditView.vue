<template>
  <section
    class="pg pg--fill virtual-list-page"
    aria-labelledby="audit-section-title"
  >
    <header class="pg-bar">
      <h1 id="audit-section-title" class="pg-title">操作记录</h1>
      <div class="pg-bar__end">
        <el-button :icon="Refresh" :loading="loading" @click="loadAuditLogs">刷新</el-button>
      </div>
    </header>

    <div v-if="loading && logs.length === 0" class="data-panel loading-panel">
      <el-skeleton :rows="7" animated role="status" aria-live="polite" aria-busy="true" aria-label="正在加载审计记录" />
    </div>

    <div v-else-if="loadError && logs.length === 0" class="load-failed">
      <RequestAlert :error="loadError" />
      <el-button :icon="Refresh" @click="loadAuditLogs">重新加载</el-button>
    </div>

    <EmptyState
      v-else-if="logs.length === 0"
      title="暂无操作记录"
      description="新的登录与配置操作会显示在这里。"
    />

    <template v-else>
      <RequestAlert
        v-if="loadError"
        :error="loadError"
        closable
        @close="loadError = null"
      />

      <div
        class="data-panel desktop-data-table virtual-list-table"
        :class="{
          'desktop-data-table--force': pageSize > 100 || pageSize === ALL_PAGE_SIZE,
        }"
        :aria-busy="loading"
      >
        <VirtualDataTable
          :columns="auditColumns"
          :data="logs"
          row-key="id"
          fill-height
          :row-height="32"
          :header-height="32"
          :loading="loading"
        >
          <template #cell="{ column, row }">
            <template v-if="column.key === 'createdAt'">
              <span class="pg-num">{{ formatTime(row.createdAt, { seconds: true }) }}</span>
            </template>
            <template v-else-if="column.key === 'username'">
              {{ row.username || "系统" }}
            </template>
            <template v-else-if="column.key === 'action'">
              <span class="audit-action pg-ellipsis" :title="row.action">{{ actionLabel(row.action) }}</span>
            </template>
            <template v-else-if="column.key === 'resource'">
              <span class="pg-ellipsis">{{ resourceLabel(row.resourceType, row.resourceId) }}</span>
            </template>
            <template v-else-if="column.key === 'result'">
              <span class="pg-dot" :class="`is-${resultMeta(row.result).type}`">{{ resultMeta(row.result).label }}</span>
            </template>
            <template v-else-if="column.key === 'requestId'">
              <code class="audit-request-id pg-ellipsis" :title="row.requestId || undefined" translate="no">{{ row.requestId || "—" }}</code>
            </template>
          </template>
        </VirtualDataTable>
      </div>

      <div v-if="pageSize <= 100" class="mobile-record-list" :aria-busy="loading">
        <article v-for="log in logs" :key="log.id" class="mobile-record">
          <header class="mobile-record__header">
            <div class="primary-stack">
              <strong :title="log.action">{{ actionLabel(log.action) }}</strong>
              <small>{{ formatTime(log.createdAt, { seconds: true }) }}</small>
            </div>
            <span class="pg-dot" :class="`is-${resultMeta(log.result).type}`">{{ resultMeta(log.result).label }}</span>
          </header>
          <dl class="mobile-kv-list">
            <div>
              <dt>管理员</dt>
              <dd>{{ log.username || "系统" }}</dd>
            </div>
            <div>
              <dt>对象</dt>
              <dd>{{ resourceLabel(log.resourceType, log.resourceId) }}</dd>
            </div>
            <div>
              <dt>请求编号</dt>
              <dd><code class="audit-request-id" translate="no">{{ log.requestId || "—" }}</code></dd>
            </div>
          </dl>
        </article>
      </div>

      <ListPagination
        :page="currentPage"
        :page-size="pageSize"
        :total="total"
        :loading="loading"
        aria-label="操作记录分页"
        @change="handlePageChange"
        @size-change="handlePageSizeChange"
      />
    </template>
  </section>
</template>

<script setup>
import { Refresh } from "@element-plus/icons-vue";
import { onBeforeUnmount, onMounted, ref } from "vue";

import { getAllAuditLogs, getAuditLogs } from "../api/admin.js";
import EmptyState from "../components/EmptyState.vue";
import ListPagination from "../components/ListPagination.vue";
import RequestAlert from "../components/RequestAlert.vue";
import VirtualDataTable from "../components/VirtualDataTable.vue";
import "../styles/pages.css";
import { createLatestRequestGate } from "../utils/asyncState.js";
import { formatTime } from "../utils/format.js";
import { auditActionLabel } from "../utils/audit.js";
import {
  ALL_PAGE_SIZE,
  DEFAULT_PAGE_SIZE,
  normalizePageSize,
} from "../utils/pagination.js";

const resourceLabels = {
  admin: "管理员",
  account: "主号",
  alias: "隐私地址",
  mail_group: "邮箱分组",
  pool: "邮箱池",
  pool_client: "邮箱池项目",
  pool_request: "邮箱领取请求",
  pool_lease: "邮箱租约",
};

const resultLabels = {
  success: { label: "成功", type: "success" },
  failed: { label: "失败", type: "danger" },
};

const pageSize = ref(DEFAULT_PAGE_SIZE);
const auditColumns = [
  { key: "createdAt", title: "时间", width: 176, flexGrow: 1 },
  { key: "username", title: "管理员", width: 130, flexGrow: 1 },
  { key: "action", title: "操作", width: 142, flexGrow: 1 },
  { key: "resource", title: "对象", width: 150, flexGrow: 1 },
  { key: "result", title: "结果", width: 92 },
  { key: "requestId", title: "请求编号", width: 180, flexGrow: 1 },
];
const logs = ref([]);
const currentPage = ref(1);
const total = ref(0);
const loading = ref(false);
const loadError = ref(null);
const loadGate = createLatestRequestGate();
let listAbortController = null;
let viewActive = true;

function actionLabel(action) {
  return auditActionLabel(action);
}

function resourceLabel(type, id) {
  const typeLabel = Object.hasOwn(resourceLabels, type) ? resourceLabels[type] : type || "系统";
  return id ? `${typeLabel} #${id}` : typeLabel;
}

function resultMeta(result) {
  return resultLabels[result] || {
    label: result || "未知",
    type: "info",
  };
}

async function loadAuditLogs() {
  const page = currentPage.value;
  const selectedPageSize = pageSize.value;
  const requestKey = `${page}\u0000${selectedPageSize}`;
  const ticket = loadGate.begin(requestKey);
  listAbortController?.abort();
  const abortController = new AbortController();
  listAbortController = abortController;
  loading.value = true;
  loadError.value = null;
  try {
    const result = selectedPageSize === ALL_PAGE_SIZE
      ? { items: await getAllAuditLogs({ signal: abortController.signal }) }
      : await getAuditLogs({
          limit: selectedPageSize,
          offset: (page - 1) * selectedPageSize,
          signal: abortController.signal,
        });
    if (
      !viewActive ||
      !loadGate.isCurrent(ticket, `${currentPage.value}\u0000${pageSize.value}`)
    ) return;
    const nextTotal = Math.max(0, Number(result?.total) || 0);
    const nextItems = Array.isArray(result?.items) ? result.items : [];
    const allItems = selectedPageSize === ALL_PAGE_SIZE;
    const resolvedTotal = allItems ? nextItems.length : nextTotal;
    const lastPage = allItems
      ? 1
      : Math.max(1, Math.ceil(resolvedTotal / selectedPageSize));
    if (!allItems && page > lastPage) {
      currentPage.value = lastPage;
      logs.value = [];
      total.value = resolvedTotal;
      void loadAuditLogs();
      return;
    }
    logs.value = nextItems;
    total.value = resolvedTotal;
  } catch (error) {
    if (
      error?.name !== "AbortError" &&
      viewActive &&
      loadGate.isCurrent(ticket, `${currentPage.value}\u0000${pageSize.value}`)
    ) {
      loadError.value = error;
    }
  } finally {
    if (
      listAbortController === abortController &&
      loadGate.isCurrent(ticket, `${currentPage.value}\u0000${pageSize.value}`)
    ) {
      loading.value = false;
    }
    if (listAbortController === abortController) listAbortController = null;
  }
}

function handlePageChange(page) {
  if (pageSize.value === ALL_PAGE_SIZE) return;
  const nextPage = Math.max(1, Number(page) || 1);
  if (nextPage === currentPage.value) return;
  currentPage.value = nextPage;
  logs.value = [];
  loadError.value = null;
  void loadAuditLogs();
}

function handlePageSizeChange(value) {
  const nextPageSize = normalizePageSize(value);
  if (nextPageSize === pageSize.value) return;
  pageSize.value = nextPageSize;
  currentPage.value = 1;
  logs.value = [];
  total.value = 0;
  loadError.value = null;
  loadGate.invalidate();
  void loadAuditLogs();
}

onMounted(loadAuditLogs);

onBeforeUnmount(() => {
  viewActive = false;
  loadGate.deactivate();
  listAbortController?.abort();
});
</script>

<style scoped>
.audit-action {
  color: var(--text);
  font-weight: 500;
}

.audit-request-id {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
}
</style>
