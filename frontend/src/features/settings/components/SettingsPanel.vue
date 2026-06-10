<script setup lang="ts">
import { Download, LogOut } from "lucide-vue-next";
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import LanguageSelect from "../../../shared/components/LanguageSelect.vue";
import type { AuthUser } from "../../auth/types";
import {
  readMemoryPreferences,
  writeMemoryPreferences,
} from "../../entries/memory-preferences";
import { moodOrder, moodSpecs } from "../../entries/moods";
import type { MoodKey } from "../../entries/types";

defineProps<{
  user: AuthUser;
  loading?: boolean;
}>();

const emit = defineEmits<{
  logout: [];
}>();

const { t } = useI18n();
const memoryPreferences = ref(readMemoryPreferences());

function setMemoryEnabled(event: Event) {
  const target = event.target as HTMLInputElement;
  updateMemoryPreferences({ enabled: target.checked });
}

function toggleExcludedMood(mood: MoodKey, event: Event) {
  const target = event.target as HTMLInputElement;
  const excluded = new Set(memoryPreferences.value.excludedMoods);
  if (target.checked) {
    excluded.add(mood);
  } else {
    excluded.delete(mood);
  }
  updateMemoryPreferences({
    excludedMoods: moodOrder.filter((key) => excluded.has(key)),
  });
}

function updateMemoryPreferences(
  next: Partial<typeof memoryPreferences.value>,
) {
  memoryPreferences.value = {
    ...memoryPreferences.value,
    ...next,
  };
  writeMemoryPreferences(memoryPreferences.value);
}
</script>

<template>
  <section class="settings ui-page">
    <h1 class="ui-heading">{{ t("settings.title") }}</h1>

    <div class="settings-block">
      <h2>{{ t("settings.account") }}</h2>
      <div class="account-card">
        <span>{{ user.username }}</span>
        <button
          class="ui-action"
          type="button"
          :disabled="loading"
          @click="emit('logout')"
        >
          <LogOut :size="16" stroke-width="1.8" />
          <span>{{ t("nav.logout") }}</span>
        </button>
      </div>
    </div>

    <div class="settings-block">
      <h2>{{ t("settings.language") }}</h2>
      <LanguageSelect />
    </div>

    <div class="settings-block">
      <h2>{{ t("settings.memories") }}</h2>
      <label class="toggle-row">
        <input
          type="checkbox"
          :checked="memoryPreferences.enabled"
          @change="setMemoryEnabled"
        />
        <span>{{ t("settings.memoriesEnabled") }}</span>
      </label>
      <div class="mood-filter-group">
        <p>{{ t("settings.memoriesExcludedMoods") }}</p>
        <div class="mood-filter-list">
          <label v-for="mood in moodOrder" :key="mood" class="mood-filter">
            <input
              type="checkbox"
              :checked="memoryPreferences.excludedMoods.includes(mood)"
              @change="toggleExcludedMood(mood, $event)"
            />
            <span>
              <component :is="moodSpecs[mood].icon" :size="14" />
              {{ t(moodSpecs[mood].labelKey) }}
            </span>
          </label>
        </div>
      </div>
    </div>

    <div class="settings-block">
      <h2>{{ t("settings.export") }}</h2>
      <div class="export-actions">
        <a class="export-link ui-action" href="/api/export/json">
          <Download :size="16" stroke-width="1.8" />
          <span>{{ t("export.json") }}</span>
        </a>
        <a class="export-link ui-action" href="/api/export/markdown">
          <Download :size="16" stroke-width="1.8" />
          <span>{{ t("export.markdown") }}</span>
        </a>
        <a class="export-link ui-action" href="/api/export/backup">
          <Download :size="16" stroke-width="1.8" />
          <span>{{ t("export.backup") }}</span>
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.settings {
  --page-max: 760px;
}

h1 {
  margin: 0 0 26px;
  font-size: 28px;
}

.settings-block {
  border-top: 1px solid var(--border-editor);
  padding-top: 20px;
}

.settings-block + .settings-block {
  margin-top: 22px;
}

h2 {
  margin: 0 0 12px;
  color: var(--color-muted);
  font-size: 14px;
  font-weight: 650;
  letter-spacing: 0;
}

.export-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.account-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.account-card > span {
  overflow: hidden;
  color: var(--color-text);
  font-size: 15px;
  font-weight: 680;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.export-link {
  min-height: 36px;
  text-decoration: none;
}

.toggle-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text);
  font-size: 14px;
}

.toggle-row input,
.mood-filter input {
  width: 15px;
  height: 15px;
}

.mood-filter-group {
  display: grid;
  gap: 8px;
  margin-top: 14px;
}

.mood-filter-group p {
  margin: 0;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1.45;
}

.mood-filter-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.mood-filter {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text-soft);
  padding: 7px 9px;
  font-size: 13px;
}

.mood-filter span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

@media (max-width: 480px) {
  h1 {
    margin-bottom: 22px;
    font-size: 24px;
  }

  .export-actions {
    display: grid;
    grid-template-columns: 1fr;
  }

  .account-card {
    align-items: stretch;
    flex-direction: column;
  }

  .mood-filter-list {
    display: grid;
    grid-template-columns: 1fr;
  }
}
</style>
