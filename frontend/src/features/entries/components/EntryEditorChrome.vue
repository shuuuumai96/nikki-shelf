<script setup lang="ts">
import { Camera, Ellipsis, Maximize2, Minimize2 } from "lucide-vue-next";
import { onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import IconButton from "../../../shared/components/IconButton.vue";
import type { Entry, SaveStatus } from "../types";

type EntryChromePanel = "actions" | "shortcuts" | null;

const props = defineProps<{
  canAutosave: boolean;
  displaySaveStatus: SaveStatus;
  entry: Entry | null;
  exportDisabled: boolean;
  exportUrl: string;
  focusMode: boolean;
  heading: string;
  imageSlotsLeft: number;
  navigationDisabled: boolean;
  saveStatus: SaveStatus;
  saving: boolean;
  statusText: string;
}>();

const emit = defineEmits<{
  deleteEntry: [];
  exitFocusMode: [];
  nextDay: [];
  pickFiles: [];
  previousDay: [];
  retryAutosave: [];
  toggleFocusMode: [];
}>();

const { t } = useI18n();
const openEntryChromePanel = ref<EntryChromePanel>(null);
const entryChromeActions = ref<HTMLElement | null>(null);

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
  document.addEventListener("pointerdown", onDocumentPointerDown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
  document.removeEventListener("pointerdown", onDocumentPointerDown);
});

function openActionsMenu() {
  openEntryChromePanel.value = "actions";
}

function openShortcutsPanel() {
  openEntryChromePanel.value = "shortcuts";
}

function closeEntryChromePanel() {
  openEntryChromePanel.value = null;
}

function toggleEntryActions() {
  if (openEntryChromePanel.value === "actions") {
    closeEntryChromePanel();
    return;
  }
  openActionsMenu();
}

function showShortcutsFromMenu() {
  openShortcutsPanel();
}

function deleteEntryFromMenu() {
  closeEntryChromePanel();
  emit("deleteEntry");
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && openEntryChromePanel.value !== null) {
    event.preventDefault();
    closeEntryChromePanel();
    return;
  }

  if (!props.focusMode || event.key !== "Escape") {
    return;
  }

  event.preventDefault();
  emit("exitFocusMode");
}

function onDocumentPointerDown(event: PointerEvent) {
  if (openEntryChromePanel.value === null) {
    return;
  }
  const target = event.target;
  if (target instanceof Node && entryChromeActions.value?.contains(target)) {
    return;
  }
  closeEntryChromePanel();
}
</script>

