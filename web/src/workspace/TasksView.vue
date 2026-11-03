<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RouterLink } from 'vue-router'
import { apiRequest } from '../api/client.js'
import { useAuth } from '../stores/auth.js'
import { formatTime } from '../utils/format.js'

const auth = useAuth()
const filter = ref('all')
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
  <section class="tasks">
    <header><div><h1>任务</h1></div><button class="btn btn-quiet" @click="load">刷新</button></header>
    <nav class="tabs"><button v-for="item in [['all', '全部'], ['history', '历史补取'], ['create', '地址创建'], ['delete', '删除']]" :key="item[0]" :class="{ active: filter === item[0] }" @click="filter = item[0]">{{ item[1] }}</button></nav>
    <p v-if="error" class="error">{{ error }}</p><div v-if="loading" class="empty">正在读取任务…</div><div v-else-if="!groups.length" class="empty">暂无任务</div>
    <div v-else class="cards"><article v-for="job in groups" :key="`${job.type}-${job.account_id}-${job.id || job.job_id || job.plan_id || 'default'}`" class="card">
      <div class="card-head"><div><h3>{{ job.account_name || '主号任务' }}</h3><small>{{ job.type === 'history' ? '历史邮件补取' : job.type === 'create' ? '隐私地址批量创建' : job.type === 'schedule' ? '自动创建计划' : '地址删除' }}</small></div><span class="status">{{ job.type === 'schedule' ? '已启用' : labels[job.status] || job.status }}</span></div>
      <p v-if="job.type === 'history'">范围：{{ formatTime(job.since) }} — {{ formatTime(job.until) }} · 已扫描 {{ job.scanned_count ?? 0 }}，已归档 {{ job.archived_count ?? 0 }}</p>
      <p v-else-if="job.type === 'create'">目标 {{ job.target ?? '—' }} · 完成 {{ job.completed ?? 0 }}<template v-if="job.interval_seconds > 0"> · 间隔 {{ formatInterval(job.interval_seconds) }}</template></p><p v-else-if="job.type === 'schedule'">最近创建：{{ formatTime(job.last_created_at) }}</p><p v-else>请求 {{ job.requested ?? 0 }} · 已处理 {{ job.processed ?? 0 }} · 删除 {{ job.deleted ?? 0 }} · 失败 {{ job.failed ?? 0 }}</p>
      <p v-if="job.last_error" class="error">{{ job.last_error }}</p><p v-if="job.type === 'create' && job.status === 'waiting' && job.interval_seconds > 0 && !job.last_error && job.next_run_at">等待下一次创建：{{ formatTime(job.next_run_at) }}</p><p v-else-if="job.next_run_at">下次运行：{{ formatTime(job.next_run_at) }}</p>
      <div class="actions"><template v-if="job.type === 'history'"><button v-if="job.status === 'running' || job.status === 'queued'" class="btn btn-quiet" :disabled="busy === job.id" @click="act(job, 'pause')">暂停</button><button v-if="job.status === 'paused'" class="btn btn-primary" :disabled="busy === job.id" @click="act(job, 'resume')">恢复</button><button v-if="['queued', 'running', 'waiting', 'paused'].includes(job.status)" class="btn btn-quiet" :disabled="busy === job.id" @click="act(job, 'cancel')">取消</button></template><button v-if="job.type === 'create' && ['queued', 'running', 'waiting'].includes(job.status)" class="btn btn-quiet" :disabled="busy === job.id" @click="act(job, 'stop')">停止</button><RouterLink v-if="job.type === 'create' || job.type === 'schedule'" class="btn btn-quiet" :to="{ name: 'account-detail', params: { id: job.account_id } }">前往主号核对</RouterLink><RouterLink v-if="job.type === 'delete'" class="btn btn-quiet" :to="{ name: 'aliases' }">查看详情</RouterLink></div>
    </article></div>
  </section>
</template>

<style scoped>
.tasks{max-width:1000px;color:var(--text);font-size:13px}.tasks header,.card-head,.actions{display:flex;align-items:center;justify-content:space-between;gap:12px}.tasks h1{margin:0;font-size:20px;font-weight:600}.tasks header{margin-bottom:16px}.tasks header p{margin:0 0 18px;color:var(--text-secondary)}.tabs{display:flex;gap:4px;border-bottom:1px solid var(--border);margin-bottom:14px}.tabs button{border:0;background:transparent;padding:9px 12px;color:var(--text-secondary);cursor:pointer}.tabs button.active{color:var(--primary);border-bottom:2px solid var(--primary)}.cards{display:grid;gap:10px}.card{padding:15px;border:1px solid var(--border);border-radius:12px;background:var(--surface)}.card h3{margin:0 0 3px}.card small,.card p{color:var(--text-secondary)}.card p{line-height:1.55}.status{padding:4px 8px;border-radius:999px;background:var(--primary-soft);color:var(--primary)}.actions{justify-content:flex-start;flex-wrap:wrap}.error{color:var(--danger)}.empty{padding:40px 12px;text-align:center;color:var(--text-secondary)}@media(max-width:520px){.tasks header{align-items:flex-start;flex-direction:column}.actions{align-items:flex-start}}
</style>
