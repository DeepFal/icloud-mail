<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { ArrowLeft, ArrowRight, Box, Close, Collection, CopyDocument, Delete, Download, Message, Plus, Refresh, Search, Star, StarFilled } from "@element-plus/icons-vue";
import { useAuth } from "../stores/auth.js";
import { apiRequest } from "../api/client.js";
import { createLiveRefresh } from "../utils/liveRefresh.js";
import { batchMessages, getMessage, getMessages, messagePath, patchMessage } from "./mail-api.js";
import { formatMailDate } from "./mail-format.js";
import { setUnreadBadge } from "../stores/page.js";
import { messageKey, useInboxMessages } from "./inbox-state.js";
import SearchableMailboxSelect from "../components/SearchableMailboxSelect.vue";
import MailboxProfileDrawer from "./MailboxProfileDrawer.vue";
import "./inbox.css";

const auth = useAuth(), route = useRoute(), router = useRouter();
const accounts = ref([]), aliases = ref([]), groups = ref([]);
const metaLoading = ref(true), metaError = ref("");
const lastAction = ref(null), undoing = ref(false), page = ref(1);
const filterOptionsOpen = ref(false), profileOpen = ref(false);
const accountOptions = computed(() => accounts.value.map((item) => ({ value: String(item.id), label: item.email, secondary: item.name })));
const aliasOptions = computed(() => aliases.value.filter((item) => !filters.accountId || String(item.accountId) === filters.accountId).map((item) => ({ value: String(item.id), label: item.address, note: item.note })));
const groupOptions = computed(() => groups.value.map((item) => ({ value: String(item.id), label: item.name })));
const filters = reactive({ folder: "inbox", query: "", accountId: String(route.query.account_id || ""), aliasId: String(route.query.alias_id || ""), groupId: "", unread: false, starred: false, hasOtp: false });
const inbox = useInboxMessages({ filters, page, getMessages, getMessage, patchMessage, batchMessages, getCsrfToken: () => auth.state.csrfToken, onError: (message) => ElMessage.error(message) });
const { messages, selected, detail, readerOpen, viewMode, loading, detailLoading, error, detailError, total, unreadCount, busy, pages, selectedIndex, previousMessage, nextMessage, loadList, selectMessage, mutate, batch, cancelList, resetReader } = inbox;
let searchTimer, metaController, metaVersion = 0, disposed = false;
const liveRefresh = createLiveRefresh(() => loadList({ silent: true }));
const sourcePath = computed(() => selected.value ? messagePath(`/mailboxes/${selected.value.aliasId}/messages/${selected.value.mailboxUid}/source`) : "");
const hasFilters = computed(() => Boolean(filters.query.trim() || filters.accountId || filters.aliasId || filters.groupId || filters.unread || filters.starred || filters.hasOtp));
const optionFilterCount = computed(() => [filters.accountId, filters.aliasId, filters.groupId].filter(Boolean).length);
const historyTarget = computed(() => {
  const account = accounts.value.find((item) => String(item.id) === filters.accountId) || accounts.value.find((item) => item.enabled);
  return account ? { name: "account-detail", params: { id: account.id } } : { name: "accounts" };
});
const emptyState = computed(() => {
  if (hasFilters.value) return { title: "没有符合条件的邮件", description: "换个关键词，或清除筛选后查看全部结果。", action: "清除筛选", kind: "filters" };
  if (!metaLoading.value && !metaError.value && !accounts.value.length) return { title: "接收你的第一封邮件", description: "添加主号并完成收件连接，来信就会出现在这里。", action: "添加主号", kind: "setup" };
  if (filters.folder === "archived") return { title: "还没有归档邮件", description: "归档后的邮件会保留在这里，随时可以移回收件箱。" };
  if (filters.folder === "trash") return { title: "垃圾箱是空的", description: "移到这里的邮件可以恢复到原来的位置。" };
  return { title: "暂时没有新来信", description: "主号收到的新邮件会自动显示在这里，也可以在主号设置中补取历史邮件。", action: "补取历史邮件", kind: "history" };
});
const pinnedMessage = computed(() => {
  if (!selected.value || selectedIndex.value >= 0) return "";
  if (filters.unread && selected.value.read) return "已标记已读，你可以继续阅读这封邮件。";
  if (filters.starred && !selected.value.starred) return "已取消星标，你可以继续阅读这封邮件。";
  return "这封邮件已不在当前列表中，正文仍为你保留。";
});
const sender = (mail) => mail.from?.[0]?.name || mail.from?.[0]?.address || "未知发件人";
const date = formatMailDate;
function contentStatus(state) { return ({ missing: "归档文件缺失", metadata_only: "仅保存了邮件信息", oversized: "邮件超过归档大小限制", evicted: "正文已按存储策略清理" })[state] || "正文当前不可用"; }
async function loadMeta() {
  const version = ++metaVersion;
  metaController?.abort(); metaController = new AbortController();
  metaLoading.value = true; metaError.value = "";
  try {
    const options = await apiRequest("/workspace/options", { signal: metaController.signal });
    if (disposed || version !== metaVersion) return;
    accounts.value = options.accounts || []; aliases.value = options.aliases || []; groups.value = options.groups || [];
  } catch (cause) { if (!disposed && version === metaVersion && cause.name !== "AbortError") metaError.value = cause.message || "筛选信息加载失败。"; }
  finally { if (!disposed && version === metaVersion) metaLoading.value = false; }
}
function clearFilters() { Object.assign(filters, { query: "", accountId: "", aliasId: "", groupId: "", unread: false, starred: false, hasOtp: false }); }
function emptyAction() {
  if (emptyState.value.kind === "filters") clearFilters();
  else router.push(emptyState.value.kind === "setup" ? { name: "account-new" } : historyTarget.value);
}
function retryDetail() { selectMessage(selected.value, { open: readerOpen.value, markRead: readerOpen.value }); }
function changePage(delta) {
  const target = page.value + delta;
  if (loading.value || target < 1 || target > pages.value) return;
  checked.clear(); resetReader(); page.value = target; loadList({ keep: false });
}
const selectMode = ref(false), checked = reactive(new Map());
const checkedMails = computed(() => messages.value.filter((mail) => checked.has(messageKey(mail))));
const allChecked = computed(() => messages.value.length > 0 && checkedMails.value.length === messages.value.length);
function toggleSelectMode() { selectMode.value = !selectMode.value; checked.clear(); if (selectMode.value) readerOpen.value = false; }
function toggleCheck(mail) { const key = messageKey(mail); if (checked.has(key)) checked.delete(key); else checked.set(key, true); }
function toggleAll() { if (allChecked.value) checked.clear(); else for (const mail of messages.value) checked.set(messageKey(mail), true); }
async function bulk(action, label) {
  const mails = checkedMails.value;
  if (!mails.length) return;
  if (await batch(action, mails)) { checked.clear(); ElMessage.success(`${label}：${mails.length} 封`); }
}
async function moveMessage(patch) {
  const mail = selected.value;
  if (!mail) return;
  const previous = Object.fromEntries(Object.keys(patch).map((key) => [key, mail[key]]));
  const snapshot = { ...mail };
  if (await mutate(patch, mail)) {
    const label = "deleted" in patch ? (patch.deleted ? "邮件已移到垃圾箱" : "邮件已恢复") : (patch.archived ? "邮件已归档" : "邮件已移回收件箱");
    lastAction.value = { label, mail: snapshot, patch: previous };
  }
}
async function undoMove() {
  const action = lastAction.value;
  if (!action || undoing.value) return;
  undoing.value = true;
  try { if (await mutate(action.patch, action.mail)) { if (lastAction.value === action) lastAction.value = null; ElMessage.success("已撤销，邮件已恢复"); } }
  finally { undoing.value = false; }
}
async function copyCode() { try { await navigator.clipboard.writeText(selected.value.otp); ElMessage.success("验证码已复制"); } catch { ElMessage.error("浏览器未允许复制，请手动选择验证码"); } }
async function copyLink(link) { try { await navigator.clipboard.writeText(link.url); ElMessage.success("邮件链接已复制"); } catch { ElMessage.error("复制失败，请手动选择链接"); } }
function profileUpdated(profile) {
  const alias = aliases.value.find((item) => String(item.id) === String(profile.alias_id));
  if (alias) alias.note = profile.note;
  for (const mail of messages.value) if (mail.address === profile.address) mail.mailboxNote = profile.note;
  if (selected.value?.address === profile.address) selected.value.mailboxNote = profile.note;
}
function profileEvent(event) { if (event.detail) profileUpdated({ alias_id: event.detail.aliasId, ...event.detail }); }
function safeHTML(value) { return `<!doctype html><html><head><meta name="referrer" content="no-referrer"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'"><style>body{font:14px/1.9 system-ui,sans-serif;margin:16px;color:#27312f;overflow-wrap:anywhere}table{border-collapse:collapse;max-width:100%}td,th{border:1px solid #ddd;padding:8px}pre{white-space:pre-wrap}</style></head><body>${value || ""}</body></html>`; }
watch(() => [filters.folder, filters.accountId, filters.aliasId, filters.groupId, filters.unread, filters.starred, filters.query, filters.hasOtp], (values, previous) => {
  clearTimeout(searchTimer); cancelList(); resetReader(); checked.clear();
  const reload = () => { page.value = 1; loadList({ keep: false }); };
  const onlyQuery = values.slice(0, -1).every((value, index) => value === previous[index]);
  if (onlyQuery) searchTimer = setTimeout(reload, 250); else reload();
});
watch(unreadCount, (count) => setUnreadBadge(count), { immediate: true });
watch(() => route.query.account_id, (value) => { filters.accountId = String(value || ""); });
watch(() => route.query.alias_id, (value) => { filters.aliasId = String(value || ""); });
onMounted(() => { loadMeta(); loadList(); liveRefresh.start({ immediate: false }); window.addEventListener("icloud:mailbox-profile-updated", profileEvent); });
onBeforeUnmount(() => { setUnreadBadge(0); window.removeEventListener("icloud:mailbox-profile-updated", profileEvent); disposed = true; metaVersion++; metaController?.abort(); clearTimeout(searchTimer); liveRefresh.stop(); inbox.dispose(); });
</script>

