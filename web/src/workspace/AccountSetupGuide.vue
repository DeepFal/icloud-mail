<script setup>
import { computed } from "vue";
const props = defineProps({
  account: { type: Object, required: true },
  managementReady: Boolean,
  custom: Boolean,
  busy: Boolean,
});
const emit = defineEmits(["connect", "configure", "sync"]);
const managementMissing = computed(() => !props.custom && !props.managementReady);
const receivingMissing = computed(() => !props.account.lastSyncedAt || props.account.lastSyncStatus !== "ok");
</script>

<template>
  <section v-if="account.enabled && (managementMissing || receivingMissing)" class="setup-status" aria-label="待完成的连接">
    <strong>连接设置</strong>
    <div v-if="managementMissing"><span>Apple 未连接</span><button :disabled="busy" @click="emit('connect')">连接 Apple</button></div>
    <div v-if="receivingMissing"><span>收件待验证</span><button :disabled="busy" @click="emit('configure')">配置凭据</button><button :disabled="busy" @click="emit('sync')">验证收件</button></div>
  </section>
</template>

<style scoped>
.setup-status { display: flex; align-items: center; flex-wrap: wrap; gap: 10px 24px; padding: 12px 16px; border: 1px solid var(--border); border-radius: 8px; background: var(--surface); font-size: 12px; }
.setup-status strong { font-weight: 600; }
.setup-status div { display: flex; align-items: center; gap: 10px; }
.setup-status span { color: var(--text-secondary); }
.setup-status button { border: 0; background: transparent; color: var(--primary); font: inherit; cursor: pointer; padding: 4px; }
.setup-status button:disabled { opacity: .5; cursor: not-allowed; }
</style>
