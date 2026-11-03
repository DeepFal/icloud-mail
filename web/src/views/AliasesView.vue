<template>
  <section
    ref="aliasSelectionContainer"
    @pointerdown.capture="aliasDrag.pointerDown"
    class="pg pg--fill virtual-list-page aliases-page"
    aria-labelledby="aliases-section-title"
  >
    <header class="pg-bar alias-bar">
      <h1 id="aliases-section-title" class="pg-title">隐私地址</h1>
      <div class="alias-list-filters" role="search" aria-label="筛选全部隐私地址">
        <el-input
          v-model="keywordDraft"
          class="alias-list-filter--query"
          clearable
          maxlength="200"
          :prefix-icon="Search"
          :spellcheck="false"
          aria-label="关键词：模糊搜索隐私地址"
          placeholder="邮箱地址或用途备注"
          @keyup.enter="applyAliasSearch"
          @clear="applyAliasSearch"
        />
        <el-select
          v-model="selectedAccountId"
          class="alias-list-filter--account"
          filterable
          remote
          reserve-keyword
          clearable
          :loading="accountsLoading"
          :remote-method="searchAccounts"
          placeholder="全部主号"
          aria-label="按所属主号筛选"
          no-data-text="暂无主号"
          no-match-text="没有匹配的主号"
          @change="handleAccountFilterChange"
        >
          <el-option label="全部主号" value="" />
          <el-option
            v-for="account in accounts"
            :key="account.id"
            :label="formatAccountIdentity(account)"
            :value="account.id"
          >
            <div class="pg-cell">
              <strong>{{ formatAccountIdentity(account) }}</strong>
              <small v-if="account.name">{{ account.name }}</small>
            </div>
          </el-option>
        </el-select>
        <el-select
          v-model="selectedGroupFilter"
          class="alias-list-filter--group"
          clearable
          :loading="groupsLoading"
          placeholder="全部分组"
          filterable
          aria-label="按邮箱分组筛选"
          @change="handleGroupFilterChange"
        >
          <el-option label="全部分组" value="" />
          <el-option label="未分组" value="none" />
          <el-option
            v-for="group in groups"
            :key="group.id"
            :label="`${group.name}（${group.aliasCount}）`"
            :value="String(group.id)"
          />
        </el-select>
        <el-select
          v-model="selectedLatestMailFilter"
          class="alias-list-filter--mail"
          placeholder="全部邮件"
          aria-label="按最新邮件筛选"
          @change="handleLatestMailFilterChange"
        >
          <el-option label="全部邮件" value="" />
          <el-option label="没有邮件" value="none" />
          <el-option label="有邮件" value="yes" />
        </el-select>
        <el-button
          v-if="hasActiveFilters || hasAppliedFilters"
          :icon="RefreshLeft"
          @click="resetAliasFilters"
        >
          重置
        </el-button>
      </div>
      <div class="pg-bar__end">
        <el-button @click="openGroupDialog()">管理分组</el-button>
        <el-popover trigger="click" placement="bottom-end" :width="220">
          <template #reference><el-button :icon="MoreFilled" aria-label="更多邮箱操作">更多</el-button></template>
          <div class="alias-more-actions">
        <el-button
          :icon="CopyDocument"
          :loading="exportingAll"
          :disabled="total === 0 || exportingAll || rotatingAllCredentials"
          @click="copyAllAliases(ALIAS_EXPORT_OTP)"
        >
          复制全部取码
        </el-button>
        <el-button
          :icon="CopyDocument"
          :loading="exportingAll"
          :disabled="total === 0 || exportingAll || rotatingAllCredentials"
          @click="copyAllAliases(ALIAS_EXPORT_IMAP)"
        >
          复制全部 IMAP
        </el-button>
        <el-button
          type="danger"
          text
          :icon="RefreshLeft"
          :loading="rotatingAllCredentials"
          :disabled="rotatingAllCredentials || exportingAll"
          aria-label="轮换所有隐私地址令牌与凭证"
          @click="rotateAllCredentials"
        >
          轮换全部令牌
        </el-button>
          </div>
        </el-popover>
        <el-button
          :icon="Refresh"
          :loading="loading"
          :disabled="rotatingAllCredentials"
          @click="loadAliases"
        >
          刷新
        </el-button>
      </div>
    </header>

    <el-dialog
      v-model="groupDialogVisible"
      title="管理邮箱分组"
      width="min(520px, calc(100vw - 28px))"
      :close-on-click-modal="false"
    >
      <el-form class="dialog-form" @submit.prevent="saveGroup">
        <el-form-item :label="editingGroupId ? '重命名分组' : '新建分组'">
          <el-input
            v-model="groupNameDraft"
            maxlength="100"
            show-word-limit
            placeholder="例如：工作、购物、注册"
            @keyup.enter="saveGroup"
          />
        </el-form-item>
        <div class="dialog-actions dialog-actions--end">
          <el-button
            type="primary"
            :loading="groupSaving"
            :disabled="!groupNameDraft.trim()"
            @click="saveGroup"
          >
            {{ editingGroupId ? "保存名称" : "新建分组" }}
          </el-button>
          <el-button
            v-if="editingGroupId"
            :disabled="groupSaving"
            @click="cancelGroupEdit"
          >
            取消编辑
          </el-button>
        </div>
      </el-form>

      <el-divider />
      <div v-if="groups.length" class="mail-group-list">
        <div v-for="group in groups" :key="group.id" class="mail-group-row">
          <div class="mail-group-row__identity">
            <strong>{{ group.name }}</strong>
            <small>{{ group.aliasCount }} 个邮箱</small>
          </div>
          <div class="mail-group-row__actions">
            <el-button link type="primary" :icon="EditPen" :disabled="groupSaving" @click="editGroup(group)">
              重命名
            </el-button>
            <el-button link type="danger" :icon="Delete" :disabled="groupDeletingId === group.id" @click="removeGroup(group)">
              删除
            </el-button>
          </div>
        </div>
      </div>
      <EmptyState v-else level="h3" title="还没有分组" description="创建分组后即可把隐私地址移动进去。" />
    </el-dialog>

    <div v-if="selectedAliasIds.length" class="alias-selection-toolbar" role="toolbar" aria-label="跨页选择与批量操作">
      <span class="selection-count">已选 {{ selectedAliasIds.length }}</span>
      <el-button text :disabled="selectionBusy || !aliases.length" @click="invertCurrentPageSelection">反选本页</el-button>
      <el-button text :disabled="selectionBusy" @click="clearAliasSelection">取消选择</el-button>
      <span class="alias-selection-toolbar__spacer" />
      <el-button :icon="CopyDocument" :disabled="selectionBusy || !selectedAliases.length" @click="copySelectedAliases(ALIAS_EXPORT_OTP)">复制取码</el-button>
      <el-button :disabled="selectionBusy || !selectedAliases.length" @click="copySelectedAliases(ALIAS_EXPORT_IMAP)">复制 IMAP</el-button>
      <el-select v-model="moveTargetGroupId" class="alias-group-bulk-select" :loading="groupsLoading || movingAliases" :disabled="selectionBusy" placeholder="移动到分组" aria-label="将勾选的隐私地址移动到分组" @change="moveSelectedAliases">
        <el-option label="未分组" value="none" />
        <el-option v-for="group in groups" :key="group.id" :label="group.name" :value="String(group.id)" />
      </el-select>
      <el-button type="danger" text :icon="Delete" :loading="deletingAliases" :disabled="selectionBusy" aria-label="从 Apple 永久删除勾选的隐私地址" @click="deleteSelectedAliases">从 Apple 删除（{{ selectedAliasIds.length }}）</el-button>
    </div>

    <section
      v-if="deletionProgressVisible"
      class="data-panel alias-deletion-progress"
      aria-labelledby="alias-deletion-title"
    >
      <div class="alias-deletion-progress__header">
        <strong id="alias-deletion-title">Apple 批量删除</strong>
        <el-tag :type="deletionJobType">{{ deletionStatusLabel }}</el-tag>
        <el-button
          :icon="Refresh"
          :loading="deletionState.checking"
          :disabled="deletingAliases || deletionState.submitting"
          @click="refreshDeletionJob"
        >刷新任务状态</el-button>
        <el-button
          v-if="isAliasDeletionJobTerminal(deletionJob)"
          text
          :disabled="deletionState.uncertain"
          aria-label="关闭删除任务提示"
          @click="deletionVisibility.dismiss()"
        >关闭</el-button>
      </div>
      <template v-if="deletionJob">
        <p role="status" aria-live="polite" aria-atomic="true">
          已处理 {{ deletionJob.processed }} / {{ deletionJob.requested }}；
          成功删除 {{ deletionJob.deleted }}；失败/未执行 {{ deletionJob.failed }}
          <span v-if="deletionJob.deferred > 0">（其中未执行 {{ deletionJob.deferred }}）</span>
        </p>
        <el-progress
          :percentage="deletionPercentage"
          :show-text="false"
          aria-label="Apple 批量删除进度"
        />
        <p class="alias-deletion-progress__meta">
          任务：{{ deletionJob.jobId }} · 更新：{{ formatTime(deletionJob.updatedAt) }}
          <span v-if="deletionJob.requestId"> · 请求编号：{{ deletionJob.requestId }}</span>
        </p>
        <p v-if="isAliasDeletionJobActive(deletionJob)" class="alias-deletion-progress__hint">
          任务在服务端持续执行，每 2 秒串行查询进度；离开或刷新页面不会取消任务。
        </p>
        <div v-if="deletionWaits.length" class="alias-deletion-progress__waits" role="status" aria-live="polite">
          <strong>以下主号触发 Apple 限流，等待后继续</strong>
          <div v-for="(wait, index) in deletionWaits" :key="`${wait.accountId}:${wait.aliasId}:${wait.operation}:${index}`">
            <span v-if="wait.accountId">主号 ID {{ wait.accountId }}</span>
            <span v-if="wait.aliasId">{{ wait.accountId ? " · " : "" }}邮箱 ID {{ wait.aliasId }}</span>
            · {{ ALIAS_DELETION_OPERATION_LABELS[wait.operation] }}
            · 预计重试时间：{{ formatTime(wait.retryAt, { seconds: true }) }}
            · 第 {{ wait.attempt }} 次重试（最多 {{ wait.maxAttempts }} 次）
          </div>
          <span class="alias-deletion-progress__hint">仅上述主号等待；其他未限流主号继续处理。等待中的邮箱及这些主号的剩余项尚未计为已处理或失败；到时由服务端继续。</span>
        </div>
      </template>
      <p v-if="deletionState.recovering" role="status">
        正在恢复当前管理员的最近任务，查询确认前暂停新建删除任务。
      </p>
      <p v-if="!deletionJob && deletionState.operationId" class="alias-deletion-progress__meta">
        本次任务：{{ deletionState.operationId }}
      </p>
      <p v-if="deletionState.uncertain" class="alias-deletion-progress__hint" role="status">
        {{ deletionState.unmatched ? "尚未查到本次提交对应的任务。" : "任务查询或提交响应异常。" }}
        删除结果待确认，保留最近已知进度；系统只重试查询，不会自动重新提交删除请求。
        <span v-if="deletionState.error?.code">（{{ deletionState.error.code }}）</span>
        <span v-if="deletionState.error?.requestId">请求编号：{{ deletionState.error.requestId }}</span>
      </p>
      <div v-if="deletionState.unmatched" class="alias-deletion-progress__hint">
        如需重新选择，请先在主号详情刷新 Apple 目录确认实际状态。
        <el-button
          :disabled="deletionState.checking || deletingAliases"
          @click="acknowledgeDeletionState"
        >已刷新 Apple 目录并核对结果</el-button>
      </div>
      <p v-if="deletionJob?.status === 'interrupted'" class="alias-deletion-progress__hint" role="status">
        任务已中断，部分 Apple 结果待确认。请在主号详情刷新 Apple 目录确认后重新选择；系统不会自动重放剩余项。
      </p>
      <div v-if="deletionRecentFailures.length" class="alias-deletion-progress__failures">
        <strong>近期未删除或待确认结果（最近 {{ deletionRecentFailures.length }} 项）</strong>
        <div v-for="failure in deletionRecentFailures" :key="failure.id">
          {{ failure.address || `ID ${failure.id}` }}：{{ formatAliasDeletionResultMessage(failure) }}
        </div>
      </div>
      <details v-if="deletionJob?.results.length" @toggle="deletionResultsExpanded = $event.target.open">
        <summary>查看服务端已返回的逐项结果（{{ deletionJob.results.length }} 项）</summary>
        <ul v-if="deletionResultsExpanded" class="alias-deletion-progress__results">
          <li v-for="result in deletionJob.results" :key="result.id">
            {{ result.address || `ID ${result.id}` }}：
            {{ formatAliasDeletionResultMessage(result) }}
          </li>
        </ul>
      </details>
    </section>

      <RequestAlert
        v-if="accountsLoadError || groupsError"
        :error="accountsLoadError || groupsError"
        closable
        @close="accountsLoadError = null; groupsError = null"
      />

    <div v-if="loading && aliases.length === 0" class="data-panel loading-panel">
      <el-skeleton :rows="6" animated role="status" aria-label="正在加载隐私地址列表" />
    </div>

    <div v-else-if="loadError && aliases.length === 0" class="load-failed">
      <RequestAlert :error="loadError" />
      <el-button :icon="Refresh" @click="loadAliases">重新加载</el-button>
    </div>

    <EmptyState
      v-else-if="aliases.length === 0"
      :title="
        appliedAliasQuery || appliedGroupId || selectedLatestMailFilter
          ? '没有匹配的隐私地址'
          : selectedAccountId
            ? '该主号暂无隐私地址'
            : '还没有隐私地址'
      "
      :description="
        appliedAliasQuery || appliedGroupId || selectedLatestMailFilter
          ? '请尝试其他关键词，或调整所属主号、邮箱分组和最新邮件筛选。'
          : selectedAccountId
            ? '请选择其他主号，或进入该主号详情页添加地址。'
            : '进入某个主号详情页添加地址。'
      "
    >
      <el-button
        v-if="appliedAliasQuery || appliedGroupId || selectedLatestMailFilter"
        :icon="RefreshLeft"
        @click="resetAliasFilters"
      >
        重置筛选
      </el-button>
      <el-button v-else type="primary" :icon="Setting" @click="openAccounts">
        查看主号
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
        :aria-busy="loading"
      >
        <VirtualDataTable
          :columns="aliasColumns"
          :data="aliases"
          row-key="id"
          fill-height
          :row-height="32"
          :header-height="32"
          :loading="loading"
        >
          <template #header-cell="{ column }">
            <el-checkbox
              v-if="column.key === 'selection'"
              :model-value="allAliasesSelected"
              :indeterminate="someExportableAliasesSelected"
              :disabled="selectionBusy || aliases.length === 0"
              aria-label="勾选本页隐私地址"
              @change="setAllAliasesSelected"
            />
            <template v-else>{{ column.title }}</template>
          </template>
          <template #cell="{ column, row }">
            <el-checkbox
              v-if="column.key === 'selection'"
              :model-value="isAliasSelected(row.id)"
              :disabled="selectionBusy"
              :data-selection-id="row.id"
              :data-selection-disabled="selectionBusy"
              :aria-label="`勾选 ${row.address}`"
              @change="setAliasSelected(row, $event)"
            />
            <template v-else-if="column.key === 'address'">
              <div class="pg-cell alias-address">
                <strong :title="row.address" translate="no">{{ row.address }}</strong>
                <small v-if="row.note || row.label" :title="row.note || row.label">{{ row.note || row.label }}</small>
                <span v-if="isAliasConfirmationPending(row)" class="pg-badge is-warning">待确认</span>
              </div>
            </template>
            <template v-else-if="column.key === 'account'">
              <el-button
                class="account-link"
                link
                @click="openAccount(row.accountId)"
              >
                {{ formatAliasAccountIdentity(row) || "查看主号" }}
              </el-button>
            </template>
            <template v-else-if="column.key === 'group'">
              <el-select
                class="alias-group-select"
                :model-value="row.groupId == null ? '' : String(row.groupId)"
                :loading="Boolean(movingAliasIds[row.id])"
                :disabled="Boolean(movingAliasIds[row.id])"
                aria-label="选择隐私地址分组"
                placeholder="未分组"
                @change="moveAlias(row, $event)"
              >
                <el-option label="未分组" value="" />
                <el-option
                  v-for="group in groups"
                  :key="group.id"
                  :label="group.name"
                  :value="String(group.id)"
                />
              </el-select>
            </template>
            <template v-else-if="column.key === 'lastAccessedAt'">
              <span class="pg-num pg-muted">{{ formatTime(row.lastAccessedAt) }}</span>
            </template>
            <template v-else-if="column.key === 'latestReceivedAt'">
              <span class="pg-num" :class="{ 'pg-muted': !row.latestReceivedAt }">{{ formatTime(row.latestReceivedAt) }}</span>
            </template>
            <template v-else-if="column.key === 'status'">
              <span v-if="isAliasConfirmationPending(row)" class="pg-dot is-warning">等待目录确认</span>
              <span v-else-if="!row.accountEnabled && row.configuredEnabled" class="pg-dot">随主号暂停</span>
              <SyncStatus v-else :item="row" details />
            </template>
            <template v-else-if="column.key === 'actions'">
              <div class="pg-row-actions alias-row-actions">
                <el-button v-if="isAliasReceiveAvailable(row)" link type="primary" size="small" @click="openAliasInbox(row)">收件</el-button>
                <el-dropdown trigger="click" placement="bottom-end">
                  <el-button class="pg-icon-btn" :icon="MoreFilled" :aria-label="`更多操作：${row.address}`" />
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item @click="openMailboxProfile(row)">备注与服务登记</el-dropdown-item>
                      <el-dropdown-item v-if="isAliasExportable(row)" :disabled="Boolean(copyLoading[`${row.id}:otp`])" @click="copyAliasLine(row, ALIAS_EXPORT_OTP)">复制取码格式</el-dropdown-item>
                      <el-dropdown-item v-if="isAliasExportable(row)" :disabled="Boolean(copyLoading[`${row.id}:imap`])" @click="copyAliasLine(row, ALIAS_EXPORT_IMAP)">复制 IMAP 格式</el-dropdown-item>
                      <el-dropdown-item v-if="isLegacyDirectLinkAvailable(row)" :disabled="Boolean(copyLoading[`${row.id}:legacy-link`])" @click="copyLegacyDirectLink(row)">复制旧直达链接</el-dropdown-item>
                      <el-dropdown-item @click="openAccount(row.accountId)">管理主号</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </template>
        </VirtualDataTable>
      </div>

      <div v-if="pageSize <= 100" class="mobile-record-list" :aria-busy="loading">
        <article v-for="alias in aliases" :key="alias.id" class="mobile-record alias-mobile-record">
          <header class="mobile-record__header">
            <div class="mobile-alias-selection">
              <el-checkbox
                class="mobile-alias-selection__checkbox"
                :model-value="isAliasSelected(alias.id)"
                :disabled="selectionBusy"
                :data-selection-id="alias.id"
                :data-selection-disabled="selectionBusy"
                :aria-label="`勾选 ${alias.address}`"
                @change="setAliasSelected(alias, $event)"
              />
              <div class="primary-stack">
                <strong>{{ alias.address }}</strong>
                <small v-if="alias.note || alias.label">{{ alias.note || alias.label }}</small>
              </div>
            </div>
            <span v-if="isAliasConfirmationPending(alias)" class="pg-dot is-warning">等待目录确认</span>
            <span v-else-if="!alias.accountEnabled && alias.configuredEnabled" class="pg-dot">随主号暂停</span>
            <SyncStatus v-else :item="alias" />
          </header>
          <dl class="mobile-kv-list">
            <div>
              <dt>所属主号</dt>
              <dd>{{ formatAliasAccountIdentity(alias) || "-" }}</dd>
            </div>
            <div>
              <dt>最近调用</dt>
              <dd>{{ formatTime(alias.lastAccessedAt) }}</dd>
            </div>
            <div>
              <dt>最新邮件</dt>
              <dd>{{ formatTime(alias.latestReceivedAt) }}</dd>
            </div>

          </dl>
          <footer class="mobile-record__actions alias-mobile-actions">
            <el-select class="mobile-alias-group-select" :model-value="alias.groupId == null ? '' : String(alias.groupId)" :loading="Boolean(movingAliasIds[alias.id])" :disabled="Boolean(movingAliasIds[alias.id])" aria-label="选择隐私地址分组" placeholder="未分组" @change="moveAlias(alias, $event)">
              <el-option label="未分组" value="" />
              <el-option v-for="group in groups" :key="group.id" :label="group.name" :value="String(group.id)" />
            </el-select>
            <el-button v-if="isAliasReceiveAvailable(alias)" link type="primary" :icon="Message" @click="openAliasInbox(alias)">收件</el-button>
            <el-dropdown trigger="click" placement="bottom-end">
              <el-button text :icon="MoreFilled" :aria-label="`更多操作：${alias.address}`" />
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="openMailboxProfile(alias)">备注与服务登记</el-dropdown-item>
                  <el-dropdown-item v-if="isAliasExportable(alias)" :disabled="Boolean(copyLoading[`${alias.id}:otp`])" @click="copyAliasLine(alias, ALIAS_EXPORT_OTP)">复制取码格式</el-dropdown-item>
                  <el-dropdown-item v-if="isAliasExportable(alias)" :disabled="Boolean(copyLoading[`${alias.id}:imap`])" @click="copyAliasLine(alias, ALIAS_EXPORT_IMAP)">复制 IMAP 格式</el-dropdown-item>
                  <el-dropdown-item v-if="isLegacyDirectLinkAvailable(alias)" :disabled="Boolean(copyLoading[`${alias.id}:legacy-link`])" @click="copyLegacyDirectLink(alias)">复制旧直达链接</el-dropdown-item>
                  <el-dropdown-item @click="openAccount(alias.accountId)">管理主号</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </footer>
        </article>
      </div>

      <ListPagination
        :page="currentPage"
        :page-size="pageSize"
        :total="total"
        :loading="loading"
        aria-label="隐私地址列表分页"
        @change="handlePageChange"
        @size-change="handlePageSizeChange"
      />
    </template>
    <MailboxProfileDrawer v-model="profileOpen" :alias-id="profileAliasId" @updated="mailboxProfileUpdated" />
  </section>
