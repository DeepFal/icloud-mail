<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Search, Refresh } from "@element-plus/icons-vue";
import { useAuth } from "../stores/auth.js";
import { apiRequest } from "../api/client.js";
import SearchableMailboxSelect from "../components/SearchableMailboxSelect.vue";
import MailboxProfileDrawer from "./MailboxProfileDrawer.vue";
import { getServices, createService, updateService, getServiceMailboxes, batchSaveRegistrations, registrationStates, registrationLabel } from "./registry-api.js";
import "./service-registry.css";
const auth = useAuth(), route = useRoute(), router = useRouter();
const services = ref([]), accounts = ref([]), groups = ref([]), aliases = ref([]);
const includeArchived = ref(false), serviceQuery = ref(""), serviceId = ref(String(route.query.service_id || ""));
const servicesLoading = ref(false), listLoading = ref(false), metaError = ref(""), serviceError = ref(""), listError = ref("");
const query = ref(""), status = ref(""), accountId = ref(""), groupId = ref(""), enabledOnly = ref(false), page = ref(1), total = ref(0), mailboxes = ref([]);
const selection = ref([]), batchOpen = ref(false), batchStatus = ref("registered"), batchNote = ref(""), batchBusy = ref(false), batchError = ref("");
const profileOpen = ref(false), profileAliasId = ref(""), profileServiceId = ref("");
const editorOpen = ref(false), editingServiceId = ref(""), editorName = ref(""), editorDomain = ref(""), editorNote = ref(""), editorBusy = ref(false), editorError = ref("");
let serviceVersion = 0, listVersion = 0, disposed = false, listController, serviceController, metaController, debounce;
const currentService = computed(() => services.value.find((item) => String(item.id) === serviceId.value));
const shownServices = computed(() => {
  const value = serviceQuery.value.trim().toLocaleLowerCase();
  return services.value.filter((item) => !value || [item.name, item.domain, item.note].some((s) => String(s || "").toLocaleLowerCase().includes(value)));
});
const serviceOptions = computed(() => services.value.map((item) => ({ value: String(item.id), label: item.name, secondary: [item.domain, item.archived ? '已归档' : ''].filter(Boolean).join(' · ') })));
const accountOptions = computed(() => accounts.value.map((item) => ({ value: String(item.id), label: item.email, secondary: item.name })));
const groupOptions = computed(() => groups.value.map((item) => ({ value: String(item.id), label: item.name })));
const aliasOptions = computed(() => aliases.value.map((item) => ({ value: String(item.id), label: item.address, note: item.note })));
const pages = computed(() => Math.max(1, Math.ceil(total.value / 50)));
const allSelected = computed(() => mailboxes.value.length > 0 && mailboxes.value.every((item) => selection.value.includes(item.alias_id)));
const partialSelected = computed(() => selection.value.length > 0 && !allSelected.value);
async function loadServices({ preferId } = {}) {
  const version = ++serviceVersion; serviceController?.abort(); serviceController = new AbortController();
  servicesLoading.value = true; serviceError.value = "";
  try {
    const rows = await getServices({ includeArchived: includeArchived.value, signal: serviceController.signal });
    if (disposed || version !== serviceVersion) return;
    services.value = rows;
    const target = String(preferId || serviceId.value);
    const next = rows.some((item) => String(item.id) === target) ? target : String(rows[0]?.id || "");
    if (next === serviceId.value) loadMailboxes();
    else serviceId.value = next;
  } catch (cause) { if (cause.name !== "AbortError" && !disposed && version === serviceVersion) serviceError.value = cause.message || "服务目录读取失败。"; }
  finally { if (!disposed && version === serviceVersion) servicesLoading.value = false; }
}
async function loadMeta() {
  metaController?.abort(); metaController = new AbortController(); metaError.value = "";
  try { const data = await apiRequest("/workspace/options", { signal: metaController.signal }); if (!disposed) { accounts.value = data.accounts; aliases.value = data.aliases; groups.value = data.groups; } }
  catch (cause) { if (!disposed && cause.name !== "AbortError") metaError.value = cause.message || "邮箱选项读取失败。"; }
}
async function loadMailboxes() {
  const version = ++listVersion; listController?.abort();
  if (!serviceId.value) { mailboxes.value = []; total.value = 0; listLoading.value = false; return; }
  listController = new AbortController(); listLoading.value = true; listError.value = "";
  try {
    const result = await getServiceMailboxes({ serviceId: serviceId.value, query: query.value, status: status.value, accountId: accountId.value, groupId: groupId.value, enabledOnly: enabledOnly.value, offset: (page.value - 1) * 50, signal: listController.signal });
    if (disposed || version !== listVersion) return;
    mailboxes.value = result.items; total.value = result.total;
    if (page.value > pages.value) { page.value = pages.value; loadMailboxes(); }
  } catch (cause) { if (!disposed && version === listVersion && cause.name !== "AbortError") listError.value = cause.message || "邮箱登记读取失败。"; }
  finally { if (!disposed && version === listVersion) listLoading.value = false; }
}
function chooseService(value) { serviceId.value = String(value || ""); }
function selectMailbox(id, checked) { selection.value = checked ? [...new Set([...selection.value, id])] : selection.value.filter((value) => value !== id); }
function selectPage(checked) { selection.value = checked ? mailboxes.value.map((item) => item.alias_id) : []; }
function changePage(delta) { if (listLoading.value || page.value + delta < 1 || page.value + delta > pages.value) return; selection.value = []; page.value += delta; loadMailboxes(); }
function openProfile(aliasId, register = false) { profileAliasId.value = aliasId; profileServiceId.value = register ? serviceId.value : ""; profileOpen.value = true; }
function profileUpdated(profile) {
  const alias = aliases.value.find((item) => item.address === profile.address); if (alias) alias.note = profile.note;
  loadMailboxes();
}
function beginService(service) {
  editingServiceId.value = service?.id || ""; editorName.value = service?.name || ""; editorDomain.value = service?.domain || ""; editorNote.value = service?.note || "";
  editorError.value = ""; editorOpen.value = true;
}
async function saveService() {
  if (editorBusy.value) return;
  if (!editorName.value.trim()) { editorError.value = "请填写服务名称。"; return; }
  editorBusy.value = true; editorError.value = "";
  const body = { name: editorName.value.trim(), domain: editorDomain.value.trim(), note: editorNote.value };
  try { const saved = editingServiceId.value ? await updateService(editingServiceId.value, body, auth.state.csrfToken) : await createService(body, auth.state.csrfToken); if (disposed) return; editorOpen.value = false; await loadServices({ preferId: saved.id }); ElMessage.success("服务已保存"); }
  catch (cause) { if (!disposed) editorError.value = cause.message || "保存失败，输入已保留。"; }
  finally { if (!disposed) editorBusy.value = false; }
}
async function toggleArchive() {
  const service = currentService.value; if (!service || editorBusy.value) return;
  try { await ElMessageBox.confirm(service.archived ? "恢复后可以为这个服务新增登记。历史登记会继续保留。" : "归档后保留已有登记，停止新增登记。仍可通过“包含已归档”查询和恢复。", service.archived ? "恢复服务" : "归档服务", { confirmButtonText: service.archived ? "恢复服务" : "归档", cancelButtonText: "取消" }); } catch { return; }
  editorBusy.value = true;
  try { await updateService(service.id, { archived: !service.archived }, auth.state.csrfToken); if (disposed) return; if (!service.archived) includeArchived.value = true; await loadServices({ preferId: service.id }); }
  catch (cause) { if (!disposed) serviceError.value = cause.message || "服务状态保存失败。"; }
  finally { if (!disposed) editorBusy.value = false; }
}
async function saveBatch() {
  if (batchBusy.value || !selection.value.length || !currentService.value) return;
  const ids = [...selection.value], service = currentService.value, chosenStatus = batchStatus.value, note = batchNote.value;
  try { await ElMessageBox.confirm(`为 ${ids.length} 个邮箱登记“${service.name} · ${registrationLabel(chosenStatus)}”。本次修改的是本系统中的记录，不会在服务上注册或注销账号。若已有相同登记，将更新其状态与说明。`, "确认批量登记", { confirmButtonText: "确认保存", cancelButtonText: "返回检查" }); } catch { return; }
  batchBusy.value = true; batchError.value = "";
  try { const result = await batchSaveRegistrations({ alias_ids: ids, service_id: service.id, status: chosenStatus, account_identifier: "", note }, auth.state.csrfToken); if (disposed) return; selection.value = []; batchOpen.value = false; batchNote.value = ""; ElMessage.success(`已保存 ${result.updated} 个邮箱的登记`); await loadMailboxes(); }
  catch (cause) { if (!disposed) batchError.value = cause.message || "批量登记失败，所选邮箱已保留。"; }
  finally { if (!disposed) batchBusy.value = false; }
}
watch(includeArchived, () => loadServices());
watch(() => route.query.service_id, (value) => { if (value) serviceId.value = String(value); });
watch(serviceId, (value) => { if (value) router.replace({ query: { ...route.query, service_id: value } }); });
watch(() => [serviceId.value, status.value, accountId.value, groupId.value, enabledOnly.value, query.value], (values, old) => {
  clearTimeout(debounce); listVersion++; listController?.abort(); selection.value = []; batchOpen.value = false; page.value = 1;
  const onlyQuery = values.slice(0,-1).every((value,index) => value === old[index]);
  if (onlyQuery) debounce = setTimeout(loadMailboxes, 250); else loadMailboxes();
});
onMounted(() => { loadMeta(); loadServices(); });
onBeforeUnmount(() => { disposed = true; serviceVersion++; listVersion++; clearTimeout(debounce); listController?.abort(); serviceController?.abort(); metaController?.abort(); });
</script>
<template>
  <section class="service-registry">
    <header class="registry-heading"><h1>服务登记</h1><div class="registry-heading-actions"><SearchableMailboxSelect model-value="" :options="aliasOptions" aria-label="查找邮箱资料" placeholder="查找邮箱资料…" @change="(id) => { if (id) openProfile(id); }" /><el-button :icon="Plus" @click="beginService()">新建服务</el-button></div></header>
    <p class="registry-intro">记录每个邮箱在哪些服务注册了账号。登记由你确认，“未记录”不等于“未注册”。</p>
    <p v-if="metaError" class="registry-error" role="alert">{{ metaError }}<el-button @click="loadMeta">重读邮箱选项</el-button></p>
    <p v-if="serviceError" class="registry-error" role="alert">{{ serviceError }}<el-button @click="loadServices()">重新读取服务</el-button></p>
    <form v-if="editorOpen" class="service-editor" @submit.prevent="saveService">
      <h2>{{ editingServiceId ? '编辑服务' : '新建服务' }}</h2>
      <div class="service-editor-fields"><label>服务名称<el-input v-model="editorName" :maxlength="100" placeholder="例如：GitHub" :disabled="editorBusy" /></label><label>域名（可选）<el-input v-model="editorDomain" :maxlength="253" placeholder="例如：github.com" :disabled="editorBusy" /></label><label class="service-note-field">服务说明（可选）<el-input v-model="editorNote" type="textarea" :rows="2" :maxlength="2000" :disabled="editorBusy" /></label></div>
      <p v-if="editorError" role="alert" class="registry-error">{{ editorError }}</p>
      <div class="service-editor-actions"><el-button :disabled="editorBusy" @click="editorOpen = false">取消</el-button><el-button type="primary" native-type="submit" :loading="editorBusy" :disabled="editorBusy">保存服务</el-button></div>
    </form>
    <div class="registry-workspace">
      <aside class="service-directory" aria-label="服务目录">
        <label class="service-directory-search"><Search /><input v-model="serviceQuery" maxlength="200" aria-label="搜索服务" placeholder="搜索服务或域名" /></label>
        <label class="service-archive-filter"><input v-model="includeArchived" type="checkbox" />包含已归档</label>
        <div v-if="servicesLoading" class="registry-empty" role="status">正在读取服务…</div>
        <template v-else><button v-for="service in shownServices" :key="service.id" :class="['service-directory-item', { active: String(service.id) === serviceId }]" :aria-pressed="String(service.id) === serviceId" :disabled="batchBusy" @click="chooseService(service.id)"><strong>{{ service.name }}<span v-if="service.archived">已归档</span></strong><small v-if="service.domain">{{ service.domain }}</small></button><div v-if="!shownServices.length" class="registry-empty">{{ services.length ? '没有匹配的服务，试试其他名称。' : '还没有服务。先创建一个，再登记邮箱。' }}</div></template>
      </aside>
      <section class="service-mailboxes" aria-label="服务邮箱登记">
        <template v-if="currentService">
          <header class="service-detail-heading"><div><h2>{{ currentService.name }}<span v-if="currentService.archived" class="service-archived">已归档</span></h2><p v-if="currentService.domain">{{ currentService.domain }}</p></div><div><el-button :disabled="editorBusy || batchBusy" @click="beginService(currentService)">编辑服务</el-button><el-button :disabled="editorBusy || batchBusy" @click="toggleArchive">{{ currentService.archived ? '恢复' : '归档' }}</el-button></div></header>
          <p v-if="currentService.note" class="service-description">{{ currentService.note }}</p>
          <div class="registry-filters"><label class="registry-query"><Search /><input v-model="query" aria-label="搜索登记邮箱" maxlength="200" placeholder="搜索邮箱、备注或账号标识…" :disabled="batchBusy" /></label><el-select v-model="status" aria-label="按登记状态筛选" placeholder="全部登记状态" :disabled="batchBusy"><el-option label="全部登记状态" value="" /><el-option label="未记录" value="unrecorded" /><el-option v-for="state in registrationStates" :key="state.value" :label="state.label" :value="state.value" /></el-select></div>
          <div class="registry-secondary-filters"><SearchableMailboxSelect v-model="accountId" :options="accountOptions" aria-label="按主号筛选登记" placeholder="所有主号" :disabled="batchBusy" /><SearchableMailboxSelect v-model="groupId" :options="groupOptions" aria-label="按分组筛选登记" placeholder="所有分组" :disabled="batchBusy" /><label><input v-model="enabledOnly" type="checkbox" :disabled="batchBusy" />仅启用的邮箱</label><el-button :icon="Refresh" :loading="listLoading" :disabled="batchBusy" @click="loadMailboxes">刷新</el-button></div>
          <div class="registry-selection"><label><input type="checkbox" :checked="allSelected" :indeterminate="partialSelected" :disabled="listLoading || batchBusy || !mailboxes.length" aria-label="选择本页邮箱" @change="selectPage($event.target.checked)" />本页全选</label><span>已选 {{ selection.length }} 个</span><button v-if="selection.length" :disabled="batchBusy" @click="selection = []">清空</button><el-button :disabled="!selection.length || batchBusy || currentService.archived" @click="batchError = ''; batchOpen = !batchOpen">批量登记</el-button><span class="registry-result-count">{{ total }} 个邮箱</span></div>
          <form v-if="batchOpen" class="registry-batch-form" @submit.prevent="saveBatch"><strong>为 {{ selection.length }} 个邮箱登记 {{ currentService.name }}</strong><label>登记状态<el-select v-model="batchStatus" aria-label="批量登记状态" :disabled="batchBusy"><el-option v-for="state in registrationStates" :key="state.value" :value="state.value" :label="state.label" /></el-select></label><label>共同说明（可选）<el-input v-model="batchNote" :maxlength="2000" type="textarea" :rows="2" :disabled="batchBusy" /></label><p class="profile-hint">仅保存本地登记。选中的多个邮箱若使用不同账号标识，请分别在邮箱资料中登记。</p><p v-if="batchError" class="registry-error" role="alert">{{ batchError }}</p><div class="service-editor-actions"><el-button :disabled="batchBusy" @click="batchOpen = false">取消</el-button><el-button native-type="submit" type="primary" :loading="batchBusy" :disabled="!selection.length || batchBusy">检查并保存</el-button></div></form>
          <p v-if="listError" class="registry-error" role="alert">{{ listError }}<el-button :disabled="listLoading" @click="loadMailboxes">重试</el-button></p>
          <div v-if="listLoading && !mailboxes.length" class="registry-empty" role="status">正在读取邮箱登记…</div>
          <div v-else-if="!mailboxes.length && !listError" class="registry-empty"><strong>没有符合条件的邮箱</strong><p>调整登记状态、主号或关键词。没有登记的邮箱只会出现在“未记录”中。</p><el-button @click="query = ''; status = ''; accountId = ''; groupId = ''; enabledOnly = false">清除筛选</el-button></div>
          <div v-else class="registry-table-wrap" :aria-busy="listLoading">
            <table class="registry-table"><thead><tr><th scope="col"><span class="sr-only">选择</span></th><th scope="col">邮箱与备注</th><th scope="col">分组</th><th scope="col">登记状态</th><th scope="col">操作</th></tr></thead><tbody><tr v-for="mailbox in mailboxes" :key="mailbox.alias_id" :class="{ selected: selection.includes(mailbox.alias_id) }"><td><input type="checkbox" :checked="selection.includes(mailbox.alias_id)" :disabled="batchBusy" :aria-label="`选择 ${mailbox.address}`" @change="selectMailbox(mailbox.alias_id, $event.target.checked)" /></td><td><strong>{{ mailbox.address }}</strong><small v-if="mailbox.note" :title="mailbox.note">{{ mailbox.note }}</small><small v-if="!mailbox.enabled">邮箱或所属主号已停用</small></td><td>{{ mailbox.group_name || '未分组' }}</td><td><span :class="['registration-status', `is-${mailbox.status}`]">{{ registrationLabel(mailbox.status) }}</span><small v-if="mailbox.registration_count > 1">{{ mailbox.registration_count }} 条登记</small></td><td><button :disabled="batchBusy" @click="openProfile(mailbox.alias_id)">资料</button><button v-if="!currentService.archived" :disabled="batchBusy" @click="openProfile(mailbox.alias_id, true)">登记</button></td></tr></tbody></table>
            <div class="registry-mobile-records"><article v-for="mailbox in mailboxes" :key="mailbox.alias_id" class="registry-mobile-record"><label><input type="checkbox" :checked="selection.includes(mailbox.alias_id)" :disabled="batchBusy" :aria-label="`选择 ${mailbox.address}`" @change="selectMailbox(mailbox.alias_id, $event.target.checked)" /><strong>{{ mailbox.address }}</strong></label><p v-if="mailbox.note">{{ mailbox.note }}</p><div><span :class="['registration-status', `is-${mailbox.status}`]">{{ registrationLabel(mailbox.status) }}</span><small v-if="!mailbox.enabled">已停用</small><button :disabled="batchBusy" @click="openProfile(mailbox.alias_id)">邮箱资料</button><button v-if="!currentService.archived" :disabled="batchBusy" @click="openProfile(mailbox.alias_id, true)">登记</button></div></article></div>
          </div>
          <footer class="registry-pager"><el-button :disabled="page <= 1 || listLoading || batchBusy" @click="changePage(-1)">上一页</el-button><span>{{ page }} / {{ pages }} 页</span><el-button :disabled="page >= pages || listLoading || batchBusy" @click="changePage(1)">下一页</el-button></footer>
        </template>
        <div v-else class="registry-welcome"><h2>从一个服务开始</h2><p>创建服务后，可以登记已注册的邮箱，也可以找出尚未记录或确认未注册的邮箱。</p><el-button :disabled="servicesLoading" @click="beginService()">创建第一个服务</el-button></div>
      </section>
    </div>
    <MailboxProfileDrawer v-model="profileOpen" :alias-id="profileAliasId" :service-id="profileServiceId" @updated="profileUpdated" />
  </section>
</template>
