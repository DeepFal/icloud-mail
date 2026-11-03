<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue";
import { ElConfigProvider, ElDialog, ElMessage, ElMessageBox, ElSwitch } from "element-plus";
import zhCn from "element-plus/es/locale/lang/zh-cn";
import {
  Message, Collection, Connection, Key, Setting, Search, Plus, Refresh,
  ArrowRight, ArrowLeft, Check, Close, Star, StarFilled, CopyDocument,
  Download, Sunny, Moon, InfoFilled, Warning, CircleCheckFilled, Link,
  Folder, Delete, View, Hide, Document, Box, Cloudy,
} from "@element-plus/icons-vue";
import { createDemoData, demoGroups } from "./mock-data.js";
import ProjectPanel from "./ProjectPanel.vue";

const STORAGE_KEY = "icloud-mail-workspace-demo-v1";
function loadData() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY));
    if (saved?.version === 1 && ["accounts", "aliases", "messages", "projects", "activities"].every((key) => Array.isArray(saved.data?.[key]))) return saved.data;
  } catch { /* A blocked or old local store does not prevent previewing. */ }
  return createDemoData();
}
const data = reactive(loadData());
const navItems = [
  { id: "inbox", label: "收件箱", icon: Message },
  { id: "aliases", label: "隐私地址", icon: Collection },
  { id: "accounts", label: "主号", icon: Connection },
  { id: "projects", label: "邮箱池", icon: Key },
  { id: "settings", label: "设置与诊断", icon: Setting },
];
function hashPage() { return navItems.some((item) => item.id === location.hash.slice(1)) ? location.hash.slice(1) : "inbox"; }
const page = ref(hashPage());
const search = ref("");
const mailFilter = ref("all");
const groupFilter = ref("");
const aliasFilter = ref("");
const aliasGroup = ref("");
const aliasQuery = ref("");
const selectedMailId = ref(data.messages[0]?.id || "");
const mobileReader = ref(false);
const syncing = ref(false);
const syncLabel = ref("刚刚更新");
const theme = ref("light");
try { theme.value = localStorage.getItem("icloud-mail-demo-theme") || "light"; } catch { /* optional preference */ }
let savedPreferences = {};
try { savedPreferences = JSON.parse(localStorage.getItem("icloud-mail-demo-preferences")) || {}; } catch { /* optional preferences */ }
const syncWindow = ref(["1", "7", "30"].includes(savedPreferences.syncWindow) ? savedPreferences.syncWindow : "7");
const keepReceiving = ref(typeof savedPreferences.keepReceiving === "boolean" ? savedPreferences.keepReceiving : true);
watch([syncWindow, keepReceiving], ([windowDays, receiving]) => {
  try { localStorage.setItem("icloud-mail-demo-preferences", JSON.stringify({ syncWindow: windowDays, keepReceiving: receiving })); } catch { /* The preview remains usable in memory. */ }
});
const showSource = ref(false);
const readerBody = ref(null);
const timers = new Set();
let persistenceTimer;
function persist() { try { localStorage.setItem(STORAGE_KEY, JSON.stringify({ version: 1, data })); } catch { /* Memory-only preview still works. */ } }
watch(data, () => { clearTimeout(persistenceTimer); persistenceTimer = setTimeout(persist, 120); }, { deep: true });
watch(theme, (value) => {
  document.documentElement.classList.toggle("dark", value === "dark");
  try { localStorage.setItem("icloud-mail-demo-theme", value); } catch { /* optional preference */ }
}, { immediate: true });
watch(page, (value) => { document.title = `${navItems.find((item) => item.id === value)?.label} · iCloud Mail 本地预览`; });
function onHashChange() { if (location.hash !== "#workspace-main") page.value = hashPage(); }
window.addEventListener("hashchange", onHashChange);
onBeforeUnmount(() => { window.removeEventListener("hashchange", onHashChange); clearTimeout(persistenceTimer); persist(); timers.forEach(clearTimeout); });
function later(callback, delay = 700) { const timer = setTimeout(() => { timers.delete(timer); callback(); }, delay); timers.add(timer); }
function go(id) { page.value = id; location.hash = id; mobileReader.value = false; nextTick(() => window.scrollTo({ top: 0 })); }
const aliasById = computed(() => Object.fromEntries(data.aliases.map((alias) => [alias.id, alias])));
const unreadCount = computed(() => data.messages.filter((mail) => mail.unread && !mail.archived).length);
const selectedMail = computed(() => data.messages.find((mail) => mail.id === selectedMailId.value));
const selectedAlias = computed(() => aliasById.value[selectedMail.value?.aliasId]);
const visibleMessages = computed(() => data.messages.filter((mail) => {
  const alias = aliasById.value[mail.aliasId];
  if (mailFilter.value === "archived" ? !mail.archived : mail.archived) return false;
  if (mailFilter.value === "unread" && !mail.unread && mail.id !== selectedMailId.value) return false;
  if (mailFilter.value === "starred" && !mail.starred) return false;
  if (groupFilter.value && alias?.group !== groupFilter.value) return false;
  if (aliasFilter.value && mail.aliasId !== aliasFilter.value) return false;
  const haystack = [mail.sender, mail.subject, mail.preview, alias?.address, alias?.purpose, ...mail.paragraphs].join(" ").toLowerCase();
  return haystack.includes(search.value.trim().toLowerCase());
}));
const filteredAliases = computed(() => data.aliases.filter((alias) => (!aliasGroup.value || alias.group === aliasGroup.value) && `${alias.address} ${alias.purpose}`.toLowerCase().includes(aliasQuery.value.trim().toLowerCase())));
const healthyAccounts = computed(() => data.accounts.filter((account) => account.receivingStatus === "ready" && account.managementStatus === "ready").length);
function accountFor(id) { return data.accounts.find((account) => account.id === id); }
function groupColor(group) { return ({ 工作: "#6c89bd", 生活: "#d9a368", 订阅: "#a68bb6", 项目: "#72a998" })[group] || "#8899aa"; }
function mailCount(id) { return data.messages.filter((mail) => mail.aliasId === id).length; }
function groupUnread(group) { return data.messages.filter((mail) => !mail.archived && mail.unread && aliasById.value[mail.aliasId]?.group === group).length; }
function chooseMail(mail) { selectedMailId.value = mail.id; mail.unread = false; mobileReader.value = true; showSource.value = false; nextTick(() => { if (readerBody.value) readerBody.value.scrollTop = 0; }); }
function changeMailFilter(value) { mailFilter.value = value; selectedMailId.value = ""; const first = visibleMessages.value[0]; if (first) selectedMailId.value = first.id; mobileReader.value = false; }
function filterGroup(group) { groupFilter.value = groupFilter.value === group ? "" : group; aliasFilter.value = ""; go("inbox"); }
function viewAlias(alias) { groupFilter.value = ""; aliasFilter.value = alias.id; search.value = ""; mailFilter.value = "all"; selectedMailId.value = data.messages.find((mail) => mail.aliasId === alias.id && !mail.archived)?.id || ""; go("inbox"); }
async function copy(value, label = "内容") {
  try { await navigator.clipboard.writeText(value); ElMessage.success(`${label}已复制`); }
  catch { ElMessage.error("浏览器未允许复制，请手动选择文本复制。"); }
}
function addActivity(title, description, type = "info") { data.activities.unshift({ id: crypto.randomUUID(), time: "刚刚", title, description, type }); data.activities.splice(30); }
function addMail(alias, subject = "新邮件已送达，一切准备就绪", paragraphs) {
  const mail = { id: crypto.randomUUID(), aliasId: alias.id, sender: "iCloud Mail", senderEmail: "hello@example.test", initial: "M", color: "#648d7c", subject,
    preview: "这是一封模拟的普通邮件。现在可以直接在收件箱中阅读完整内容。", time: "刚刚", date: "今天", unread: true, starred: false, category: "普通邮件", otp: null,
    paragraphs: paragraphs || ["你好，林岚：", `发送到 ${alias.address} 的测试邮件已经出现在这里。`, "你可以阅读正文、标记星标，也可以归档这封邮件。验证码只在邮件确实包含验证码时出现。", "这封邮件和整个工作区的数据均为本地模拟。"], attachments: [] };
  data.messages.unshift(mail); return mail;
}
function simulateSync() {
  if (syncing.value) return;
  syncing.value = true;
  later(() => {
    const alias = data.aliases.find((item) => item.enabled && accountFor(item.accountId)?.receivingStatus === "ready" && item.poolState === "personal");
    if (alias) addMail(alias, "你的工作区多了一封新来信");
    syncing.value = false; syncLabel.value = "刚刚更新";
    addActivity("模拟收件完成", alias ? `一封示例邮件已送达 ${alias.address}` : "暂无可接收邮件的演示地址", alias ? "success" : "warning");
    ElMessage({ type: alias ? "success" : "warning", message: alias ? "已加入一封模拟新邮件" : "请先启用一个地址并完成收件连接" });
  });
}
function moveSelection() { selectedMailId.value = visibleMessages.value[0]?.id || ""; mobileReader.value = false; }
function archiveMail() { const mail = selectedMail.value; if (!mail) return; mail.archived = !mail.archived; const archived = mail.archived; moveSelection(); ElMessage.success(archived ? "邮件已归档，可在「已归档」中查看" : "邮件已移回收件箱"); }
async function deleteMail() {
  const id = selectedMail.value?.id; if (!id) return;
  try { await ElMessageBox.confirm("这会删除本地演示中的这封邮件，可以在设置里重置演示数据。", "删除模拟邮件", { confirmButtonText: "删除", cancelButtonText: "取消", type: "warning" }); }
  catch { return; }
  data.messages.splice(data.messages.findIndex((mail) => mail.id === id), 1); moveSelection();
}
function downloadAttachment(attachment) {
  const url = URL.createObjectURL(new Blob([attachment.content], { type: "text/plain;charset=utf-8" }));
  const anchor = document.createElement("a"); anchor.href = url; anchor.download = attachment.name; anchor.click(); later(() => URL.revokeObjectURL(url), 1500);
}
const linkDialog = ref(false);
const shownLink = ref(null);
function openLink(link) { shownLink.value = link; linkDialog.value = true; }

