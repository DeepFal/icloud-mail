<template>
  <el-tooltip v-if="compact && reasons.length > 1" :content="reasons.join('、')" :trigger="['hover', 'focus']" placement="top">
    <span class="attention-reasons attention-reasons--compact" tabindex="0" :aria-label="reasons.join('、')">
      <el-tag type="danger" effect="plain" size="small" round>{{ reasons[0] }}</el-tag>
      <span class="attention-reasons__more" aria-hidden="true">+{{ reasons.length - 1 }}</span>
    </span>
  </el-tooltip>
  <div v-else-if="reasons.length" class="attention-reasons">
    <el-tag v-for="reason in reasons" :key="reason" type="danger" effect="plain" size="small" round>{{ reason }}</el-tag>
  </div>
  <span v-else>—</span>
</template>

<script setup>
import { computed } from "vue";

import { accountAttentionReasons } from "../utils/workspaceConnection.js";

const props = defineProps({ account: { type: Object, required: true }, compact: { type: Boolean, default: false } });
const reasons = computed(() => accountAttentionReasons(props.account));
</script>

<style scoped>
.attention-reasons {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.attention-reasons--compact { flex-wrap: nowrap; align-items: center; border-radius: 4px; }
.attention-reasons--compact:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 2px; }
.attention-reasons__more { color: var(--danger); font-size: 12px; font-variant-numeric: tabular-nums; }
</style>
