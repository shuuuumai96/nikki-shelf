<script setup lang="ts">
import { Image, SearchX } from "lucide-vue-next";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import PastelBadge from "../../../shared/components/PastelBadge.vue";
import { moodSpecs } from "../moods";
import type { Entry, MoodKey } from "../types";

const props = defineProps<{
  entries: Entry[];
  selectedId?: number;
  selectedDate: string;
  loading?: boolean;
  query?: string;
  mood?: string;
  tag?: string;
  hasFilters?: boolean;
  hasMore?: boolean;
}>();

const emit = defineEmits<{
  select: [entry: Entry];
  clearFilters: [];
  loadMore: [];
}>();

const initialVisibleCount = 60;
const visibleIncrement = 60;
const visibleCount = ref(initialVisibleCount);
const { locale, t } = useI18n();

type EntryMonthGroup = {
  key: string;
  label: string;
  entries: Entry[];
};

type SnippetSegment = {
  text: string;
  match: boolean;
};

type EntrySnippet = {
  key: string;
  segments: SnippetSegment[];
};

const monthFormatter = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, { month: "long", timeZone: "UTC" }),
);

const visibleEntries = computed(() =>
  props.entries.slice(0, visibleCount.value),
);

const entryGroups = computed<EntryMonthGroup[]>(() =>
  groupEntriesByMonth(visibleEntries.value),
);

const canLoadMore = computed(
  () => visibleEntries.value.length < props.entries.length || props.hasMore,
);

const snippets = computed(() => {
  const byEntryId = new Map<number, EntrySnippet>();

  for (const entry of visibleEntries.value) {
    const snippet = createEntrySnippet(entry);
    if (snippet) {
      byEntryId.set(entry.id, snippet);
    }
  }

  return byEntryId;
});

const activeFilters = computed(() => {
  const items: string[] = [];

  if (props.query) {
    items.push(t("entries.filterSearch", { query: props.query }));
  }

  if (props.mood) {
    items.push(t("entries.filterMood", { mood: moodLabel(props.mood) }));
  }

  if (props.tag) {
    items.push(t("entries.filterTag", { tag: props.tag }));
  }

  return items;
});

const emptyMessage = computed(() =>
  props.hasFilters
    ? t("entries.noEntriesMatchFilters")
    : t("entries.noEntriesAvailable"),
);

const emptyDetail = computed(() =>
  props.hasFilters
    ? t("entries.clearFiltersDetail")
    : t("entries.entriesAppearDetail"),
);

watch(
  () => props.entries,
  () => {
    visibleCount.value = initialVisibleCount;
    includeSelectedEntry();
  },
);

watch(
  () => props.selectedId,
  () => {
    includeSelectedEntry();
  },
);

function groupEntriesByMonth(entries: Entry[]): EntryMonthGroup[] {
  const groups: EntryMonthGroup[] = [];
  const byMonth = new Map<string, EntryMonthGroup>();

  entries.forEach((entry) => {
    const key = entry.entryDate.slice(0, 7);
    let group = byMonth.get(key);

    if (!group) {
      group = {
        key,
        label: monthLabel(key),
        entries: [],
      };
      byMonth.set(key, group);
      groups.push(group);
    }

    group.entries.push(entry);
  });

  return groups;
}

function includeSelectedEntry() {
  if (!props.selectedId) {
    return;
  }

  const selectedIndex = props.entries.findIndex(
    (entry) => entry.id === props.selectedId,
  );
  if (selectedIndex < 0 || selectedIndex < visibleCount.value) {
    return;
  }

  // Keep the active item mounted after a filter/page refresh so keyboard focus
  // and aria-current do not point at an entry hidden by lazy rendering.
  visibleCount.value =
    Math.ceil((selectedIndex + 1) / visibleIncrement) * visibleIncrement;
}

function loadMore() {
  if (visibleEntries.value.length >= props.entries.length && props.hasMore) {
    emit("loadMore");
    return;
  }

  visibleCount.value = Math.min(
    props.entries.length,
    visibleCount.value + visibleIncrement,
  );
}

function createEntrySnippet(entry: Entry): EntrySnippet | null {
  const query = props.query?.trim();
  if (!query || !entry.body.trim()) {
    return null;
  }

  const match = firstQueryMatch(entry.body, query);
  if (!match) {
    return null;
  }

  const contextStart = Math.max(0, match.index - 58);
  const contextEnd = Math.min(
    entry.body.length,
    match.index + match.text.length + 82,
  );
  const contextText = entry.body.slice(contextStart, contextEnd);
  const prefix = contextStart > 0 ? "..." : "";
  const suffix = contextEnd < entry.body.length ? "..." : "";
  const leadingTrim = leadingTrimCount(contextText);
  const snippetText = `${prefix}${contextText.trim()}${suffix}`;
  // Trimming leading whitespace shifts the visible match; compensate so the
  // highlighted segment still lines up with the original body match.
  const adjustedIndex =
    prefix.length + Math.max(0, match.index - contextStart - leadingTrim);
  const segments = splitSnippet(snippetText, adjustedIndex, match.text.length);

  return {
    key: `${entry.id}:${query}:${match.index}`,
    segments,
  };
}