const aliasDialog = ref(false);
const editingAliasId = ref("");
const aliasForm = reactive({ purpose: "", group: "生活", accountId: "", sendMail: true, address: "" });
function openAlias(alias) {
  editingAliasId.value = alias?.id || "";
  Object.assign(aliasForm, { purpose: alias?.purpose || "", group: alias?.group || "生活", accountId: alias?.accountId || data.accounts.find((account) => account.managementStatus === "ready")?.id || "", sendMail: true,
    address: alias?.address || `private.${Math.random().toString(36).slice(2, 10)}@example.test` });
  aliasDialog.value = true;
}
function saveAlias() {
  if (!aliasForm.purpose.trim() || !aliasForm.accountId) { ElMessage.warning("请填写用途并选择主号"); return; }
  if (accountFor(aliasForm.accountId)?.managementStatus !== "ready") { ElMessage.warning("请先完成该主号的管理连接"); return; }
  if (editingAliasId.value) {
    const alias = aliasById.value[editingAliasId.value]; if (!alias) return;
    alias.purpose = aliasForm.purpose.trim(); alias.group = aliasForm.group;
    ElMessage.success("地址用途已更新");
  } else {
    const alias = { id: crypto.randomUUID(), accountId: aliasForm.accountId, address: aliasForm.address, purpose: aliasForm.purpose.trim(), group: aliasForm.group, enabled: true, poolState: "personal", projectId: null, createdAt: "2026-10-29" };
    data.aliases.unshift(alias);
    if (aliasForm.sendMail && accountFor(alias.accountId)?.receivingStatus === "ready") addMail(alias, `${alias.purpose}：地址已准备好`);
    addActivity("创建了一个隐私地址", `${alias.purpose} · ${alias.address}`, "success");
    ElMessage.success("模拟地址已创建，可以在收件箱查看示例邮件");
  }
  aliasDialog.value = false;
}
async function toggleAlias(alias) {
  const next = !alias.enabled;
  try { await ElMessageBox.confirm(next ? "恢复此演示地址的接收状态，已保存的历史邮件仍然保留。" : "暂停后将不再为此地址模拟接收新邮件，历史邮件仍然保留。", next ? "恢复接收" : "暂停接收", { confirmButtonText: next ? "恢复" : "暂停", cancelButtonText: "取消" }); }
  catch { return; }
  alias.enabled = next; addActivity(next ? "恢复地址接收" : "暂停地址接收", alias.address);
}

