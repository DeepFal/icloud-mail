<template>
  <div v-if="tags.length" class="apple-connection-tags">
    <el-tag v-for="tag in tags" :key="tag.name" :type="tag.type" effect="plain" size="small" round>
      {{ tag.name }}：{{ tag.label }}
    </el-tag>
    <small v-if="cooldownUntil">创建冷却至 {{ formatTime(cooldownUntil) }}</small>
  </div>
  <span v-else>—</span>
</template>

<script setup>
import { computed } from "vue";

import { formatTime } from "../utils/format.js";
import { appleConnectionTags } from "../utils/workspaceConnection.js";

const props = defineProps({ account: { type: Object, required: true } });
const tags = computed(() => appleConnectionTags(props.account.apple));
const cooldownUntil = computed(() => props.account.apple?.creationCooldownUntil || "");
</script>

<style scoped>
.apple-connection-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}
.apple-connection-tags small {
  flex-basis: 100%;
  color: var(--el-text-color-secondary);
}
</style>
