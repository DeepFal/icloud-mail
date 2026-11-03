<template>
  <section
    class="page-stack virtual-list-page"
    aria-labelledby="accounts-section-title"
  >
    <SectionHeader
      id="accounts-section-title"
      title="邮箱主号"
    >
      <template #actions>
        <el-radio-group :model-value="attentionOnly ? 'attention' : 'all'" size="default" aria-label="主号筛选" @change="setAttentionFilter">
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="attention">需处理</el-radio-button>
        </el-radio-group>
        <el-tooltip content="刷新主号列表" placement="bottom">
          <el-button
            :icon="Refresh"
            circle
            :loading="loading"
            aria-label="刷新主号列表"
            @click="loadAccounts"
          />
        </el-tooltip>
        <el-button type="primary" :icon="Plus" @click="openNewAccount">
          添加主号
        </el-button>
      </template>
    </SectionHeader>

    <div v-if="loading && accounts.length === 0" class="data-panel loading-panel">
      <el-skeleton :rows="5" animated role="status" aria-label="正在加载主号列表" />
    </div>

    <div v-else-if="loadError && accounts.length === 0" class="load-failed">
      <RequestAlert :error="loadError" />
      <el-button :icon="Refresh" @click="loadAccounts">重新加载</el-button>
    </div>

    <EmptyState
      v-else-if="accounts.length === 0 && attentionOnly"
      title="没有需要处理的主号"
      description="所有启用的主号收件和 Apple 连接都正常。"
    />

    <EmptyState
      v-else-if="accounts.length === 0"
      title="还没有主号"
      description="添加一个 iCloud 或自定义邮箱主号及其 IMAP 密码，然后登记或生成邮箱。"
    >
      <el-button type="primary" :icon="Plus" @click="openNewAccount">
        添加第一个主号
      </el-button>
    </EmptyState>

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
        :style="accounts.length <= 10 ? { flex: '0 0 auto', height: `${accounts.length * 56 + 50}px`, minHeight: '0' } : undefined"
        :aria-busy="loading"
      >
        <VirtualDataTable
          :columns="accountColumns"
          :data="accounts"
          row-key="id"
          fill-height
          :loading="loading"
        >
          <template #cell="{ column, row }">
            <template v-if="column.key === 'account'">
              <div class="primary-stack">
                <strong>{{ row.mailboxType === "custom" ? `@${row.emailSuffix}` : row.email }}</strong>
                <small>{{ row.name || "未填写备注" }}</small>
              </div>
            </template>
            <template v-else-if="column.key === 'status'">
              <SyncStatus :item="row" />
            </template>
            <template v-else-if="column.key === 'apple'">
              <AppleConnectionTags :account="row" />
            </template>
            <template v-else-if="column.key === 'attention'">
              <AttentionReasons :account="row" />
            </template>
            <template v-else-if="column.key === 'aliasCount'">
              {{ row.aliasCount }}
            </template>
            <template v-else-if="column.key === 'lastSyncedAt'">
              {{ formatTime(row.lastSyncedAt, { seconds: true }) }}
            </template>
            <template v-else-if="column.key === 'actions'">
              <el-button
                link
                type="primary"
                :icon="Setting"
                @click="openAccount(row.id)"
              >
                管理
              </el-button>
            </template>
          </template>
        </VirtualDataTable>
      </div>

      <div v-if="pageSize <= 100" class="mobile-record-list" :aria-busy="loading">
        <article v-for="account in accounts" :key="account.id" class="mobile-record">
          <header class="mobile-record__header">
            <div class="primary-stack">
              <strong>{{ account.mailboxType === "custom" ? `@${account.emailSuffix}` : account.email }}</strong>
              <small>{{ account.name || "未填写备注" }}</small>
            </div>
            <SyncStatus :item="account" />
          </header>
          <dl class="mobile-kv-list">
            <div v-if="account.apple">
              <dt>Apple 连接</dt>
              <dd><AppleConnectionTags :account="account" /></dd>
            </div>
            <div v-if="attentionReasons(account).length">
              <dt>需处理</dt>
              <dd><AttentionReasons :account="account" /></dd>
            </div>
            <div>
              <dt>隐私地址</dt>
              <dd>{{ account.aliasCount }}</dd>
            </div>
            <div>
              <dt>最近同步</dt>
              <dd>{{ formatTime(account.lastSyncedAt, { seconds: true }) }}</dd>
            </div>
          </dl>
          <footer class="mobile-record__actions">
            <el-button :icon="Setting" @click="openAccount(account.id)">
              管理主号
            </el-button>
          </footer>
        </article>
      </div>

      <ListPagination
        v-if="!attentionOnly"
        :page="currentPage"
        :page-size="pageSize"
        :total="total"
        :loading="loading"
        aria-label="主号列表分页"
        @change="handlePageChange"
        @size-change="handlePageSizeChange"
      />
    </template>
  </section>
</template>

<script setup>
import { Plus, Refresh, Setting } from "@element-plus/icons-vue";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

