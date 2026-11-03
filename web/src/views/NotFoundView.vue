<template>
  <main class="not-found-page">
    <section class="not-found-panel" aria-labelledby="not-found-title">
      <AppBrand title="iCloud 隐私邮箱" />
      <h1 id="not-found-title">页面不存在</h1>
      <p>这个地址没有对应的后台页面，可能是链接已过期或输入有误。</p>
      <el-button type="primary" @click="returnHome">返回首页</el-button>
    </section>
  </main>
</template>

<script setup>
import { useRouter } from "vue-router";

import { getAccountPage } from "../api/admin.js";
import AppBrand from "../components/AppBrand.vue";
import { resolveLandingRoute } from "../utils/authFlow.js";

const router = useRouter();

async function returnHome() {
  router.replace(await resolveLandingRoute(getAccountPage));
}
</script>

<style scoped>
.not-found-page {
  background: var(--app-bg);
}

.not-found-panel {
  display: flex;
  width: min(100%, 360px);
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 24px;
  border-radius: var(--radius-overlay);
  text-align: left;
}

.not-found-panel h1 {
  margin: 8px 0 0;
  color: var(--text);
  font-size: 16px;
  line-height: 24px;
  font-weight: 600;
}

.not-found-panel p {
  margin: 0 0 8px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 20px;
}
</style>
