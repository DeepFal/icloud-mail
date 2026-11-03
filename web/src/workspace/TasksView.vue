<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { apiRequest } from '../api/client.js'
import { useAuth } from '../stores/auth.js'
import { formatTime } from '../utils/format.js'
import '../styles/pages.css'

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const FILTERS = [['all', '全部'], ['history', '历史补取'], ['create', '地址创建'], ['delete', '删除']]
const filter = computed(() => FILTERS.some(([key]) => key === route.query.type) ? route.query.type : 'all')
function setFilter(value) { router.replace({ query: value === 'all' ? {} : { type: value } }) }
const data = ref({ history_jobs: [], creation_jobs: [], deletion_jobs: [], automatic_plans: [] })
const loading = ref(false)
const error = ref('')
const busy = ref('')
let timer
let abort
let disposed = false

const labels = {
  queued: '排队中', running: '进行中', waiting: '等待中', paused: '已暂停', completed: '已完成',
  failed: '失败', cancelled: '已取消', interrupted: '已中断', stopped: '已停止',
}

const groups = computed(() => {
  const schedules = (data.value.automatic_plans || []).map((item) => ({ ...item, type: 'schedule' }))
  if (filter.value === 'all') return [...schedules, ...data.value.history_jobs.map((item) => ({ ...item, type: 'history' })), ...data.value.creation_jobs.map((item) => ({ ...item, type: 'create' })), ...data.value.deletion_jobs.map((item) => ({ ...item, type: 'delete' }))]
  if (filter.value === 'history') return data.value.history_jobs.map((item) => ({ ...item, type: 'history' }))
  if (filter.value === 'create') return [...schedules, ...data.value.creation_jobs.map((item) => ({ ...item, type: 'create' }))]
  return data.value.deletion_jobs.map((item) => ({ ...item, type: 'delete' }))
})
const active = computed(() => groups.value.some((item) => ['queued', 'running', 'waiting'].includes(item.status)))
const LIVE = ['queued', 'running', 'waiting', 'paused']
const sections = computed(() => [
  { key: 'live', title: '进行中', items: groups.value.filter((item) => item.type !== 'schedule' && LIVE.includes(item.status)) },
  { key: 'schedule', title: '自动创建计划', items: groups.value.filter((item) => item.type === 'schedule') },
  { key: 'done', title: '已结束', items: groups.value.filter((item) => item.type !== 'schedule' && !LIVE.includes(item.status)) },
].filter((section) => section.items.length))
const typeLabels = { history: '历史补取', create: '批量创建', schedule: '自动创建', delete: '地址删除' }
function statusTone(job) {
  if (job.type === 'schedule') return 'is-success'
  if (job.status === 'running') return 'is-primary'
  if (['queued', 'waiting', 'paused', 'interrupted'].includes(job.status)) return 'is-warning'
  if (job.status === 'failed') return 'is-danger'
  if (job.status === 'completed') return 'is-success'
  return ''
}
function jobKey(job) { return `${job.type}-${job.account_id}-${job.id || job.job_id || job.plan_id || 'default'}` }
function formatInterval(value) { const seconds = Number(value) || 0; return seconds < 60 ? '连续创建' : `${Math.round(seconds / 60)} 分钟` }

function stopOffline() { clearTimeout(timer); timer = undefined }

async function load() {
  if (loading.value || disposed) return
  abort?.abort(); abort = new AbortController(); loading.value = true
  try {
    const response = await apiRequest('/workspace/tasks', { signal: abort.signal })
    if (!disposed) { data.value = response.data || response; error.value = '' }
  } catch (e) {
    if (!disposed && e.name !== 'AbortError') error.value = e.message || '任务读取失败'
  } finally { loading.value = false }
}

function schedule() {
  clearTimeout(timer); timer = undefined
  if (disposed || document.hidden || !navigator.onLine) return
  timer = setTimeout(async () => { if (!disposed) { await load(); if (!disposed) schedule() } }, active.value ? 3000 : 30000)
}

