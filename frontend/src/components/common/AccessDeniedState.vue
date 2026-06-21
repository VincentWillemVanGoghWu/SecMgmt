<script setup lang="ts">
defineProps<{
  title?: string
  description?: string
}>()
</script>

<template>
  <div class="access-denied">
    <div class="access-denied__icon">403</div>
    <strong>{{ title || "当前页面无访问权限" }}</strong>
    <p>{{ description || "请联系管理员为当前账号分配菜单权限或数据范围后再试。" }}</p>
    <div v-if="$slots.actions" class="access-denied__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.access-denied {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 40px 24px;
  border: 1px dashed #c9d9ea;
  border-radius: 16px;
  background: linear-gradient(180deg, #f8fbfe 0%, #ffffff 100%);
  text-align: center;
}

.access-denied__icon {
  min-width: 76px;
  padding: 10px 18px;
  border-radius: 999px;
  background: rgba(208, 83, 91, 0.1);
  color: #d0535b;
  font-size: 26px;
  font-weight: 700;
}

.access-denied strong {
  color: #173754;
  font-size: 20px;
}

.access-denied p {
  margin: 0;
  max-width: 520px;
  color: #698096;
  line-height: 1.7;
}

.access-denied__actions {
  display: flex;
  gap: 12px;
}
</style>