</template>

<script setup>
import {
  CopyDocument,
  Delete,
  EditPen,
  FolderAdd,
  Message,
  MoreFilled,
  Refresh,
  RefreshLeft,
  Search,
  Setting,
} from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

import {
  createMailGroup,
  deleteMailGroup,
  getAccount,
  getAccountPage,
  getAliasPage,
  getAliasDeletionJob,
  getAllAliases,
  getMailGroups,
  getLatestAliasDeletionJob,
  moveAliasToGroup,
  moveAliasesToGroup,
  rotateAllAliasCredentials,
  startAliasDeletionJob,
  updateMailGroup,
} from "../api/admin.js";
import EmptyState from "../components/EmptyState.vue";
import MailboxProfileDrawer from "../workspace/MailboxProfileDrawer.vue";
import ListPagination from "../components/ListPagination.vue";
import RequestAlert from "../components/RequestAlert.vue";
import "../styles/pages.css";
import SyncStatus from "../components/SyncStatus.vue";
import VirtualDataTable from "../components/VirtualDataTable.vue";
import { useAuth } from "../stores/auth.js";
import {
  ALIAS_DELETION_OPERATION_LABELS,
  createAliasDeletionController,
  createAliasDeletionStorage,
  formatAliasDeletionResultMessage,
  isAliasDeletionJobActive,
  isAliasDeletionJobTerminal,
} from "../utils/aliasDeletionJob.js";
import { createAliasDeletionVisibility } from "../utils/aliasDeletionVisibility.js";
import { ADMIN_BASE_PATH } from "../utils/runtimePath.js";
import {
  createActionLock,
  createLatestRequestGate,
} from "../utils/asyncState.js";
import {
  ALIAS_EXPORT_IMAP,
  ALIAS_EXPORT_OTP,
  buildAliasReceiveLink,
  buildAliasExportText,
} from "../utils/aliasExport.js";
import { buildRecentMailDirectLink, copyText } from "../utils/clipboard.js";
import {
  confirmationCancelled,
  showRequestError,
  successMessage,
} from "../utils/feedback.js";
import { formatTime } from "../utils/format.js";
import { createLiveRefresh } from "../utils/liveRefresh.js";
import { createCheckboxDragSelection } from "../utils/checkboxDragSelection.js";
import {
  ALL_PAGE_SIZE,
  DEFAULT_PAGE_SIZE,
  normalizePageSize,
} from "../utils/pagination.js";

