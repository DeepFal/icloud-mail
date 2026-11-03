<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useRouter } from "vue-router";
import { useAuth } from "../stores/auth.js";
import "../styles/pages.css";
import SearchableMailboxSelect from "../components/SearchableMailboxSelect.vue";
import { getMailboxProfile, getServices, saveMailboxNote, saveRegistration, removeRegistration, registrationStates, registrationLabel } from "./registry-api.js";
const props = defineProps({ modelValue: Boolean, aliasId: { type: [Number, String], default: "" }, serviceId: { type: [Number, String], default: "" } });
const emit = defineEmits(["update:modelValue", "updated"]);
const auth = useAuth(), router = useRouter();
const profile = ref(null), services = ref([]), note = ref(""), loading = ref(false), savingNote = ref(false), savingRegistration = ref(false), removing = ref(0);
const error = ref(""), serviceError = ref(""), registrationError = ref("");
const formOpen = ref(false), editingId = ref(0), formService = ref(""), formAccount = ref(""), formStatus = ref("pending"), formNote = ref("");
let version = 0, controller, disposed = false;
const busy = computed(() => savingNote.value || savingRegistration.value || Boolean(removing.value));
const dirtyNote = computed(() => profile.value && note.value !== profile.value.note);
const serviceOptions = computed(() => services.value.filter((item) => !item.archived || Number(formService.value) === item.id).map((item) => ({ value: String(item.id), label: item.name, secondary: [item.domain, item.archived ? '已归档' : ''].filter(Boolean).join(' · ') })));
function updateProfile(value) { profile.value = value; emit("updated", value); window.dispatchEvent(new CustomEvent("icloud:mailbox-profile-updated", { detail: { aliasId: value.alias_id, address: value.address, note: value.note } })); }
function resetForm(entry) {
  editingId.value = entry?.id || 0; formService.value = String(entry?.service_id || props.serviceId || "");
  formAccount.value = entry?.account_identifier || ""; formStatus.value = entry?.status || "pending"; formNote.value = entry?.note || "";
  registrationError.value = ""; formOpen.value = true;
}
async function load() {
  if (!props.modelValue || !props.aliasId) return;
  const current = ++version; controller?.abort(); controller = new AbortController();
  loading.value = true; error.value = ""; serviceError.value = ""; profile.value = null; formOpen.value = false; services.value = [];
  savingNote.value = false; savingRegistration.value = false; removing.value = 0;
  const results = await Promise.allSettled([getMailboxProfile(props.aliasId, { signal: controller.signal }), getServices({ includeArchived: true, signal: controller.signal })]);
  if (disposed || current !== version) return;
  if (results[0].status === "fulfilled") { profile.value = results[0].value; note.value = profile.value.note; }
  else if (results[0].reason.name !== "AbortError") error.value = results[0].reason.message || "邮箱信息读取失败，请重试。";
  if (results[1].status === "fulfilled") services.value = results[1].value;
  else if (results[1].reason.name !== "AbortError") serviceError.value = "服务目录暂时无法读取，请重新加载。";
  loading.value = false;
  if (props.serviceId && profile.value) resetForm();
}
async function saveNote() {
  if (busy.value || !profile.value || !dirtyNote.value) return;
  savingNote.value = true; error.value = "";
  const current = version, aliasId = props.aliasId;
  try { const saved = await saveMailboxNote(aliasId, note.value, auth.state.csrfToken); if (!disposed && current === version) { updateProfile(saved); note.value = saved.note; ElMessage.success("邮箱备注已保存"); } }
  catch (cause) { if (current === version) error.value = cause.message || "备注保存失败，输入已保留。"; }
  finally { if (current === version) savingNote.value = false; }
}
async function submitRegistration() {
  if (busy.value || !profile.value) return;
  if (!formService.value) { registrationError.value = "请选择服务。"; return; }
  savingRegistration.value = true; registrationError.value = "";
  const current = version;
  try {
    const saved = await saveRegistration(props.aliasId, { service_id: Number(formService.value), account_identifier: formStatus.value === "not_registered" ? "" : formAccount.value.trim(), status: formStatus.value, note: formNote.value }, auth.state.csrfToken);
    if (!disposed && current === version) { updateProfile(saved); formOpen.value = false; ElMessage.success("服务登记已保存"); }
  } catch (cause) { if (current === version) registrationError.value = cause.message || "登记保存失败，请重试。"; }
  finally { if (current === version) savingRegistration.value = false; }
}
async function remove(entry) {
  if (busy.value) return;
  try { await ElMessageBox.confirm(`移除这条 ${entry.service_name} 登记后，该记录不再参与筛选。这不会注销或删除服务上的实际账号。`, "移除本地登记", { confirmButtonText: "移除登记", cancelButtonText: "保留", type: "warning" }); } catch { return; }
  removing.value = entry.id; const current = version;
  try { const saved = await removeRegistration(props.aliasId, entry.id, auth.state.csrfToken); if (!disposed && current === version) updateProfile(saved); }
  catch (cause) { if (current === version) registrationError.value = cause.message || "移除失败，请重试。"; }
  finally { if (current === version) removing.value = 0; }
}
async function beforeClose(done) {
  if (busy.value) return;
  if (dirtyNote.value || formOpen.value) {
    try { await ElMessageBox.confirm("尚有未保存的输入。关闭后会丢弃这些输入。", "关闭邮箱资料", { confirmButtonText: "丢弃并关闭", cancelButtonText: "继续编辑" }); } catch { return; }
  }
  done();
}
function manageServices() { beforeClose(() => { emit("update:modelValue", false); router.push({ name: "services" }); }); }
watch(() => [props.modelValue, props.aliasId], () => { if (props.modelValue) load(); else { version++; controller?.abort(); } });
onBeforeUnmount(() => { disposed = true; version++; controller?.abort(); });
</script>
<template>
  <el-drawer :model-value="modelValue" title="邮箱资料" size="min(480px, 100vw)" :before-close="beforeClose" append-to-body class="pg-drawer mailbox-profile-drawer" @update:model-value="emit('update:modelValue', $event)">
    <p v-if="loading" class="mpd-muted" role="status">正在读取邮箱资料…</p>
    <p v-if="error" class="mpd-error" role="alert">{{ error }}<el-button v-if="!profile" link type="primary" @click="load">重新读取</el-button></p>
    <template v-if="profile">
      <div class="mpd-identity">
        <strong translate="no">{{ profile.address }}</strong>
        <span class="mpd-muted">备注和服务登记只保存在本系统，不修改 Apple 标签。</span>
      </div>
      <section class="mpd-section" aria-labelledby="mpd-note-title">
        <h3 id="mpd-note-title" class="mpd-heading">邮箱备注</h3>
        <form @submit.prevent="saveNote">
          <el-input id="mailbox-note-input" v-model="note" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" :maxlength="2000" show-word-limit placeholder="例如：个人开发工具、工作用途…（不要填写密码或验证码）" aria-labelledby="mpd-note-title" :disabled="busy" />
          <div class="mpd-actions"><el-button :disabled="!dirtyNote || busy" :loading="savingNote" native-type="submit" type="primary">保存备注</el-button></div>
        </form>
      </section>
      <section class="mpd-section" aria-labelledby="mpd-reg-title">
        <div class="mpd-heading-row">
          <h3 id="mpd-reg-title" class="mpd-heading">服务登记 <span class="pg-count">{{ profile.registrations.length }}</span></h3>
          <el-button size="small" :disabled="busy || Boolean(serviceError)" @click="resetForm()">新增登记</el-button>
        </div>
        <p v-if="serviceError" class="mpd-error" role="alert">{{ serviceError }}<el-button link type="primary" @click="load">重新加载</el-button></p>
        <p v-if="!profile.registrations.length" class="mpd-muted">还没有登记。未记录不代表未注册，完成注册后在这里登记。</p>
        <ul v-else class="mpd-list">
          <li v-for="entry in profile.registrations" :key="entry.id" class="mpd-entry">
            <div class="mpd-entry__line">
              <strong class="pg-ellipsis" :title="entry.service_name">{{ entry.service_name }}</strong>
              <span :class="['pg-badge', entry.status === 'registered' ? 'is-success' : entry.status === 'pending' ? 'is-warning' : '']">{{ registrationLabel(entry.status) }}</span>
              <span v-if="entry.account_identifier" class="mpd-muted pg-ellipsis" :title="entry.account_identifier" translate="no">{{ entry.account_identifier }}</span>
              <span class="mpd-entry__actions">
                <el-button link type="primary" :disabled="busy" @click="resetForm(entry)">编辑</el-button>
                <el-button link :disabled="busy" @click="remove(entry)">{{ removing === entry.id ? "移除中…" : "移除" }}</el-button>
              </span>
            </div>
            <p v-if="entry.note" class="mpd-entry__note">{{ entry.note }}</p>
          </li>
        </ul>
        <form v-if="formOpen" class="mpd-form" @submit.prevent="submitRegistration">
          <h4 class="mpd-heading">{{ editingId ? "编辑登记" : "新增登记" }}</h4>
          <label>服务<SearchableMailboxSelect v-model="formService" :options="serviceOptions" aria-label="登记服务" placeholder="搜索服务名称或域名…" :disabled="busy || Boolean(editingId)" /></label>
          <p v-if="!services.length && !serviceError" class="mpd-muted">还没有服务。<el-button link type="primary" @click="manageServices">前往服务登记创建</el-button></p>
          <label>状态<el-select v-model="formStatus" aria-label="登记状态" :disabled="busy"><el-option v-for="state in registrationStates" :key="state.value" :value="state.value" :label="state.label" /></el-select></label>
          <label v-if="formStatus !== 'not_registered'">账号标识（可选）<el-input v-model="formAccount" :maxlength="200" :disabled="busy || Boolean(editingId)" autocomplete="off" :spellcheck="false" placeholder="区分同一服务的多个账号，不填写密码" /></label>
          <label>登记说明（可选）<el-input v-model="formNote" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" :maxlength="2000" :disabled="busy" /></label>
          <p v-if="registrationError" class="mpd-error" role="alert">{{ registrationError }}</p>
          <div class="mpd-actions"><el-button :disabled="busy" @click="formOpen = false">取消</el-button><el-button type="primary" native-type="submit" :loading="savingRegistration" :disabled="busy || !formService">保存登记</el-button></div>
        </form>
        <p v-else-if="registrationError" class="mpd-error" role="alert">{{ registrationError }}</p>
      </section>
    </template>
  </el-drawer>
