<script setup lang="ts">
import { Filter, Image, Search, X } from "lucide-vue-next";
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { searchEntries } from "../api";
import { moodOrder, moodSpecs } from "../moods";
import type { EntrySearchResult, MoodKey } from "../types";

const props = defineProps<{
  tags: string[];
  selectedDate: string;
}>();

const emit = defineEmits<{
  selectDate: [date: string];
}>();

const query = ref("");
const focused = ref(false);
const filtersOpen = ref(false);
const filters = reactive({
  from: "",
  to: "",
  mood: "",
  tag: "",
  hasImage: false,
});
const results = ref<EntrySearchResult[]>([]);
const loading = ref(false);
const error = ref("");
const { t } = useI18n();

let debounceID = 0;
let requestID = 0;
let controller: AbortController | null = null;
let timeoutID = 0;

const hasSecondaryFilters = computed(() =>
  Boolean(
    filters.from ||
    filters.to ||
    filters.mood ||
    filters.tag ||
    filters.hasImage,
  ),
);
const hasSearchInput = computed(() =>
  Boolean(query.value.trim() || hasSecondaryFilters.value),
);
const searchMode = computed(() => focused.value || hasSearchInput.value);
const activeFilterChips = computed(() => {
  const chips: string[] = [];

  if (filters.from) {
    chips.push(t("entries.filterFrom", { date: filters.from }));
  }
  if (filters.to) {
    chips.push(t("entries.filterTo", { date: filters.to }));
  }
  if (filters.mood) {
    chips.push(t("entries.filterMood", { mood: moodLabel(filters.mood) }));
  }
  if (filters.tag) {
    chips.push(t("entries.filterExactTag", { tag: filters.tag }));
  }
  if (filters.hasImage) {
    chips.push(t("entries.hasImage"));
  }

  return chips;
});
const resultSummary = computed(() => {
  if (!hasSearchInput.value || loading.value || error.value) {
    return "";
  }

  return t("entries.matchingEntries", results.value.length);
});

watch(
  () => [
    query.value,
    filters.from,
    filters.to,
    filters.mood,
    filters.tag,
    filters.hasImage,
  ],
  () => {
    scheduleSearch();
  },
);

onBeforeUnmount(() => {
  window.clearTimeout(debounceID);
  window.clearTimeout(timeoutID);
  controller?.abort();
});

function scheduleSearch() {
  window.clearTimeout(debounceID);
  debounceID = window.setTimeout(() => {
    void runSearch();
  }, 360);
}

async function runSearch() {
  controller?.abort();
  error.value = "";

  if (!hasSearchInput.value) {
    results.value = [];
    loading.value = false;
    return;
  }

  // Abort stops the network request; requestID also ignores stale completions
  // from browsers or intermediaries that still resolve an older request.
  const currentRequest = requestID + 1;
  requestID = currentRequest;
  controller = new AbortController();
  let timedOut = false;
  window.clearTimeout(timeoutID);
  timeoutID = window.setTimeout(() => {
    timedOut = true;
    controller?.abort();
  }, 10000);
  loading.value = true;

  try {
    const response = await searchEntries(
      {
        q: query.value.trim(),
        from: filters.from,
        to: filters.to,
        mood: filters.mood,
        tag: filters.tag,
        hasImage: filters.hasImage ? "true" : "",
        limit: "50",
      },
      controller.signal,
    );
    if (requestID === currentRequest) {
      results.value = response.results;
    }
  } catch (err) {
    if (isAbortError(err)) {
      if (timedOut && requestID === currentRequest) {
        results.value = [];
        error.value = t("entries.searchFailedRetry");
      }
      return;
    }
    if (requestID === currentRequest) {
      results.value = [];
      error.value =
        err instanceof Error ? err.message : t("entries.searchFailed");
    }
  } finally {
    if (requestID === currentRequest) {
      window.clearTimeout(timeoutID);
    }
    if (requestID === currentRequest) {
      loading.value = false;
    }
  }
}

function clearSearch() {
  query.value = "";
  clearFilters();
  results.value = [];
  error.value = "";
  filtersOpen.value = false;
}

function clearFilters() {
  filters.from = "";
  filters.to = "";
  filters.mood = "";
  filters.tag = "";
  filters.hasImage = false;
}

function selectResult(result: EntrySearchResult) {
  emit("selectDate", result.entryDate);
}