const router = useRouter();
const route = useRoute();
const profileAliasId = ref(""), profileOpen = ref(false);
function openMailboxProfile(alias) { profileAliasId.value = alias.id; profileOpen.value = true; }
function mailboxProfileUpdated(profile) { for (const alias of aliases.value) if (alias.address === profile.address) alias.note = profile.note; }
const auth = useAuth();
const ACCOUNT_OPTION_LIMIT = 50;
const pageSize = ref(DEFAULT_PAGE_SIZE);
const aliasColumns = [
  { key: "selection", title: "", width: 40, align: "center" },
  { key: "address", title: "隐私地址", width: 280, flexGrow: 3 },
  { key: "account", title: "所属主号", width: 180, flexGrow: 1, hideBelow: 1050 },
  { key: "group", title: "分组", width: 120 },
  { key: "lastAccessedAt", title: "最近调用", width: 136, hideBelow: 1250 },
  { key: "latestReceivedAt", title: "最新邮件", width: 136 },
  { key: "status", title: "状态", width: 184, hideBelow: 800 },
  { key: "actions", title: "", width: 88, align: "right" },
];
const aliases = ref([]);
const accounts = ref([]);
const selectedAccountId = ref("");
const selectedGroupFilter = ref("");
const appliedGroupId = ref("");
const selectedLatestMailFilter = ref("");
const moveTargetGroupId = ref("");
const groups = ref([]);
const groupsLoading = ref(false);
const groupsError = ref(null);
const groupDialogVisible = ref(false);
const groupNameDraft = ref("");
const editingGroupId = ref(null);
const groupSaving = ref(false);
const groupDeletingId = ref(null);
const movingAliases = ref(false);
const movingAliasIds = reactive({});
const keywordDraft = ref("");
const appliedAliasQuery = ref("");

