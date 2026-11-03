<template>
  <div v-if="reasons.length" class="attention-reasons">
    <el-tag v-for="reason in reasons" :key="reason" type="danger" effect="plain" size="small" round>{{ reason }}</el-tag>
  </div>
  <span v-else>—</span>
</template>

<script setup>
import { computed } from "vue";

import { accountAttentionReasons } from "../utils/workspaceConnection.js";

const props = defineProps({ account: { type: Object, required: true } });
const reasons = computed(() => accountAttentionReasons(props.account));
</script>

<style scoped>
.attention-reasons {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>