<template>
  <header
    class="entry-surface__chrome"
    :class="{ 'entry-surface__chrome--focus': focusMode }"
  >
    <div
      class="entry-surface__date-nav"
      :aria-label="t('entries.dateNavigation')"
    >
      <button
        type="button"
        class="entry-surface__chrome-action"
        :aria-label="t('common.previousDay')"
        :disabled="navigationDisabled"
        @click="emit('previousDay')"
      >
        <span aria-hidden="true">‹</span>
        <span class="entry-surface__nav-label">{{
          t("common.previousDay")
        }}</span>
      </button>
      <h1 class="ui-heading entry-surface__date">
        <span>{{ heading }}</span>
      </h1>
      <button
        type="button"
        class="entry-surface__chrome-action"
        :aria-label="t('common.nextDay')"
        :disabled="navigationDisabled"
        @click="emit('nextDay')"
      >
        <span class="entry-surface__nav-label">{{ t("common.nextDay") }}</span>
        <span aria-hidden="true">›</span>
      </button>
    </div>

    <div
      class="actions entry-surface__chrome-side"
      :class="{ 'actions--focus': focusMode }"
    >
      <button
        v-if="focusMode"
        type="button"
        class="focus-exit"
        @click="emit('exitFocusMode')"
      >
        <Minimize2 :size="15" stroke-width="1.8" />
        <span>{{ t("common.exitFocusMode") }}</span>
      </button>
      <IconButton
        v-else
        :icon="Maximize2"
        :label="t('common.focusMode')"
        @click="emit('toggleFocusMode')"
      />
      <button
        type="button"
        class="mobile-photo-action"
        :aria-label="t('images.addPhotos')"
        :title="t('images.addPhotos')"
        :disabled="imageSlotsLeft === 0"
        @click="emit('pickFiles')"
      >
        <Camera :size="17" stroke-width="1.8" />
      </button>
      <div v-if="entry" ref="entryChromeActions" class="entry-actions">
        <button
          type="button"
          class="entry-actions__trigger"
          :aria-label="t('common.entryActions')"
          :title="t('common.entryActions')"
          :aria-expanded="openEntryChromePanel === 'actions'"
          aria-controls="editor-entry-actions"
          @click="toggleEntryActions"
          @keydown.esc="closeEntryChromePanel"
        >
          <Ellipsis :size="17" stroke-width="1.8" />
        </button>
        <div
          v-if="openEntryChromePanel === 'actions'"
          id="editor-entry-actions"
          class="entry-actions__menu"
          role="menu"
          @keydown.esc="closeEntryChromePanel"
        >
          <a
            v-if="!exportDisabled"
            class="entry-actions__item"
            role="menuitem"
            :href="exportUrl"
            @click="closeEntryChromePanel"
          >
            {{ t("common.exportMarkdown") }}
          </a>
          <button
            v-else
            type="button"
            class="entry-actions__item"
            role="menuitem"
            disabled
            :title="t('common.saveBeforeExport')"
          >
            {{ t("common.exportMarkdown") }}
          </button>
          <button
            type="button"
            class="entry-actions__item"
            role="menuitem"
            @click="showShortcutsFromMenu"
          >
            {{ t("common.keyboardShortcuts") }}
          </button>
          <div class="entry-actions__separator" role="separator"></div>
          <button
            type="button"
            class="entry-actions__item entry-actions__item--danger"
            role="menuitem"
            :disabled="saving"
            @click="deleteEntryFromMenu"
          >
            {{ t("common.delete") }}
          </button>
        </div>
        <div
          v-if="openEntryChromePanel === 'shortcuts'"
          id="writing-shortcuts-help"
          class="entry-shortcuts__panel"
          role="region"
          :aria-label="t('common.keyboardShortcuts')"
          @keydown.esc="closeEntryChromePanel"
        >
          <p>{{ t("common.keyboardShortcuts") }}</p>
          <dl>
            <div>
              <dt>
                <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                ><kbd>F</kbd>
              </dt>
              <dd>{{ t("common.focusMode") }}</dd>
            </div>
            <div>
              <dt><kbd>Esc</kbd></dt>
              <dd>{{ t("common.exitFocusMode") }}</dd>
            </div>
            <div>
              <dt>
                <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                ><kbd>T</kbd>
              </dt>
              <dd>{{ t("common.today") }}</dd>
            </div>
            <div>
              <dt>
                <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                ><kbd>ArrowLeft</kbd>
              </dt>
              <dd>{{ t("common.previousDay") }}</dd>
            </div>
            <div>
              <dt>
                <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                ><kbd>ArrowRight</kbd>
              </dt>
              <dd>{{ t("common.nextDay") }}</dd>
            </div>
          </dl>
        </div>
      </div>
      <div
        class="save-status"
        :class="[
          `status-${displaySaveStatus}`,
          {
            'save-status--normal': !focusMode,
            'save-status--focus': focusMode,
          },
        ]"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >
        <span v-if="statusText">{{ statusText }}</span>
        <button
          v-if="saveStatus === 'failed'"
          type="button"
          :disabled="!canAutosave"
          @click="emit('retryAutosave')"
        >
          {{ t("common.retry") }}
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.entry-surface__chrome {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 12px;
  min-height: 34px;
  margin-bottom: 24px;
  color: var(--color-muted);
  font-size: 13px;
  line-height: 1.4;
}

.entry-surface__date-nav {
  display: inline-flex;
  grid-column: 2;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 14px;
}

