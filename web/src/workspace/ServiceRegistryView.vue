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
import "../styles/pages.css";
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
function registrationTone(status) { return status === "registered" ? "is-success" : status === "pending" ? "is-warning" : ""; }
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
  <section class="pg pg--fill service-registry" aria-labelledby="registry-title">
    <header class="pg-bar">
      <h1 id="registry-title" class="pg-title">服务登记</h1>
      <SearchableMailboxSelect class="registry-find" model-value="" :options="aliasOptions" aria-label="查找邮箱资料" placeholder="查找邮箱资料…" @change="(id) => { if (id) openProfile(id); }" />
      <div class="pg-bar__end"><el-button :icon="Plus" @click="beginService()">新建服务</el-button></div>
    </header>
    <p v-if="metaError" class="registry-error" role="alert">{{ metaError }}<el-button link type="primary" @click="loadMeta">重读邮箱选项</el-button></p>
    <p v-if="serviceError" class="registry-error" role="alert">{{ serviceError }}<el-button link type="primary" @click="loadServices()">重新读取服务</el-button></p>
    <div class="registry-workspace">
      <aside class="service-directory" aria-label="服务目录">
        <div class="service-directory-head">
          <el-input v-model="serviceQuery" :prefix-icon="Search" maxlength="200" aria-label="搜索服务" placeholder="搜索服务或域名…" clearable :spellcheck="false" />
          <el-checkbox v-model="includeArchived" size="small">包含已归档</el-checkbox>
        </div>
        <div class="service-directory-list">
          <p v-if="servicesLoading" class="registry-muted" role="status">正在读取服务…</p>
          <template v-else>
            <button v-for="service in shownServices" :key="service.id" type="button" :class="['service-directory-item', { active: String(service.id) === serviceId }]" :aria-pressed="String(service.id) === serviceId" :disabled="batchBusy" @click="chooseService(service.id)">
              <strong class="pg-ellipsis">{{ service.name }}</strong>
              <small v-if="service.archived" class="pg-badge">已归档</small>
              <small v-else-if="service.domain" class="pg-ellipsis">{{ service.domain }}</small>
            </button>
            <p v-if="!shownServices.length" class="registry-muted">{{ services.length ? "没有匹配的服务。" : "还没有服务。" }}</p>
          </template>
        </div>
      </aside>
      <section class="service-mailboxes" aria-label="服务邮箱登记">
        <template v-if="currentService">
          <header class="service-detail-heading">
            <h2 class="pg-ellipsis">{{ currentService.name }}</h2>
            <span v-if="currentService.domain" class="pg-muted pg-ellipsis" translate="no">{{ currentService.domain }}</span>
            <span v-if="currentService.archived" class="pg-badge">已归档</span>
            <span v-if="currentService.note" class="service-description pg-ellipsis" :title="currentService.note">{{ currentService.note }}</span>
            <div class="service-detail-actions">
              <el-button size="small" :disabled="editorBusy || batchBusy" @click="beginService(currentService)">编辑服务</el-button>
              <el-button size="small" :disabled="editorBusy || batchBusy" @click="toggleArchive">{{ currentService.archived ? "恢复" : "归档" }}</el-button>
            </div>
          </header>
          <div v-if="selection.length" class="registry-filters registry-selection" role="toolbar" aria-label="批量操作">
            <span class="registry-selection__count">已选 {{ selection.length }} 个</span>
            <el-button size="small" type="primary" :disabled="batchBusy || currentService.archived" @click="batchError = ''; batchOpen = true">批量登记</el-button>
            <el-button size="small" :disabled="batchBusy" @click="selection = []">取消选择</el-button>
            <span class="registry-result-count">{{ total }} 个邮箱</span>
          </div>
          <div v-else class="registry-filters">
            <el-input v-model="query" class="registry-query" :prefix-icon="Search" aria-label="搜索登记邮箱" maxlength="200" placeholder="邮箱、备注或账号标识…" clearable :spellcheck="false" :disabled="batchBusy" />
            <el-select v-model="status" class="registry-status" aria-label="按登记状态筛选" placeholder="全部状态" :disabled="batchBusy"><el-option label="全部状态" value="" /><el-option label="未记录" value="unrecorded" /><el-option v-for="state in registrationStates" :key="state.value" :label="state.label" :value="state.value" /></el-select>
            <SearchableMailboxSelect v-model="accountId" class="registry-filter-select" :options="accountOptions" aria-label="按主号筛选登记" placeholder="所有主号" :disabled="batchBusy" />
            <SearchableMailboxSelect v-model="groupId" class="registry-filter-select" :options="groupOptions" aria-label="按分组筛选登记" placeholder="所有分组" :disabled="batchBusy" />
            <el-checkbox v-model="enabledOnly" size="small" :disabled="batchBusy">仅启用</el-checkbox>
            <span class="registry-result-count">{{ total }} 个邮箱</span>
            <el-button size="small" :icon="Refresh" :loading="listLoading" :disabled="batchBusy" aria-label="刷新登记列表" title="刷新" @click="loadMailboxes" />
          </div>
          <p v-if="listError" class="registry-error" role="alert">{{ listError }}<el-button link type="primary" :disabled="listLoading" @click="loadMailboxes">重试</el-button></p>
          <p v-if="listLoading && !mailboxes.length" class="registry-muted" role="status">正在读取邮箱登记…</p>
          <div v-else-if="!mailboxes.length && !listError" class="registry-empty">
            <p>没有符合条件的邮箱。没有登记的邮箱只会出现在“未记录”中。</p>
            <el-button size="small" @click="query = ''; status = ''; accountId = ''; groupId = ''; enabledOnly = false">清除筛选</el-button>
          </div>
          <div v-else class="registry-table-wrap" :aria-busy="listLoading">
            <table class="registry-table">
              <thead><tr>
                <th scope="col"><el-checkbox :model-value="allSelected" :indeterminate="partialSelected" :disabled="listLoading || batchBusy || !mailboxes.length" aria-label="选择本页邮箱" @change="selectPage" /></th>
                <th scope="col">邮箱与备注</th><th scope="col">分组</th><th scope="col">登记状态</th><th scope="col"><span class="sr-only">操作</span></th>
              </tr></thead>
              <tbody>
                <tr v-for="mailbox in mailboxes" :key="mailbox.alias_id" :class="{ selected: selection.includes(mailbox.alias_id) }">
                  <td><el-checkbox :model-value="selection.includes(mailbox.alias_id)" :disabled="batchBusy" :aria-label="`选择 ${mailbox.address}`" @change="(checked) => selectMailbox(mailbox.alias_id, checked)" /></td>
                  <td><div class="pg-cell"><strong :title="mailbox.address" translate="no">{{ mailbox.address }}</strong><small v-if="mailbox.note" :title="mailbox.note">{{ mailbox.note }}</small><small v-if="!mailbox.enabled" class="registry-disabled">已停用</small></div></td>
                  <td class="pg-ellipsis">{{ mailbox.group_name || "未分组" }}</td>
                  <td><span :class="['pg-badge', registrationTone(mailbox.status)]">{{ registrationLabel(mailbox.status) }}</span><small v-if="mailbox.registration_count > 1" class="registry-muted registry-count">{{ mailbox.registration_count }} 条</small></td>
                  <td><div class="registry-row-actions"><el-button link type="primary" size="small" :disabled="batchBusy" @click="openProfile(mailbox.alias_id)">资料</el-button><el-button v-if="!currentService.archived" link type="primary" size="small" :disabled="batchBusy" @click="openProfile(mailbox.alias_id, true)">登记</el-button></div></td>
                </tr>
              </tbody>
            </table>
            <div class="registry-mobile-records"><article v-for="mailbox in mailboxes" :key="mailbox.alias_id" class="registry-mobile-record"><el-checkbox :model-value="selection.includes(mailbox.alias_id)" :disabled="batchBusy" :aria-label="`选择 ${mailbox.address}`" @change="(checked) => selectMailbox(mailbox.alias_id, checked)"><strong>{{ mailbox.address }}</strong></el-checkbox><p v-if="mailbox.note">{{ mailbox.note }}</p><div><span :class="['pg-badge', registrationTone(mailbox.status)]">{{ registrationLabel(mailbox.status) }}</span><small v-if="!mailbox.enabled">已停用</small><el-button link type="primary" :disabled="batchBusy" @click="openProfile(mailbox.alias_id)">邮箱资料</el-button><el-button v-if="!currentService.archived" link type="primary" :disabled="batchBusy" @click="openProfile(mailbox.alias_id, true)">登记</el-button></div></article></div>
          </div>
          <footer v-if="pages > 1" class="registry-pager"><el-button size="small" :disabled="page <= 1 || listLoading || batchBusy" @click="changePage(-1)">上一页</el-button><span class="pg-num">{{ page }} / {{ pages }}</span><el-button size="small" :disabled="page >= pages || listLoading || batchBusy" @click="changePage(1)">下一页</el-button></footer>
        </template>
        <div v-else class="registry-empty"><p>还没有服务。创建服务后，可以登记已注册的邮箱，也能找出尚未记录的邮箱。</p><el-button size="small" type="primary" :disabled="servicesLoading" @click="beginService()">创建第一个服务</el-button></div>
      </section>
    </div>
    <el-dialog v-model="editorOpen" :title="editingServiceId ? '编辑服务' : '新建服务'" width="min(480px, 94vw)" class="pg-dialog" :close-on-click-modal="!editorBusy" :show-close="!editorBusy" append-to-body>
      <form id="service-editor-form" class="service-editor" @submit.prevent="saveService">
        <label>服务名称<el-input v-model="editorName" :maxlength="100" placeholder="例如：GitHub" autocomplete="off" :disabled="editorBusy" /></label>
        <label>域名（可选）<el-input v-model="editorDomain" :maxlength="253" placeholder="例如：github.com" autocomplete="off" :spellcheck="false" :disabled="editorBusy" /></label>
        <label>服务说明（可选）<el-input v-model="editorNote" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" :maxlength="2000" :disabled="editorBusy" /></label>
        <p v-if="editorError" role="alert" class="registry-error">{{ editorError }}</p>
      </form>
      <template #footer><el-button :disabled="editorBusy" @click="editorOpen = false">取消</el-button><el-button type="primary" native-type="submit" form="service-editor-form" :loading="editorBusy" :disabled="editorBusy">保存服务</el-button></template>
    </el-dialog>
    <el-dialog v-model="batchOpen" :title="currentService ? `批量登记 ${currentService.name}` : '批量登记'" width="min(480px, 94vw)" class="pg-dialog" :close-on-click-modal="!batchBusy" :show-close="!batchBusy" append-to-body>
      <form id="registry-batch-form" class="registry-batch-form" @submit.prevent="saveBatch">
        <p class="registry-muted">为已选的 {{ selection.length }} 个邮箱保存本地登记。若它们使用不同账号标识，请分别在邮箱资料中登记。</p>
        <label>登记状态<el-select v-model="batchStatus" aria-label="批量登记状态" :disabled="batchBusy"><el-option v-for="state in registrationStates" :key="state.value" :value="state.value" :label="state.label" /></el-select></label>
        <label>共同说明（可选）<el-input v-model="batchNote" :maxlength="2000" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" :disabled="batchBusy" /></label>
        <p v-if="batchError" class="registry-error" role="alert">{{ batchError }}</p>
      </form>
      <template #footer><el-button :disabled="batchBusy" @click="batchOpen = false">取消</el-button><el-button native-type="submit" form="registry-batch-form" type="primary" :loading="batchBusy" :disabled="!selection.length || batchBusy">检查并保存</el-button></template>
    </el-dialog>
    <MailboxProfileDrawer v-model="profileOpen" :alias-id="profileAliasId" :service-id="profileServiceId" @updated="profileUpdated" />
  </section>
</template>
