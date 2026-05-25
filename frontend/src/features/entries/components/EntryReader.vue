<script setup lang="ts">
import { Ellipsis } from "lucide-vue-next";
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  formatDateLabel,
  nextLocalDayISO,
  previousLocalDayISO,
} from "../../../shared/utils/date";
import { renderEntryMarkdown } from "../markdown";
import { moodSpecs } from "../moods";
import type { Entry } from "../types";
import EntryImageAttachment from "./EntryImageAttachment.vue";

const props = defineProps<{
  entry: Entry;
  activeDate: string;
  isNavigating?: boolean;
}>();

const emit = defineEmits<{
  edit: [];
  navigateDate: [date: string];
}>();

const { locale, t } = useI18n();
const heading = computed(() => formatDateLabel(props.activeDate, locale.value));
const title = computed(() => props.entry.title.trim() || t("common.untitled"));
const body = computed(() => props.entry.body.trim());
const bodyHTML = computed(() => renderEntryMarkdown(props.entry.body));
const mood = computed(() => moodSpecs[props.entry.mood]);
const exportURL = computed(
  () => `/api/export/entries/${props.entry.id}/markdown`,
);
const entryActionsOpen = ref(false);
const entryActions = ref<HTMLElement | null>(null);

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onDocumentPointerDown);
  window.removeEventListener("keydown", onKeydown);
});

onMounted(() => {
  document.addEventListener("pointerdown", onDocumentPointerDown);
  window.addEventListener("keydown", onKeydown);
});

function navigatePreviousDay() {
  emit("navigateDate", previousLocalDayISO(props.activeDate));
}

function navigateNextDay() {
  emit("navigateDate", nextLocalDayISO(props.activeDate));
}

function toggleEntryActions() {
  entryActionsOpen.value = !entryActionsOpen.value;
}

function closeEntryActions() {
  entryActionsOpen.value = false;
}

function onDocumentPointerDown(event: PointerEvent) {
  if (!entryActionsOpen.value) {
    return;
  }
  const target = event.target;
  if (target instanceof Node && entryActions.value?.contains(target)) {
    return;
  }
  closeEntryActions();
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && entryActionsOpen.value) {
    event.preventDefault();
    closeEntryActions();
  }
}
</script>

<template>
  <article class="reader entry-surface ui-page">
    <header class="entry-surface__chrome">
      <div
        class="entry-surface__date-nav"
        :aria-label="t('entries.dateNavigation')"
      >
        <button
          type="button"
          class="entry-surface__chrome-action"
          :disabled="isNavigating"
          @click="navigatePreviousDay"
        >
          <span aria-hidden="true">‹</span>
          <span>{{ t("common.previousDay") }}</span>
        </button>
        <h1 class="ui-heading entry-surface__date">
          <span>{{ heading }}</span>
        </h1>
        <button
          type="button"
          class="entry-surface__chrome-action"
          :disabled="isNavigating"
          @click="navigateNextDay"
        >
          <span>{{ t("common.nextDay") }}</span>
          <span aria-hidden="true">›</span>
        </button>
      </div>

      <div class="entry-surface__chrome-side">
        <button
          type="button"
          class="entry-surface__chrome-action entry-surface__edit"
          @click="emit('edit')"
        >
          <span>{{ t("common.edit") }}</span>
        </button>
        <div ref="entryActions" class="entry-actions">
          <button
            type="button"
            class="entry-actions__trigger"
            :aria-label="t('common.entryActions')"
            :title="t('common.entryActions')"
            :aria-expanded="entryActionsOpen"
            aria-controls="reader-entry-actions"
            @click="toggleEntryActions"
            @keydown.esc="closeEntryActions"
          >
            <Ellipsis :size="17" stroke-width="1.8" />
          </button>
          <div
            v-if="entryActionsOpen"
            id="reader-entry-actions"
            class="entry-actions__menu"
            role="menu"
            @keydown.esc="closeEntryActions"
          >
            <a
              class="entry-actions__item"
              role="menuitem"
              :href="exportURL"
              @click="closeEntryActions"
            >
              {{ t("common.exportMarkdown") }}
            </a>
          </div>
        </div>
      </div>
    </header>

    <section class="entry-content" :aria-label="t('entries.diary')">
      <h2
        class="entry-surface__title"
        :class="{ 'entry-surface__title--placeholder': !entry.title.trim() }"
      >
        {{ title }}
      </h2>

      <div class="meta-line entry-surface__meta">
        <span v-if="mood" class="mood-chip">
          <component :is="mood.icon" :size="14" />
          {{ t(mood.labelKey) }}
        </span>
        <span v-for="tag in entry.tags" :key="tag" class="tag-chip">{{
          tag
        }}</span>
      </div>

      <div
        v-if="body"
        class="entry-surface__body entry-markdown-body"
        v-html="bodyHTML"
      ></div>
      <p
        v-else
        class="body-text entry-surface__body entry-surface__body--empty"
      >
        {{ t("entries.noBody") }}
      </p>

      <div
        v-if="entry.images.length"
        class="image-grid entry-surface__attachments"
        :aria-label="t('images.images')"
      >
        <EntryImageAttachment
          v-for="image in entry.images"
          :key="image.id"
          :image="image"
          :entry-date="entry.entryDate"
          show-caption
        />
      </div>
    </section>
  </article>