function firstQueryMatch(
  body: string,
  query: string,
): { index: number; text: string } | null {
  const fullMatch = findCaseInsensitive(body, query);
  if (fullMatch) {
    return fullMatch;
  }

  for (const term of queryTerms(query)) {
    const termMatch = findCaseInsensitive(body, term);
    if (termMatch) {
      return termMatch;
    }
  }

  return null;
}

function findCaseInsensitive(
  source: string,
  needle: string,
): { index: number; text: string } | null {
  const normalizedNeedle = needle.trim();
  if (!normalizedNeedle) {
    return null;
  }

  const index = source
    .toLocaleLowerCase()
    .indexOf(normalizedNeedle.toLocaleLowerCase());
  if (index < 0) {
    return null;
  }

  return {
    index,
    text: source.slice(index, index + normalizedNeedle.length),
  };
}

function queryTerms(query: string): string[] {
  return Array.from(
    new Set(
      query
        .split(/\s+/)
        .map((term) => term.trim())
        .filter(Boolean),
    ),
  );
}

function leadingTrimCount(value: string): number {
  return value.length - value.trimStart().length;
}

function splitSnippet(
  snippet: string,
  matchIndex: number,
  matchLength: number,
): SnippetSegment[] {
  const safeIndex = Math.max(0, Math.min(snippet.length, matchIndex));
  const safeEnd = Math.max(
    safeIndex,
    Math.min(snippet.length, safeIndex + matchLength),
  );

  return [
    { text: snippet.slice(0, safeIndex), match: false },
    { text: snippet.slice(safeIndex, safeEnd), match: true },
    { text: snippet.slice(safeEnd), match: false },
  ].filter((segment) => segment.text.length > 0);
}

function monthLabel(key: string): string {
  const [year, month] = key.split("-");
  const monthIndex = Number(month) - 1;

  if (!year || Number.isNaN(monthIndex)) {
    return key;
  }

  return `${year} ${monthFormatter.value.format(new Date(Date.UTC(Number(year), monthIndex, 1)))}`;
}

function moodLabel(mood: string): string {
  return moodSpecs[mood as MoodKey]
    ? t(moodSpecs[mood as MoodKey].labelKey)
    : mood;
}
</script>

<template>
  <div class="list">
    <div
      v-if="activeFilters.length"
      class="active-filter-summary"
      :aria-label="t('entries.activeDiaryFilters')"
    >
      <div class="active-filter-chips">
        <span v-for="item in activeFilters" :key="item">{{ item }}</span>
      </div>
      <button class="ui-action" type="button" @click="emit('clearFilters')">
        {{ t("common.clearFilters") }}
      </button>
    </div>

    <div v-if="!loading && entries.length === 0" class="empty">
      <SearchX :size="18" />
      <div>
        <span>{{ emptyMessage }}</span>
        <p>{{ emptyDetail }}</p>
      </div>
    </div>

    <section
      v-for="group in entryGroups"
      :key="group.key"
      class="month-group"
      :aria-labelledby="`month-${group.key}`"
    >
      <h2 :id="`month-${group.key}`" class="month-header">{{ group.label }}</h2>

      <div class="month-entries">
        <template v-for="(entry, index) in group.entries" :key="entry.id">
          <div
            v-if="
              index === 0 ||
              group.entries[index - 1]?.entryDate !== entry.entryDate
            "
            class="date-group"
            :class="{ current: selectedDate === entry.entryDate }"
          >
            {{ entry.entryDate }}
          </div>

          <button
            class="entry-row ui-soft-row"
            :class="{
              active: selectedId === entry.id,
              current: selectedDate === entry.entryDate,
            }"
            type="button"
            :aria-current="
              selectedDate === entry.entryDate ? 'date' : undefined
            "
            @click="emit('select', entry)"
          >
            <div class="entry-main">
              <div class="entry-title">
                <span>{{ entry.title || t("common.untitled") }}</span>
                <PastelBadge :color="moodSpecs[entry.mood].color">
                  <component :is="moodSpecs[entry.mood].icon" :size="13" />
                  {{ t(moodSpecs[entry.mood].labelKey) }}
                </PastelBadge>
              </div>
              <p
                v-if="snippets.get(entry.id)"
                :key="snippets.get(entry.id)?.key"
                class="entry-snippet"
              >
                <template
                  v-for="(segment, segmentIndex) in snippets.get(entry.id)
                    ?.segments"
                  :key="segmentIndex"
                >
                  <mark v-if="segment.match">{{ segment.text }}</mark>
                  <span v-else>{{ segment.text }}</span>
                </template>
              </p>
              <p v-else-if="entry.body">{{ entry.body }}</p>
              <div
                v-if="entry.tags.length || entry.images.length"
                class="entry-meta"
              >
                <span v-if="entry.tags.length">{{
                  entry.tags.join(" / ")
                }}</span>
                <span v-if="entry.images.length" class="with-icon">
                  <Image :size="13" />
                  {{ entry.images.length }}
                </span>
              </div>
            </div>
          </button>
        </template>
      </div>
    </section>

    <div v-if="entries.length > 0" class="list-footer">
      <p class="entry-count">
        {{
          t("entries.showingEntries", {
            visible: visibleEntries.length,
            total: entries.length,
          })
        }}
      </p>
      <button
        v-if="canLoadMore"
        class="load-more ui-action"
        type="button"
        @click="loadMore"
      >
        {{ t("entries.loadMore") }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.list {
  display: grid;
  min-width: 0;
  gap: 22px;
}

.empty {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-height: 80px;
  color: var(--color-muted);
  padding: 0 6px;
}

.empty span {
  display: block;
  color: var(--color-text-soft);
  font-size: 13px;
  font-weight: 680;
  line-height: 1.4;
}

.empty p {
  display: block;
  margin: 3px 0 0;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1.45;
  -webkit-line-clamp: unset;
  line-clamp: unset;
}

.active-filter-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  border-bottom: 1px solid var(--border-subtle);
  padding: 0 0 12px;
}

