<script setup lang="ts">
import { useLocaleSwitcher } from "../i18n/useLocaleSwitcher";

const { currentLocale, localeOptions, setLocale } = useLocaleSwitcher();
</script>

<template>
  <label class="language-select">
    <span class="sr-only">Language / 表示言語</span>
    <select
      :value="currentLocale"
      aria-label="Language / 表示言語"
      @change="setLocale(($event.target as HTMLSelectElement).value)"
    >
      <option
        v-for="option in localeOptions"
        :key="option.code"
        :value="option.code"
      >
        {{ option.nativeLabel }}
      </option>
    </select>
  </label>
</template>

<style scoped>
.language-select {
  display: inline-flex;
  align-items: center;
}

.language-select select {
  min-height: 30px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-muted);
  padding: 0 8px;
  font-size: 12px;
}

.language-select select:hover,
.language-select select:focus {
  border-color: var(--border-active);
  color: var(--color-text);
  outline: 0;
}
</style>