import { getAccountPage, getAllAccounts } from "../api/admin.js";
import EmptyState from "../components/EmptyState.vue";
import ListPagination from "../components/ListPagination.vue";
import RequestAlert from "../components/RequestAlert.vue";
import SectionHeader from "../components/SectionHeader.vue";
import SyncStatus from "../components/SyncStatus.vue";
import VirtualDataTable from "../components/VirtualDataTable.vue";
import AppleConnectionTags from "../components/AppleConnectionTags.vue";
import AttentionReasons from "../components/AttentionReasons.vue";
import { formatTime } from "../utils/format.js";
import { accountAttentionReasons, accountNeedsAttention } from "../utils/workspaceConnection.js";
import { createLatestRequestGate } from "../utils/asyncState.js";
import { createLiveRefresh } from "../utils/liveRefresh.js";
import {
  ALL_PAGE_SIZE,
  DEFAULT_PAGE_SIZE,
  normalizePageSize,
} from "../utils/pagination.js";

const router = useRouter();
const route = useRoute();
const attentionOnly = computed(() => route.query.attention === "1");
const attentionReasons = accountAttentionReasons;
const pageSize = ref(DEFAULT_PAGE_SIZE);
const accountColumns = [
  { key: "account", title: "主号", width: 250, flexGrow: 3 },
  { key: "status", title: "状态", width: 140, flexGrow: 1 },
  { key: "apple", title: "Apple 连接", width: 220, flexGrow: 2 },
  { key: "attention", title: "需处理", width: 150, flexGrow: 1 },
  { key: "aliasCount", title: "隐私地址", width: 112, align: "center" },
  { key: "lastSyncedAt", title: "最近同步", width: 180, flexGrow: 1 },
  { key: "actions", title: "操作", width: 118, align: "right", fixed: "right" },
];
const accounts = ref([]);
const currentPage = ref(1);
const total = ref(0);
const loading = ref(false);
const loadError = ref(null);
const loadGate = createLatestRequestGate();
let listAbortController = null;
let viewActive = true;

async function loadAccounts({ silent = false } = {}) {
  const page = currentPage.value;
  const selectedPageSize = pageSize.value;
  const requestKey = `${page}\u0000${selectedPageSize}`;
  const ticket = loadGate.begin(requestKey);
  listAbortController?.abort();
  const abortController = new AbortController();
  listAbortController = abortController;
  if (!silent) {
    loading.value = true;
    loadError.value = null;
  }
  try {
    // The attention filter is evaluated locally, so it needs every account.
    const result = selectedPageSize === ALL_PAGE_SIZE || attentionOnly.value
      ? {
          items: await getAllAccounts({ signal: abortController.signal }),
        }
      : await getAccountPage({
          limit: selectedPageSize,
          offset: (page - 1) * selectedPageSize,
          signal: abortController.signal,
        });
    if (
      !viewActive ||
      !loadGate.isCurrent(ticket, `${currentPage.value}\u0000${pageSize.value}`)
    ) return;
    const nextTotal = Math.max(0, Number(result?.total) || 0);
    const loaded = Array.isArray(result?.items) ? result.items : [];
    const nextItems = attentionOnly.value ? loaded.filter(accountNeedsAttention) : loaded;
    const allItems = selectedPageSize === ALL_PAGE_SIZE || attentionOnly.value;
    const resolvedTotal = allItems ? nextItems.length : nextTotal;
    const lastPage = allItems
      ? 1
      : Math.max(1, Math.ceil(resolvedTotal / selectedPageSize));
    if (!allItems && page > lastPage) {
      currentPage.value = lastPage;
      accounts.value = [];
      total.value = resolvedTotal;
      void loadAccounts();
      return;
    }
    accounts.value = nextItems;
    total.value = resolvedTotal;
    loadError.value = null;
  } catch (error) {
    if (
      error?.name !== "AbortError" &&
      viewActive &&
      loadGate.isCurrent(ticket, `${currentPage.value}\u0000${pageSize.value}`) &&
      !silent
    ) {
      loadError.value = error;
    }
    if (silent) return false;
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

function setAttentionFilter(value) {
  router.replace({ query: value === "attention" ? { attention: "1" } : {} });
}

watch(attentionOnly, () => {
  currentPage.value = 1;
  accounts.value = [];
  total.value = 0;
  loadError.value = null;
  loadGate.invalidate();
  void loadAccounts();
});

function handlePageChange(page) {
  if (pageSize.value === ALL_PAGE_SIZE) return;
  const nextPage = Math.max(1, Number(page) || 1);
  if (nextPage === currentPage.value) return;
  currentPage.value = nextPage;
  accounts.value = [];
  loadError.value = null;
  void loadAccounts();
}

function handlePageSizeChange(value) {
  const nextPageSize = normalizePageSize(value);
  if (nextPageSize === pageSize.value) return;
  pageSize.value = nextPageSize;
  currentPage.value = 1;
  accounts.value = [];
  total.value = 0;
  loadError.value = null;
  loadGate.invalidate();
  void loadAccounts();
}

const liveRefresh = createLiveRefresh(() => loadAccounts({ silent: true }), {
  getIntervalMs: () => accounts.value.some((account) => account.syncProgress?.active) ? 5_000 : undefined,
});

function openNewAccount() {
  router.push({ name: "account-new" });
}

function openAccount(id) {
  router.push({ name: "account-detail", params: { id } });
}

onMounted(() => {
  loadAccounts();
  liveRefresh.start({ immediate: false });
});

onBeforeUnmount(() => {
  viewActive = false;
  loadGate.deactivate();
  listAbortController?.abort();
  liveRefresh.stop();
});
</script>
