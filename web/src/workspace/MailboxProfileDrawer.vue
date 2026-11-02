<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useRouter } from "vue-router";
import { useAuth } from "../stores/auth.js";
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
  <el-drawer :model-value="modelValue" title="邮箱资料" size="min(560px, 100vw)" :before-close="beforeClose" append-to-body class="mailbox-profile-drawer" @update:model-value="emit('update:modelValue', $event)">
    <div v-if="loading" class="profile-loading" role="status">正在读取邮箱资料…</div>
    <div v-if="error" class="profile-error" role="alert">{{ error }}<button v-if="!profile" class="btn" @click="load">重新读取</button></div>
    <template v-if="profile">
      <div class="profile-identity"><strong>{{ profile.address }}</strong><span>备注和服务登记保存在本系统中，不会修改 Apple 标签。</span></div>
      <section class="profile-section">
        <h3>邮箱备注</h3>
        <form @submit.prevent="saveNote"><label class="sr-only" for="mailbox-note-input">邮箱备注</label><el-input id="mailbox-note-input" v-model="note" type="textarea" :rows="3" :maxlength="2000" show-word-limit placeholder="例如：个人开发工具、工作用途；不要填写密码或验证码。" :disabled="busy" /><div class="profile-form-actions"><el-button :disabled="!dirtyNote || busy" :loading="savingNote" native-type="submit" type="primary">保存备注</el-button></div></form>
      </section>
      <section class="profile-section">
        <div class="profile-section-heading"><h3>服务登记</h3><el-button :disabled="busy || Boolean(serviceError)" @click="resetForm()">新增登记</el-button></div>
        <p class="profile-hint">未记录不代表未注册。请按实际情况登记；同一服务可记录多个账号。</p>
        <p v-if="serviceError" class="profile-error" role="alert">{{ serviceError }}<button class="btn" @click="load">重新加载</button></p>
        <div v-if="!profile.registrations.length" class="profile-empty">尚未记录服务使用情况。完成注册后，在这里登记。</div>
        <div v-for="entry in profile.registrations" :key="entry.id" class="registration-entry">
          <div class="registration-entry-head"><strong>{{ entry.service_name }}</strong><span :class="['registration-status', `is-${entry.status}`]">{{ registrationLabel(entry.status) }}</span></div>
          <span v-if="entry.account_identifier" class="registration-account">账号：{{ entry.account_identifier }}</span>
          <p v-if="entry.note">{{ entry.note }}</p>
          <div class="registration-entry-actions"><button :disabled="busy" @click="resetForm(entry)">编辑</button><button :disabled="busy" @click="remove(entry)">{{ removing === entry.id ? '移除中…' : '移除登记' }}</button></div>
        </div>
        <form v-if="formOpen" class="registration-form" @submit.prevent="submitRegistration">
          <h4>{{ editingId ? '编辑登记' : '新增服务登记' }}</h4>
          <label>服务<SearchableMailboxSelect v-model="formService" :options="serviceOptions" aria-label="登记服务" placeholder="搜索服务名称或域名" :disabled="busy || Boolean(editingId)" /></label>
          <p v-if="!services.length && !serviceError" class="profile-hint">还没有服务。<button type="button" class="profile-link" @click="manageServices">前往服务登记创建</button></p>
          <label>状态<el-select v-model="formStatus" aria-label="登记状态" :disabled="busy"><el-option v-for="state in registrationStates" :key="state.value" :value="state.value" :label="state.label" /></el-select></label>
          <label v-if="formStatus !== 'not_registered'">账号标识（可选）<el-input v-model="formAccount" :maxlength="200" :disabled="busy || Boolean(editingId)" placeholder="用于区分同一服务的多个账号，不填写密码" /></label>
          <label>登记说明（可选）<el-input v-model="formNote" type="textarea" :rows="2" :maxlength="2000" :disabled="busy" /></label>
          <p v-if="registrationError" class="profile-error" role="alert">{{ registrationError }}</p>
          <div class="profile-form-actions"><el-button :disabled="busy" @click="formOpen = false">取消</el-button><el-button type="primary" native-type="submit" :loading="savingRegistration" :disabled="busy || !formService">保存登记</el-button></div>
        </form>
        <p v-else-if="registrationError" class="profile-error" role="alert">{{ registrationError }}</p>
      </section>
    </template>
  </el-drawer>
</template>
<style>
.mailbox-profile-drawer .el-drawer__header { margin-bottom: 0; padding-bottom: 18px; border-bottom: 1px solid var(--border); }
.mailbox-profile-drawer .el-drawer__body { padding: 24px; }
.profile-identity { display: grid; gap: 8px; padding-bottom: 24px; }
.profile-identity strong { font-size: 16px; overflow-wrap: anywhere; }
.profile-identity span, .profile-hint { color: var(--text-secondary); font-size: 12px; line-height: 1.8; }
.profile-section { padding: 20px 0; border-top: 1px solid var(--border); }
.profile-section h3 { margin: 0 0 14px; font-size: 15px; font-weight: 600; }
.profile-section-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
.profile-section-heading h3 { margin-bottom: 0; }
.profile-section .profile-hint { margin: 12px 0; }
.profile-form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
.profile-error { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin: 12px 0; color: var(--danger); font-size: 13px; line-height: 1.8; overflow-wrap: anywhere; }
.profile-loading, .profile-empty { padding: 20px 0; color: var(--text-secondary); font-size: 13px; line-height: 1.8; }
.registration-entry { padding: 16px 0; border-top: 1px solid var(--border); }
.registration-entry-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.registration-entry-head strong { font-size: 14px; overflow-wrap: anywhere; }
.registration-status { color: var(--text-secondary); font-size: 12px; white-space: nowrap; }
.registration-status.is-registered { color: var(--success); }.registration-status.is-pending { color: var(--warning); }
.registration-account { display: block; margin-top: 7px; color: var(--text-secondary); font-size: 12px; overflow-wrap: anywhere; }
.registration-entry p { margin: 8px 0 0; font-size: 13px; line-height: 1.7; white-space: pre-wrap; overflow-wrap: anywhere; }
.registration-entry-actions { display: flex; gap: 16px; margin-top: 8px; }
.registration-entry-actions button, .profile-link { min-height: 32px; padding: 3px 0; border: 0; background: transparent; color: var(--primary); cursor: pointer; font: inherit; font-size: 12px; }
.registration-entry-actions button:disabled { opacity: .5; cursor: not-allowed; }
.registration-form { display: grid; gap: 14px; margin-top: 16px; padding-top: 20px; border-top: 1px solid var(--border); }
.registration-form h4 { margin: 0; font-size: 14px; font-weight: 600; }
.registration-form > label { display: grid; gap: 8px; color: var(--text-secondary); font-size: 13px; }
.registration-form .el-select, .registration-form .searchable-mailbox-select { width: 100%; }
@media(max-width:740px) { .mailbox-profile-drawer .el-drawer__body { padding: 18px; }.mailbox-profile-drawer .el-button, .registration-entry-actions button, .profile-link { min-height: 44px; }.mailbox-profile-drawer .el-drawer__close-btn { min-width: 44px; min-height: 44px; } }
</style>
