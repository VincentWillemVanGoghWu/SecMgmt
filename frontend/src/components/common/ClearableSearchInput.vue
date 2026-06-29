<script setup lang="ts">
import { computed, nextTick, ref, useAttrs } from "vue"

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<{
    modelValue?: string | number | null
    modelModifiers?: {
      trim?: boolean
    }
  }>(),
  {
    modelValue: "",
    modelModifiers: () => ({}),
  },
)

const emit = defineEmits<{
  "update:modelValue": [value: string]
  clear: []
  enter: []
}>()

const attrs = useAttrs()
const inputRef = ref<HTMLInputElement | null>(null)

const normalizedValue = computed(() => (props.modelValue == null ? "" : String(props.modelValue)))
const hasValue = computed(() => normalizedValue.value.length > 0)

const handleInput = (event: Event) => {
  const rawValue = (event.target as HTMLInputElement).value
  const nextValue = props.modelModifiers.trim ? rawValue.trim() : rawValue
  const hadValue = hasValue.value
  emit("update:modelValue", nextValue)
  if (hadValue && nextValue === "") {
    emit("clear")
  }
}

const handleClear = async () => {
  if (!hasValue.value) {
    return
  }
  emit("update:modelValue", "")
  emit("clear")
  await nextTick()
  inputRef.value?.focus()
}
</script>

<template>
  <div class="clearable-search-input">
    <input
      ref="inputRef"
      v-bind="attrs"
      :value="normalizedValue"
      @input="handleInput"
      @keydown.enter="emit('enter')"
    />
    <button
      v-if="hasValue"
      type="button"
      class="clearable-search-input__clear"
      aria-label="清除查询内容"
      @click="handleClear"
    />
  </div>
</template>

<style scoped>
.clearable-search-input {
  position: relative;
  width: 100%;
}

.clearable-search-input input {
  padding-right: 32px;
}

.clearable-search-input__clear {
  position: absolute;
  top: 50%;
  right: 8px;
  appearance: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  border: none;
  border-radius: 999px;
  background: rgba(111, 128, 147, 0.14);
  color: #6f8093;
  line-height: 0;
  transform: translateY(-50%);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.clearable-search-input__clear::before,
.clearable-search-input__clear::after {
  content: "";
  position: absolute;
  width: 8px;
  height: 1.5px;
  border-radius: 999px;
  background: currentColor;
}

.clearable-search-input__clear::before {
  transform: rotate(45deg);
}

.clearable-search-input__clear::after {
  transform: rotate(-45deg);
}

.clearable-search-input__clear:hover {
  background: rgba(22, 54, 87, 0.16);
  color: #163657;
}
</style>
