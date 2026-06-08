<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { SaveStatus } from "../types";

defineProps<{
  navigationMessage: string;
  recoveryCopyText: string;
  recoveryDraftText: string;
  recoveryHelpText: string;
  saveError: string;
  saveStatus: SaveStatus;
  showRecoveryPanel: boolean;
}>();

const emit = defineEmits<{
  copyRecoveryDraft: [];
  dismissRecoveryDraft: [];
  reloadServerVersion: [];
  restoreRecoveryDraft: [];
}>();

const { t } = useI18n();
</script>

<template>
  <div class="entry-surface__messages">
    <p v-if="saveStatus === 'failed' && saveError" class="save-error">
      {{ saveError }}
    </p>
    <p v-if="navigationMessage" class="navigation-warning" aria-live="polite">
      {{ navigationMessage }}
    </p>
    <div
      v-if="showRecoveryPanel"
      class="recovery-panel"
      :class="{ 'recovery-panel--retained': saveStatus !== 'conflict' }"
      aria-live="polite"
    >
      <div class="recovery-panel__copy">
        <strong>{{ t("entries.recoveryDraftHeading") }}</strong>
        <p v-if="saveStatus === 'conflict' && saveError">{{ saveError }}</p>
        <p>{{ recoveryHelpText }}</p>
      </div>
      <label class="sr-only" for="entry-recovery-draft">
        {{ t("entries.recoveryDraftLabel") }}
      </label>
      <textarea
        id="entry-recovery-draft"
        class="recovery-panel__draft"
        readonly
        :value="recoveryDraftText"
      ></textarea>
      <div class="recovery-panel__actions">
        <button
          v-if="saveStatus === 'conflict'"
          type="button"
          @click="emit('reloadServerVersion')"
        >
          {{ t("entries.loadServerVersion") }}
        </button>
        <button v-else type="button" @click="emit('restoreRecoveryDraft')">
          {{ t("entries.restoreRecoveryDraft") }}
        </button>
        <button
          type="button"
          :disabled="!recoveryDraftText"
          @click="emit('copyRecoveryDraft')"
        >
          {{ recoveryCopyText }}
        </button>
        <button
          v-if="saveStatus !== 'conflict'"
          type="button"
          @click="emit('dismissRecoveryDraft')"
        >
          {{ t("entries.dismissRecoveryDraft") }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.entry-surface__messages {
  margin-bottom: 14px;
}

.entry-surface__messages:empty {
  display: none;
}

.save-error {
  max-width: 520px;
  margin: -10px 0 14px;
  color: var(--color-danger);
  font-size: 12px;
  line-height: 1.6;
}

.navigation-warning {
  max-width: 520px;
  margin: -10px 0 14px;
  color: var(--color-danger);
  font-size: 12px;
  line-height: 1.6;
}

.recovery-panel {
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-danger-bg);
  color: var(--color-danger);
  margin: -10px 0 14px;
  padding: 9px 10px;
  font-size: 12px;
  line-height: 1.6;
}

.recovery-panel--retained {
  border-color: var(--border-subtle);
  background: var(--color-surface);
  color: var(--color-text-soft);
}

.recovery-panel__copy {
  display: grid;
  gap: 4px;
  margin-bottom: 8px;
}

.recovery-panel__copy strong {
  color: var(--color-text);
  font-size: 13px;
  line-height: 1.4;
}

.recovery-panel__copy p {
  margin: 0;
}

.recovery-panel__draft {
  display: block;
  width: 100%;
  min-height: 128px;
  resize: vertical;
  border: 1px solid color-mix(in srgb, currentColor 22%, transparent);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-surface) 84%, transparent);
  color: var(--color-text);
  font: inherit;
  font-size: 12px;
  line-height: 1.55;
  padding: 8px 9px;
  white-space: pre-wrap;
}

.recovery-panel__draft:focus {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 55%, transparent);
  outline-offset: 2px;
}

.recovery-panel__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}

.recovery-panel button {
  min-height: 30px;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-danger);
  padding: 0 9px;
}
</style>