</template>
<style>
.mailbox-profile-drawer .mpd-muted { margin: 0; color: var(--text-muted); font-size: 12px; line-height: 16px; }
.mailbox-profile-drawer .mpd-error { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin: 8px 0; color: var(--danger); font-size: 12px; line-height: 16px; overflow-wrap: anywhere; }
.mailbox-profile-drawer .mpd-identity { display: grid; gap: 4px; padding-bottom: 12px; }
.mailbox-profile-drawer .mpd-identity strong { color: var(--text); font-family: var(--font-mono); font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
.mailbox-profile-drawer .mpd-section { display: flex; flex-direction: column; gap: 8px; padding: 12px 0; border-top: 1px solid var(--border); }
.mailbox-profile-drawer .mpd-heading { display: flex; align-items: center; gap: 6px; margin: 0; color: var(--text-secondary); font-size: 12px; line-height: 16px; font-weight: 600; }
.mailbox-profile-drawer .mpd-heading-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 28px; }
.mailbox-profile-drawer .mpd-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; }
.mailbox-profile-drawer .mpd-list { margin: 0; padding: 0; list-style: none; }
.mailbox-profile-drawer .mpd-entry { padding: 4px 0; border-bottom: 1px solid var(--border); }
.mailbox-profile-drawer .mpd-entry:first-child { border-top: 1px solid var(--border); }
.mailbox-profile-drawer .mpd-entry__line { display: flex; align-items: center; gap: 8px; min-height: 28px; min-width: 0; }
.mailbox-profile-drawer .mpd-entry__line > strong { flex: 0 1 auto; color: var(--text); font-weight: 500; }
.mailbox-profile-drawer .mpd-entry__actions { display: flex; flex: 0 0 auto; gap: 8px; margin-left: auto; }
.mailbox-profile-drawer .mpd-entry__note { margin: 0 0 4px; color: var(--text-secondary); font-size: 12px; line-height: 18px; white-space: pre-wrap; overflow-wrap: anywhere; }
.mailbox-profile-drawer .mpd-form { display: grid; gap: 12px; margin-top: 4px; padding-top: 12px; border-top: 1px solid var(--border); }
.mailbox-profile-drawer .mpd-form > label { display: grid; gap: 4px; color: var(--text-secondary); font-size: 12px; }
.mailbox-profile-drawer .mpd-form .el-select, .mailbox-profile-drawer .mpd-form .searchable-mailbox-select { width: 100%; }
@media (max-width: 740px) { .mailbox-profile-drawer .el-drawer__close-btn { min-width: 44px; min-height: 44px; } }
</style>
