<template>
  <section class="page-card">
    <header v-if="title || $slots.headerActions" class="page-card__header">
      <div>
        <h3 v-if="title" class="page-card__title">{{ title }}</h3>
        <p v-if="description" class="page-card__description">{{ description }}</p>
      </div>
      <div v-if="$slots.headerActions" class="page-card__actions">
        <slot name="headerActions" />
      </div>
    </header>
    <div class="page-card__body">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title?: string
  description?: string
}>()
</script>
