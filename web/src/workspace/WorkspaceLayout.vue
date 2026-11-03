<template>
  <div :class="['workspace-app', { 'is-collapsed': collapsed }]">
    <a class="workspace-skip" href="#workspace-main">跳到主要内容</a>
    <aside id="workspace-sidebar" class="workspace-sidebar" aria-label="后台导航">
      <div class="workspace-brand"><AppBrand title="iCloud Mail" /></div>
      <nav class="workspace-nav" aria-label="主要导航">
        <span class="workspace-nav__group">工作区</span>
        <RouterLink v-for="item in primary" :key="item.name" :to="item.to" :class="navClass(item)" :title="collapsed ? item.label : undefined" :aria-current="active(item) ? 'page' : undefined"><component :is="item.icon" aria-hidden="true" /><span>{{ item.label }}</span></RouterLink>
        <span class="workspace-nav__group">管理</span>
        <RouterLink v-for="item in admin" :key="item.name" :to="item.to" :class="navClass(item)" :title="collapsed ? item.label : undefined" :aria-current="active(item) ? 'page' : undefined"><component :is="item.icon" aria-hidden="true" /><span>{{ item.label }}</span></RouterLink>
      </nav>
      <div class="workspace-sidebar__bottom"><RouterLink class="workspace-connection" :to="connectionTarget" :title="collapsed ? connectionState.label : undefined"><i :class="`is-${connectionState.kind}`"></i><span>{{ connectionState.label }}</span></RouterLink><div class="workspace-user"><span>{{ auth.state.username }}</span><button type="button" :disabled="logoutLoading" :title="collapsed ? '退出' : undefined" @click="logout"><SwitchButton aria-hidden="true" /><span>退出</span></button></div></div>
    </aside>
    <el-drawer id="workspace-drawer" v-model="drawerOpen" class="workspace-drawer" direction="ltr" size="min(84vw, 290px)" :with-header="false" title="后台导航" append-to-body>
      <div class="workspace-drawer__inner"><AppBrand title="iCloud Mail" /><RouterLink class="workspace-connection" :to="connectionTarget" @click="drawerOpen=false"><i :class="`is-${connectionState.kind}`"></i><span>{{ connectionState.label }}</span></RouterLink><nav class="workspace-nav" aria-label="移动导航"><RouterLink v-for="item in [...primary, ...admin]" :key="item.name" :to="item.to" :class="navClass(item)" :aria-current="active(item) ? 'page' : undefined" @click="drawerOpen=false"><component :is="item.icon" aria-hidden="true" /><span>{{ item.label }}</span></RouterLink></nav></div>
    </el-drawer>
    <div class="workspace-main">
      <header class="workspace-topbar">
        <button type="button" class="workspace-icon-button workspace-menu" aria-label="打开导航" :aria-expanded="drawerOpen" aria-controls="workspace-drawer" @click="drawerOpen=true"><Menu aria-hidden="true" /></button>
        <button type="button" class="workspace-icon-button workspace-collapse" :aria-label="collapsed ? '展开侧栏' : '收起侧栏'" :title="collapsed ? '展开侧栏' : '收起侧栏'" :aria-expanded="!collapsed" aria-controls="workspace-sidebar" @click="toggleCollapsed"><Expand v-if="collapsed" aria-hidden="true" /><Fold v-else aria-hidden="true" /></button>
        <div class="workspace-location"><RouterLink v-if="parent" :to="parent.to">{{ parent.label }}</RouterLink><span v-else>{{ group }}</span><span aria-hidden="true">/</span><h1 v-if="parent">{{ page.title }}</h1><span v-else aria-current="page">{{ page.title }}</span></div>
        <div class="workspace-topbar__right"><ThemeControl /></div>
      </header>
      <main ref="contentContainer" id="workspace-main" :class="['workspace-content', { 'is-split': route.meta.split }]"><router-view /></main>
    </div>
  </div>
