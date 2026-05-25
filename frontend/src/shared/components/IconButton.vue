<script setup lang="ts">
import type { Component } from "vue";

defineProps<{
  icon: Component;
  label: string;
  active?: boolean;
  disabled?: boolean;
}>();
</script>

<template>
  <button
    class="icon-button"
    :class="{ active }"
    :title="label"
    :aria-label="label"
    :disabled="disabled"
  >
    <component :is="icon" :size="17" stroke-width="1.8" />
  </button>
</template>

<style scoped>
.icon-button {
  display: inline-grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  transition:
    background-color 140ms ease,
    border-color 140ms ease,
    color 140ms ease;
}

.icon-button:hover {
  background: var(--surface-hover);
  color: var(--color-text);
}

.icon-button:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 64%, transparent);
  outline-offset: 2px;
}

.icon-button.active {
  background: var(--surface-active);
  border-color: var(--border-active);
  color: var(--color-text);
}

.icon-button:disabled {
  opacity: 0.5;
}

@media (max-width: 480px) {
  .icon-button {
    width: 40px;
    height: 40px;
  }
}
</style>
