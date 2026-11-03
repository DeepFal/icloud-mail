<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { ArrowLeft, ArrowRight, Box, Close, Collection, CopyDocument, Delete, Download, Message, Refresh, Search, Star, StarFilled } from "@element-plus/icons-vue";
import { syncAccount } from "../api/admin.js";
import { useAuth } from "../stores/auth.js";
import { apiRequest } from "../api/client.js";
import { createLiveRefresh } from "../utils/liveRefresh.js";
import { batchMessages, getMessage, getMessages, messagePath, patchMessage } from "./mail-api.js";
import { formatMailDate } from "./mail-format.js";
import { setUnreadBadge } from "../stores/page.js";
import { useTheme } from "../stores/theme.js";
import { copyText } from "../utils/clipboard.js";
import { hasBlockedImages, isDesignedMail, mailFrameDocument, mailRenderMode, measureFrameHeight, readMailPalette, readRenderPreference, saveRenderPreference } from "./mail-frame.js";
import { messageKey, useInboxMessages, watchInboxFilters } from "./inbox-state.js";
import { useInboxReceive } from "./inbox-receive.js";
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
let metaController, metaVersion = 0, disposed = false;
const liveRefresh = createLiveRefresh(() => loadList({ silent: true }));
const { receiving, notice: receiveNotice, receive } = useInboxReceive({ getAccounts: () => accounts.value, getAccountFilter: () => filters.accountId, syncAccount, getCsrfToken: () => auth.state.csrfToken, reloadList: () => loadList({ silent: true }), isDisposed: () => disposed, onSubmitted: () => window.dispatchEvent(new Event("icloud:receiving-updated")) });
const sourcePath = computed(() => selected.value ? messagePath(`/mailboxes/${selected.value.aliasId}/messages/${selected.value.mailboxUid}/source`) : "");
const hasFilters = computed(() => Boolean(filters.query.trim() || filters.accountId || filters.aliasId || filters.groupId || filters.unread || filters.starred || filters.hasOtp));
const optionFilterCount = computed(() => [filters.accountId, filters.aliasId, filters.groupId].filter(Boolean).length);
const folders = [{ id: "inbox", name: "收件箱" }, { id: "archived", name: "已归档" }, { id: "trash", name: "垃圾箱" }];
const historyTarget = computed(() => {
  const account = accounts.value.find((item) => String(item.id) === filters.accountId) || accounts.value.find((item) => item.enabled);
  return account ? { name: "account-detail", params: { id: account.id } } : { name: "accounts" };
});
const emptyState = computed(() => {
  if (hasFilters.value) return { title: "没有符合条件的邮件。", action: "清除筛选", kind: "filters" };
  if (!metaLoading.value && !metaError.value && !accounts.value.length) return { title: "还没有主号，添加并连接后来信会出现在这里。", action: "添加主号", kind: "setup" };
  if (filters.folder === "archived") return { title: "没有归档邮件。" };
  if (filters.folder === "trash") return { title: "垃圾箱是空的。" };
  return { title: "暂时没有来信。", action: "补取历史邮件", kind: "history" };
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
async function copyCode(mail = selected.value) { if (!mail?.otp) return; if (await copyText(mail.otp)) ElMessage.success(`验证码 ${mail.otp} 已复制`); else ElMessage.error("浏览器未允许复制，请手动选择验证码"); }
async function copyLink(link) { if (await copyText(link.url)) ElMessage.success("邮件链接已复制"); else ElMessage.error("复制失败，请手动选择链接"); }
function profileUpdated(profile) {
  const alias = aliases.value.find((item) => String(item.id) === String(profile.alias_id));
  if (alias) alias.note = profile.note;
  for (const mail of messages.value) if (mail.address === profile.address) mail.mailboxNote = profile.note;
  if (selected.value?.address === profile.address) selected.value.mailboxNote = profile.note;
}
function profileEvent(event) { if (event.detail) profileUpdated({ alias_id: event.detail.aliasId, ...event.detail }); }
// Mail HTML: sandboxed frame; dark theme adapts plain mail and keeps designed mail on light paper unless the user chose otherwise.
const { resolvedTheme } = useTheme();
const renderPreference = ref(readRenderPreference());
const designedMail = computed(() => isDesignedMail(detail.value?.html));
const renderMode = computed(() => mailRenderMode({ dark: resolvedTheme.value === "dark", designed: designedMail.value, preference: renderPreference.value }));
const frameDocument = computed(() => detail.value?.html ? mailFrameDocument(detail.value.html, { mode: renderMode.value, palette: resolvedTheme.value === "dark" ? readMailPalette() : undefined }) : "");
const blockedImages = computed(() => viewMode.value === "html" && hasBlockedImages(detail.value?.html));
function setRenderPreference(value) { renderPreference.value = value; saveRenderPreference(value); }
// The frame is same-origin but script-free (scripts are never enabled in its sandbox): the parent sizes it to its content so the reading pane is the only scroller.
const mailFrame = ref(null), readerScroll = ref(null), frameHeight = ref(160);
let frameWidth = 0;
function fitFrame() { const height = measureFrameHeight(mailFrame.value?.contentDocument); if (height) frameHeight.value = height; }
const frameResize = typeof ResizeObserver === "function" ? new ResizeObserver(([entry]) => { const width = Math.round(entry.contentRect.width); if (width !== frameWidth) { frameWidth = width; fitFrame(); } }) : null;
watch(mailFrame, (frame, previous) => { if (previous) frameResize?.unobserve(previous); frameWidth = 0; if (frame) frameResize?.observe(frame); });
watch(() => messageKey(selected.value), () => { frameHeight.value = 160; readerScroll.value?.scrollTo({ top: 0 }); });
// List column width: dragged or arrow keys on the divider, remembered per browser.
const LIST_WIDTH_KEY = "icloud-inbox-list-width";
const clampWidth = (value) => Math.min(520, Math.max(280, Math.round(Number(value) || 340)));
const listWidth = ref(clampWidth((() => { try { return localStorage.getItem(LIST_WIDTH_KEY); } catch { return null; } })()));
const resizing = ref(false);
function saveListWidth() { try { localStorage.setItem(LIST_WIDTH_KEY, String(listWidth.value)); } catch { /* optional */ } }
function startResize(event) {
  if (event.button !== 0) return;
  const startX = event.clientX, startWidth = listWidth.value;
  const move = (e) => { listWidth.value = clampWidth(startWidth + e.clientX - startX); };
  const stop = () => { resizing.value = false; window.removeEventListener("pointermove", move); window.removeEventListener("pointerup", stop); window.removeEventListener("pointercancel", stop); saveListWidth(); };
  resizing.value = true;
  window.addEventListener("pointermove", move); window.addEventListener("pointerup", stop); window.addEventListener("pointercancel", stop);
}
function resizeKey(event) { const step = { ArrowLeft: -16, ArrowRight: 16 }[event.key]; if (!step) return; event.preventDefault(); listWidth.value = clampWidth(listWidth.value + step); saveListWidth(); }
const searchInput = ref(null), listScroll = ref(null);
function openMessage(mail) { selectMessage(mail); nextTick(() => listScroll.value?.querySelector(".message-row.selected")?.scrollIntoView({ block: "nearest" })); }
// j/k move, Enter opens, e archives, c copies the code, / searches. Typing and overlays are left alone.
function shortcut(event) {
  if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || profileOpen.value) return;
  if (event.target?.closest?.("input, textarea, select, [contenteditable], .el-overlay, .el-popper")) return;
  const key = event.key, mail = selected.value;
  if (key === "/") { event.preventDefault(); searchInput.value?.focus(); return; }
  if (selectMode.value) return;
  if (key === "j" || key === "k") { const target = key === "j" ? nextMessage.value : previousMessage.value; if (target) { event.preventDefault(); openMessage(target); } }
  else if (key === "Enter" && mail && !event.target?.closest?.("button, a, [role='separator']")) readerOpen.value = true;
  else if (key === "Escape" && readerOpen.value) readerOpen.value = false;
  else if (key === "e" && mail && !mail.deleted) moveMessage({ archived: !mail.archived });
  else if (key === "c" && mail?.otp) copyCode(mail);
}
const stopFilterWatch = watchInboxFilters(filters, { onChange: () => { cancelList(); resetReader(); checked.clear(); }, reload: () => { page.value = 1; loadList({ keep: false }); } });
watch(unreadCount, (count) => setUnreadBadge(count), { immediate: true });
watch(() => route.query.account_id, (value) => { filters.accountId = String(value || ""); });
watch(() => route.query.alias_id, (value) => { filters.aliasId = String(value || ""); });
onMounted(() => { loadMeta(); loadList(); liveRefresh.start({ immediate: false }); window.addEventListener("icloud:mailbox-profile-updated", profileEvent); window.addEventListener("keydown", shortcut); });
onBeforeUnmount(() => { setUnreadBadge(0); window.removeEventListener("icloud:mailbox-profile-updated", profileEvent); window.removeEventListener("keydown", shortcut); frameResize?.disconnect(); disposed = true; metaVersion++; metaController?.abort(); stopFilterWatch(); liveRefresh.stop(); inbox.dispose(); });
</script>

<template>
  <section :class="['work-inbox', { 'is-resizing': resizing }]" :style="{ '--list-width': `${listWidth}px` }">
    <h1 class="sr-only">收件箱</h1>
    <div v-if="selectMode" class="inbox-bar is-bulk" role="toolbar" aria-label="批量操作">
      <label class="bulk-all"><input type="checkbox" :checked="allChecked" :indeterminate.prop="checkedMails.length > 0 && !allChecked" @change="toggleAll" />全选</label>
      <span class="bulk-count" role="status">已选 {{ checkedMails.length }} 封</span>
      <button class="bar-button" :disabled="!checkedMails.length" @click="bulk('read', '已标为已读')"><Message aria-hidden="true" />已读</button>
      <button v-if="filters.folder !== 'trash'" class="bar-button" :disabled="!checkedMails.length" @click="bulk(filters.folder === 'archived' ? 'unarchive' : 'archive', filters.folder === 'archived' ? '已移回收件箱' : '已归档')"><Box aria-hidden="true" />{{ filters.folder === 'archived' ? '移回' : '归档' }}</button>
      <button class="bar-button" :disabled="!checkedMails.length" @click="bulk(filters.folder === 'trash' ? 'restore' : 'trash', filters.folder === 'trash' ? '已恢复' : '已移到垃圾箱')"><Collection v-if="filters.folder === 'trash'" aria-hidden="true" /><Delete v-else aria-hidden="true" />{{ filters.folder === 'trash' ? '恢复' : '删除' }}</button>
      <button class="list-select bar-button" :aria-pressed="selectMode" @click="toggleSelectMode">完成</button>
    </div>
    <div v-else class="inbox-bar">
      <nav class="segmented" aria-label="邮件文件夹"><button v-for="folder in folders" :key="folder.id" :class="{ active: filters.folder === folder.id }" :aria-pressed="filters.folder === folder.id" @click="filters.folder = folder.id">{{ folder.name }}</button></nav>
      <div class="state-toggles" role="group" aria-label="按状态筛选">
        <button :aria-pressed="filters.unread" @click="filters.unread = !filters.unread">未读<span v-if="unreadCount" class="count">{{ unreadCount }}</span></button>
        <button :aria-pressed="filters.starred" @click="filters.starred = !filters.starred">星标</button>
        <button :aria-pressed="filters.hasOtp" @click="filters.hasOtp = !filters.hasOtp">验证码</button>
      </div>
      <label class="mail-search"><Search aria-hidden="true" /><input ref="searchInput" v-model="filters.query" type="search" name="mail-search" autocomplete="off" spellcheck="false" aria-label="搜索标题、发件人或地址" placeholder="搜索标题、发件人或地址…" /><kbd aria-hidden="true">/</kbd></label>
      <button class="bar-button filter-toggle" :class="{ active: optionFilterCount }" :aria-expanded="filterOptionsOpen" aria-controls="mail-filter-options" @click="filterOptionsOpen = !filterOptionsOpen">筛选<span v-if="optionFilterCount" class="count">{{ optionFilterCount }}</span></button>
      <span class="bar-spacer"></span>
      <button class="list-refresh" :disabled="loading" @click="loadList()" aria-label="刷新列表" title="刷新列表（只读本地归档）"><Refresh :class="{ spinning: loading }" aria-hidden="true" /></button>
      <button class="list-receive" :disabled="receiving || metaLoading || Boolean(metaError)" title="向启用的主号提交收件请求" @click="receive">{{ receiving ? '提交中…' : '收取' }}</button>
      <button class="list-select bar-button" :aria-pressed="selectMode" :disabled="!messages.length" @click="toggleSelectMode">多选</button>
    </div>
    <div v-show="filterOptionsOpen && !selectMode" id="mail-filter-options" class="inbox-filter-row">
      <SearchableMailboxSelect v-model="filters.accountId" :options="accountOptions" aria-label="按主号筛选" placeholder="所有主号" :loading="metaLoading" @change="filters.aliasId = ''" />
      <SearchableMailboxSelect v-model="filters.aliasId" :options="aliasOptions" aria-label="按隐私地址筛选" placeholder="所有隐私地址" :loading="metaLoading" />
      <SearchableMailboxSelect v-model="filters.groupId" :options="groupOptions" aria-label="按分组筛选" placeholder="所有分组" :loading="metaLoading" />
      <button v-if="hasFilters" class="link-button" @click="clearFilters">清除全部筛选</button>
      <button class="link-button" @click="router.push({ name: 'aliases' })">管理隐私地址</button>
    </div>
    <span class="sr-only" role="status" aria-live="polite">{{ loading ? '正在读取邮件列表' : `当前列表共有 ${total} 封邮件` }}</span>
    <div :class="['inbox-panes', { 'reader-open': readerOpen }]">
      <section :class="['message-list', { 'select-mode': selectMode }]" aria-label="邮件列表" :aria-busy="loading">
        <div v-if="error" class="state error" :class="{ compact: messages.length }" role="alert"><p>{{ error }}</p><button class="bar-button" @click="loadList()">重新加载</button></div>
        <div v-else-if="loading && !messages.length" class="state" role="status">正在加载邮件…</div>
        <div v-else-if="!messages.length" class="state"><p>{{ emptyState.title }}</p><button v-if="emptyState.action" class="bar-button" :disabled="metaLoading" @click="emptyAction">{{ emptyState.action }}</button></div>
        <div v-if="messages.length" ref="listScroll" class="message-list-scroll">
          <div v-for="mail in messages" :key="messageKey(mail)" :class="['message-row', { selected: !selectMode && messageKey(selected) === messageKey(mail), unread: !mail.read, checked: checked.has(messageKey(mail)), 'has-otp': mail.otp, starred: mail.starred }]">
            <label v-if="selectMode" class="row-check"><input type="checkbox" :checked="checked.has(messageKey(mail))" :aria-label="`选择：${sender(mail)} ${mail.subject}`" @change="toggleCheck(mail)" /></label>
            <button class="message-row-main" :aria-label="`${sender(mail)}：${mail.subject}${mail.read ? '' : '，未读'}`" :aria-pressed="selectMode ? checked.has(messageKey(mail)) : messageKey(selected) === messageKey(mail)" @click="selectMode ? toggleCheck(mail) : openMessage(mail)">
              <span class="row-line"><strong class="row-from">{{ sender(mail) }}</strong><span class="row-to" :title="mail.address" translate="no">{{ mail.mailboxNote || mail.address }}</span><time :datetime="mail.receivedAt">{{ date(mail.receivedAt) }}</time></span>
              <span class="row-subject">{{ mail.subject }}</span>
            </button>
            <span class="row-trail">
              <button v-if="mail.otp" class="otp-chip" translate="no" :title="`复制验证码 ${mail.otp}`" :aria-label="`复制验证码 ${mail.otp}`" @click="copyCode(mail)">{{ mail.otp }}</button>
              <button class="star" :disabled="busy.has(messageKey(mail))" :title="mail.starred ? '取消星标' : '标记星标'" :aria-label="mail.starred ? '取消星标' : '标记星标'" :aria-pressed="mail.starred" @click="mutate({ starred: !mail.starred }, mail)"><StarFilled v-if="mail.starred" class="star-on" aria-hidden="true" /><Star v-else aria-hidden="true" /></button>
            </span>
          </div>
        </div>
        <footer class="pager"><span>共 {{ total }} 封</span><button :disabled="page <= 1 || loading" aria-label="上一页" @click="changePage(-1)"><ArrowLeft aria-hidden="true" /></button><span>{{ page }} / {{ pages }}</span><button :disabled="page >= pages || loading" aria-label="下一页" @click="changePage(1)"><ArrowRight aria-hidden="true" /></button></footer>
      </section>
      <div class="pane-divider" role="separator" aria-orientation="vertical" aria-label="调整列表宽度" tabindex="0" :aria-valuenow="listWidth" aria-valuemin="280" aria-valuemax="520" @pointerdown="startResize" @keydown="resizeKey"></div>
      <section class="message-detail" aria-label="邮件阅读" :aria-busy="detailLoading">
        <div class="reader-actions">
          <button class="back" @click="readerOpen = false"><ArrowLeft aria-hidden="true" />返回</button>
          <template v-if="selected">
            <button :disabled="busy.has(messageKey(selected))" :title="selected.archived ? '移回收件箱 (E)' : '归档 (E)'" :aria-label="selected.archived ? '移回收件箱' : '归档邮件'" @click="moveMessage({ archived: !selected.archived })"><Box aria-hidden="true" /><span class="reader-action-label">{{ selected.archived ? '移回' : '归档' }}</span></button>
            <button :disabled="busy.has(messageKey(selected))" :title="selected.deleted ? '恢复邮件' : '移到垃圾箱'" :aria-label="selected.deleted ? '恢复邮件' : '移到垃圾箱'" @click="moveMessage({ deleted: !selected.deleted })"><Collection v-if="selected.deleted" aria-hidden="true" /><Delete v-else aria-hidden="true" /><span class="reader-action-label">{{ selected.deleted ? '恢复' : '删除' }}</span></button>
            <button :disabled="busy.has(messageKey(selected))" :title="selected.read ? '标记未读' : '标记已读'" :aria-label="selected.read ? '标记未读' : '标记已读'" @click="mutate({ read: !selected.read })"><Message aria-hidden="true" /><span class="reader-action-label">{{ selected.read ? '标为未读' : '标为已读' }}</span></button>
            <button :disabled="busy.has(messageKey(selected))" :aria-pressed="selected.starred" :aria-label="selected.starred ? '取消星标' : '标记星标'" :title="selected.starred ? '取消星标' : '标记星标'" @click="mutate({ starred: !selected.starred })"><StarFilled v-if="selected.starred" class="star-on" aria-hidden="true" /><Star v-else aria-hidden="true" /></button>
            <span class="reader-position">{{ selectedIndex >= 0 ? `${selectedIndex + 1} / ${messages.length}` : '阅读中' }}</span>
            <button class="reader-previous" :disabled="!previousMessage" title="上一封 (K)" aria-label="上一封" @click="openMessage(previousMessage)"><ArrowLeft aria-hidden="true" /></button><button class="reader-next" :disabled="!nextMessage" title="下一封 (J)" aria-label="下一封" @click="openMessage(nextMessage)"><ArrowRight aria-hidden="true" /></button>
          </template>
        </div>
        <p v-if="!selected" class="state reader-blank">选择一封邮件查看正文。</p>
        <div v-else ref="readerScroll" class="reader-scroll">
          <header class="reader-head">
            <p v-if="pinnedMessage" class="reader-context-note" role="status">{{ pinnedMessage }}</p>
            <h2>{{ selected.subject }}</h2>
            <div class="reader-meta">
              <strong>{{ sender(selected) }}</strong><span class="mono" translate="no">{{ selected.from?.[0]?.address || '发件地址未知' }}</span>
              <span class="reader-to">发送至 <span class="mono" translate="no">{{ selected.address }}</span></span>
              <time :datetime="selected.receivedAt">{{ date(selected.receivedAt) }}</time>
            </div>
            <div class="reader-note">
              <span v-if="selected.mailboxNote" class="reader-note-text" :title="selected.mailboxNote">备注：{{ selected.mailboxNote }}</span>
              <button class="link-button" @click="profileOpen = true">{{ selected.mailboxNote ? '编辑备注' : '添加备注与服务登记' }}</button>
              <button v-if="selected.otp" class="otp-copy" :aria-label="`复制验证码 ${selected.otp}`" title="复制验证码 (C)" @click="copyCode(selected)"><span>验证码</span><strong translate="no">{{ selected.otp }}</strong><CopyDocument aria-hidden="true" /></button>
            </div>
          </header>
          <div v-if="detail && !detailLoading && !detailError" class="body-tools">
            <div v-if="detail.html && detail.contentAvailable" class="segmented small" role="group" aria-label="正文显示方式"><button :class="{ active: viewMode === 'html' }" :aria-pressed="viewMode === 'html'" @click="viewMode = 'html'">排版</button><button :class="{ active: viewMode === 'text' }" :aria-pressed="viewMode === 'text'" @click="viewMode = 'text'">纯文本</button></div>
            <div v-if="viewMode === 'html' && detail.html && designedMail && resolvedTheme === 'dark'" class="segmented small" role="group" aria-label="深色模式下的正文"><button :class="{ active: renderMode === 'paper' }" :aria-pressed="renderMode === 'paper'" @click="setRenderPreference('paper')">原样</button><button :class="{ active: renderMode === 'adapt' }" :aria-pressed="renderMode === 'adapt'" @click="setRenderPreference('adapt')">适配深色</button></div>
            <span v-if="blockedImages" class="body-hint" title="远程图片不会自动加载，避免向发件方暴露阅读行为">远程图片未加载</span>
            <span v-if="detail.body_truncated" class="body-hint warning">正文过长，仅显示部分</span>
            <span class="bar-spacer"></span>
            <el-popover v-if="detail.links?.length" trigger="click" placement="bottom-end" :width="320" popper-class="inbox-popover">
              <template #reference><button class="tool-button">链接 {{ detail.links.length }}</button></template>
              <ul class="popover-list" aria-label="邮件中的链接"><li v-for="link in detail.links" :key="link.url"><span :title="link.url" translate="no">{{ link.host }}</span><button :aria-label="`复制链接 ${link.host}`" @click="copyLink(link)"><CopyDocument aria-hidden="true" />复制</button></li></ul>
            </el-popover>
            <el-popover v-if="detail.attachments.length" trigger="click" placement="bottom-end" :width="320" popper-class="inbox-popover">
              <template #reference><button class="tool-button">附件 {{ detail.attachments.length }}</button></template>
              <ul class="popover-list" aria-label="附件"><li v-for="attachment in detail.attachments" :key="attachment.part"><span :title="attachment.name">{{ attachment.name }}</span><small>{{ Math.ceil(attachment.size / 1024) }}&nbsp;KB</small><a :href="messagePath(attachment.download_path)" :download="attachment.name" :aria-label="`下载 ${attachment.name}`"><Download aria-hidden="true" /></a></li></ul>
            </el-popover>
            <a v-if="detail.contentAvailable" class="tool-button" :href="sourcePath" download><Download aria-hidden="true" />原文</a>
          </div>
          <div class="reader-body">
            <div v-if="detailLoading" class="state" role="status">正在读取正文…</div>
            <div v-else-if="detailError" class="state error" role="alert"><p>{{ detailError }}</p><div class="state-actions"><button class="bar-button" @click="retryDetail">重新读取</button><a v-if="sourcePath" :href="sourcePath" download>下载原始邮件</a></div></div>
            <p v-else-if="detail && !detail.contentAvailable" class="state warning">{{ contentStatus(detail.message.contentState) }}。标题与时间仍然保留。</p>
            <div v-else-if="detail && viewMode === 'html' && detail.html" :class="['html-frame', { 'is-paper': renderMode === 'paper' }]"><iframe ref="mailFrame" class="html-body" sandbox="allow-same-origin" referrerpolicy="no-referrer" scrolling="no" :srcdoc="frameDocument" :style="{ height: `${frameHeight}px` }" title="邮件正文" @load="fitFrame"></iframe></div>
            <article v-else-if="detail" class="text-body">{{ detail.text || '这封邮件没有文字正文。' }}</article>
          </div>
        </div>
      </section>
    </div>
    <div class="inbox-toasts">
      <div v-if="metaError" class="inbox-toast warning" role="alert"><span>{{ metaError }}</span><button class="link-button" @click="loadMeta">重新读取</button></div>
      <div v-if="receiveNotice" class="inbox-toast" :class="{ warning: receiveNotice.error }" :role="receiveNotice.error ? 'alert' : 'status'"><span>{{ receiveNotice.text }}</span><RouterLink v-if="receiveNotice.to" :to="receiveNotice.to">{{ receiveNotice.link }}</RouterLink><button v-if="!receiveNotice.pending" class="toast-close" aria-label="关闭收件提示" @click="receiveNotice = null"><Close aria-hidden="true" /></button></div>
      <div v-if="lastAction" class="inbox-toast" role="status"><span>{{ lastAction.label }}</span><button class="link-button" :disabled="undoing" @click="undoMove">{{ undoing ? '撤销中…' : '撤销' }}</button><button class="toast-close" aria-label="关闭操作提示" @click="lastAction = null"><Close aria-hidden="true" /></button></div>
    </div>
    <MailboxProfileDrawer v-model="profileOpen" :alias-id="selected?.aliasId || ''" @updated="profileUpdated" />
  </section>
</template>