const setupDialog = ref(false);
const setupStep = ref(0);
const setupBusy = ref(false);
const setup = reactive({ id: "", name: "", email: "", managed: false, received: false, transport: "imap" });
function openSetup(account) {
  Object.assign(setup, { id: account?.id || "", name: account?.name || "", email: account?.email || "", managed: account?.managementStatus === "ready", received: false, transport: "imap" });
  setupStep.value = account ? 2 : 0; setupDialog.value = true;
}
function setupNext() {
  if (setupStep.value === 0 && (!setup.name.trim() || !/^[^\s@]+@example\.test$/.test(setup.email.trim()))) { ElMessage.warning("请填写名称与 example.test 域名的演示邮箱"); return; }
  setupStep.value++;
}
function simulateConnection() { setupBusy.value = true; later(() => { setupBusy.value = false; setup.managed = true; }); }
function simulateTest() { setupBusy.value = true; later(() => { setupBusy.value = false; setup.received = true; }); }
function finishSetup() {
  if (!setup.received) return;
  let account = accountFor(setup.id);
  if (account) { account.receivingStatus = "ready"; account.managementStatus = "ready"; account.detail = "地址管理和收件验证均已完成。"; account.lastSync = "刚刚"; }
  else {
    account = { id: crypto.randomUUID(), name: setup.name.trim(), email: setup.email.trim(), initial: setup.name.trim().slice(0, 1), color: "#749e90", managementStatus: "ready", receivingStatus: "ready", lastSync: "刚刚", aliasLimit: 100, detail: "地址管理和收件验证均已完成。" };
    data.accounts.push(account);
  }
  let alias = data.aliases.find((item) => item.accountId === account.id && item.enabled);
  if (!alias) { alias = { id: crypto.randomUUID(), accountId: account.id, address: `test.${Math.random().toString(36).slice(2, 8)}@example.test`, purpose: "收件连接测试", group: "工作", enabled: true, poolState: "personal", projectId: null, createdAt: "2026-10-29" }; data.aliases.unshift(alias); }
  addMail(alias, "收件验证完成，你的连接可以使用了");
  addActivity("主号收件验证完成", account.email, "success"); setupDialog.value = false;
  ElMessage.success("模拟收件验证通过，测试邮件已出现在收件箱");
}
async function projectAction(action) {
  const alias = aliasById.value[action.aliasId];
  const project = data.projects.find((item) => item.id === action.projectId);
  if (action.type === "claim") {
    if (!project?.enabled) { ElMessage.warning("请先启用项目"); return; }
    const candidate = data.aliases.find((item) => item.poolState === "available" && item.enabled && project.accountIds.includes(item.accountId) && accountFor(item.accountId)?.receivingStatus === "ready");
    if (!candidate) { ElMessage.warning("没有可领取的地址，请先加入空闲库存"); return; }
    candidate.poolState = "leased"; candidate.projectId = project.id; candidate.leaseExpiresAt = Date.now() + 1800000;
    addActivity("项目领取了一个地址", `${project.name} · ${candidate.address}`, "success"); ElMessage.success("已模拟领取，为项目占用 30 分钟");
  } else if (action.type === "commit" && alias?.poolState === "leased") {
    alias.poolState = "used"; addActivity("项目确认使用地址", alias.address, "success"); ElMessage.success("已确认使用，此地址将保留给当前项目");
  } else if (action.type === "review" && alias?.poolState === "leased") {
    alias.poolState = "review"; ElMessage.warning("已模拟到期：地址进入待确认，未重新分配");
  } else if (action.type === "release" && alias && ["leased", "review"].includes(alias.poolState)) {
    try { await ElMessageBox.confirm("请确认该地址尚未绑定任何业务。确认后，它会回到空闲库存供后续分配。", "核实未使用并归还", { confirmButtonText: "确认未使用，归还", cancelButtonText: "取消", type: "warning" }); } catch { return; }
    alias.poolState = "available"; alias.projectId = null; delete alias.leaseExpiresAt; ElMessage.success("已归还到空闲库存");
  } else if (action.type === "enroll" && alias?.enabled && alias.poolState === "personal" && !alias.projectId) {
    alias.poolState = "available"; ElMessage.success("已加入可分配库存");
  } else if (action.type === "create-project" && action.name?.trim()) {
    if (data.projects.some((item) => item.name === action.name.trim())) { ElMessage.warning("已有同名项目"); return; }
    data.projects.push({ id: crypto.randomUUID(), name: action.name.trim(), description: action.description?.trim() || "通过 API 分配与使用隐私地址", initial: action.name.trim().slice(0, 1), color: "#648d7c", enabled: true, keyHint: "demo", accountIds: data.accounts.map((account) => account.id) }); ElMessage.success("模拟项目已创建");
  } else if (action.type === "toggle-project" && project) {
    project.enabled = !project.enabled; ElMessage.success(project.enabled ? "演示项目已启用" : "演示项目访问已暂停");
  }
}
async function resetDemo() {
  try { await ElMessageBox.confirm("恢复初始示例邮件、地址和项目，清除本次预览中所做的修改。", "重置演示数据", { confirmButtonText: "重置", cancelButtonText: "取消", type: "warning" }); } catch { return; }
  Object.assign(data, createDemoData()); search.value = ""; aliasQuery.value = ""; aliasFilter.value = ""; groupFilter.value = ""; aliasGroup.value = ""; mailFilter.value = "all"; selectedMailId.value = data.messages[0].id; go("inbox"); persist(); ElMessage.success("已恢复初始演示数据");
}
</script>

