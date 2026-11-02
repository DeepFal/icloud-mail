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
    <p v-if="error" class="error" role="alert">{{ error }} <button @click="load">重新读取</button></p>
    <div class="setting-card"><div class="card-head"><div><h3>后台常驻收件</h3></div><ElSwitch v-model="backgroundEnabled" aria-label="后台常驻收件" :disabled="loading || saving" /></div><div class="control-row"><label>默认补取范围<select v-model.number="historyDays" :disabled="loading || saving"><option v-for="days in [1, 7, 30, 90, 365, 3650]" :key="days" :value="days">最近 {{ days }} 天</option></select></label><button class="btn btn-primary" :disabled="loading || saving" @click="save">{{ saving ? '保存中…' : '保存设置' }}</button></div></div>
    <div class="setting-card"><h3>历史邮件补取</h3><div class="control-row"><select v-model.number="fetchDays" aria-label="历史补取范围" :disabled="loading || jobBusy"><option v-for="days in [1, 7, 30, 90, 365, 3650]" :key="days" :value="days">最近 {{ days }} 天</option></select><button class="btn btn-primary" :disabled="loading || jobBusy || active" @click="start">开始补取</button></div>
      <div v-if="job" class="history-job"><div class="job-line"><strong>{{ statuses[job.status] || job.status }}</strong><span>{{ date(job.since) }} — {{ date(job.until) }}</span></div><p>已检查 {{ job.scanned_count }} 封候选邮件，新增归档 {{ job.archived_count }} 封。</p><p v-if="job.initialized" class="job-position">扫描位置 UID {{ job.last_uid }} / {{ job.target_uid }}</p><p v-if="job.last_error" class="error">{{ job.last_error }}</p><p v-if="job.next_run_at">下次尝试：{{ date(job.next_run_at) }}</p><div class="job-actions"><button v-if="['running', 'queued', 'waiting'].includes(job.status)" class="btn" :disabled="jobBusy" @click="act('pause')">暂停</button><button v-if="['paused', 'failed'].includes(job.status)" class="btn btn-primary" :disabled="jobBusy" @click="act('resume')">恢复</button><button v-if="active || job.status === 'failed'" class="btn" :disabled="jobBusy" @click="act('cancel')">取消任务</button></div></div>
    </div>
  </section>
</template>
<style scoped>
.receiving-settings>.error{grid-column:1/-1}.receiving-settings{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;font-size:12px;margin:0;color:var(--text)}.setting-card{padding:16px;border:1px solid var(--border);border-radius:9px;background:var(--surface)}h3{font-size:14px;font-weight:550;margin:0}p{margin:8px 0 15px;color:var(--text-secondary);line-height:1.8}.card-head,.control-row,.job-line,.job-actions{display:flex;align-items:center;justify-content:space-between;gap:16px}.control-row{margin-top:15px}label{display:flex;align-items:center;gap:12px;color:var(--text-secondary)}select{border:1px solid var(--border);border-radius:6px;padding:8px;background:var(--surface);color:var(--text);font:inherit}.history-job{margin-top:18px;border-top:1px solid var(--border);padding-top:16px}.job-line span,.job-position{font-size:10px;color:var(--text-secondary)}.job-actions{justify-content:flex-start}.error{color:var(--danger);overflow-wrap:anywhere}.error button{color:var(--primary);background:transparent;border:0;cursor:pointer}@media(max-width:900px){.receiving-settings{grid-template-columns:1fr}}@media(max-width:600px){.card-head{align-items:flex-start}.control-row,.job-line{flex-wrap:wrap}label{flex-wrap:wrap}.setting-card{padding:16px}}
</style>