<template>
  <section class="work-inbox">
    <header class="inbox-toolbar">
      <h1>收件箱 <small>{{ total }} 封</small></h1>
      <div class="inbox-actions">
        <button class="manage-addresses" @click="router.push({ name: 'aliases' })"><Collection /><span>管理隐私地址</span></button>
      </div>
    </header>
    <div v-if="lastAction" class="inbox-feedback" role="status"><span>{{ lastAction.label }}</span><button class="undo-action" :disabled="undoing" @click="undoMove">{{ undoing ? '撤销中…' : '撤销' }}</button><button class="feedback-dismiss" aria-label="关闭操作提示" @click="lastAction = null"><Close /></button></div>
    <div class="inbox-filters">
      <label class="mail-search"><Search /><input v-model="filters.query" aria-label="搜索标题、发件人或地址" placeholder="搜索标题、发件人或地址…" /></label>
      <button class="filter-options-toggle" :aria-expanded="filterOptionsOpen" aria-controls="mail-filter-options" @click="filterOptionsOpen = !filterOptionsOpen">筛选<span v-if="optionFilterCount">{{ optionFilterCount }}</span></button>
      <div id="mail-filter-options" class="mail-filter-options" :class="{ expanded: filterOptionsOpen }">
        <SearchableMailboxSelect v-model="filters.accountId" :options="accountOptions" aria-label="按主号筛选" placeholder="所有主号" :loading="metaLoading" @change="filters.aliasId = ''" />
        <SearchableMailboxSelect v-model="filters.aliasId" :options="aliasOptions" aria-label="按隐私地址筛选" placeholder="所有隐私地址" :loading="metaLoading" />
        <SearchableMailboxSelect v-model="filters.groupId" :options="groupOptions" aria-label="按分组筛选" placeholder="所有分组" :loading="metaLoading" />
      </div>
    </div>
    <p v-if="metaError" class="inbox-notice" role="alert">{{ metaError }}<button @click="loadMeta">重新读取筛选选项</button></p>
    <div class="inbox-list-tools">
      <nav class="folder-tabs" aria-label="邮件文件夹"><button v-for="folder in [{ id: 'inbox', name: '全部来信' }, { id: 'archived', name: '已归档' }, { id: 'trash', name: '垃圾箱' }]" :key="folder.id" :class="{ active: filters.folder === folder.id }" :aria-pressed="filters.folder === folder.id" @click="filters.folder = folder.id">{{ folder.name }}</button></nav>
      <div class="mail-state-filters"><label :class="{ active: filters.unread }"><input v-model="filters.unread" type="checkbox" />未读 <span>{{ unreadCount }}</span></label><label :class="{ active: filters.starred }"><input v-model="filters.starred" type="checkbox" />星标</label><label :class="{ active: filters.hasOtp }"><input v-model="filters.hasOtp" type="checkbox" />含验证码</label></div>
      <button class="list-refresh" :disabled="loading" @click="loadList()"><Refresh :class="{ spinning: loading }" /><span>{{ loading ? '刷新中…' : '刷新' }}</span></button>
      <button class="list-select" :aria-pressed="selectMode" :disabled="!messages.length && !selectMode" @click="toggleSelectMode">{{ selectMode ? '完成' : '多选' }}</button>
    </div>
    <div v-if="hasFilters" class="active-mail-filters"><span>已应用筛选 · {{ total }} 封结果</span><button @click="clearFilters">清除筛选<Close /></button></div>
    <span class="sr-only" role="status" aria-live="polite">{{ loading ? '正在读取邮件列表' : `当前列表共有 ${total} 封邮件` }}</span>
    <div :class="['inbox-grid', { 'reader-open': readerOpen }]">
      <section :class="['message-list', { 'select-mode': selectMode }]" aria-label="邮件列表" :aria-busy="loading">
        <div v-if="selectMode" class="bulk-bar" role="toolbar" aria-label="批量操作">
          <label class="bulk-all"><input type="checkbox" :checked="allChecked" :indeterminate.prop="checkedMails.length > 0 && !allChecked" @change="toggleAll" />全选</label>
          <span class="bulk-count" role="status">已选 {{ checkedMails.length }} 封</span>
          <button :disabled="!checkedMails.length" @click="bulk('read', '已标为已读')"><Message />已读</button>
          <button v-if="filters.folder !== 'trash'" :disabled="!checkedMails.length" @click="bulk(filters.folder === 'archived' ? 'unarchive' : 'archive', filters.folder === 'archived' ? '已移回收件箱' : '已归档')"><Box />{{ filters.folder === 'archived' ? '移回' : '归档' }}</button>
          <button :disabled="!checkedMails.length" @click="bulk(filters.folder === 'trash' ? 'restore' : 'trash', filters.folder === 'trash' ? '已恢复' : '已移到垃圾箱')"><Collection v-if="filters.folder === 'trash'" /><Delete v-else />{{ filters.folder === 'trash' ? '恢复' : '删除' }}</button>
        </div>
        <div v-if="error" class="state error" :class="{ compact: messages.length }" role="alert"><strong>邮件列表暂时无法更新</strong><p>{{ error }}</p><button @click="loadList()">重新加载</button></div>
        <div v-else-if="loading && !messages.length" class="state" role="status"><Refresh class="spinning" /><span>正在加载邮件…</span></div>
        <div v-else-if="!messages.length" class="state empty-mail-list"><Message /><strong>{{ emptyState.title }}</strong><p>{{ emptyState.description }}</p><button v-if="emptyState.action" :disabled="metaLoading" @click="emptyAction"><Plus v-if="emptyState.kind === 'setup'" />{{ emptyState.action }}</button></div>
        <div v-if="messages.length" class="message-list-scroll">
          <div v-for="mail in messages" :key="messageKey(mail)" :class="['message-row', { selected: !selectMode && messageKey(selected) === messageKey(mail), unread: !mail.read, checked: checked.has(messageKey(mail)) }]">
            <label v-if="selectMode" class="row-check"><input type="checkbox" :checked="checked.has(messageKey(mail))" :aria-label="`选择：${sender(mail)} ${mail.subject}`" @change="toggleCheck(mail)" /></label>
            <button class="message-row-main" :aria-label="`${sender(mail)}：${mail.subject}${mail.read ? '' : '，未读'}`" :aria-pressed="selectMode ? checked.has(messageKey(mail)) : messageKey(selected) === messageKey(mail)" @click="selectMode ? toggleCheck(mail) : selectMessage(mail)"><span class="message-row-top"><strong><i v-if="!mail.read"></i>{{ sender(mail) }}</strong><time :datetime="mail.receivedAt">{{ date(mail.receivedAt) }}</time></span><b>{{ mail.subject }}</b><small><span class="message-address" :title="mail.address">{{ mail.address }}</span><span v-if="mail.otp" class="message-code-tag">验证码</span></small><span v-if="mail.mailboxNote" class="message-mailbox-note" :title="mail.mailboxNote">{{ mail.mailboxNote }}</span></button>
            <button class="star" :disabled="busy.has(messageKey(mail))" :title="mail.starred ? '取消星标' : '标记星标'" :aria-label="mail.starred ? '取消星标' : '标记星标'" :aria-pressed="mail.starred" @click="mutate({ starred: !mail.starred }, mail)"><StarFilled v-if="mail.starred" class="starred" /><Star v-else /></button>
          </div>
        </div>
        <footer class="pager"><button :disabled="page <= 1 || loading" @click="changePage(-1)">上一页</button><span>{{ page }} / {{ pages }} · {{ total }} 封</span><button :disabled="page >= pages || loading" @click="changePage(1)">下一页</button></footer>
      </section>
      <section class="message-detail" aria-label="邮件阅读" :aria-busy="detailLoading">
        <div class="reader-actions">
          <button class="back" @click="readerOpen = false"><ArrowLeft />返回</button>
          <template v-if="selected">
            <button :disabled="busy.has(messageKey(selected))" :title="selected.archived ? '移回收件箱' : '归档邮件'" :aria-label="selected.archived ? '移回收件箱' : '归档邮件'" @click="moveMessage({ archived: !selected.archived })"><Box /><span class="reader-action-label">{{ selected.archived ? '移回' : '归档' }}</span></button>
            <button :disabled="busy.has(messageKey(selected))" :title="selected.deleted ? '恢复邮件' : '移到垃圾箱'" :aria-label="selected.deleted ? '恢复邮件' : '移到垃圾箱'" @click="moveMessage({ deleted: !selected.deleted })"><Collection v-if="selected.deleted" /><Delete v-else /><span class="reader-action-label">{{ selected.deleted ? '恢复' : '移到垃圾箱' }}</span></button>
            <button :disabled="busy.has(messageKey(selected))" :title="selected.read ? '标记未读' : '标记已读'" :aria-label="selected.read ? '标记未读' : '标记已读'" @click="mutate({ read: !selected.read })"><Message /><span class="reader-action-label">{{ selected.read ? '标未读' : '标已读' }}</span></button>
            <span class="reader-position">{{ selectedIndex >= 0 ? `${selectedIndex + 1} / ${messages.length}` : '阅读中' }}</span>
            <button class="reader-previous" :disabled="!previousMessage" title="上一封" aria-label="上一封" @click="selectMessage(previousMessage)"><ArrowLeft /></button><button class="reader-next" :disabled="!nextMessage" title="下一封" aria-label="下一封" @click="selectMessage(nextMessage)"><ArrowRight /></button>
          </template>
        </div>
        <div v-if="!selected" class="state reader-blank"><Message /><strong>选择一封邮件</strong><p>在左侧查看来信，正文会显示在这里。</p></div>
        <div v-else class="message-content">
          <p v-if="pinnedMessage" class="reader-context-note" role="status">{{ pinnedMessage }}</p>
          <h2>{{ selected.subject }}</h2>
          <div class="mail-sender"><div><strong>{{ sender(selected) }}</strong><small>{{ selected.from?.[0]?.address || '发件地址未知' }}</small><span>发送至 {{ selected.address }}</span></div><time :datetime="selected.receivedAt">{{ date(selected.receivedAt) }}</time></div>
          <div class="reader-mailbox-context"><div><span>邮箱备注</span><p>{{ selected.mailboxNote || '尚未添加备注' }}</p></div><button @click="profileOpen = true">编辑备注与服务登记</button></div>
          <div v-if="selected.otp" class="mail-otp"><span>邮件验证码</span><strong>{{ selected.otp }}</strong><button @click="copyCode"><CopyDocument />复制</button></div>
          <div v-if="detailLoading" class="state" role="status">正在读取邮件正文…</div>
          <div v-else-if="detailError" class="state error" role="alert"><strong>暂时无法读取正文</strong><p>{{ detailError }}</p><div class="state-actions"><button @click="retryDetail">重新读取正文</button><a v-if="sourcePath" :href="sourcePath" download>下载原始邮件</a></div></div>
          <template v-else-if="detail">
            <div class="body-tools"><button v-if="detail.html" :class="{ active: viewMode === 'html' }" :aria-pressed="viewMode === 'html'" @click="viewMode = 'html'">排版</button><button :class="{ active: viewMode === 'text' }" :aria-pressed="viewMode === 'text'" @click="viewMode = 'text'">纯文本</button><a v-if="detail.contentAvailable" :href="sourcePath" download><Download />下载原文</a></div>
            <div v-if="!detail.contentAvailable" class="state warning">{{ contentStatus(detail.message.contentState) }}。邮件标题与时间仍然保留。</div>
            <template v-else><p v-if="detail.body_truncated" class="inbox-notice">正文超过显示预算，当前显示部分内容；可下载原文查看。</p><iframe v-if="viewMode === 'html' && detail.html" class="html-body" sandbox="" referrerpolicy="no-referrer" :srcdoc="safeHTML(detail.html)" title="邮件正文"></iframe><article v-else class="text-body">{{ detail.text || '这封邮件没有文字正文。' }}</article></template>
            <div v-if="detail.links?.length" class="mail-links"><strong>邮件中的链接</strong><button v-for="link in detail.links" :key="link.url" @click="copyLink(link)"><span>{{ link.host }}</span><CopyDocument />复制链接</button></div>
            <div v-if="detail.attachments.length" class="attachments"><strong>{{ detail.attachments.length }} 个附件</strong><a v-for="attachment in detail.attachments" :key="attachment.part" :href="messagePath(attachment.download_path)" :download="attachment.name"><span>{{ attachment.name }}<small>{{ Math.ceil(attachment.size / 1024) }} KB</small></span><Download /></a></div>
          </template>
        </div>
      </section>
    </div>
    <MailboxProfileDrawer v-model="profileOpen" :alias-id="selected?.aliasId || ''" @updated="profileUpdated" />
  </section>
</template>
