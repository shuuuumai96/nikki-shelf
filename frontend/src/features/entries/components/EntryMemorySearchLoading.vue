<script setup lang="ts">
import { Search } from "lucide-vue-next";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
</script>

<template>
  <section class="memory-search-loading" aria-hidden="true">
    <div class="memory-input">
      <Search :size="15" stroke-width="1.8" />
      <input
        type="search"
        :placeholder="t('entries.searchPastEntries')"
        disabled
      />
    </div>
  </section>
</template>

<style scoped>
.memory-search-loading {
  display: grid;
  gap: 8px;
  margin-top: 14px;
}

.memory-input {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  align-items: center;
  gap: 6px;
  min-height: 34px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-muted);
  padding: 0 9px;
}

.memory-input input {
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--color-muted);
  font-size: 13px;
}
</style>
