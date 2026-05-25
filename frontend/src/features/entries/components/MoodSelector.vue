<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { moodOrder, moodSpecs } from "../moods";
import type { MoodKey } from "../types";

defineProps<{
  modelValue: MoodKey;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: MoodKey];
}>();

const { t } = useI18n();
</script>

<template>
  <div class="moods" role="radiogroup" :aria-label="t('entries.mood')">
    <button
      v-for="key in moodOrder"
      :key="key"
      class="mood"
      :class="{ active: modelValue === key }"
      :style="{ '--mood-color': moodSpecs[key].color }"
      type="button"
      @click="emit('update:modelValue', key)"
    >
      <component :is="moodSpecs[key].icon" :size="16" stroke-width="1.8" />
      <span>{{ t(moodSpecs[key].labelKey) }}</span>
    </button>
  </div>
</template>

<style scoped>
.moods {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.mood {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text-badge);
  padding: 0 10px;
  font-size: 13px;
  transition:
    background-color 140ms ease,
    border-color 140ms ease,
    color 140ms ease;
}

.mood:hover {
  background: var(--surface-hover);
}

.mood.active {
  border-color: var(--color-text);
  background: var(--color-text);
  color: var(--color-text);
}

.mood.active {
  color: #ffffff;
}

.mood span {
  white-space: nowrap;
}

@media (max-width: 480px) {
  .moods {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .mood {
    min-width: 0;
    min-height: 42px;
    justify-content: center;
    padding: 0 8px;
  }
}
</style>
