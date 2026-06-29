<script setup lang="ts">
import { computed, reactive, ref } from "vue"
import { useRoute, useRouter } from "vue-router"

import { useAuthStore, usePermissionStore } from "../../stores"

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const permissionStore = usePermissionStore()

const loading = ref(false)
const errorMessage = ref("")
const formState = reactive({
  username: "",
  password: "",
})
const canSubmit = computed(() => formState.username.trim() !== "" && formState.password.trim() !== "")

const handleSubmit = async () => {
  if (!canSubmit.value) {
    errorMessage.value = "请输入用户名和密码。"
    return
  }

  loading.value = true
  errorMessage.value = ""

  try {
    await authStore.login(formState)
    const redirect = route.query.redirect
    if (typeof redirect === "string" && redirect) {
      await router.replace(redirect)
    } else if (permissionStore.defaultRouteName) {
      await router.replace({ name: permissionStore.defaultRouteName })
    } else {
      await router.replace("/dashboard")
    }
  } catch (error) {
    errorMessage.value = error instanceof Error && error.message ? error.message : "登录失败，请检查用户名和密码。"
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-page__grid"></div>
    <div class="login-page__smoke-stack" aria-hidden="true">
      <span class="login-page__smoke login-page__smoke--1"></span>
      <span class="login-page__smoke login-page__smoke--2"></span>
      <span class="login-page__smoke login-page__smoke--3"></span>
      <span class="login-page__smoke login-page__smoke--4"></span>
      <span class="login-page__smoke login-page__smoke--5"></span>
    </div>
    <div class="login-page__mask">
      <div class="login-page__panel">
        <img class="login-page__corner-logo" src="/bgLogo.png" alt="北港新材料" />

        <div class="login-page__content">
          <header class="login-page__header">
            <div class="login-page__header-line"></div>
            <h1 class="login-page__title">固定污染源视频监控系统</h1>
            <div class="login-page__header-line login-page__header-line--short"></div>
          </header>

          <div class="login-page__form-card">
            <div class="login-page__field">
              <div class="login-page__field-label">账号</div>
              <input
                v-model.trim="formState.username"
                class="login-page__input"
                type="text"
                placeholder="请输入用户名"
                autocomplete="username"
                @keydown.enter="handleSubmit"
              />
            </div>

            <div class="login-page__field">
              <div class="login-page__field-label">密码</div>
              <input
                v-model.trim="formState.password"
                class="login-page__input"
                type="password"
                placeholder="请输入密码"
                autocomplete="current-password"
                @keydown.enter="handleSubmit"
              />
            </div>

            <div v-if="errorMessage" class="login-page__error">{{ errorMessage }}</div>

            <button
              class="app-button app-button--primary login-page__submit"
              :disabled="loading || !canSubmit"
              @click="handleSubmit"
            >
              {{ loading ? "登录中..." : "登录" }}
            </button>

            <div class="login-page__footer">建议使用 Chrome 或 Edge 浏览器访问</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: stretch;
  justify-content: center;
  overflow: hidden;
  background: #0f1b2c;
}

.login-page::before {
  content: "";
  position: fixed;
  inset: 0;
  background: url("/BG1.png") center center / cover no-repeat;
  pointer-events: none;
}

.login-page::after {
  content: "";
  position: fixed;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(18, 35, 56, 0.34), rgba(18, 37, 60, 0.38)),
    rgba(31, 49, 74, 0.14);
  pointer-events: none;
}

.login-page__grid {
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(rgba(129, 157, 191, 0.12) 1px, transparent 1px),
    linear-gradient(90deg, rgba(129, 157, 191, 0.12) 1px, transparent 1px);
  background-size: 56px 56px;
  opacity: 0.16;
  pointer-events: none;
}

.login-page__smoke-stack {
  position: fixed;
  left: 36.0%;
  top: 3.8%;
  width: 96px;
  height: 180px;
  pointer-events: none;
  z-index: 3;
}