<template>
  <ElConfigProvider :locale="zhCn">
    <div class="demo-shell">
      <a class="skip-link" href="#workspace-main">跳到主要内容</a>
      <aside class="workspace-sidebar">
        <a class="workspace-brand" href="#inbox" @click="go('inbox')"><span class="brand-mark"><Cloudy /></span><span>iCloud Mail<span class="brand-caption">让每个地址，各有所用。</span></span></a>
        <div class="workspace-identity"><span class="identity-avatar">岚</span><div><strong>林岚的工作区</strong><span>个人空间 <span class="small-dot"></span> {{ data.accounts.length }} 个主号</span></div><span class="workspace-initial">个人</span></div>
        <nav aria-label="主要导航" class="workspace-nav"><span class="nav-caption">工作空间</span><button v-for="item in navItems" :key="item.id" :class="['nav-item', { active: page === item.id }]" :aria-current="page === item.id ? 'page' : undefined" @click="go(item.id)"><component :is="item.icon" /><span>{{ item.label }}</span><span v-if="item.id === 'inbox' && unreadCount" class="nav-count">{{ unreadCount }}</span><span v-if="item.id === 'accounts' && healthyAccounts < data.accounts.length" class="nav-attention"></span></button></nav>
        <div class="sidebar-groups"><div class="nav-caption">地址分组 <button class="icon-button" aria-label="管理地址分组" @click="go('aliases')"><ArrowRight /></button></div><button v-for="group in demoGroups" :key="group" :class="['group-link', { active: page === 'inbox' && groupFilter === group }]" @click="filterGroup(group)"><span class="group-dot" :style="{ background: groupColor(group) }"></span><span>{{ group }}</span><span class="group-count">{{ groupUnread(group) || '' }}</span></button></div>
        <div class="sidebar-bottom"><div class="connection-note"><span :class="['status-dot', { warning: healthyAccounts < data.accounts.length }]"></span><span>{{ healthyAccounts < data.accounts.length ? '有一个收件连接待处理' : '所有收件连接正常' }}</span><button aria-label="查看连接状态" class="icon-button" @click="go('accounts')"><ArrowRight /></button></div><div class="demo-label"><span class="demo-dot"></span> 本地预览 <span>模拟数据 · v1</span></div></div>
      </aside>
      <div class="workspace-main">
        <header class="workspace-topbar"><div class="breadcrumbs"><Cloudy /><span>我的工作区</span><span>/</span><strong>{{ navItems.find(item => item.id === page)?.label }}</strong></div><div class="topbar-right"><span class="preview-badge"><span></span>演示模式</span><button class="icon-button theme-toggle" :aria-label="theme === 'dark' ? '切换浅色' : '切换深色'" @click="theme = theme === 'dark' ? 'light' : 'dark'"><component :is="theme === 'dark' ? Sunny : Moon" /></button><span class="topbar-avatar">L</span></div></header>
        <nav class="mobile-tabs" aria-label="移动导航"><button v-for="item in navItems" :key="item.id" :class="{ active: page === item.id }" @click="go(item.id)"><component :is="item.icon" /><span>{{ item.label }}</span></button></nav>
        <main id="workspace-main">
          <section v-if="page === 'inbox'" class="inbox-workspace">
            <div class="inbox-heading"><div><div class="page-eyebrow">YOUR PRIVATE MAIL</div><h1>收件箱<span class="heading-count">{{ data.messages.filter(mail => !mail.archived).length }}</span></h1><p>所有隐私地址的来信，都在这里。</p></div><button class="btn btn-primary" @click="openAlias()"><Plus />新建隐私地址</button></div>
            <div class="inbox-tools"><label class="search-field"><Search /><input v-model="search" aria-label="搜索邮件" placeholder="搜索邮件、发件人或地址…" /><span class="search-key">⌕</span></label><select v-model="aliasFilter" class="filter-select" aria-label="按隐私地址筛选"><option value="">所有隐私地址</option><option v-for="alias in data.aliases" :key="alias.id" :value="alias.id">{{ alias.purpose }}</option></select><button class="btn btn-quiet sync-button" aria-label="接收新邮件" :disabled="syncing" @click="simulateSync"><Refresh :class="{ spinning: syncing }" /><span>{{ syncing ? '接收中' : '接收新邮件' }}</span></button></div>
            <div v-if="groupFilter || aliasFilter" class="active-filter"><span>{{ groupFilter ? `分组：${groupFilter}` : `地址：${aliasById[aliasFilter]?.address || ''}` }}</span><button class="icon-button" aria-label="清除筛选" @click="groupFilter = ''; aliasFilter = ''"><Close /></button></div>
            <div :class="['mail-workspace', { 'reader-open': mobileReader }]">
              <section class="mail-list-pane" aria-label="邮件列表"><div class="mail-list-controls"><div class="mail-tabs"><button v-for="tab in [{id:'all',label:'全部'},{id:'unread',label:'未读'},{id:'starred',label:'星标'},{id:'archived',label:'已归档'}]" :key="tab.id" :class="{ active: mailFilter === tab.id }" :aria-pressed="mailFilter === tab.id" @click="changeMailFilter(tab.id)">{{ tab.label }}</button></div><span>{{ visibleMessages.length }} 封</span></div><div class="mail-list-scroll"><template v-for="(mail, index) in visibleMessages" :key="mail.id"><div v-if="index === 0 || visibleMessages[index - 1].date !== mail.date" class="mail-date-label">{{ mail.date }}</div><div :class="['mail-row', { selected: selectedMailId === mail.id, unread: mail.unread }]"><button class="mail-row-main" :aria-label="`${mail.sender}：${mail.subject}${mail.unread ? '，未读' : ''}`" :aria-pressed="selectedMailId === mail.id" @click="chooseMail(mail)"><div class="mail-row-top"><span class="sender-name"><span v-if="mail.unread" class="unread-dot"></span>{{ mail.sender }}</span><time>{{ mail.time }}</time></div><strong class="mail-row-subject">{{ mail.subject }}</strong><p>{{ mail.preview }}</p><div class="mail-row-meta"><span :style="{ color: groupColor(aliasById[mail.aliasId]?.group) }"><span class="group-dot" :style="{ background: groupColor(aliasById[mail.aliasId]?.group) }"></span>{{ aliasById[mail.aliasId]?.purpose || '隐私地址' }}</span><span v-if="mail.otp" class="code-label">验证码</span><Document v-else-if="mail.attachments.length" /></div></button><button class="mail-star" :aria-label="mail.starred ? '取消星标' : '标记星标'" :aria-pressed="mail.starred" @click="mail.starred = !mail.starred"><component :is="mail.starred ? StarFilled : Star" :class="{ starred: mail.starred }" /></button></div></template><div v-if="!visibleMessages.length" class="empty-state"><Message /><strong>没有符合条件的邮件</strong><p>试试其他关键词，或清除筛选条件。</p><button class="btn" @click="search = ''; aliasFilter = ''; groupFilter = ''; changeMailFilter('all')">清除筛选</button></div></div><div class="mail-list-footer"><span class="status-dot"></span>{{ syncLabel }}<span class="local-tag">模拟收件</span></div></section>
              <section class="mail-reader" aria-label="邮件阅读"><template v-if="selectedMail && visibleMessages.some(mail => mail.id === selectedMailId)"><div class="reader-toolbar"><button class="icon-button reader-back" aria-label="返回邮件列表" @click="mobileReader = false"><ArrowLeft /></button><div class="reader-toolbar-start"><button class="icon-button" :aria-label="selectedMail.archived ? '移回收件箱' : '归档邮件'" @click="archiveMail"><Box /></button><button class="icon-button" aria-label="删除邮件" @click="deleteMail"><Delete /></button><span class="toolbar-divider"></span><button class="icon-button" :aria-label="selectedMail.unread ? '标记已读' : '标记未读'" @click="selectedMail.unread = !selectedMail.unread"><Message /></button></div><span class="reader-position">{{ visibleMessages.findIndex(mail => mail.id === selectedMailId) + 1 }} / {{ visibleMessages.length }}</span><button class="icon-button" :disabled="visibleMessages.findIndex(mail => mail.id === selectedMailId) <= 0" aria-label="上一封邮件" @click="chooseMail(visibleMessages[visibleMessages.findIndex(mail => mail.id === selectedMailId) - 1])"><ArrowLeft /></button><button class="icon-button" :disabled="visibleMessages.findIndex(mail => mail.id === selectedMailId) >= visibleMessages.length - 1" aria-label="下一封邮件" @click="chooseMail(visibleMessages[visibleMessages.findIndex(mail => mail.id === selectedMailId) + 1])"><ArrowRight /></button></div><div ref="readerBody" class="reader-scroll"><div class="reader-subject"><span class="reader-category">{{ selectedMail.category }}</span><h2>{{ selectedMail.subject }}</h2></div><div class="sender-details"><span class="sender-avatar" :style="{ '--sender-color': selectedMail.color }">{{ selectedMail.initial }}</span><div><strong>{{ selectedMail.sender }} <span>&lt;{{ selectedMail.senderEmail }}&gt;</span></strong><button class="receiver-details-toggle" :aria-expanded="showSource" @click="showSource = !showSource">发送至 {{ selectedAlias?.address }} <ArrowRight :class="{ expanded: showSource }" /></button></div><time>{{ selectedMail.date }} {{ selectedMail.time.replace('昨天 ', '') }}</time><button class="icon-button" :aria-label="selectedMail.starred ? '取消星标' : '标记星标'" @click="selectedMail.starred = !selectedMail.starred"><component :is="selectedMail.starred ? StarFilled : Star" :class="{ starred: selectedMail.starred }" /></button></div><dl v-if="showSource" class="message-source"><div><dt>实际收件箱</dt><dd>{{ accountFor(selectedAlias?.accountId)?.email }}</dd></div><div><dt>地址用途</dt><dd>{{ selectedAlias?.purpose }}</dd></div><div><dt>数据来源</dt><dd>本地模拟邮件</dd></div></dl><div v-if="selectedMail.otp" class="otp-strip"><span><Key />邮件验证码</span><strong>{{ selectedMail.otp }}</strong><button class="btn" @click="copy(selectedMail.otp, '验证码')"><CopyDocument />复制</button></div><article class="email-body"><p v-for="(paragraph, index) in selectedMail.paragraphs" :key="index">{{ paragraph }}</p><button v-if="selectedMail.link" class="mail-document-link" @click="openLink(selectedMail.link)"><Link />{{ selectedMail.link.label }}<ArrowRight /></button></article><div v-if="selectedMail.attachments.length" class="attachments"><span>{{ selectedMail.attachments.length }} 个附件</span><button v-for="attachment in selectedMail.attachments" :key="attachment.name" class="attachment" @click="downloadAttachment(attachment)"><span class="attachment-icon"><Document /></span><span><strong>{{ attachment.name }}</strong><small>模拟附件 · {{ attachment.size }}</small></span><Download /></button></div><div class="delivery-note"><span class="status-dot"></span>通过隐私地址送达<span class="pill">{{ selectedAlias?.group }}</span><button class="text-button" @click="copy(selectedAlias.address, '隐私地址')">复制地址<CopyDocument /></button></div></div></template><div v-else class="empty-state reader-empty"><Message /><strong>让每一封来信，都有自己的位置</strong><p>选择左侧邮件，阅读完整正文。</p></div></section>
            </div>
          </section>

          <section v-else-if="page === 'aliases'" class="workspace-page"><div class="page-heading"><div><div class="page-eyebrow">PRIVATE ADDRESSES</div><h1>隐私地址</h1><p class="page-description">给不同的事情，一个独立的地址。</p></div><button class="btn btn-primary" @click="openAlias()"><Plus />新建隐私地址</button></div><div class="address-summary"><span><strong>{{ data.aliases.length }}</strong> 个隐私地址</span><span><span class="status-dot"></span>{{ data.aliases.filter(alias => alias.enabled).length }} 个接收中</span><span><Folder />{{ demoGroups.length }} 个分组</span></div><div class="section-toolbar"><label class="search-field"><Search /><input v-model="aliasQuery" aria-label="搜索隐私地址" placeholder="搜索地址或用途…" /></label><select v-model="aliasGroup" class="filter-select" aria-label="筛选地址分组"><option value="">所有分组</option><option v-for="group in demoGroups" :key="group">{{ group }}</option></select></div><div class="table-scroll"><table class="data-table alias-table"><thead><tr><th>隐私地址 / 用途</th><th>分组</th><th>所属主号</th><th>接收状态</th><th>邮件</th><th class="actions-cell">操作</th></tr></thead><tbody><tr v-for="alias in filteredAliases" :key="alias.id"><td><div class="address-cell"><span class="address-symbol"><Message /></span><div><strong>{{ alias.purpose }}</strong><button class="address-copy" @click="copy(alias.address, '隐私地址')">{{ alias.address }}<CopyDocument /></button></div></div></td><td><span class="group-pill"><span class="group-dot" :style="{ background: groupColor(alias.group) }"></span>{{ alias.group }}</span></td><td><span class="account-table-name">{{ accountFor(alias.accountId)?.name }}</span></td><td><span :class="['pill', alias.enabled ? 'pill-success' : 'pill-muted']"><span :class="['status-dot', { muted: !alias.enabled }]"></span>{{ alias.enabled ? '接收中' : '已暂停' }}</span></td><td>{{ mailCount(alias.id) }}</td><td><div class="row-actions"><button class="btn btn-small" @click="viewAlias(alias)">查看来信</button><button class="icon-button" :aria-label="`编辑${alias.purpose}`" @click="openAlias(alias)"><Setting /></button><button class="icon-button" :aria-label="`${alias.enabled ? '暂停' : '恢复'}${alias.purpose}`" @click="toggleAlias(alias)"><component :is="alias.enabled ? Hide : View" /></button></div></td></tr></tbody></table><div v-if="!filteredAliases.length" class="empty-state"><Search /><strong>没有找到这个地址</strong><p>可以修改关键词或分组。</p></div></div><div class="table-footnote">{{ filteredAliases.length }} 个地址<span>地址、用途与接收状态均为模拟数据</span></div></section>

          <section v-else-if="page === 'accounts'" class="workspace-page"><div class="page-heading"><div><div class="page-eyebrow">CONNECTED ACCOUNTS</div><h1>主号</h1><p class="page-description">管理地址和读取邮件，各自的状态都看得见。</p></div><button class="btn btn-primary" @click="openSetup()"><Plus />添加主号</button></div><div class="account-list"><article v-for="account in data.accounts" :key="account.id" class="account-card"><div class="account-card-header"><span class="account-avatar" :style="{ '--sender-color': account.color }">{{ account.initial }}</span><div><h2>{{ account.name }}</h2><p>{{ account.email }}</p></div><span :class="['pill', account.receivingStatus === 'ready' ? 'pill-success' : 'pill-warning']">{{ account.receivingStatus === 'ready' ? '连接正常' : '需要处理' }}</span></div><div class="account-capabilities"><div><span class="capability-icon"><Collection /></span><div><strong>管理隐私地址</strong><span>创建、同步和暂停地址</span></div><span :class="['capability-status', { warning: account.managementStatus !== 'ready' }]"><component :is="account.managementStatus === 'ready' ? CircleCheckFilled : Warning" />{{ account.managementStatus === 'ready' ? '可以使用' : '需要连接' }}</span></div><div><span class="capability-icon"><Message /></span><div><strong>读取邮件</strong><span>{{ account.receivingStatus === 'ready' ? '收件验证已通过' : '收件凭据需要更新，历史邮件仍可查看' }}</span></div><span :class="['capability-status', { warning: account.receivingStatus !== 'ready' }]"><component :is="account.receivingStatus === 'ready' ? CircleCheckFilled : Warning" />{{ account.receivingStatus === 'ready' ? '可以收件' : '待验证' }}</span></div></div><div class="account-card-footer"><span>{{ data.aliases.filter(alias => alias.accountId === account.id).length }} 个隐私地址<span class="footer-dot">·</span>最近收件 {{ account.lastSync }}</span><button class="btn" @click="openSetup(account)">{{ account.receivingStatus === 'ready' ? '检查连接' : '更新并验证收件' }}<ArrowRight /></button></div></article></div><div class="context-note"><InfoFilled /><p>创建地址需要 Apple 管理连接，读取邮件需要收件连接。添加主号时会分别验证，避免“地址能建，但邮件收不到”。</p></div></section>

          <ProjectPanel v-else-if="page === 'projects'" :aliases="data.aliases" :projects="data.projects" :accounts="data.accounts" @action="projectAction" />

          <section v-else class="workspace-page settings-page"><div class="page-heading"><div><div class="page-eyebrow">WORKSPACE SETTINGS</div><h1>设置与诊断</h1><p class="page-description">让收件方式符合你的使用习惯。</p></div></div><div class="settings-section"><h2>收件偏好</h2><div class="setting-row"><div><strong>首次同步范围</strong><p>新接入主号时，补取这个时间范围内的历史邮件。</p></div><select v-model="syncWindow" class="filter-select" aria-label="首次同步范围"><option value="1">最近 1 天</option><option value="7">最近 7 天</option><option value="30">最近 30 天</option></select></div><div class="setting-row"><div><strong>后台接收新邮件</strong><p>无需打开取码链接，也能接收新来信。</p></div><ElSwitch v-model="keepReceiving" aria-label="后台接收新邮件" /></div><div class="setting-row"><div><strong>外观</strong><p>选择适合当前环境的工作区外观。</p></div><div class="segmented"><button :class="{ active: theme === 'light' }" @click="theme = 'light'"><Sunny />浅色</button><button :class="{ active: theme === 'dark' }" @click="theme = 'dark'"><Moon />深色</button></div></div><p class="setting-demo-note">收件偏好在本地预览中仅展示配置效果，不会启动真实同步。</p></div><div class="settings-section"><h2>最近活动</h2><div v-for="activity in data.activities.slice(0, 8)" :key="activity.id" class="activity-row"><span :class="['activity-icon', activity.type]"><component :is="activity.type === 'warning' ? Warning : activity.type === 'success' ? Check : Refresh" /></span><div><strong>{{ activity.title }}</strong><p>{{ activity.description }}</p></div><time>{{ activity.time }}</time></div></div><div class="settings-section reset-section"><div><h2>本地演示数据</h2><p>全部邮件、地址和连接均为模拟。操作保存在当前浏览器中。</p></div><button class="btn" @click="resetDemo"><Refresh />重置演示</button></div></section>
        </main>
      </div>
    </div>

    <ElDialog v-model="aliasDialog" :title="editingAliasId ? '编辑隐私地址' : '新建隐私地址'" width="460px" class="demo-dialog" align-center><form id="alias-form" class="demo-form" @submit.prevent="saveAlias"><label>这个地址用来做什么？<input v-model="aliasForm.purpose" maxlength="60" placeholder="例如：旅行预订、工作联络" required autofocus /></label><label>所属主号<select v-model="aliasForm.accountId" :disabled="Boolean(editingAliasId)"><option v-for="account in data.accounts" :key="account.id" :value="account.id">{{ account.name }} · {{ account.email }}</option></select></label><label>分组<select v-model="aliasForm.group"><option v-for="group in demoGroups" :key="group">{{ group }}</option></select></label><div class="generated-address"><span>隐私地址</span><strong>{{ aliasForm.address }}</strong><small>演示地址，仅在当前浏览器生效</small></div><label v-if="!editingAliasId" class="checkbox-label"><input v-model="aliasForm.sendMail" type="checkbox" />同时接收一封示例邮件</label></form><template #footer><button class="btn" @click="aliasDialog = false">取消</button><button class="btn btn-primary" type="submit" form="alias-form">{{ editingAliasId ? '保存修改' : '创建地址' }}<ArrowRight /></button></template></ElDialog>
    <ElDialog v-model="setupDialog" title="把主号连接到工作区" width="540px" class="demo-dialog" align-center :close-on-click-modal="false" :before-close="(done) => { if (!setupBusy) done(); }"><div class="setup-steps"><span v-for="(step, index) in ['账户', '地址管理', '邮件接收', '收件验证']" :key="step" :class="{ active: setupStep === index, done: setupStep > index }"><b><Check v-if="setupStep > index" /><template v-else>{{ index + 1 }}</template></b>{{ step }}</span></div><div class="setup-notice"><InfoFilled />连接过程为模拟，无需提供真实密码。</div><div v-if="setupStep === 0" class="demo-form"><label>主号名称<input v-model="setup.name" placeholder="例如：我的工作主号" maxlength="40" /></label><label>演示邮箱<input v-model="setup.email" placeholder="name@example.test" type="email" /><small>请使用 example.test 域名的示例地址。</small></label></div><div v-else-if="setupStep === 1" class="setup-stage"><span :class="['setup-stage-icon', { success: setup.managed }]"><component :is="setup.managed ? CircleCheckFilled : Connection" /></span><h3>{{ setup.managed ? '隐私地址管理已连接' : '连接 Apple 地址管理' }}</h3><p>连接后，可以在工作区创建和同步隐私地址。</p><button class="btn" :disabled="setupBusy || setup.managed" @click="simulateConnection"><Refresh v-if="setupBusy" class="spinning" />{{ setup.managed ? '连接成功' : setupBusy ? '正在模拟连接…' : '模拟连接 Apple' }}</button></div><div v-else-if="setupStep === 2" class="setup-stage"><span class="setup-stage-icon"><Message /></span><h3>选择邮件接收方式</h3><p>从主号的实际收件箱读取邮件，再归到各个隐私地址。</p><div class="transport-options"><label :class="{ selected: setup.transport === 'imap' }"><input v-model="setup.transport" type="radio" value="imap" /><strong>IMAP 收件</strong><span>使用邮箱的标准收件连接</span></label><label :class="{ selected: setup.transport === 'webmail' }"><input v-model="setup.transport" type="radio" value="webmail" /><strong>iCloud 网页收件</strong><span>使用已连接的网页会话</span></label></div></div><div v-else class="setup-stage"><span :class="['setup-stage-icon', { success: setup.received }]"><component :is="setup.received ? CircleCheckFilled : Message" /></span><h3>{{ setup.received ? '邮件已收到，连接验证通过' : '最后一步：真正收到一封邮件' }}</h3><p>{{ setup.received ? '完成后，测试邮件会出现在收件箱。' : '模拟发出并读取测试邮件，确认接收链路可以使用。' }}</p><button class="btn" :disabled="setupBusy || setup.received" @click="simulateTest"><Refresh v-if="setupBusy" class="spinning" />{{ setup.received ? '测试邮件已收到' : setupBusy ? '正在模拟收件…' : '模拟发送并接收测试邮件' }}</button></div><template #footer><button class="btn" :disabled="setupBusy" @click="setupStep > 0 ? setupStep-- : setupDialog = false">{{ setupStep > 0 ? '上一步' : '取消' }}</button><button v-if="setupStep < 3" class="btn btn-primary" :disabled="setupBusy || (setupStep === 1 && !setup.managed)" @click="setupNext">继续<ArrowRight /></button><button v-else class="btn btn-primary" :disabled="!setup.received || setupBusy" @click="finishSetup">完成连接<Check /></button></template></ElDialog>
    <ElDialog v-model="linkDialog" :title="shownLink?.label || '邮件链接'" width="460px" class="demo-dialog" align-center><div class="demo-link-info"><Link /><p>这是模拟邮件里的示例链接。</p><code>{{ shownLink?.url }}</code><span>预览不会跳转到外部服务。</span></div><template #footer><button class="btn" @click="linkDialog = false">关闭</button><button class="btn btn-primary" @click="copy(shownLink.url, '示例链接')"><CopyDocument />复制链接</button></template></ElDialog>
  </ElConfigProvider>
</template>
