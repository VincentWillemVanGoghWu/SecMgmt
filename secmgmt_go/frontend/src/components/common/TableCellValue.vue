<script setup lang="ts">
import StatusTag from './StatusTag.vue'
import type { StatusValue } from '../../types/navigation'

const props = defineProps<{
  value: string | number | StatusValue
}>()

const isStatusValue = (value: string | number | StatusValue): value is StatusValue => {
  return typeof value === 'object' && value !== null && 'text' in value && 'tone' in value
}
</script>

<template>
  <StatusTag v-if="isStatusValue(props.value)" :text="props.value.text" :tone="props.value.tone" />
  <span v-else>{{ props.value }}</span>
</template>