.login-page__smoke {
  position: absolute;
  bottom: 0;
  left: 50%;
  border-radius: 50% 50% 42% 42%;
  transform: translateX(-50%);
  background:
    radial-gradient(ellipse at 48% 24%, rgba(255, 228, 168, 0.82) 0%, rgba(255, 168, 72, 0.78) 18%, rgba(239, 76, 48, 0.76) 42%, rgba(188, 31, 38, 0.62) 64%, rgba(145, 38, 87, 0.38) 82%, rgba(145, 38, 87, 0) 100%),
    radial-gradient(ellipse at 30% 62%, rgba(219, 74, 39, 0.34) 0%, rgba(219, 74, 39, 0) 72%),
    radial-gradient(ellipse at 72% 58%, rgba(146, 18, 48, 0.28) 0%, rgba(146, 18, 48, 0) 74%);
  filter: blur(7px);
  mix-blend-mode: multiply;
  opacity: 0;
  transform-origin: center bottom;
  animation: smoke-rise 7.2s ease-in-out infinite;
}

.login-page__smoke::before,
.login-page__smoke::after {
  content: "";
  position: absolute;
  border-radius: 50%;
  pointer-events: none;
  background: inherit;
  opacity: 0.82;
}

.login-page__smoke::before {
  width: 62%;
  height: 56%;
  left: -10%;
  top: 18%;
  transform: rotate(-14deg);
}

.login-page__smoke::after {
  width: 58%;
  height: 50%;
  right: -8%;
  top: 30%;
  transform: rotate(16deg);
  opacity: 0.72;
}

.login-page__smoke--1 {
  width: 34px;
  height: 78px;
  margin-left: -7px;
  animation-delay: 0s;
  animation-duration: 6.2s;
}

.login-page__smoke--2 {
  width: 40px;
  height: 94px;
  margin-left: 4px;
  animation-delay: 1s;
  animation-duration: 6.8s;
}

.login-page__smoke--3 {
  width: 48px;
  height: 112px;
  margin-left: -14px;
  animation-delay: 2.1s;
  animation-duration: 7.4s;
}

.login-page__smoke--4 {
  width: 42px;
  height: 98px;
  margin-left: 11px;
  animation-delay: 3.2s;
  animation-duration: 6.6s;
}

.login-page__smoke--5 {
  width: 54px;
  height: 126px;
  margin-left: -3px;
  animation-delay: 4.4s;
  animation-duration: 7.8s;
}

.login-page__mask {
  position: relative;
  z-index: 2;
  flex: 1;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 20px;
}

.login-page__panel {
  position: relative;
  z-index: 5;
  width: min(570px, 100%);
  min-height: 420px;
  padding: 18px 24px 20px;
  display: flex;
  flex-direction: column;
  overflow: visible;
}

.login-page__corner-logo {
  position: absolute;
  top: 6px;
  left: 8px;
  width: 166px;
  max-width: 40%;
  height: auto;
  opacity: 0.98;
  filter: brightness(1.1) contrast(1.04);
  z-index: 7;
}

.login-page__content {
  position: relative;
  z-index: 6;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 24px;
}

.login-page__header {
  max-width: 430px;
  text-align: center;
  margin-top: 54px;
}

.login-page__header-line {
  width: 120px;
  height: 2px;
  margin: 0 auto 16px;
  background: linear-gradient(90deg, rgba(106, 163, 232, 0), rgba(132, 193, 255, 0.9), rgba(106, 163, 232, 0));
  box-shadow: 0 0 14px rgba(90, 160, 235, 0.18);
}

.login-page__header-line--short {
  width: 84px;
  margin: 16px auto 0;
}

.login-page__title {
  margin: 0 0 10px;
  color: #f7fafc;
  font-size: 32px;
  font-weight: 700;
  line-height: 1.35;
  letter-spacing: 1px;
  text-shadow: 0 4px 20px rgba(9, 16, 28, 0.3);
}

.login-page__form-card {
  position: relative;
  width: min(100%, 376px);
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 24px 22px 18px;
  border: 1px solid rgba(112, 184, 255, 0.62);
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(28, 58, 96, 0.97), rgba(16, 37, 63, 0.98)),
    rgba(16, 37, 63, 0.98);
  box-shadow:
    0 24px 52px rgba(8, 16, 29, 0.36),
    0 0 0 1px rgba(82, 162, 243, 0.24),
    0 0 22px rgba(67, 144, 230, 0.2),
    inset 0 1px 0 rgba(214, 232, 255, 0.12),
    inset 0 0 34px rgba(92, 164, 255, 0.18);
  overflow: hidden;
  z-index: 8;
}