function moodLabel(mood: string): string {
  return moodSpecs[mood as MoodKey]
    ? t(moodSpecs[mood as MoodKey].labelKey)
    : mood;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}
</script>

<template>
  <section
    class="memory-search"
    :class="{ active: searchMode }"
    aria-labelledby="memory-search-label"
  >
    <label id="memory-search-label" class="sr-only" for="memory-search-input">{{
      t("entries.searchPastEntries")
    }}</label>
    <div class="memory-input">
      <Search :size="15" stroke-width="1.8" />
      <input
        id="memory-search-input"
        v-model="query"
        type="search"
        autocomplete="off"
        :placeholder="t('entries.searchPastEntries')"
        @focus="focused = true"
        @blur="focused = false"
        @keydown.enter.prevent
      />
      <button
        v-if="hasSearchInput"
        type="button"
        class="icon-button"
        :aria-label="t('common.clearFilters')"
        :title="t('common.clearFilters')"
        @click="clearSearch"
      >
        <X :size="14" stroke-width="1.9" />
      </button>
      <button
        type="button"
        class="icon-button"
        :class="{ active: hasSecondaryFilters }"
        :aria-expanded="filtersOpen"
        aria-controls="memory-search-filters"
        :aria-label="t('entries.searchFilters')"
        :title="t('entries.searchFilters')"
        @click="filtersOpen = !filtersOpen"
      >
        <Filter :size="14" stroke-width="1.9" />
      </button>
    </div>

    <div v-if="filtersOpen" id="memory-search-filters" class="memory-filters">
      <div class="date-row">
        <label>
          <span>{{ t("entries.from") }}</span>
          <input v-model="filters.from" type="date" />
        </label>
        <label>
          <span>{{ t("entries.to") }}</span>
          <input v-model="filters.to" type="date" />
        </label>
      </div>
      <select v-model="filters.mood" :aria-label="t('entries.mood')">
        <option value="">{{ t("entries.mood") }}</option>
        <option v-for="key in moodOrder" :key="key" :value="key">
          {{ t(moodSpecs[key].labelKey) }}
        </option>
      </select>
      <select v-model="filters.tag" :aria-label="t('entries.tag')">
        <option value="">{{ t("entries.tag") }}</option>
        <option v-for="tag in props.tags" :key="tag" :value="tag">
          {{ tag }}
        </option>
      </select>
      <label class="image-toggle">
        <input v-model="filters.hasImage" type="checkbox" />
        <span>{{ t("entries.hasImage") }}</span>
      </label>
      <button
        v-if="hasSecondaryFilters"
        type="button"
        class="clear-filters"
        @click="clearFilters"
      >
        {{ t("common.clearFilters") }}
      </button>
    </div>

    <div v-if="searchMode" class="memory-results" aria-live="polite">
      <p v-if="!hasSearchInput" class="memory-hint">
        {{ t("entries.searchHint") }}
      </p>
      <p v-else-if="loading" class="memory-hint">
        {{ t("entries.searching") }}
      </p>
      <p v-else-if="error" class="memory-error">{{ error }}</p>
      <template v-else>
        <div v-if="hasSecondaryFilters || resultSummary" class="result-status">
          <span v-if="resultSummary">{{ resultSummary }}</span>
          <div
            v-if="activeFilterChips.length"
            class="active-filters"
            :aria-label="t('entries.activeSearchFilters')"
          >
            <span v-for="chip in activeFilterChips" :key="chip">{{
              chip
            }}</span>
          </div>
        </div>

        <div v-if="results.length === 0" class="memory-empty">
          <strong>{{ t("entries.noSearchMatches") }}</strong>
          <span>{{ t("entries.noSearchMatchesDetail") }}</span>
          <button
            v-if="hasSecondaryFilters"
            type="button"
            @click="clearFilters"
          >
            {{ t("entries.removeFilters") }}
          </button>
        </div>
        <div v-else class="result-list">
          <button
            v-for="result in results"
            :key="result.id"
            type="button"
            class="result-card"
            :class="{ current: selectedDate === result.entryDate }"
            :aria-current="
              selectedDate === result.entryDate ? 'date' : undefined
            "
            @click="selectResult(result)"
          >
            <span class="result-date">{{ result.entryDate }}</span>
            <strong>{{ result.title || t("common.untitled") }}</strong>
            <span class="result-meta">
              <span v-if="result.mood" class="mood-chip">{{
                moodLabel(result.mood)
              }}</span>
              <span v-for="tag in result.tags" :key="tag" class="tag-chip">{{
                tag
              }}</span>
              <span v-if="result.hasImage" class="image-chip">
                <Image :size="12" stroke-width="1.8" />
                {{ result.imageCount }}
              </span>
            </span>
            <span v-if="result.preview" class="result-preview">{{
              result.preview
            }}</span>
          </button>
        </div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.memory-search {
  display: grid;
  gap: 8px;
  margin-top: 14px;
}

