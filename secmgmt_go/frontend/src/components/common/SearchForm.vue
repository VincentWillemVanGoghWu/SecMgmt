<template>
  <section class="search-form">
    <div class="search-form__fields">
      <slot />
    </div>
    <div v-if="$slots.actions" class="search-form__actions">
      <slot name="actions" />
    </div>
  </section>
</template>