async function act(job, name) {
  if (busy.value) return
  if (name === 'cancel') {
    try { await ElMessageBox.confirm('取消后将停止此任务。', '确认取消', { confirmButtonText: '取消任务', cancelButtonText: '返回' }) } catch { return }
  }
  busy.value = job.id
  try {
    const path = name === 'stop' ? `/accounts/${job.account_id}/aliases/creation-job/stop` : `/accounts/${job.account_id}/history-jobs/${encodeURIComponent(job.id)}/${name}`
    await apiRequest(path, { method: 'POST', body: {}, csrfToken: auth.state.csrfToken })
    ElMessage.info('操作已提交，状态仍以任务为准'); await load()
  } catch (e) { ElMessage.error(e.message || '操作失败') } finally { busy.value = '' }
}

onMounted(() => {
  disposed = false; load().then(schedule)
  document.addEventListener('visibilitychange', schedule)
  window.addEventListener('online', schedule); window.addEventListener('offline', stopOffline)
})
onBeforeUnmount(() => {
  disposed = true; abort?.abort(); clearTimeout(timer); timer = undefined
  document.removeEventListener('visibilitychange', schedule)
  window.removeEventListener('online', schedule); window.removeEventListener('offline', stopOffline)
})
</script>

<template>
  <section class="pg tasks" aria-labelledby="tasks-title">
    <header class="pg-bar">
      <h1 id="tasks-title" class="pg-title">任务</h1>
      <el-radio-group :model-value="filter" aria-label="任务类型" @change="setFilter">
        <el-radio-button v-for="item in FILTERS" :key="item[0]" :value="item[0]">{{ item[1] }}</el-radio-button>
      </el-radio-group>
      <div class="pg-bar__end">
        <el-button :loading="loading" @click="load">刷新</el-button>
      </div>
    </header>

    <p v-if="error" class="tasks__error" role="alert">{{ error }}</p>
    <p v-if="loading && !groups.length" class="pg-hint" role="status">正在读取任务…</p>
    <p v-else-if="!groups.length" class="tasks__empty">没有{{ filter === 'all' ? '' : FILTERS.find((item) => item[0] === filter)[1] }}任务。补取历史邮件或批量创建地址后会显示在这里。</p>

    <section v-for="section in sections" :key="section.key" class="tasks__section" :aria-labelledby="`tasks-${section.key}`">
      <h2 :id="`tasks-${section.key}`" class="tasks__heading">{{ section.title }} <span class="pg-count">{{ section.items.length }}</span></h2>
      <ul class="tasks__list">
        <li v-for="job in section.items" :key="jobKey(job)" class="task-row" :class="{ 'is-live': section.key === 'live' }">
          <span class="pg-dot task-row__status" :class="statusTone(job)">{{ job.type === 'schedule' ? '已启用' : labels[job.status] || job.status }}</span>
          <span class="task-row__type">{{ typeLabels[job.type] }}</span>
          <RouterLink class="task-row__account pg-ellipsis" :to="{ name: 'account-detail', params: { id: job.account_id } }" :title="job.account_name || undefined">{{ job.account_name || '主号任务' }}</RouterLink>
          <span class="task-row__progress pg-num">
            <template v-if="job.type === 'history'">
              <span>{{ formatTime(job.since) }} – {{ formatTime(job.until) }}</span>
              <span>已扫描 {{ job.scanned_count ?? 0 }}</span>
              <span>已归档 {{ job.archived_count ?? 0 }}</span>
            </template>
            <template v-else-if="job.type === 'create'">
              <span>完成 {{ job.completed ?? 0 }} / {{ job.target ?? '—' }}</span>
              <span v-if="job.interval_seconds > 0">间隔 {{ formatInterval(job.interval_seconds) }}</span>
            </template>
            <template v-else-if="job.type === 'schedule'">
              <span>最近创建 {{ formatTime(job.last_created_at) }}</span>
            </template>
            <template v-else>
              <span>已处理 {{ job.processed ?? 0 }} / {{ job.requested ?? 0 }}</span>
              <span>删除 {{ job.deleted ?? 0 }}</span>
              <span v-if="job.failed" class="task-row__failed">失败 {{ job.failed }}</span>
            </template>
          </span>
          <span class="task-row__time pg-num pg-muted">
            <template v-if="job.type === 'create' && job.status === 'waiting' && job.interval_seconds > 0 && !job.last_error && job.next_run_at">下一次 {{ formatTime(job.next_run_at) }}</template>
            <template v-else-if="job.next_run_at">下次运行 {{ formatTime(job.next_run_at) }}</template>
          </span>
          <span class="task-row__actions">
            <template v-if="job.type === 'history'">
              <el-button v-if="job.status === 'running' || job.status === 'queued'" size="small" :disabled="busy === job.id" @click="act(job, 'pause')">暂停</el-button>
              <el-button v-if="job.status === 'paused'" size="small" type="primary" :disabled="busy === job.id" @click="act(job, 'resume')">恢复</el-button>
              <el-button v-if="['queued', 'running', 'waiting', 'paused'].includes(job.status)" size="small" :disabled="busy === job.id" @click="act(job, 'cancel')">取消</el-button>
            </template>
            <el-button v-if="job.type === 'create' && ['queued', 'running', 'waiting'].includes(job.status)" size="small" :disabled="busy === job.id" @click="act(job, 'stop')">停止</el-button>
            <RouterLink v-if="job.type === 'delete'" class="task-row__link" :to="{ name: 'aliases' }">查看地址</RouterLink>
          </span>
          <p v-if="job.last_error" class="task-row__error">{{ job.last_error }}</p>
        </li>
      </ul>
    </section>
  </section>