.login-page__form-card::before {
  content: "";
  position: absolute;
  inset: 0;
  background:
    linear-gradient(135deg, rgba(107, 188, 255, 0.24), transparent 28%),
    linear-gradient(315deg, rgba(107, 188, 255, 0.14), transparent 24%);
  pointer-events: none;
}

.login-page__form-card::after {
  content: "";
  position: absolute;
  left: 12px;
  right: 12px;
  top: 12px;
  bottom: 12px;
  border: 1px solid rgba(112, 184, 255, 0.3);
  box-shadow: inset 0 0 18px rgba(94, 173, 255, 0.06);
  border-radius: 13px;
  pointer-events: none;
}

.login-page__field {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.login-page__field-label {
  color: #d8e3ef;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.6px;
}

.login-page__input {
  width: 100%;
  height: 42px;
  padding: 0 14px;
  border: 1px solid rgba(97, 136, 182, 0.34);
  border-radius: 10px;
  background: rgba(11, 26, 45, 0.88);
  color: #eff6ff;
  font-size: 14px;
  line-height: 42px;
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    background-color 0.2s ease;
}

.login-page__input::placeholder {
  color: #86a0bc;
}

.login-page__input:focus {
  outline: none;
  border-color: #6aafff;
  box-shadow:
    0 0 0 3px rgba(81, 137, 209, 0.12),
    0 0 18px rgba(71, 141, 230, 0.18);
  background: rgba(14, 32, 56, 0.96);
}

.login-page__error {
  position: relative;
  z-index: 1;
  padding: 12px 14px;
  border: 1px solid rgba(255, 146, 146, 0.26);
  border-radius: 14px;
  background: rgba(110, 34, 44, 0.38);
  color: #ffd0d3;
  font-size: 14px;
  line-height: 1.5;
}

.login-page__submit {
  position: relative;
  z-index: 1;
  width: 100%;
  height: 44px;
  border: none;
  border-radius: 10px;
  background: linear-gradient(90deg, #4d9cff 0%, #3178dc 100%);
  box-shadow:
    0 10px 20px rgba(35, 91, 180, 0.22),
    inset 0 1px 0 rgba(255, 255, 255, 0.18);
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 1.2px;
}

.login-page__submit:disabled {
  opacity: 0.72;
  cursor: not-allowed;
}

.login-page__footer {
  position: relative;
  z-index: 1;
  color: #99a8bb;
  font-size: 11px;
  text-align: center;
  line-height: 1.5;
}

@keyframes smoke-rise {
  0% {
    opacity: 0;
    transform: translateX(-50%) translateY(18px) scaleX(0.62) scaleY(0.52) rotate(-5deg);
  }

  10% {
    opacity: 0.5;
  }

  32% {
    opacity: 0.46;
    transform: translateX(calc(-50% - 6px)) translateY(-40px) scaleX(0.86) scaleY(0.9) rotate(-9deg);
  }

  58% {
    opacity: 0.34;
    transform: translateX(calc(-50% + 10px)) translateY(-88px) scaleX(1.02) scaleY(1.18) rotate(6deg);
  }

  82% {
    opacity: 0.18;
    transform: translateX(calc(-50% - 8px)) translateY(-134px) scaleX(1.1) scaleY(1.34) rotate(-4deg);
  }

  100% {
    opacity: 0;
    transform: translateX(calc(-50% + 4px)) translateY(-168px) scaleX(1.18) scaleY(1.48) rotate(7deg);
  }
}

@media (max-width: 768px) {
  .login-page__mask {
    padding: 20px 14px;
  }

  .login-page__smoke-stack {
    left: 56.5%;
    top: 19.5%;
    width: 72px;
    height: 138px;
  }

  .login-page__panel {
    min-height: 460px;
    padding: 18px 16px 18px;
  }

  .login-page__corner-logo {
    top: 8px;
    left: 8px;
    width: 126px;
    max-width: 38%;
  }

  .login-page__content {
    gap: 20px;
  }

  .login-page__title {
    font-size: 28px;
  }

  .login-page__form-card {
    width: 100%;
    padding: 20px 16px 16px;
  }
}
</style>