.memory-search.active {
  min-height: 0;
}

.memory-input {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 6px;
  min-height: 34px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-muted);
  padding: 0 6px 0 9px;
}

.memory-input:focus-within {
  border-color: var(--border-active);
  background: var(--surface-glass);
}

.memory-input input {
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--color-text);
  font-size: 13px;
}

.icon-button {
  display: grid;
  width: 24px;
  height: 24px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  padding: 0;
}

.icon-button:hover,
.icon-button.active {
  border-color: var(--border-subtle);
  background: var(--surface-hover);
  color: var(--color-text);
}

.memory-filters {
  display: grid;
  gap: 7px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  padding: 8px;
}

.date-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
}

.memory-filters label {
  min-width: 0;
}

.memory-filters label span {
  display: block;
  margin-bottom: 2px;
  color: var(--color-muted);
  font-size: 10px;
}

.memory-filters input,
.memory-filters select {
  width: 100%;
  min-width: 0;
  height: 30px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--surface-glass);
  color: var(--color-text);
  padding: 0 7px;
  font-size: 12px;
}

.image-toggle {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--color-text);
  font-size: 12px;
}

.image-toggle input {
  width: 14px;
  height: 14px;
}

.image-toggle span {
  margin: 0;
}

.clear-filters,
.memory-empty button {
  justify-self: start;
  border: 0;
  background: transparent;
  color: var(--color-muted);
  padding: 0;
  font-size: 12px;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.memory-results {
  min-height: 0;
  max-height: min(54dvh, 520px);
  overflow: auto;
  padding-right: 2px;
  scrollbar-gutter: stable;
}

.memory-hint,
.memory-error,
.memory-empty {
  margin: 0;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1.45;
  padding: 4px 2px 6px;
}

.memory-error {
  color: var(--color-danger);
}

.memory-empty {
  display: grid;
  gap: 5px;
}

.memory-empty strong {
  color: var(--color-text);
  font-size: 12px;
}

.result-status {
  display: grid;
  gap: 6px;
  margin-bottom: 8px;
  color: var(--color-muted);
  font-size: 11px;
  line-height: 1.35;
}

.active-filters {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 5px;
}

.active-filters span,
.mood-chip,
.tag-chip {
  max-width: 100%;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--surface-glass-soft);
  padding: 2px 6px;
  overflow-wrap: anywhere;
}

.result-list {
  display: grid;
  gap: 7px;
}

.result-card {
  display: grid;
  gap: 3px;
  width: 100%;
  min-width: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  padding: 9px 10px;
  text-align: left;
}

.result-card:hover {
  border-color: var(--border-active);
  background: var(--surface-glass-soft);
}

.result-card.current {
  border-color: color-mix(in srgb, var(--color-border) 82%, #fff);
  background: color-mix(in srgb, var(--color-surface-soft) 72%, #fff);
}

.result-card.current::before {
  display: block;
  width: 24px;
  height: 2px;
  border-radius: var(--radius-sm);
  background: var(--color-line);
  content: "";
}

.result-card:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 42%, #fff);
  outline-offset: 2px;
}

.result-date {
  color: var(--color-muted);
  font-size: 11px;
  line-height: 1.25;
}

.result-card strong {
  overflow: hidden;
  color: var(--color-text);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-preview {
  display: -webkit-box;
  overflow: hidden;
  color: var(--color-text-soft);
  font-size: 12px;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow-wrap: anywhere;
}

.result-meta {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 5px;
  color: var(--color-muted);
  font-size: 11px;
  line-height: 1.35;
}

.result-meta span {
  max-width: 100%;
  overflow-wrap: anywhere;
}

.image-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--surface-glass-soft);
  padding: 2px 6px;
}
</style>