// Filters live in the URL (?q, account, group, mail) so the list can be refreshed or shared.
function readAliasFiltersFromRoute() {
  const pick = (key) => (typeof route.query[key] === "string" ? route.query[key].trim() : "");
  const account = pick("account");
  keywordDraft.value = pick("q");
  appliedAliasQuery.value = keywordDraft.value;
  selectedAccountId.value = /^\d+$/.test(account) ? Number(account) : "";
  selectedGroupFilter.value = pick("group");
  appliedGroupId.value = selectedGroupFilter.value;
  selectedLatestMailFilter.value = ["none", "yes"].includes(pick("mail")) ? pick("mail") : "";
}
readAliasFiltersFromRoute();
watch(
  () => [appliedAliasQuery.value, selectedAccountId.value, appliedGroupId.value, selectedLatestMailFilter.value],
  ([q, account, group, mail]) => {
    if (route.name !== "aliases") return;
    const query = {};
    if (q) query.q = q;
    if (account !== "" && account != null) query.account = String(account);
    if (group) query.group = group;
    if (mail) query.mail = mail;
    const current = route.query;
    const same = ["q", "account", "group", "mail"].every((key) => (current[key] ?? "") === (query[key] ?? ""));
    if (!same) void router.replace({ query });
  },
);
const currentPage = ref(1);
const total = ref(0);
const selectedAliasIds = ref([]);
const selectedAliasRecords = ref(new Map());
const aliasSelectionContainer = ref(null);
const selectionBusy = computed(() => loading.value || deletionJobBlocked.value || deletingAliases.value || movingAliases.value || exportingAll.value || rotatingAllCredentials.value);
const loading = ref(false);
const loadError = ref(null);
const accountsLoading = ref(false);
const accountsLoadError = ref(null);
const exportingAll = ref(false);
const rotatingAllCredentials = ref(false);
const deletingAliases = ref(false);
const copyLoading = reactive({});
const copyLock = createActionLock();
const aliasLoadGate = createLatestRequestGate();
const accountsLoadGate = createLatestRequestGate();
const groupsLoadGate = createLatestRequestGate();
let accountSearchTimer = null;
let aliasAbortController = null;
let viewActive = true;
const aliasSelectionActive = ref(false);
const aliasDrag = createCheckboxDragSelection({
  getContainer: () => aliasSelectionContainer.value,
  getSelected: () => selectedAliasIds.value,
  isDisabled: () => selectionBusy.value,
  onChange: updateAliasSelection,
  onActiveChange: (active) => { aliasSelectionActive.value = active; },
});

const deletionState = ref({});
const deletionResultsExpanded = ref(false);
const deletionProgressVisible = ref(false);
const deletionVisibility = createAliasDeletionVisibility({
  onChange: (visible) => { deletionProgressVisible.value = visible; },
});
watch(() => deletionState.value, (state) => deletionVisibility.update(state), { immediate: true, deep: true });
let deletionController = makeDeletionController();
deletionState.value = deletionController.getState();
const deletionJob = computed(() => deletionState.value.job);
const deletionJobBlocked = computed(() => deletionState.value.blocked);
const deletionPercentage = computed(() => deletionJob.value?.requested
  ? Math.min(100, Math.max(0, deletionJob.value.processed / deletionJob.value.requested * 100))
  : 0);
const deletionRecentFailures = computed(() =>
  (deletionJob.value?.results || []).filter((item) => !item.deleted).slice(-5).reverse(),
);
const deletionWaits = computed(() => deletionJob.value?.status === "running"
  ? deletionJob.value.waits || []
  : []);
const deletionJobType = computed(() => {
  if (deletionState.value.uncertain || deletionWaits.value.length || deletionJob.value?.failed || deletionJob.value?.status === "interrupted") return "warning";
  return deletionJob.value?.status === "completed" ? "success" : "info";
});
const deletionStatusLabel = computed(() => {
  if (deletionState.value.submitting) return "正在提交";
  if (deletionState.value.uncertain) return "结果待确认";
  if (deletionState.value.recovering) return "恢复任务中";
  if (deletionWaits.value.length) return "执行中（主号限流等待）";
  return { queued: "排队中", running: "执行中", completed: "已完成", interrupted: "已中断" }[deletionJob.value?.status] || "查询中";
});

function makeDeletionController() {
  const username = auth.state.username;
  let observedJobId = "";
  let observedSubmission = false;
  return createAliasDeletionController({
    startJob: startAliasDeletionJob,
    getJob: getAliasDeletionJob,
    getLatestJob: getLatestAliasDeletionJob,
    storage: createAliasDeletionStorage(ADMIN_BASE_PATH, username),
    onChange(next) {
      if (!viewActive || auth.state.username !== username) return;
      const previousJob = deletionState.value.job;
      if (next.submitting || next.uncertain) observedSubmission = true;
      if (isAliasDeletionJobActive(next.job)) observedJobId = next.job.jobId;
      deletionState.value = next;
      if (next.job && next.job.jobId !== previousJob?.jobId) {
        clearAliasSelection();
        deletionResultsExpanded.value = false;
      }
      if (isAliasDeletionJobTerminal(next.job) && !next.blocked &&
          (observedSubmission || observedJobId === next.job.jobId)) {
        observedSubmission = false;
        observedJobId = "";
        clearAliasSelection();
        void Promise.all([
          loadAliases({ silent: true }),
          loadAccounts({ silent: true }),
          loadGroups({ silent: true }),
        ]);
      }
    },
  });
}

watch(() => auth.state.username, () => {
  deletionController.stop();
  deletionController = makeDeletionController();
  deletionState.value = deletionController.getState();
  deletionResultsExpanded.value = false;
  clearAliasSelection();
  if (viewActive && auth.state.username) void deletionController.start();
}, { flush: "sync" });

function refreshDeletionJob() {
  return deletionController.refresh({ latest: true });
}

function acknowledgeDeletionState() {
  if (!deletionController.acknowledgeUnmatched()) return;
  clearAliasSelection();
  void loadAliases();
}

const selectedAliases = computed(() => {
  return selectedAliasIds.value.map((id) => selectedAliasRecords.value.get(id)).filter(Boolean).filter(isAliasExportable);
});

const allAliasesSelected = computed(
  () => aliases.value.length > 0 && aliases.value.every((alias) => isAliasSelected(alias.id)),
);

// Keep the established selector name for view/test compatibility. Selection
// now includes every mailbox so legacy aliases can also be moved in bulk.
const someExportableAliasesSelected = computed(() => {
  const selectedCount = aliases.value.filter((alias) => isAliasSelected(alias.id)).length;
  return selectedCount > 0 && selectedCount < aliases.value.length;
});

const withoutLatestMail = computed(
  () => selectedLatestMailFilter.value === "none",
);
const withLatestMail = computed(
  () => selectedLatestMailFilter.value === "yes",
);

const hasActiveFilters = computed(() =>
  Boolean(
    selectedAccountId.value ||
      selectedGroupFilter.value ||
      selectedLatestMailFilter.value ||
      keywordDraft.value.trim(),
  ),
);

