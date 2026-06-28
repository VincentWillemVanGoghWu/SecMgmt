<script setup lang="ts">
import { reactive, ref } from "vue"
import { useRoute, useRouter } from "vue-router"

import { useAuthStore } from "../../stores"

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const loading = ref(false)
const errorMessage = ref("")
const formState = reactive({
  username: "admin",
  password: "admin123456",
})

const handleSubmit = async () => {
  loading.value = true
  errorMessage.value = ""

  try {
    await authStore.login(formState)
    const redirect = String(route.query.redirect ?? "/dashboard")
    await router.replace(redirect)
  } catch (error) {
    errorMessage.value = "登录失败，请检查用户名和密码。"
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-page__panel">
      <div class="login-page__hero">
        <div class="login-page__brand">SmartLink</div>
        <div class="login-page__subtitle">南宁慧联电子科技有限公司</div>
      </div>

      <div class="login-page__form-card">
        <h1 class="login-page__title">系统登录</h1>
        <p class="login-page__hint">新开账户或密码有误请联系管理员。</p>

        <div class="app-field">
          <label>用户名</label>
          <input v-model="formState.username" type="text" placeholder="请输入用户名" />
        </div>

        <div class="app-field">
          <label>密码</label>
          <input
            v-model="formState.password"
            type="password"
            placeholder="请输入密码"
            @keydown.enter="handleSubmit"
          />
        </div>

        <div v-if="errorMessage" class="login-page__error">{{ errorMessage }}</div>

        <button class="app-button app-button--primary login-page__submit" :disabled="loading" @click="handleSubmit">
          {{ loading ? "登录中..." : "登录" }}
        </button>
      </div>
    </div>
  </div>
</template>
