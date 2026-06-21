<template>
  <span class="status-tag" :class="`status-tag--${tone}`">{{ text }}</span>
</template>

<script setup lang="ts">
import type { StatusTone } from '../../types/navigation'

defineProps<{
  text: string
  tone?: StatusTone
}>()
</script>
