<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage, ElMessageBox, ElSwitch } from "element-plus";
import { apiRequest } from "../api/client.js";
import { useAuth } from "../stores/auth.js";

const props = defineProps({ accountId: { type: [String, Number], required: true } });
const auth = useAuth();
const loading = ref(false), saving = ref(false), jobBusy = ref(false), error = ref("");
const job = ref(null), backgroundEnabled = ref(false), historyDays = ref(7), fetchDays = ref(7);
let generation = 0, controller, timer, disposed = false, jobLoading = false;
const statuses = { queued: "排队中", running: "补取中", waiting: "等待重试", paused: "已暂停", completed: "已完成", failed: "需要处理", cancelled: "已取消" };
const active = computed(() => job.value && ["queued", "running", "waiting", "paused"].includes(job.value.status));
const jobTone = computed(() => ({ running: "is-primary", queued: "is-warning", waiting: "is-warning", paused: "is-warning", completed: "is-success", failed: "is-danger" })[job.value?.status] || "");
const path = () => `/accounts/${encodeURIComponent(props.accountId)}`;
function date(value) { if (!value) return "—"; const at = new Date(value); return Number.isFinite(at.getTime()) ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "short", timeStyle: "short" }).format(at) : "时间未知"; }
function stopPoll() { clearTimeout(timer); }
function schedule() { stopPoll(); if (disposed || document.hidden || !navigator.onLine) return; const running = ["queued", "running", "waiting"].includes(job.value?.status); timer = setTimeout(() => loadJob(), running ? 3000 : 30000); }
async function loadJob() {
  if (jobLoading || !props.accountId || disposed) return; jobLoading = true; const current = generation;
  try { const result = await apiRequest(`${path()}/history-jobs`, { signal: controller?.signal }); if (current === generation && !disposed) job.value = result.job; }
  catch (cause) { if (cause.name !== "AbortError" && current === generation && !disposed) error.value = cause.message || "任务加载失败"; }
  finally { jobLoading = false; if (current === generation) schedule(); }
}
async function load() {
  const current = ++generation; stopPoll(); controller?.abort(); controller = new AbortController();
  jobLoading = false; loading.value = true; error.value = ""; job.value = null;
  try { const [preferences, result] = await Promise.all([apiRequest(`${path()}/mail-preferences`, { signal: controller.signal }), apiRequest(`${path()}/history-jobs`, { signal: controller.signal })]); if (disposed || current !== generation) return; backgroundEnabled.value = preferences.background_enabled; historyDays.value = preferences.history_days; fetchDays.value = preferences.history_days; job.value = result.job; }
  catch (cause) { if (cause.name !== "AbortError" && current === generation && !disposed) error.value = cause.message || "设置加载失败"; }
  finally { if (current === generation && !disposed) { loading.value = false; schedule(); } }
}
async function save() {
  if (loading.value || saving.value) return; const current = generation; saving.value = true;
  try { await apiRequest(`${path()}/mail-preferences`, { method: "PUT", body: { background_enabled: backgroundEnabled.value, history_days: Number(historyDays.value) }, csrfToken: auth.state.csrfToken, signal: controller.signal }); if (current === generation && !disposed) ElMessage.success("收件设置已保存"); }
  catch (cause) { if (cause.name !== "AbortError") ElMessage.error(cause.message || "设置保存失败"); }
  finally { if (current === generation) saving.value = false; }
}
async function start() {
  if (loading.value || jobBusy.value || active.value) return; const current = generation; jobBusy.value = true;
  try { const result = await apiRequest(`${path()}/history-jobs`, { method: "POST", body: { operation_id: crypto.randomUUID(), days: Number(fetchDays.value) }, csrfToken: auth.state.csrfToken, signal: controller.signal }); if (disposed || current !== generation) return; job.value = result; ElMessage.success("历史补取已排队，关闭页面后仍会继续"); schedule(); }
  catch (cause) { if (current !== generation || disposed) return; if (cause.status === 409) { error.value = cause.message; await loadJob(); } else if (cause.name !== "AbortError") error.value = `${cause.message || "提交结果未知"}。请刷新任务确认结果后再提交。`; }
  finally { if (current === generation) jobBusy.value = false; }
}
async function act(action) {
  if (jobBusy.value || !job.value?.id) return;
  if (action === "cancel") { try { await ElMessageBox.confirm("取消后停止后续补取，已归档的历史邮件继续保留。", "取消补取任务", { confirmButtonText: "取消任务", cancelButtonText: "返回" }); } catch { return; } }
  const current = generation, id = job.value.id; jobBusy.value = true;
  try { const result = await apiRequest(`${path()}/history-jobs/${encodeURIComponent(id)}/${action}`, { method: "POST", body: {}, csrfToken: auth.state.csrfToken, signal: controller.signal }); if (current !== generation || disposed) return; job.value = result; error.value = ""; schedule(); }
  catch (cause) { if (cause.name !== "AbortError" && current === generation && !disposed) { error.value = cause.message || "操作结果未知，请刷新任务确认"; await loadJob(); } }
  finally { if (current === generation) jobBusy.value = false; }
}
function visibility() { if (!document.hidden && navigator.onLine) loadJob(); schedule(); }
watch(() => props.accountId, () => { saving.value = false; jobBusy.value = false; load(); }, { immediate: true });
document.addEventListener("visibilitychange", visibility); window.addEventListener("online", visibility); window.addEventListener("offline", schedule);
onBeforeUnmount(() => { disposed = true; generation++; controller?.abort(); stopPoll(); document.removeEventListener("visibilitychange", visibility); window.removeEventListener("online", visibility); window.removeEventListener("offline", schedule); });
</script>

