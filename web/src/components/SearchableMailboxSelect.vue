<script setup>
import { computed, ref, watch } from "vue";
import { matchesMailboxOption } from "../workspace/registry-api.js";
const props = defineProps({
  modelValue: { type: [String, Number], default: "" },
  options: { type: Array, default: () => [] },
  placeholder: { type: String, default: "选择邮箱" },
  ariaLabel: { type: String, required: true },
  loading: Boolean,
  disabled: Boolean,
});
const emit = defineEmits(["update:modelValue", "change"]);
const keyword = ref("");
const filtered = computed(() => props.options.filter((option) => matchesMailboxOption(option, keyword.value)));
watch(() => props.modelValue, () => { keyword.value = ""; });
function change(value) { keyword.value = ""; emit("update:modelValue", value ?? ""); emit("change", value ?? ""); }
</script>
<template>
  <el-select
    class="searchable-mailbox-select"
    :model-value="modelValue" filterable remote clearable
    :remote-method="(query) => keyword = query" :loading="loading" :disabled="disabled"
    :placeholder="placeholder" :aria-label="ariaLabel"
    :fit-input-width="false" :teleported="true" popper-class="mailbox-select-popper"
    no-data-text="没有匹配结果" no-match-text="没有匹配结果"
    @update:model-value="change" @visible-change="(visible) => { if (!visible) keyword = ''; }"
  >
    <el-option v-for="item in filtered" :key="item.value" :value="item.value" :label="item.label" :disabled="item.disabled">
      <div class="mailbox-option"><span>{{ item.label }}</span><small v-if="item.note || item.secondary">{{ item.note || item.secondary }}</small></div>
    </el-option>
  </el-select>
</template>
<style>
.mailbox-select-popper { max-width: calc(100vw - 24px); }
.el-popper.mailbox-select-popper { width: max-content; }
/* Allow content width to grow within the viewport, accounting for the popper's borders. */
.mailbox-select-popper .el-select-dropdown,
.mailbox-select-popper .el-select-dropdown__list { max-width: calc(100vw - 26px); }
.mailbox-select-popper .el-select-dropdown__wrap { max-height: 260px; }
.mailbox-select-popper .el-select-dropdown__item { display: flex; align-items: center; height: auto; min-height: 34px; line-height: normal; }
.mailbox-option { display: grid; align-content: center; gap: 2px; width: 100%; min-width: 0; padding: 6px 0; line-height: 20px; }
.mailbox-option > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
.mailbox-option > small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-secondary); font-size: 12px; line-height: 18px; font-weight: 400; }
.mailbox-select-popper .is-selected .mailbox-option > small { color: var(--primary); }
@media (max-width: 740px) {
  .searchable-mailbox-select .el-select__wrapper,
  .mailbox-select-popper .el-select-dropdown__item { min-height: 44px; }
}
</style>