</template>

<style scoped>
.tasks { width: 100%; max-width: 1200px; }
.tasks__error { margin: 0; color: var(--danger); }
.tasks__empty { margin: 0; color: var(--text-secondary); }
.tasks__section { display: flex; flex-direction: column; }
.tasks__heading { display: flex; align-items: center; gap: 8px; margin: 0; padding: 8px 0 4px; color: var(--text-secondary); font-size: 12px; line-height: 16px; font-weight: 600; }
.tasks__list { margin: 0; padding: 0; list-style: none; border-top: 1px solid var(--border); }
.task-row {
  display: grid;
  grid-template-columns: 76px 72px minmax(120px, 200px) minmax(0, 1fr) 168px max-content;
  align-items: center;
  column-gap: 12px;
  min-height: 36px;
  padding: 4px 0;
  border-bottom: 1px solid var(--border);
}
.task-row:hover { background: var(--row-hover); }
.task-row.is-live .task-row__account { font-weight: 600; }
.task-row__type { color: var(--text-secondary); white-space: nowrap; }
.task-row__account { color: var(--text); text-decoration: none; }
.task-row__account:hover { color: var(--primary); }
.task-row__progress { display: flex; flex-wrap: wrap; gap: 0 12px; min-width: 0; color: var(--text-secondary); }
.task-row__failed { color: var(--danger); }
.task-row__time { font-size: 12px; white-space: nowrap; }
.task-row__actions { display: flex; align-items: center; justify-content: flex-end; gap: 4px; }
.task-row__link { color: var(--primary); font-size: 12px; text-decoration: none; }
.task-row__link:hover { text-decoration: underline; }
.task-row__account:focus-visible, .task-row__link:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 2px; border-radius: 2px; }
.task-row__error { grid-column: 3 / -1; margin: 2px 0 0; color: var(--danger); font-size: 12px; line-height: 16px; overflow-wrap: anywhere; }
@media (max-width: 900px) {
  .task-row { grid-template-columns: 76px minmax(0, 1fr) max-content; row-gap: 2px; padding: 6px 0; }
  .task-row__type { display: none; }
  .task-row__progress, .task-row__time, .task-row__error { grid-column: 2 / -1; }
}
</style>