const hasAppliedFilters = computed(() =>
  Boolean(
    selectedAccountId.value ||
      appliedGroupId.value ||
      selectedLatestMailFilter.value ||
      appliedAliasQuery.value,
  ),
);

function isAliasConfirmationPending(alias) {
  return (
    !alias?.enabled &&
    String(alias?.lastSyncError || "").trim() ===
      "APPLE_ALIAS_CONFIRMATION_PENDING"
  );
}

function formatAccountIdentity(account) {
  const email = String(account?.email || account?.accountEmail || "");
  if (account?.mailboxType === "custom") {
    const suffix = String(account?.emailSuffix || "").replace(/^@+/, "");
    if (suffix) return `@${suffix}`;
  }
  if (account?.mailboxType == null && email.startsWith("custom@")) {
    return email.slice("custom".length);
  }
  return email;
}

function formatAliasAccountIdentity(alias) {
  const account = accounts.value.find(
    (item) => String(item.id) === String(alias?.accountId),
  );
  return formatAccountIdentity(account || alias);
}

function isAliasExportable(alias) {
  return Boolean(
    alias?.address &&
      alias?.apiKey &&
      alias?.imapPassword &&
      alias?.clientId &&
      alias?.refreshToken &&
      alias?.otpUrlPath,
  );
}

function isLegacyDirectLinkAvailable(alias) {
  return Boolean(
    !isAliasConfirmationPending(alias) &&
      alias?.credentialMode === "legacy" &&
      alias?.directLinkPath,
  );
}

function isAliasReceiveAvailable(alias) {
  return Boolean(
    !isAliasConfirmationPending(alias) &&
      (alias?.otpUrlPath || alias?.directLinkPath || alias?.legacyDirectLinkPath),
  );
}

async function loadAccounts({ silent = false, query = "" } = {}) {
  const normalizedQuery = String(query || "").trim();
  const ticket = accountsLoadGate.begin(normalizedQuery);
  if (!silent) {
    accountsLoading.value = true;
    accountsLoadError.value = null;
  }
  try {
    const result = await getAccountPage({
      limit: ACCOUNT_OPTION_LIMIT,
      offset: 0,
      query: normalizedQuery,
    });
    if (!accountsLoadGate.isCurrent(ticket, normalizedQuery)) return;
    const nextAccounts = Array.isArray(result?.items) ? result.items : [];
    const selected = accounts.value.find(
      (account) => String(account.id) === String(selectedAccountId.value),
    );
    accounts.value =
      selected &&
      !nextAccounts.some(
        (account) => String(account.id) === String(selected.id),
      )
        ? [selected, ...nextAccounts]
        : nextAccounts;
    accountsLoadError.value = null;
  } catch (error) {
    if (
      accountsLoadGate.isCurrent(ticket, normalizedQuery) &&
      !silent
    ) {
      accountsLoadError.value = error;
    }
  } finally {
    if (accountsLoadGate.isCurrent(ticket, normalizedQuery)) {
      accountsLoading.value = false;
    }
  }
}

function searchAccounts(query) {
  if (accountSearchTimer !== null) {
    window.clearTimeout(accountSearchTimer);
  }
  accountSearchTimer = window.setTimeout(() => {
    accountSearchTimer = null;
    void loadAccounts({ query });
  }, query ? 220 : 0);
}

async function loadGroups({ silent = false } = {}) {
  const ticket = groupsLoadGate.begin("groups");
  if (!silent) {
    groupsLoading.value = true;
    groupsError.value = null;
  }
  try {
    const nextGroups = await getMailGroups();
    if (!groupsLoadGate.isCurrent(ticket, "groups")) return;
    groups.value = nextGroups;
    groupsError.value = null;
  } catch (error) {
    if (!silent && groupsLoadGate.isCurrent(ticket, "groups")) {
      groupsError.value = error;
    }
    if (silent) return false;
  } finally {
    if (groupsLoadGate.isCurrent(ticket, "groups")) {
      groupsLoading.value = false;
    }
  }
}

async function loadAliases({ silent = false } = {}) {
  if (rotatingAllCredentials.value) return;
  if (silent && (aliasSelectionActive.value || loading.value || deletingAliases.value || movingAliases.value)) return;
  const accountId = selectedAccountId.value;
  const query = appliedAliasQuery.value;
  const groupId = appliedGroupId.value;
  const latestMailFilter = selectedLatestMailFilter.value;
  const withoutLatestMailOnly = withoutLatestMail.value;
  const withLatestMailOnly = withLatestMail.value;
  const page = currentPage.value;
  const selectedPageSize = pageSize.value;
  const requestKey = `${accountId}\u0000${groupId}\u0000${query}\u0000${latestMailFilter}\u0000${page}\u0000${selectedPageSize}`;
  const ticket = aliasLoadGate.begin(requestKey);
  aliasAbortController?.abort();
  const abortController = new AbortController();
  aliasAbortController = abortController;
  if (!silent) {
    loading.value = true;
    loadError.value = null;
  }
  try {
    const result = selectedPageSize === ALL_PAGE_SIZE
      ? {
          items: await getAllAliases(accountId, {
            query,
            groupId,
            withoutLatestMail: withoutLatestMailOnly,
            withLatestMail: withLatestMailOnly,
            signal: abortController.signal,
          }),
        }
      : await getAliasPage(accountId, {
          limit: selectedPageSize,
          offset: (page - 1) * selectedPageSize,
          query,
          groupId,
          withoutLatestMail: withoutLatestMailOnly,
          withLatestMail: withLatestMailOnly,
          signal: abortController.signal,
        });
    const currentKey = `${selectedAccountId.value}\u0000${appliedGroupId.value}\u0000${appliedAliasQuery.value}\u0000${selectedLatestMailFilter.value}\u0000${currentPage.value}\u0000${pageSize.value}`;
    if (!aliasLoadGate.isCurrent(ticket, currentKey)) return;
    const nextTotal = Math.max(0, Number(result?.total) || 0);
    const nextAliases = Array.isArray(result?.items) ? result.items : [];
    const allItems = selectedPageSize === ALL_PAGE_SIZE;
    const resolvedTotal = allItems ? nextAliases.length : nextTotal;
    const lastPage = allItems
      ? 1
      : Math.max(1, Math.ceil(resolvedTotal / selectedPageSize));
    if (!allItems && page > lastPage) {
      currentPage.value = lastPage;
      aliases.value = [];
      total.value = resolvedTotal;
      void loadAliases();
      return;
    }
    aliases.value = nextAliases;
    total.value = resolvedTotal;
    const records = new Map(selectedAliasRecords.value);
    nextAliases.forEach((alias) => { if (selectedAliasIds.value.includes(alias.id)) records.set(alias.id, alias); });
    selectedAliasRecords.value = records;
    loadError.value = null;
  } catch (error) {
    if (
      error?.name !== "AbortError" &&
      aliasLoadGate.isCurrent(
        ticket,
        `${selectedAccountId.value}\u0000${appliedGroupId.value}\u0000${appliedAliasQuery.value}\u0000${selectedLatestMailFilter.value}\u0000${currentPage.value}\u0000${pageSize.value}`,
      ) &&
      !silent
    ) {
      loadError.value = error;
    }
    if (silent) return false;
  } finally {
    if (
      aliasAbortController === abortController &&
      aliasLoadGate.isCurrent(
        ticket,
        `${selectedAccountId.value}\u0000${appliedGroupId.value}\u0000${appliedAliasQuery.value}\u0000${selectedLatestMailFilter.value}\u0000${currentPage.value}\u0000${pageSize.value}`,
      )
    ) {
      loading.value = false;
    }
    if (aliasAbortController === abortController) {
      aliasAbortController = null;
    }
  }
}

function clearAliasSelection() {
  aliasDrag.stop();
  selectedAliasIds.value = [];
  selectedAliasRecords.value = new Map();
}

function beginAliasMutation() {
  aliasDrag.stop();
  aliasLoadGate.invalidate();
  groupsLoadGate.invalidate();
  aliasAbortController?.abort();
  loading.value = false;
  groupsLoading.value = false;
}

function reloadAliasesForFilters() {
  currentPage.value = 1;
  clearAliasSelection();
  aliases.value = [];
  total.value = 0;
  loadError.value = null;
  void loadAliases();
}

function applyAliasSearch() {
  const query = keywordDraft.value.trim();
  if (query === appliedAliasQuery.value) return;
  appliedAliasQuery.value = query;
  reloadAliasesForFilters();
}