.active-filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.active-filter-chips span {
  max-width: 100%;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-muted);
  padding: 4px 7px;
  font-size: 12px;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.month-group {
  min-width: 0;
}

.month-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 10px;
  color: var(--color-text);
  font-size: 14px;
  font-weight: 760;
  line-height: 1.3;
  letter-spacing: 0;
}

.month-header::after {
  flex: 1;
  height: 1px;
  min-width: 24px;
  background: var(--border-subtle);
  content: "";
}

.month-entries {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.date-group {
  margin: 8px 0 0;
  color: var(--color-muted);
  font-size: 12px;
  font-weight: 680;
  letter-spacing: 0;
}

.date-group.current {
  color: var(--color-text-soft);
}

.month-entries > .date-group:first-child {
  margin-top: 0;
}

.entry-row {
  position: relative;
  width: 100%;
  min-width: 0;
  border-color: var(--border-subtle);
  background: var(--color-surface);
  padding: 14px 15px 14px 17px;
  text-align: left;
}

.entry-row::before {
  position: absolute;
  top: 10px;
  bottom: 10px;
  left: 0;
  width: 3px;
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  background: transparent;
  content: "";
}

.entry-row:hover {
  border-color: color-mix(in srgb, var(--color-line) 38%, #fff);
  background: color-mix(in srgb, var(--color-wash) 34%, #fff);
}

.entry-row.active {
  border-color: var(--color-border);
  background: var(--color-surface-soft);
}

.entry-row.current {
  border-color: color-mix(in srgb, var(--color-border) 82%, #fff);
  background: color-mix(in srgb, var(--color-surface-soft) 74%, #fff);
}

.entry-row.active::before,
.entry-row.current::before {
  background: var(--color-line);
}

.entry-row:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 42%, #fff);
  outline-offset: 3px;
}

.entry-main {
  min-width: 0;
}

.entry-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.entry-title > span {
  overflow: hidden;
  min-width: 0;
  color: var(--color-text);
  font-size: 15px;
  font-weight: 690;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

p {
  display: -webkit-box;
  overflow: hidden;
  margin: 6px 0 7px;
  color: var(--color-text-soft);
  font-size: 13px;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.entry-snippet {
  color: var(--color-muted);
}

.entry-snippet mark {
  border-radius: 3px;
  background: color-mix(in srgb, var(--color-accent) 20%, #fff);
  color: var(--color-text);
  padding: 0 2px;
}

.entry-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  color: var(--color-muted);
  font-size: 12px;
  min-width: 0;
}

.entry-meta span {
  max-width: 100%;
  overflow-wrap: anywhere;
}

.with-icon {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.list-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 14px;
}

.entry-count {
  margin: 0;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1.4;
}

.load-more {
  flex: 0 0 auto;
}

@media (max-width: 480px) {
  .list {
    gap: 18px;
  }

  .month-header {
    margin-bottom: 8px;
    font-size: 13px;
  }

  .month-entries {
    gap: 8px;
  }

  .entry-row {
    min-height: 84px;
    padding: 12px 12px 12px 14px;
  }

  .entry-title {
    align-items: flex-start;
    gap: 8px;
  }

  .entry-title > span {
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  p {
    line-height: 1.55;
  }

  .entry-meta {
    gap: 8px;
  }

  .active-filter-summary {
    align-items: flex-start;
    flex-direction: column;
  }

  .list-footer {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