.entry-surface__chrome-action {
  appearance: none;
  display: inline-flex;
  min-height: auto;
  align-items: center;
  gap: 3px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  box-shadow: none;
  color: var(--color-muted);
  cursor: pointer;
  font: inherit;
  letter-spacing: 0;
  line-height: 1.4;
  padding: 2px 4px;
  text-decoration: none;
}

.entry-surface__chrome-action:hover:not(:disabled) {
  color: var(--color-text);
  text-decoration: underline;
  text-underline-offset: 0.18em;
}

.entry-surface__chrome-action:focus-visible {
  outline: 1px solid color-mix(in srgb, var(--color-accent) 58%, transparent);
  outline-offset: 2px;
}

.entry-surface__chrome-action:disabled {
  cursor: not-allowed;
  color: var(--color-placeholder);
  opacity: 0.55;
  text-decoration: none;
}

.entry-surface__date {
  margin: 0;
  color: var(--color-text);
  font-size: 17px;
  font-weight: 680;
  letter-spacing: 0;
  line-height: 1.35;
}

.entry-surface__chrome-side {
  justify-self: end;
}

.focus-exit {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 10px;
  font-size: 13px;
  line-height: 1;
  white-space: nowrap;
}

.focus-exit:hover {
  background: var(--surface-hover);
}

.focus-exit:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 64%, transparent);
  outline-offset: 2px;
}

.entry-actions {
  position: relative;
  display: inline-flex;
}

.entry-actions__trigger {
  appearance: none;
  display: inline-grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  cursor: pointer;
}

.entry-actions__trigger:hover,
.entry-actions__trigger[aria-expanded="true"] {
  background: var(--surface-hover);
  color: var(--color-text-soft);
}

.entry-actions__trigger:focus-visible {
  outline: 1px solid color-mix(in srgb, var(--color-accent) 58%, transparent);
  outline-offset: 2px;
}

.entry-actions__menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 6;
  min-width: 172px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  box-shadow: var(--shadow-soft);
  padding: 6px;
}

.entry-actions__item {
  appearance: none;
  display: flex;
  width: 100%;
  align-items: center;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--color-text-soft);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  letter-spacing: 0;
  line-height: 1.3;
  padding: 8px 9px;
  text-align: left;
  text-decoration: none;
  white-space: nowrap;
}

.entry-actions__item:hover:not(:disabled),
.entry-actions__item:focus-visible {
  background: var(--surface-hover);
  color: var(--color-text);
  outline: none;
}

.entry-actions__item:disabled {
  cursor: not-allowed;
  opacity: 0.52;
}

.entry-actions__item--danger {
  color: var(--color-danger);
}

