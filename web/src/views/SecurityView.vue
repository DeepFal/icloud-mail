<template>
  <section class="pg" aria-labelledby="security-form-title">
    <header class="pg-bar">
      <h1 id="security-form-title" class="pg-title">安全设置</h1>
    </header>

    <el-form
      ref="formRef"
      class="pg-form pg-form--aligned security-form"
      :model="form"
      :rules="rules"
      label-position="left"
      label-width="140px"
      :disabled="submitting"
      status-icon
      @submit.prevent="submit"
    >
      <fieldset class="pg-fieldset">
        <legend>管理员密码</legend>
        <RequestAlert
          v-if="submitError"
          class="security-form__alert"
          :error="submitError"
          closable
          @close="submitError = null"
        />

        <el-form-item label="当前密码" prop="currentPassword">
          <el-input
            v-model="form.currentPassword"
            type="password"
            name="current-password"
            autocomplete="current-password"
            show-password
          />
        </el-form-item>

        <el-form-item label="新密码" prop="newPassword">
          <el-input
            v-model="form.newPassword"
            type="password"
            name="new-password"
            autocomplete="new-password"
            show-password
          />
          <p class="pg-help">12–72 字节，当前 {{ newPasswordBytes }} 字节。</p>
        </el-form-item>

        <el-form-item label="确认新密码" prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            name="confirm-password"
            autocomplete="new-password"
            show-password
          />
        </el-form-item>

        <el-form-item class="security-form__submit">
          <el-button
            native-type="submit"
            type="primary"
            :loading="submitting"
          >
            {{ submitting ? "更新中…" : "更新密码" }}
          </el-button>
          <p class="pg-help">更新后所有管理员会话都会退出，需要重新登录。</p>
        </el-form-item>
      </fieldset>
    </el-form>
  </section>
</template>

<script setup>
import { computed, nextTick, reactive, ref } from "vue";
import { useRouter } from "vue-router";

import { updatePassword } from "../api/admin.js";
import RequestAlert from "../components/RequestAlert.vue";
import { useAuth } from "../stores/auth.js";
import { createActionLock } from "../utils/asyncState.js";
import { utf8Length } from "../utils/format.js";

const router = useRouter();
const auth = useAuth();
const formRef = ref(null);
const submitting = ref(false);
const submitError = ref(null);
const submitLock = createActionLock();
const form = reactive({
  currentPassword: "",
  newPassword: "",
  confirmPassword: "",
});

const newPasswordBytes = computed(() => utf8Length(form.newPassword));

function validateNewPassword(_, value, callback) {
  const length = utf8Length(value);
  if (length < 12 || length > 72) {
    callback(new Error("新密码长度需要在 12 到 72 字节之间"));
    return;
  }
  callback();
}

function validateConfirmation(_, value, callback) {
  if (!value) {
    callback(new Error("请再次填写新密码"));
    return;
  }
  if (value !== form.newPassword) {
    callback(new Error("两次填写的新密码不一致"));
    return;
  }
  callback();
}

const rules = {
  currentPassword: [
    { required: true, message: "请填写当前密码", trigger: "blur" },
  ],
  newPassword: [{ validator: validateNewPassword, trigger: ["blur", "change"] }],
  confirmPassword: [
    { validator: validateConfirmation, trigger: ["blur", "change"] },
  ],
};

function clearPasswords() {
  Object.assign(form, {
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
  });
}

async function submit() {
  if (!submitLock.acquire()) return;
  submitting.value = true;
  try {
    submitError.value = null;
    const valid = await formRef.value?.validate().catch(() => false);
    if (!valid) return;

    await updatePassword(
      {
        current_password: form.currentPassword,
        new_password: form.newPassword,
        confirm_password: form.confirmPassword,
      },
      auth.state.csrfToken,
    );
    clearPasswords();
    auth.clearSession({ checked: false });
    await router.replace({ name: "login", query: { notice: "password_changed" } });
  } catch (error) {
    clearPasswords();
    submitError.value = error;
    await nextTick();
    formRef.value?.clearValidate();
  } finally {
    submitting.value = false;
    submitLock.release();
  }
}
</script>

<style scoped>
.security-form__alert {
  margin-bottom: 12px;
}

.security-form__submit {
  margin-top: 8px;
}
</style>
