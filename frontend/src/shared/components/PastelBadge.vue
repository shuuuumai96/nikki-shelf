<script setup lang="ts">
defineProps<{
  color?: string;
}>();
</script>

<template>
  <span
    class="badge"
    :style="{ '--badge-color': color || 'var(--color-mint)' }"
  >
    <slot />
  </span>
</template>

<style scoped>
.badge {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  gap: 6px;
  border: 1px solid color-mix(in srgb, var(--badge-color) 78%, #fff);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--badge-color) 34%, #fff);
  color: var(--color-text-badge);
  padding: 2px 8px;
  font-size: 12px;
  font-weight: 620;
  line-height: 1.4;
  white-space: nowrap;
}
</style>
