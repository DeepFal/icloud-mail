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
    <div v-if="managementMissing" class="setup-status__item">
      <span class="pg-dot is-warning">Apple 未连接</span>
      <el-button link type="primary" :disabled="busy" @click="emit('connect')">连接 Apple</el-button>
    </div>
    <div v-if="receivingMissing" class="setup-status__item">
      <span class="pg-dot is-warning">收件待验证</span>
      <el-button link type="primary" :disabled="busy" @click="emit('configure')">配置凭据</el-button>
      <el-button link type="primary" :disabled="busy" @click="emit('sync')">验证收件</el-button>
    </div>
  </section>
</template>

<style scoped>
.setup-status { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 24px; min-height: 28px; font-size: 13px; }
.setup-status__item { display: flex; align-items: center; gap: 12px; }
.setup-status__item .pg-dot { color: var(--text); }
</style>
