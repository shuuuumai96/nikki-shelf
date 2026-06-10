<script setup lang="ts">
import { Download, LogOut, Trash2, X } from "lucide-vue-next";
import { computed, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import LanguageSelect from "../../../shared/components/LanguageSelect.vue";
import type { AuthUser, DeleteAccountInput } from "../../auth/types";
import {
  readMemoryPreferences,
  writeMemoryPreferences,
} from "../../entries/memory-preferences";
import { moodOrder, moodSpecs } from "../../entries/moods";
import type { MoodKey } from "../../entries/types";

const props = defineProps<{
  error?: string;
  user: AuthUser;
  loading?: boolean;
}>();

const emit = defineEmits<{
  deleteAccount: [input: DeleteAccountInput];
  logout: [];
}>();

const { t } = useI18n();
const memoryPreferences = ref(readMemoryPreferences());
const deletePanelOpen = ref(false);
const deleteError = ref("");
const deleteForm = reactive({
  username: "",
  password: "",
});

const deleteUsernameMatches = computed(
  () =>
    deleteForm.username.trim().toLowerCase() ===
    props.user.username.toLowerCase(),
);
const canSubmitDelete = computed(
  () =>
    deleteUsernameMatches.value &&
    deleteForm.password.trim().length > 0 &&
    !props.loading,
);
const deleteVisibleError = computed(
  () => deleteError.value || props.error || "",
);

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

function openDeletePanel() {
  deletePanelOpen.value = true;
  deleteError.value = "";
  deleteForm.username = "";
  deleteForm.password = "";
}

function closeDeletePanel() {
  deletePanelOpen.value = false;
  deleteError.value = "";
  deleteForm.username = "";
  deleteForm.password = "";
}

function submitDeleteAccount() {
  if (!deleteUsernameMatches.value) {
    deleteError.value = t("settings.deleteAccountUsernameMismatch");
    return;
  }
  if (!deleteForm.password.trim()) {
    deleteError.value = t("settings.deleteAccountPasswordRequired");
    return;
  }

  deleteError.value = "";
  emit("deleteAccount", {
    username: deleteForm.username,
    password: deleteForm.password,
  });
}
</script>

<template>
  <section class="settings ui-page">
    <h1 class="ui-heading">{{ t("settings.title") }}</h1>

    <div class="settings-block">
      <h2>{{ t("settings.account") }}</h2>
      <div class="account-card">
        <span>{{ user.username }}</span>
        <div class="account-actions">
          <button
            class="ui-action"
            type="button"
            :disabled="loading"
            @click="emit('logout')"
          >
            <LogOut :size="16" stroke-width="1.8" />
            <span>{{ t("nav.logout") }}</span>
          </button>
          <button
            class="ui-action danger-action"
            data-testid="open-delete-account"
            type="button"
            :disabled="loading"
            @click="openDeletePanel"
          >
            <Trash2 :size="16" stroke-width="1.8" />
            <span>{{ t("settings.deleteAccount") }}</span>
          </button>
        </div>
      </div>
      <form
        v-if="deletePanelOpen"
        class="danger-panel"
        data-testid="delete-account-panel"
        @submit.prevent="submitDeleteAccount"
      >
        <div class="danger-panel__head">
          <h3>{{ t("settings.deleteAccount") }}</h3>
          <button
            class="icon-button"
            type="button"
            :aria-label="t('common.close')"
            :disabled="loading"
            @click="closeDeletePanel"
          >
            <X :size="16" stroke-width="1.8" />
          </button>
        </div>
        <p>{{ t("settings.deleteAccountBody") }}</p>
        <label>
          <span>{{ t("settings.deleteAccountUsernameConfirm") }}</span>
          <input
            v-model="deleteForm.username"
            autocomplete="username"
            data-testid="delete-account-username"
            :disabled="loading"
            :aria-invalid="Boolean(deleteVisibleError)"
          />
        </label>
        <label>
          <span>{{ t("settings.deleteAccountPasswordConfirm") }}</span>
          <input
            v-model="deleteForm.password"
            autocomplete="current-password"
            data-testid="delete-account-password"
            type="password"
            :disabled="loading"
            :aria-invalid="Boolean(deleteVisibleError)"
          />
        </label>
        <p v-if="deleteVisibleError" class="danger-panel__error">
          {{ deleteVisibleError }}
        </p>
        <div class="danger-panel__actions">
          <button
            class="ui-action"
            type="button"
            :disabled="loading"
            @click="closeDeletePanel"
          >
            <X :size="16" stroke-width="1.8" />
            <span>{{ t("settings.deleteAccountCancel") }}</span>
          </button>
          <button
            class="ui-action danger-action"
            data-testid="delete-account-submit"
            type="submit"
            :disabled="!canSubmitDelete"
          >
            <Trash2 :size="16" stroke-width="1.8" />
            <span>{{ t("settings.deleteAccountSubmit") }}</span>
          </button>
        </div>
      </form>
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

.account-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.danger-action {
  border-color: var(--color-danger-border);
  color: var(--color-danger);
}

.danger-action:hover:not(:disabled),
.danger-action:focus-visible {
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

.danger-panel {
  display: grid;
  gap: 12px;
  margin-top: 14px;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-danger-bg);
  padding: 14px;
}

.danger-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.danger-panel h3 {
  margin: 0;
  color: var(--color-danger);
  font-size: 15px;
}

.danger-panel p {
  margin: 0;
  color: var(--color-text-soft);
  font-size: 13px;
  line-height: 1.55;
}

.danger-panel label {
  display: grid;
  gap: 6px;
  color: var(--color-muted);
  font-size: 12px;
}

.danger-panel input {
  min-height: 38px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 10px;
  font: inherit;
}

.danger-panel input:focus {
  border-color: var(--border-active);
  outline: 0;
}

.danger-panel input[aria-invalid="true"] {
  border-color: var(--color-danger-border);
}

.danger-panel__error {
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-danger) !important;
  padding: 8px 10px;
}

.danger-panel__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.icon-button {
  display: inline-grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
}

.icon-button:hover:not(:disabled),
.icon-button:focus-visible {
  border-color: var(--border-subtle);
  background: var(--color-surface);
  color: var(--color-text);
  outline: 0;
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

  .account-actions,
  .danger-panel__actions {
    display: grid;
    grid-template-columns: 1fr;
  }

  .mood-filter-list {
    display: grid;
    grid-template-columns: 1fr;
  }
}
</style>