function handleAccountFilterChange(value) {
  selectedAccountId.value = value == null ? "" : value;
  appliedAliasQuery.value = keywordDraft.value.trim();
  reloadAliasesForFilters();
}

function handleGroupFilterChange(value) {
  selectedGroupFilter.value = value == null ? "" : String(value);
  appliedGroupId.value = selectedGroupFilter.value;
  appliedAliasQuery.value = keywordDraft.value.trim();
  reloadAliasesForFilters();
}

function handleLatestMailFilterChange(value) {
  selectedLatestMailFilter.value = value === "none" || value === "yes" ? value : "";
  appliedAliasQuery.value = keywordDraft.value.trim();
  reloadAliasesForFilters();
}

function resetAliasFilters() {
  if (!hasActiveFilters.value && !hasAppliedFilters.value) return;
  keywordDraft.value = "";
  appliedAliasQuery.value = "";
  selectedAccountId.value = "";
  selectedGroupFilter.value = "";
  appliedGroupId.value = "";
  selectedLatestMailFilter.value = "";
  reloadAliasesForFilters();
}

function handlePageChange(page) {
  if (pageSize.value === ALL_PAGE_SIZE) return;
  const nextPage = Math.max(1, Number(page) || 1);
  if (nextPage === currentPage.value) return;
  aliasDrag.stop();
  currentPage.value = nextPage;
  aliases.value = [];
  loadError.value = null;
  void loadAliases();
}

function handlePageSizeChange(value) {
  const nextPageSize = normalizePageSize(value);
  if (nextPageSize === pageSize.value) return;
  aliasDrag.stop();
  pageSize.value = nextPageSize;
  currentPage.value = 1;
  aliases.value = [];
  total.value = 0;
  loadError.value = null;
  aliasLoadGate.invalidate();
  void loadAliases();
}

const liveRefresh = createLiveRefresh(() => {
  return Promise.all([
    loadAliases({ silent: true }),
    loadGroups({ silent: true }),
  ]).then((results) => results.includes(false) ? false : undefined);
}, {
  getIntervalMs: () => deletionWaits.value.length || ["queued", "running"].includes(deletionJob.value?.status) ? 5_000 : undefined,
});

function isAliasSelected(id) {
  return selectedAliasIds.value.includes(id);
}

function setAliasSelected(alias, selected) {
  if (selectionBusy.value) return;
  const selectedIds = new Set(selectedAliasIds.value);
  if (selected) {
    selectedIds.add(alias.id);
  } else {
    selectedIds.delete(alias.id);
  }
  selectedAliasIds.value = [...selectedIds];
  const records = new Map(selectedAliasRecords.value);
  if (selected) records.set(alias.id, alias); else records.delete(alias.id);
  selectedAliasRecords.value = records;
}

function setAllAliasesSelected(selected) {
  if (selectionBusy.value) return;
  const pageIds = new Set(aliases.value.map((alias) => alias.id));
  const ids = selectedAliasIds.value.filter((id) => !pageIds.has(id));
  if (selected) ids.push(...aliases.value.map((alias) => alias.id));
  selectedAliasIds.value = [...new Set(ids)];
  const records = new Map(selectedAliasRecords.value);
  if (selected) aliases.value.forEach((alias) => records.set(alias.id, alias));
  else pageIds.forEach((id) => records.delete(id));
  selectedAliasRecords.value = records;
}

function updateAliasSelection(ids) {
  const rows = new Map(aliases.value.map((alias) => [alias.id, alias]));
  const records = new Map();
  for (const id of ids) {
    const row = rows.get(id) || selectedAliasRecords.value.get(id);
    if (row) records.set(id, row);
  }
  selectedAliasRecords.value = records;
  selectedAliasIds.value = [...records.keys()];
}

function invertCurrentPageSelection() {
  if (selectionBusy.value) return;
  const ids = new Set(selectedAliasIds.value);
  for (const alias of aliases.value) {
    if (ids.has(alias.id)) ids.delete(alias.id); else ids.add(alias.id);
  }
  updateAliasSelection([...ids]);
}

function batchDeleteAccountState(detail) {
  const account = detail?.account;
  const mailboxType = String(account?.mailboxType || "")
    .trim()
    .toLowerCase();
  const status = String(account?.lastSyncStatus || "")
    .trim()
    .toLowerCase();
  const syncError = String(account?.lastSyncError || "").trim();
  if (mailboxType !== "icloud") return "custom";
  if (status === "error" || syncError) return "error";
  if (detail?.appleSession?.status !== "authenticated") return "login";
  return "ok";
}

async function deleteSelectedAliases() {
  if (
    !viewActive ||
    !auth.state.username ||
    deletingAliases.value ||
    deletionJobBlocked.value ||
    movingAliases.value ||
    exportingAll.value ||
    rotatingAllCredentials.value ||
    !selectedAliasIds.value.length
  ) {
    return;
  }

  const controller = deletionController;
  const username = auth.state.username;
  const isCurrent = () => viewActive && deletionController === controller && auth.state.username === username;
  const selectedIds = [...selectedAliasIds.value];
  const selected = selectedIds.map((id) => selectedAliasRecords.value.get(id)).filter(Boolean);
  if (selected.length !== selectedIds.length) {
    clearAliasSelection();
    showRequestError(
      { message: "列表已发生变化，请刷新后重新选择隐私地址。" },
      "批量删除未执行。",
    );
    return;
  }
  if (selected.some(isAliasConfirmationPending)) {
    ElMessage.warning("等待 Apple 目录确认的隐私地址暂时不能批量删除。");
    return;
  }

  deletingAliases.value = true;
  liveRefresh.stop();
  beginAliasMutation();
  try {
    const accountIds = [...new Set(selected.map((alias) => String(alias.accountId)))];
    const details = await Promise.all(
      accountIds.map((accountId) => getAccount(accountId, { limit: 1, offset: 0 })),
    );
    if (!isCurrent()) return;
    const states = details.map(batchDeleteAccountState);
    if (states.includes("custom")) {
      ElMessage.warning("批量删除仅支持 Apple 已登录的 iCloud 主号。");
      return;
    }
    if (states.includes("error")) {
      ElMessage.warning("所选 iCloud 主号存在同步错误，请先处理错误后再批量删除。");
      return;
    }
    if (states.includes("login")) {
      ElMessage.warning("请先让所选 iCloud 主号完成 Apple 登录，再批量删除隐私地址。");
      return;
    }

    await ElMessageBox.confirm(
      `将从 Apple 永久删除所选的 ${selected.length} 个隐私地址，并清除本项目中的对应记录。Apple 端删除不可恢复，继续吗？`,
      "从 Apple 批量永久删除隐私地址",
      {
        type: "warning",
        confirmButtonText: "继续删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
        autofocus: false,
      },
    );
    if (!isCurrent()) return;
    await ElMessageBox.prompt(
      "这是 Apple 端不可恢复的操作。请输入 DELETE_APPLE_ALIASES 以确认。",
      "最终确认批量删除",
      {
        type: "error",
        inputPlaceholder: "DELETE_APPLE_ALIASES",
        inputValidator: (value) =>
          value === "DELETE_APPLE_ALIASES" || "请输入完整的 DELETE_APPLE_ALIASES",
        confirmButtonText: "永久删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
        autofocus: false,
      },
    );

    if (!isCurrent()) return;
    const submitted = await controller.submit(selectedIds, auth.state.csrfToken);
    if (!isCurrent()) return;
    if (submitted) clearAliasSelection();
    else ElMessage.warning("删除任务状态已变化，请先查看任务进度。");
  } catch (error) {
    if (!isCurrent() || confirmationCancelled(error)) return;
    showRequestError(
      error,
      "删除任务未提交，请检查提示后再操作。",
    );
  } finally {
    deletingAliases.value = false;
    if (viewActive) {
      liveRefresh.start({ immediate: false });
    }
  }
}

async function copyAliases(items, format, scope) {
  const exportableItems = items.filter(isAliasExportable);
  if (!exportableItems.length) return;

  try {
    const content = buildAliasExportText(exportableItems, format);
    const copied = await copyText(content);
    if (!copied) throw new Error("clipboard rejected copy");
    const label = format === ALIAS_EXPORT_OTP ? "取码链接" : "IMAP/OAuth";
    successMessage(`已复制${scope}${exportableItems.length} 个邮箱的${label}。`);
  } catch {
    ElMessage({
      type: "error",
      message: "邮箱凭证复制失败，请检查浏览器剪切板权限后重试。",
      grouping: true,
    });
  }
}

