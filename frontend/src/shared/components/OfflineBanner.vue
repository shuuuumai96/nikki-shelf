<script setup lang="ts">
import { useI18n } from "vue-i18n";

const { t } = useI18n();
</script>

<template>
  <p class="offline-banner" role="status" aria-live="polite">
    {{ t("common.offline") }}
  </p>
</template>

<style scoped>
.offline-banner {
  position: fixed;
  z-index: 60;
  top: max(12px, env(safe-area-inset-top));
  left: 50%;
  width: min(360px, calc(100vw - 32px));
  margin: 0;
  transform: translateX(-50%);
  border: 1px solid var(--color-warning-border);
  border-radius: var(--radius-sm);
  background: var(--color-warning-bg);
  color: var(--color-warning-text);
  box-shadow: var(--shadow-soft);
  padding: 8px 11px;
  font-size: 13px;
  line-height: 1.45;
  text-align: center;
}
</style>