</template>

<style scoped>
.entry-surface {
  --page-max: 760px;
  min-height: 100%;
  padding-top: 38px;
}

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

.entry-surface__edit {
  justify-self: end;
}

.entry-surface__chrome-side {
  display: inline-flex;
  justify-self: end;
  align-items: center;
  gap: 8px;
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
  display: flex;
  width: 100%;
  align-items: center;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--color-text-soft);
  font: inherit;
  font-size: 13px;
  letter-spacing: 0;
  line-height: 1.3;
  padding: 8px 9px;
  text-align: left;
  text-decoration: none;
  white-space: nowrap;
}

.entry-actions__item:hover,
.entry-actions__item:focus-visible {
  background: var(--surface-hover);
  color: var(--color-text);
  outline: none;
}

.entry-surface__date {
  margin: 0;
  color: var(--color-text);
  font-size: 17px;
  font-weight: 680;
  letter-spacing: 0;
  line-height: 1.35;
}

.entry-content {
  display: block;
}

.entry-surface__title {
  margin: 0;
  color: var(--color-text);
  font-size: 28px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.32;
  overflow-wrap: anywhere;
}

.entry-surface__title {
  margin-bottom: 16px;
  min-height: 39px;
}

.entry-surface__title--placeholder,
.entry-surface__body--empty {
  color: var(--color-muted);
}

.entry-surface__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 24px;
  min-height: 79px;
  min-width: 0;
}

.mood-chip,
.tag-chip {
  display: inline-flex;
  min-height: 28px;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text-soft);
  padding: 0 9px;
  font-size: 12px;
  line-height: 1;
  overflow-wrap: anywhere;
}

.entry-surface__body {
  margin: 0;
  color: var(--color-text);
  overflow-wrap: anywhere;
}

.entry-surface__attachments {
  margin-top: 24px;
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 720px) {
  .entry-surface {
    padding-top: 26px;
  }

  .entry-surface__chrome {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .entry-surface__date-nav {
    grid-column: 1;
    justify-content: flex-start;
  }

  .image-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .entry-surface {
    padding-top: 22px;
    padding-bottom: calc(154px + env(safe-area-inset-bottom));
  }

  .entry-surface__chrome {
    gap: 10px;
    margin-bottom: 22px;
  }

  .entry-surface__date-nav {
    flex-wrap: wrap;
    gap: 8px;
  }

  .entry-surface__date {
    font-size: 16px;
  }

  .entry-surface__edit {
    align-self: start;
  }

  .entry-surface__title {
    font-size: 22px;
  }
}
</style>