function copySelectedAliases(format) {
  return copyAliases(selectedAliases.value, format, "勾选的");
}

function copyAllAliases(format) {
  if (exportingAll.value || rotatingAllCredentials.value) return;
  exportingAll.value = true;
  getAllAliases(selectedAccountId.value, {
    query: appliedAliasQuery.value,
    groupId: appliedGroupId.value === "none" ? "none" : appliedGroupId.value,
    withoutLatestMail: withoutLatestMail.value,
    withLatestMail: withLatestMail.value,
  })
    .then((items) => {
      if (viewActive) {
        return copyAliases(items, format, "全部");
      }
      return undefined;
    })
    .catch((error) => {
      if (viewActive) {
        showRequestError(error, "复制隐私地址凭证失败，请稍后重试。");
      }
    })
    .finally(() => {
      exportingAll.value = false;
    });
}

function rotateAllCredentialsErrorMessage(error) {
  if (error?.code === "CURRENT_PASSWORD_INVALID") {
    return "当前管理员密码验证失败，请重新输入。";
  }
  if (error?.code === "RATE_LIMITED") {
    return "安全验证尝试过于频繁，请 15 分钟后再试。";
  }
  return "全部令牌与凭证轮换请求失败，结果状态未知；自动刷新已暂停，请确认后再手动刷新。";
}

async function rotateAllCredentials() {
  if (rotatingAllCredentials.value || exportingAll.value) return;
  rotatingAllCredentials.value = true;
  let currentPassword = "";
  let rotationSubmitted = false;
  try {
    currentPassword = String(
      (
        await ElMessageBox.prompt(
          "此操作会让所有旧 V1 直达链接，以及 V2 取码令牌、API Key、IMAP 密码和 OAuth 凭据立即失效；旧版邮箱会强制升级为 V2，Apple 确认中的邮箱也会同步轮换。成功后当前管理员的所有后台会话将被撤销。请输入当前管理员密码继续。",
          "验证身份并轮换全部凭据",
          {
            type: "error",
            inputType: "password",
            inputPlaceholder: "当前管理员密码",
            inputValidator: (value) =>
              (typeof value === "string" && value.length > 0) ||
              "请输入当前管理员密码",
            confirmButtonText: "验证并继续",
            cancelButtonText: "取消",
            confirmButtonClass: "el-button--danger",
            autofocus: false,
          },
        )
      )?.value ?? "",
    );
    await ElMessageBox.prompt(
      "该操作不可撤销。请输入 ROTATE_ALL 以最终确认轮换全部令牌与凭据。",
      "最终确认",
      {
        type: "error",
        inputPlaceholder: "ROTATE_ALL",
        inputValidator: (value) =>
          value === "ROTATE_ALL" || "请输入完整的 ROTATE_ALL",
        confirmButtonText: "永久轮换全部",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
        autofocus: false,
      },
    );
    liveRefresh.stop();
    beginAliasMutation();
    rotationSubmitted = true;
    const summary = await rotateAllAliasCredentials(
      currentPassword,
      auth.state.csrfToken,
    );
    currentPassword = "";
    aliases.value = [];
    total.value = 0;
    clearAliasSelection();
    successMessage(
      `轮换完成：共检查 ${summary.total} 个，成功轮换 ${summary.rotated} 个（V1 升级 V2 ${summary.migratedLegacy} 个，现有 V2 已轮换 ${summary.rotatedV2} 个，Apple 确认中同步轮换 ${summary.rotatedPending} 个）。新凭据未自动加载，当前管理员的后台会话已撤销，请重新登录后仅在可信环境中按需显式获取。`,
      10000,
    );
    auth.clearSession({ checked: false });
    await router.replace({
      name: "login",
      query: { notice: "credentials_rotated" },
    });
  } catch (error) {
    if (confirmationCancelled(error)) return;
    if (!rotationSubmitted) {
      if (viewActive) {
        const message = rotateAllCredentialsErrorMessage(error);
        showRequestError({ ...error, message }, message);
      }
      return;
    }
    if (
      error?.code === "CURRENT_PASSWORD_INVALID" ||
      error?.code === "RATE_LIMITED"
    ) {
      if (viewActive) {
        liveRefresh.start({ immediate: false });
        const message = rotateAllCredentialsErrorMessage(error);
        showRequestError({ ...error, message }, message);
      }
      return;
    }
    aliases.value = [];
    total.value = 0;
    clearAliasSelection();
    auth.clearSession({ checked: false });
    await router.replace({
      name: "login",
      query: {
        notice:
          error?.code === "CREDENTIALS_CHANGED"
            ? "credentials_changed"
            : error?.code === "ROTATION_RESULT_INVALID"
              ? "rotation_result_invalid"
              : "rotation_status_unknown",
      },
    });
  } finally {
    currentPassword = "";
    rotatingAllCredentials.value = false;
  }
}

async function copyAliasLine(alias, format) {
  const lockKey = `${alias?.id}:${format}`;
  if (!isAliasExportable(alias) || !copyLock.acquire(lockKey)) return;
  copyLoading[lockKey] = true;
  try {
    const copied = await copyText(buildAliasExportText([alias], format));
    if (!viewActive) return;
    if (!copied) {
      ElMessage({
        type: "error",
        message: "邮箱凭证复制失败，请检查浏览器剪切板权限后重试。",
        grouping: true,
      });
      return;
    }
    successMessage(format === ALIAS_EXPORT_OTP ? "取码链接格式已复制。" : "IMAP/OAuth 格式已复制。");
  } catch {
    if (!viewActive) return;
    ElMessage({
      type: "error",
      message: "邮箱凭证复制失败，请刷新页面后重试。",
      grouping: true,
    });
  } finally {
    delete copyLoading[lockKey];
    copyLock.release(lockKey);
  }
}

async function copyLegacyDirectLink(alias) {
  const lockKey = `${alias?.id}:legacy-link`;
  if (!isLegacyDirectLinkAvailable(alias) || !copyLock.acquire(lockKey)) return;
  copyLoading[lockKey] = true;
  try {
    const directLink = buildRecentMailDirectLink(alias.directLinkPath);
    const copied = await copyText(directLink);
    if (!viewActive) return;
    if (!copied) throw new Error("clipboard rejected copy");
    successMessage("旧邮件 API 直达链接已复制。");
  } catch {
    if (viewActive) {
      ElMessage({
        type: "error",
        message: "旧直达链接复制失败，请刷新页面后重试。",
        grouping: true,
      });
    }
  } finally {
    delete copyLoading[lockKey];
    copyLock.release(lockKey);
  }
}

function openAliasInbox(alias) {
  if (!alias?.id) return;
  router.push({ name: "inbox", query: { alias_id: String(alias.id) } });
}

function openGroupDialog(group = null) {
  editingGroupId.value = group?.id ?? null;
  groupNameDraft.value = group?.name || "";
  groupDialogVisible.value = true;
}

function editGroup(group) {
  openGroupDialog(group);
}

function cancelGroupEdit() {
  editingGroupId.value = null;
  groupNameDraft.value = "";
}

async function saveGroup() {
  const name = groupNameDraft.value.trim();
  if (!name || groupSaving.value) return;
  beginAliasMutation();
  groupSaving.value = true;
  try {
    if (editingGroupId.value) {
      const updated = await updateMailGroup(
        editingGroupId.value,
        name,
        auth.state.csrfToken,
      );
      groups.value = groups.value
        .map((group) => (group.id === updated.id ? updated : group))
        .sort((left, right) => left.name.localeCompare(right.name));
      successMessage("邮箱分组名称已更新。");
    } else {
      const created = await createMailGroup(name, auth.state.csrfToken);
      groups.value = [...groups.value, created].sort((left, right) =>
        left.name.localeCompare(right.name),
      );
      successMessage("邮箱分组已创建。");
    }
    cancelGroupEdit();
    await loadAliases();
  } catch (error) {
    showRequestError(error, "邮箱分组保存失败，请稍后重试。");
  } finally {
    groupSaving.value = false;
  }
}