<template>
  <section class="receiving-settings" aria-label="后台收件与历史补取">
    <p v-if="error" class="receiving-settings__error" role="alert">{{ error }} <el-button link type="primary" @click="load">重新读取</el-button></p>
    <div class="setting-row">
      <span id="receiving-background-label" class="setting-row__label">后台常驻收件</span>
      <div class="setting-row__control">
        <ElSwitch v-model="backgroundEnabled" aria-labelledby="receiving-background-label" :disabled="loading || saving" />
        <span class="setting-row__sub">默认补取范围</span>
        <el-select v-model="historyDays" class="setting-row__days" aria-label="默认补取范围" :disabled="loading || saving">
          <el-option v-for="days in [1, 7, 30, 90, 365, 3650]" :key="days" :label="`最近 ${days} 天`" :value="days" />
        </el-select>
        <el-button :loading="saving" :disabled="loading" @click="save">{{ saving ? "保存中…" : "保存设置" }}</el-button>
      </div>
    </div>
    <div class="setting-row">
      <span class="setting-row__label">历史邮件补取</span>
      <div class="setting-row__control">
        <el-select v-model="fetchDays" class="setting-row__days" aria-label="历史补取范围" :disabled="loading || jobBusy">
          <el-option v-for="days in [1, 7, 30, 90, 365, 3650]" :key="days" :label="`最近 ${days} 天`" :value="days" />
        </el-select>
        <el-button :disabled="loading || jobBusy || active" @click="start">开始补取</el-button>
      </div>
    </div>
    <div v-if="job" class="setting-row history-job">
      <span class="setting-row__label">补取任务</span>
      <div class="history-job__body">
        <div class="history-job__line">
          <span class="pg-dot" :class="jobTone">{{ statuses[job.status] || job.status }}</span>
          <span class="pg-num">{{ date(job.since) }} – {{ date(job.until) }}</span>
          <span class="pg-num">已检查 {{ job.scanned_count }}</span>
          <span class="pg-num">新增归档 {{ job.archived_count }}</span>
          <span v-if="job.initialized" class="pg-num pg-muted">UID {{ job.last_uid }} / {{ job.target_uid }}</span>
          <span v-if="job.next_run_at" class="pg-num pg-muted">下次尝试 {{ date(job.next_run_at) }}</span>
          <span class="history-job__actions">
            <el-button v-if="['running', 'queued', 'waiting'].includes(job.status)" size="small" :disabled="jobBusy" @click="act('pause')">暂停</el-button>
            <el-button v-if="['paused', 'failed'].includes(job.status)" size="small" type="primary" :disabled="jobBusy" @click="act('resume')">恢复</el-button>
            <el-button v-if="active || job.status === 'failed'" size="small" :disabled="jobBusy" @click="act('cancel')">取消任务</el-button>
          </span>
        </div>
        <p v-if="job.last_error" class="receiving-settings__error">{{ job.last_error }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.receiving-settings { display: flex; flex-direction: column; color: var(--text); font-size: 13px; }
.receiving-settings__error { margin: 0 0 8px; color: var(--danger); font-size: 12px; overflow-wrap: anywhere; }
.setting-row { display: grid; grid-template-columns: 140px minmax(0, 1fr); align-items: center; column-gap: 12px; min-height: 36px; padding: 4px 0; }
.setting-row + .setting-row { border-top: 1px solid var(--border); }
.setting-row__label { color: var(--text-secondary); }
.setting-row__control { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 12px; }
.setting-row__sub { margin-left: 12px; color: var(--text-secondary); }
.setting-row__days { width: 128px; }
.history-job { align-items: start; }
.history-job .setting-row__label { line-height: 28px; }
.history-job__line { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 16px; min-height: 28px; }
.history-job__actions { display: inline-flex; gap: 4px; }
.history-job__body .receiving-settings__error { margin: 4px 0 0; }
@media (max-width: 600px) {
  .setting-row { grid-template-columns: minmax(0, 1fr); row-gap: 4px; }
  .setting-row__sub { margin-left: 0; }
}
</style>