</template>
<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { Collection, Connection, Document, Expand, Fold, Key, Lock, Menu, Message, Notebook, SwitchButton, Tickets } from "@element-plus/icons-vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import AppBrand from "../components/AppBrand.vue";
import ThemeControl from "../components/ThemeControl.vue";
import { useAuth } from "../stores/auth.js";
import { usePageHeader } from "../stores/page.js";
import { getAllAccounts } from "../api/admin.js";
import { createLiveRefresh } from "../utils/liveRefresh.js";
import { connectionTarget as getConnectionTarget, summarizeConnections } from "../utils/workspaceConnection.js";
import "./workspace-shell.css";
const auth = useAuth(); const page = usePageHeader(); const route = useRoute(); const router = useRouter(); const drawerOpen = ref(false); const logoutLoading = ref(false); const accounts = ref([]); const contentContainer = ref(null);
const primary = [{ name: "inbox", label: "收件箱", to: { name: "inbox" }, icon: Message }, { name: "aliases", label: "隐私地址", to: { name: "aliases" }, icon: Collection }, { name: "services", label: "服务登记", to: { name: "services" }, icon: Notebook }, { name: "accounts", label: "主号", to: { name: "accounts" }, icon: Connection }, { name: "pool", label: "邮箱池", to: { name: "pool" }, icon: Key }, { name: "tasks", label: "任务", to: { name: "tasks" }, icon: Tickets }];
const admin = [{ name: "audit", label: "操作记录", to: { name: "audit" }, icon: Document }, { name: "logs", label: "运行日志", to: { name: "logs" }, icon: Tickets }, { name: "security", label: "安全设置", to: { name: "security" }, icon: Lock }];
const COLLAPSE_KEY = "icloud-admin-sidebar";
const collapsed = ref(readCollapsed());
function readCollapsed() { try { return localStorage.getItem(COLLAPSE_KEY) === "collapsed"; } catch { return false; } }
function toggleCollapsed() { collapsed.value = !collapsed.value; try { localStorage.setItem(COLLAPSE_KEY, collapsed.value ? "collapsed" : "open"); } catch { /* optional */ } }
// The top bar only says where you are; each page owns its single h1 (nested pages use this one).
const parent = computed(() => primary.find((item) => item.name === route.meta.parent) || null);
const group = computed(() => admin.some((item) => active(item)) ? "管理" : "工作区");
const accountsLoaded = ref(false);
const accountsError = ref(false);
const connectionState = computed(() => summarizeConnections(accounts.value, accountsLoaded.value, accountsError.value));
const connectionTarget = computed(() => getConnectionTarget(accounts.value, accountsLoaded.value, accountsError.value));
function active(item) { if (item.name === "accounts") return route.path === "/accounts" || route.path.startsWith("/accounts/"); return route.name === item.name || route.path.startsWith(`/${item.name}`); }
function navClass(item) { return ["workspace-nav__item", { active: active(item) }]; }
async function logout() { if (logoutLoading.value) return; logoutLoading.value = true; try { await auth.logout(); await router.replace({ name: "login" }); } catch (e) { ElMessage.error(e.message || "退出失败，请稍后重试"); } finally { logoutLoading.value = false; } }
watch(() => route.fullPath, () => { drawerOpen.value = false; contentContainer.value?.scrollTo({ top: 0 }); }, { flush: "post" });
let disposed = false;
async function loadAccounts() { try { const v = await getAllAccounts(); if (disposed) return true; accounts.value = Array.isArray(v) ? v : []; accountsError.value = false; } catch { if (!disposed) accountsError.value = true; return false; } finally { if (!disposed) accountsLoaded.value = true; } return true; }
const liveRefresh = createLiveRefresh(() => loadAccounts(), { intervalMs: 30_000 });
onMounted(() => { liveRefresh.start(); window.addEventListener("icloud:receiving-updated", liveRefresh.refreshNow); });
onUnmounted(() => { disposed = true; liveRefresh.stop(); window.removeEventListener("icloud:receiving-updated", liveRefresh.refreshNow); });
</script>