async function removeGroup(group) {
  if (!group || groupDeletingId.value) return;
  try {
    await ElMessageBox.confirm(
      group.aliasCount
        ? `删除“${group.name}”后，其中 ${group.aliasCount} 个邮箱会变为未分组。继续吗？`
        : `确定删除分组“${group.name}”吗？`,
      "删除邮箱分组",
      {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
        autofocus: false,
      },
    );
  } catch {
    return;
  }
  groupDeletingId.value = group.id;
  beginAliasMutation();
  try {
    await deleteMailGroup(group.id, auth.state.csrfToken);
    groups.value = groups.value.filter((item) => item.id !== group.id);
    if (editingGroupId.value === group.id) {
      cancelGroupEdit();
    }
    if (selectedGroupFilter.value === String(group.id)) {
      selectedGroupFilter.value = "";
      appliedGroupId.value = "";
      reloadAliasesForFilters();
    } else {
      await loadAliases();
    }
    successMessage("邮箱分组已删除，邮箱已恢复为未分组。");
  } catch (error) {
    showRequestError(error, "邮箱分组删除失败，请稍后重试。");
  } finally {
    groupDeletingId.value = null;
  }
}

async function moveSelectedAliases(groupValue) {
  if (
    !selectedAliasIds.value.length ||
    movingAliases.value ||
    deletingAliases.value ||
    deletionJobBlocked.value
  ) {
    return;
  }
  beginAliasMutation();
  movingAliases.value = true;
  moveTargetGroupId.value = groupValue == null ? "" : String(groupValue);
  const targetGroupId = moveTargetGroupId.value === "none"
    ? null
    : moveTargetGroupId.value;
  try {
    await moveAliasesToGroup(
      selectedAliasIds.value,
      targetGroupId || null,
      auth.state.csrfToken,
    );
    clearAliasSelection();
    await Promise.all([loadAliases(), loadGroups()]);
    successMessage("已将勾选的隐私地址移动到所选分组。");
  } catch (error) {
    showRequestError(error, "邮箱分组移动失败，请稍后重试。");
  } finally {
    movingAliases.value = false;
    moveTargetGroupId.value = "";
  }
}

async function moveAlias(alias, groupValue) {
  if (!alias || movingAliasIds[alias.id]) return;
  beginAliasMutation();
  movingAliasIds[alias.id] = true;
  try {
    const updated = await moveAliasToGroup(
      alias.id,
      groupValue === "" || groupValue == null ? null : groupValue,
      auth.state.csrfToken,
    );
    aliases.value = aliases.value.map((item) =>
      item.id === updated.id ? updated : item,
    );
    await Promise.all([
      loadAliases(),
      loadGroups(),
    ]);
    successMessage("隐私地址分组已更新。");
  } catch (error) {
    showRequestError(error, "隐私地址分组移动失败，请稍后重试。");
  } finally {
    delete movingAliasIds[alias.id];
  }
}

function openAccounts() {
  router.push({ name: "accounts" });
}

function openAccount(id) {
  router.push({ name: "account-detail", params: { id } });
}

onMounted(() => {
  if (auth.state.username) void deletionController.start();
  loadAccounts();
  loadGroups();
  loadAliases();
  liveRefresh.start({ immediate: false });
});

onBeforeUnmount(() => {
  viewActive = false;
  aliasDrag.stop();
  deletionController.stop();
  deletionVisibility.stop();
  if (accountSearchTimer !== null) {
    window.clearTimeout(accountSearchTimer);
    accountSearchTimer = null;
  }
  aliasLoadGate.deactivate();
  accountsLoadGate.deactivate();
  groupsLoadGate.deactivate();
  aliasAbortController?.abort();
  liveRefresh.stop();
});
</script>

<style scoped>
.alias-bar { flex-wrap: nowrap; }
.alias-list-filters { display: flex; flex: 1 1 auto; align-items: center; gap: 8px; min-width: 0; }
.alias-list-filter--query { flex: 0 1 240px; width: 240px; min-width: 150px; }
.alias-list-filter--account { flex: 0 1 200px; width: 200px; min-width: 120px; }
.alias-list-filter--group { flex: 0 1 140px; width: 140px; min-width: 100px; }
.alias-list-filter--mail { flex: 0 0 104px; width: 104px; }

.alias-selection-toolbar { display: flex; flex: 0 0 auto; align-items: center; gap: 4px 8px; min-height: 36px; margin-inline: calc(-1 * var(--pg-pad)); padding: 0 var(--pg-pad); background: var(--surface-subtle); border-block: 1px solid var(--border); }
.selection-count { color: var(--text); font-size: 13px; font-weight: 600; font-variant-numeric: tabular-nums; }
.alias-selection-toolbar__spacer { flex: 1; }
.alias-more-actions { display: grid; gap: 4px; }
.alias-more-actions .el-button { width: 100%; justify-content: flex-start; margin-left: 0; }
.alias-address > strong { font-family: var(--font-mono); font-size: 12px; }
.alias-address > .pg-badge { flex: 0 0 auto; }

.alias-deletion-progress {
  display: grid;
  flex: 0 0 auto;
  min-width: 0;
  max-height: 40vh;
  gap: 8px;
  padding: 8px 0 12px;
  overflow: auto;
  overflow-wrap: anywhere;
  border: 0;
  border-bottom: 1px solid var(--border);
  border-radius: 0;
  background: transparent;
}

.alias-deletion-progress__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.alias-deletion-progress__meta,
.alias-deletion-progress__hint {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.alias-deletion-progress__failures,
.alias-deletion-progress__waits {
  display: grid;
  gap: 4px;
  font-size: 12px;
}

.alias-deletion-progress summary {
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.alias-deletion-progress__results {
  max-height: 220px;
  padding-left: 20px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.7;
}

.alias-group-bulk-select {
  width: 140px;
}

.alias-group-select {
  width: 100%;
}

.alias-group-select :deep(.el-select__wrapper) {
  min-height: 24px;
  padding: 0 6px;
  background: transparent;
  box-shadow: none;
  font-size: 12px;
}

.alias-group-select :deep(.el-select__suffix) {
  opacity: 0;
}

.alias-group-select:hover :deep(.el-select__wrapper),
.alias-group-select :deep(.el-select__wrapper.is-focused) {
  background: var(--surface);
  box-shadow: 0 0 0 1px var(--border-strong) inset;
}

.alias-group-select:hover :deep(.el-select__suffix),
.alias-group-select :deep(.el-select__wrapper.is-focused .el-select__suffix) {
  opacity: 1;
}

.alias-mobile-record .mobile-record__header { padding: 12px; gap: 8px; }
.alias-mobile-record .mobile-kv-list { padding: 0 12px; }
.alias-mobile-record .mobile-kv-list > div { padding: 6px 0; }
.alias-mobile-actions { padding: 8px 12px; gap: 6px; flex-wrap: nowrap; }
.alias-mobile-actions .mobile-alias-group-select { width: 124px; min-width: 0; margin-right: auto; }
.alias-mobile-actions > .el-button { flex: initial; }
.alias-mobile-actions :deep(.el-button) { padding: 5px; }


.mail-group-list {
  display: grid;
  border-top: 1px solid var(--border);
}

.mail-group-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 36px;
  padding: 4px 0;
  border-bottom: 1px solid var(--border);
}

.mail-group-row__identity {
  display: flex;
  align-items: baseline;
  min-width: 0;
  gap: 8px;
}

.mail-group-row__identity strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mail-group-row__identity small {
  color: var(--text-secondary);
  font-size: 12px;
}

.mail-group-row__actions {
  display: flex;
  flex: 0 0 auto;
  gap: 4px;
}

.account-link {
  max-width: 100%;
  justify-content: flex-start;
  color: var(--text-secondary);
  font-weight: 400;
}

.account-link:hover {
  color: var(--primary);
}

.account-link :deep(span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-alias-selection {
  display: flex;
  width: 100%;
  min-width: 0;
  max-width: 100%;
  flex: 1 1 auto;
  align-items: flex-start;
  gap: 10px;
}

.mobile-alias-selection .primary-stack {
  min-width: 0;
  flex: 1 1 auto;
}

.mobile-alias-selection__checkbox {
  flex: 0 0 auto;
  margin-top: 1px;
}

@media (max-width: 1000px) {
  .alias-bar { flex-wrap: wrap; }
  .alias-list-filters { order: 10; flex-basis: 100%; }
}
@media (max-width: 740px) {
  .alias-list-filters { flex-wrap: wrap; }
  .alias-list-filter--query { flex: 1 1 100%; width: 100%; }
  .alias-list-filter--account, .alias-list-filter--group, .alias-list-filter--mail { flex: 1 1 40%; width: auto; }
  .alias-selection-toolbar { flex-wrap: wrap; padding-block: 6px; }
}
</style>
