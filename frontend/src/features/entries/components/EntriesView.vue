<script setup lang="ts">
import { Filter, Search } from "lucide-vue-next";
import { watch } from "vue";
import { useI18n } from "vue-i18n";
import { useDateJump } from "../composables/useDateJump";
import { useEntryFilters } from "../composables/useEntryFilters";
import { moodOrder, moodSpecs } from "../moods";
import type { Entry, EntryFilter } from "../types";
import EntryArchiveNav from "./EntryArchiveNav.vue";
import EntryList from "./EntryList.vue";

defineProps<{
  entries: Entry[];
  hasMore: boolean;
  loading: boolean;
  selectedDate: string;
  selectedId?: number;
  tags: string[];
}>();

const emit = defineEmits<{
  clearFilters: [];
  loadEntries: [filter: EntryFilter];
  loadMore: [];
  selectDate: [date: string, source: "archive"];
  selectEntry: [entry: Entry];
}>();

const { t } = useI18n();
const entriesMode = defineModel<"list" | "archive">("mode", {
  default: "list",
});
const { clearEntryFilters, filters, hasEntryFilters } = useEntryFilters();
const { clearDateJumpError, dateJumpError, dateJumpValue, jumpToDate } =
  useDateJump({
    selectDate: (date) => emit("selectDate", date, "archive"),
  });

watch(
  () => [filters.query, filters.mood, filters.tag],
  () => {
    emit("loadEntries", {
      mood: filters.mood,
      query: filters.query,
      tag: filters.tag,
    });
  },
);
</script>

<template>
  <section class="entries-view ui-page">
    <div class="entries-inner">
      <header class="entries-header">
        <h1 class="ui-heading">{{ t("entries.diary") }}</h1>
        <div class="entries-header-actions">
          <div class="entries-mode" :aria-label="t('entries.viewMode')">
            <button
              type="button"
              :class="{ active: entriesMode === 'list' }"
              :aria-pressed="entriesMode === 'list'"
              @click="entriesMode = 'list'"
            >
              {{ t("entries.list") }}
            </button>
            <button
              type="button"
              :class="{ active: entriesMode === 'archive' }"
              :aria-pressed="entriesMode === 'archive'"
              @click="entriesMode = 'archive'"
            >
              {{ t("entries.archive") }}
            </button>
          </div>
          <form
            class="date-jump"
            :aria-label="t('entries.jumpLabel')"
            @submit.prevent="jumpToDate"
          >
            <label class="sr-only" for="date-jump-input">{{
              t("entries.jumpToDate")
            }}</label>
            <input
              id="date-jump-input"
              v-model="dateJumpValue"
              type="date"
              @input="clearDateJumpError"
            />
            <button type="submit">{{ t("entries.jump") }}</button>
          </form>
          <div class="search-box ui-search">
            <Search :size="16" stroke-width="1.8" />
            <input v-model="filters.query" :placeholder="t('entries.search')" />
          </div>
        </div>
      </header>

      <p v-if="dateJumpError" class="date-jump-error">
        {{ dateJumpError }}
      </p>

      <div class="filters">
        <div class="filter-label">
          <Filter :size="15" stroke-width="1.8" />
        </div>
        <select v-model="filters.mood" class="ui-select">
          <option value="">{{ t("entries.mood") }}</option>
          <option v-for="key in moodOrder" :key="key" :value="key">
            {{ t(moodSpecs[key].labelKey) }}
          </option>
        </select>
        <select v-model="filters.tag" class="ui-select">
          <option value="">{{ t("entries.tag") }}</option>
          <option v-for="tag in tags" :key="tag" :value="tag">
            {{ tag }}
          </option>
        </select>
      </div>

      <div class="entries-scroll" :aria-label="t('entries.diaryEntries')">
        <EntryList
          v-if="entriesMode === 'list'"
          :entries="entries"
          :selected-id="selectedId"
          :selected-date="selectedDate"
          :loading="loading"
          :query="filters.query"
          :mood="filters.mood"
          :tag="filters.tag"
          :has-filters="hasEntryFilters"
          :has-more="hasMore"
          @select="emit('selectEntry', $event)"
          @clear-filters="clearEntryFilters"
          @load-more="emit('loadMore')"
        />
        <EntryArchiveNav
          v-else
          :entries="entries"
          :selected-date="selectedDate"
          :loading="loading"
          :has-filters="hasEntryFilters"
          :has-more="hasMore"
          @select-date="emit('selectDate', $event, 'archive')"
          @clear-filters="clearEntryFilters"
          @load-more="emit('loadMore')"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.entries-view {
  --page-max: 900px;
  display: block;
  height: 100%;
  overflow: hidden;
}

.entries-inner {
  display: grid;
  height: 100%;
  min-height: 0;
  width: min(820px, 100%);
  margin: 0 auto;
  min-width: 0;
  grid-template-rows: auto auto minmax(0, 1fr);
}

.entries-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.entries-header h1 {
  font-size: 28px;
}

.entries-header-actions {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.entries-mode {
  display: inline-grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  min-width: 172px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 3px;
}

.entries-mode button {
  min-height: 30px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  padding: 0 10px;
  font-size: 13px;
}

.entries-mode button:hover {
  color: var(--color-text);
}

.entries-mode button.active {
  border-color: var(--border-active);
  background: var(--surface-active);
  color: var(--color-text);
}

.date-jump {
  display: grid;
  grid-template-columns: minmax(124px, 1fr) auto;
  align-items: center;
  gap: 6px;
  min-width: 192px;
}

.date-jump input {
  width: 100%;
  min-width: 0;
  height: 34px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 8px;
  font-size: 12px;
}

.date-jump input:focus {
  border-color: var(--border-active);
  outline: 0;
}

.date-jump button {
  min-height: 34px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-muted);
  padding: 0 10px;
  font-size: 12px;
}

.date-jump button:hover {
  border-color: var(--border-active);
  background: var(--surface-hover);
  color: var(--color-text);
}

.date-jump-error {
  margin: -6px 0 10px;
  color: var(--color-danger);
  font-size: 12px;
}

.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  min-width: 0;
}

.filter-label {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  color: var(--color-muted);
}

.entries-scroll {
  min-height: 0;
  overflow: auto;
  padding: 0 4px 24px 0;
  scrollbar-gutter: stable;
}

@media (max-width: 720px) {
  .entries-header {
    align-items: stretch;
    flex-direction: column;
  }

  .entries-header-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .entries-mode {
    width: 100%;
  }

  .date-jump {
    width: 100%;
  }

  .search-box {
    width: 100%;
  }
}

@media (max-width: 480px) {
  .entries-header {
    gap: 12px;
    margin-bottom: 12px;
  }

  .entries-header h1 {
    font-size: 24px;
  }

  .entries-mode button {
    min-height: 36px;
  }

  .filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    display: grid;
    gap: 8px;
    margin-bottom: 14px;
  }

  .filter-label {
    display: none;
  }

  .filters .ui-select {
    width: 100%;
    min-width: 0;
  }

  .entries-scroll {
    padding-right: 0;
  }
}
</style>