.entry-actions__item--danger:hover:not(:disabled),
.entry-actions__item--danger:focus-visible {
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

.entry-actions__separator {
  height: 1px;
  background: var(--border-subtle);
  margin: 5px 3px;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mobile-photo-action {
  display: none;
}

.entry-shortcuts__panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 6;
  width: min(300px, calc(100vw - 32px));
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  box-shadow: var(--shadow-soft);
  color: var(--color-text-soft);
  padding: 12px;
}

.entry-shortcuts__panel p {
  margin: 0 0 10px;
  color: var(--color-text);
  font-size: 12px;
  font-weight: 650;
  line-height: 1.3;
}

.entry-shortcuts__panel dl {
  display: grid;
  gap: 8px;
  margin: 0;
}

.entry-shortcuts__panel dl div {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(92px, 0.9fr);
  align-items: center;
  gap: 10px;
}

.entry-shortcuts__panel dt {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 3px;
  color: var(--color-muted);
  font-size: 11px;
  line-height: 1.6;
}

.entry-shortcuts__panel dd {
  margin: 0;
  color: var(--color-text-soft);
  font-size: 12px;
  line-height: 1.4;
}

.entry-shortcuts__panel kbd {
  border: 1px solid var(--border-control);
  border-radius: 4px;
  background: color-mix(in srgb, var(--color-bg) 78%, var(--color-surface));
  color: var(--color-text-soft);
  padding: 1px 5px;
  font: inherit;
  font-size: 11px;
  line-height: 1.45;
  white-space: nowrap;
}

.save-status {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 8px;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1;
  white-space: nowrap;
}

.save-status span {
  display: inline-flex;
  align-items: center;
}

.save-status.status-saving {
  color: var(--color-text-soft);
}

.save-status.status-saved {
  color: var(--color-muted);
}

.save-status.status-dirty {
  color: var(--color-text-note);
}

.save-status.status-failed {
  color: var(--color-danger);
}

.save-status.status-conflict {
  color: var(--color-danger);
}

.save-status button {
  min-height: 28px;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: inherit;
  padding: 0 8px;
  font-size: 12px;
}

.save-status button:hover {
  background: var(--color-danger-bg);
}

.entry-surface__chrome--focus {
  position: sticky;
  top: 0;
  z-index: 3;
  border-bottom: 1px solid
    color-mix(in srgb, var(--border-subtle) 54%, transparent);
  background: color-mix(in srgb, var(--color-bg) 94%, transparent);
  padding: 0 0 12px;
}

.entry-surface__chrome--focus .entry-surface__date-nav {
  justify-content: flex-start;
}

@media (max-width: 720px) {
  .entry-surface__chrome {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .entry-surface__date-nav {
    grid-column: 1;
    justify-content: flex-start;
  }
}

@media (max-width: 480px) {
  .entry-surface__chrome {
    position: relative;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
  }

  .entry-surface__date-nav {
    grid-column: 1;
    display: grid;
    grid-template-columns: 36px minmax(0, 1fr) 36px;
    width: 100%;
    gap: 4px;
  }

  .entry-surface__date {
    align-self: center;
    font-size: 17px;
    text-align: center;
    white-space: nowrap;
  }

  .entry-surface__chrome-action {
    width: 36px;
    min-height: 36px;
    justify-content: center;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    background: transparent;
    padding: 0;
    font-size: 22px;
    text-decoration: none;
  }

  .entry-surface__chrome-action:hover:not(:disabled),
  .entry-surface__chrome-action:focus-visible {
    background: var(--surface-hover);
    text-decoration: none;
  }

  .entry-surface__nav-label {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .entry-actions__trigger,
  .entry-surface__chrome-side :deep(.icon-button) {
    width: 36px;
    height: 36px;
  }

  .mobile-photo-action {
    display: inline-grid;
    width: 36px;
    height: 36px;
    flex: 0 0 auto;
    place-items: center;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-muted);
    padding: 0;
  }

  .mobile-photo-action:hover:not(:disabled),
  .mobile-photo-action:focus-visible {
    background: var(--surface-hover);
    color: var(--color-text);
  }

  .mobile-photo-action:disabled {
    opacity: 0.5;
  }

  .actions {
    grid-column: 2;
    width: auto;
    justify-content: flex-end;
  }

  .actions:not(.actions--focus) {
    align-items: center;
    gap: 4px;
  }

  .save-status {
    min-height: 36px;
    font-size: 11px;
  }

  .save-status--normal {
    order: -1;
    min-width: 0;
    margin-right: 0;
    white-space: normal;
  }

  .save-status--normal span {
    max-width: 74px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .entry-surface__chrome--focus {
    grid-template-columns: minmax(0, 1fr);
    align-items: stretch;
    gap: 8px;
  }

  .entry-surface__chrome--focus .entry-surface__date-nav {
    grid-column: 1;
    grid-template-columns: 40px minmax(0, 1fr) 40px;
    width: 100%;
  }

  .actions--focus {
    grid-column: 1;
    align-items: center;
    gap: 6px;
  }

  .save-status--focus {
    order: -1;
    min-width: 0;
    margin-right: auto;
  }

  .focus-exit {
    width: 40px;
    min-width: 40px;
    justify-content: center;
    padding: 0;
  }

  .focus-exit span {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
}
</style>
