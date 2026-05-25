<script setup lang="ts">
import { SearchX } from "lucide-vue-next";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import CalendarMonth from "../../calendar/components/CalendarMonth.vue";
import type { Entry } from "../types";

const props = defineProps<{
  entries: Entry[];
  selectedDate: string;
  loading?: boolean;
  hasFilters?: boolean;
  hasMore?: boolean;
}>();

const emit = defineEmits<{
  selectDate: [date: string];
  clearFilters: [];
  loadMore: [];
}>();

type ArchiveMonth = {
  key: string;
  label: string;
  count: number;
  entries: Entry[];
};

type ArchiveYear = {
  year: string;
  months: ArchiveMonth[];
};

const selectedMonth = ref(
  props.entries[0]?.entryDate.slice(0, 7) || props.selectedDate.slice(0, 7),
);
const { locale, t } = useI18n();

const monthFormatter = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, { month: "long", timeZone: "UTC" }),
);

const archiveYears = computed<ArchiveYear[]>(() => {
  const years: ArchiveYear[] = [];
  const yearsByKey = new Map<string, ArchiveYear>();
  const monthsByKey = new Map<string, ArchiveMonth>();

  props.entries.forEach((entry) => {
    const monthKey = entry.entryDate.slice(0, 7);
    const yearKey = monthKey.slice(0, 4);
    let year = yearsByKey.get(yearKey);

    if (!year) {
      year = { year: yearKey, months: [] };
      yearsByKey.set(yearKey, year);
      years.push(year);
    }

    let month = monthsByKey.get(monthKey);
    if (!month) {
      month = {
        key: monthKey,
        label: monthLabel(monthKey),
        count: 0,
        entries: [],
      };
      monthsByKey.set(monthKey, month);
      year.months.push(month);
    }

    month.count += 1;
    month.entries.push(entry);
  });

  return years;
});

const selectedMonthEntries = computed(() => {
  return props.entries.filter((entry) =>
    entry.entryDate.startsWith(selectedMonth.value),
  );
});

const hasEntries = computed(() => props.entries.length > 0);

watch(
  () => props.entries,
  () => {
    const currentMonthExists = props.entries.some((entry) =>
      entry.entryDate.startsWith(selectedMonth.value),
    );
    if (!currentMonthExists) {
      selectedMonth.value =
        props.entries[0]?.entryDate.slice(0, 7) ||
        props.selectedDate.slice(0, 7);
    }
  },
);

watch(
  () => props.selectedDate,
  (date) => {
    const month = date.slice(0, 7);
    if (props.entries.some((entry) => entry.entryDate.startsWith(month))) {
      selectedMonth.value = month;
    }
  },
);

function selectMonth(month: string) {
  selectedMonth.value = month;
}

function monthLabel(key: string): string {
  const [year, month] = key.split("-");
  const monthIndex = Number(month) - 1;

  if (!year || Number.isNaN(monthIndex)) {
    return key;
  }

  return `${year} ${monthFormatter.value.format(new Date(Date.UTC(Number(year), monthIndex, 1)))}`;
}
</script>

<template>
  <div class="archive">
    <div v-if="!loading && !hasEntries" class="archive-empty">
      <SearchX :size="18" />
      <span>{{
        hasFilters
          ? t("entries.noEntriesMatchCurrentFilters")
          : t("entries.noEntriesYet")
      }}</span>
      <button
        v-if="hasFilters"
        class="ui-action"
        type="button"
        @click="emit('clearFilters')"
      >
        {{ t("common.clearFilters") }}
      </button>
    </div>

    <template v-else>
      <aside class="month-index" :aria-label="t('entries.archiveMonths')">
        <section
          v-for="year in archiveYears"
          :key="year.year"
          class="year-group"
          :aria-labelledby="`archive-${year.year}`"
        >
          <h2 :id="`archive-${year.year}`">{{ year.year }}</h2>
          <div class="month-buttons">
            <button
              v-for="month in year.months"
              :key="month.key"
              type="button"
              class="month-button"
              :class="{
                active: selectedMonth === month.key,
                current: selectedDate.startsWith(month.key),
              }"
              :aria-pressed="selectedMonth === month.key"
              :aria-current="
                selectedDate.startsWith(month.key) ? 'true' : undefined
              "
              @click="selectMonth(month.key)"
            >
              <span>{{ month.label }}</span>
              <strong>{{ month.count }}</strong>
            </button>
          </div>
        </section>
      </aside>

      <section
        class="month-detail"
        :aria-label="
          t('entries.archiveMonthCalendar', {
            month: monthLabel(selectedMonth),
          })
        "
      >
        <header class="month-detail-header">
          <div>
            <p>{{ t("entries.archiveMonth") }}</p>
            <h2>{{ monthLabel(selectedMonth) }}</h2>
          </div>
          <span>{{
            t("entries.entriesCount", { count: selectedMonthEntries.length })
          }}</span>
        </header>

        <div v-if="selectedMonthEntries.length === 0" class="month-empty">
          {{ t("entries.noEntriesInMonth") }}
        </div>
        <CalendarMonth
          v-else
          :entries="selectedMonthEntries"
          :selected-date="selectedDate"
          :visible-month="selectedMonth"
          entry-only
          @update:visible-month="selectMonth"
          @select="emit('selectDate', $event)"
        />
      </section>

      <footer v-if="hasMore" class="archive-footer">
        <button class="ui-action" type="button" @click="emit('loadMore')">
          {{ t("entries.loadMore") }}
        </button>
      </footer>
    </template>
  </div>
</template>

<style scoped>
.archive {
  display: grid;
  min-height: 0;
  grid-template-columns: minmax(210px, 260px) minmax(0, 1fr);
  gap: 18px;
}

.archive-footer {
  display: flex;
  grid-column: 1 / -1;
  justify-content: flex-end;
}

.archive-empty {
  display: flex;
  grid-column: 1 / -1;
  min-height: 180px;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--color-muted);
}

.month-index {
  min-height: 0;
  overflow: auto;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 12px;
}

.year-group + .year-group {
  margin-top: 16px;
}

.year-group h2 {
  margin: 0 0 8px;
  color: var(--color-muted);
  font-size: 12px;
  font-weight: 760;
  letter-spacing: 0;
}

.month-buttons {
  display: grid;
  gap: 6px;
}

.month-button {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-text-soft);
  padding: 0 8px;
  text-align: left;
}

.month-button:hover {
  border-color: var(--border-subtle);
  background: var(--surface-hover);
  color: var(--color-text);
}

.month-button.active {
  border-color: var(--border-active);
  background: var(--surface-active);
  color: var(--color-text);
}

.month-button.current {
  border-color: color-mix(in srgb, var(--color-border) 84%, #fff);
  color: var(--color-text);
}

.month-button.current strong {
  color: var(--color-text-soft);
}

.month-button span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.month-button strong {
  color: var(--color-muted);
  font-size: 12px;
}

.month-detail {
  min-width: 0;
  min-height: 0;
  overflow: auto;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--color-surface) 72%, transparent);
}

.month-detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 18px 0;
}

.month-detail-header p {
  margin: 0 0 2px;
  color: var(--color-muted);
  font-size: 12px;
}

.month-detail-header h2 {
  margin: 0;
  color: var(--color-text);
  font-size: 18px;
  font-weight: 760;
  letter-spacing: 0;
}

.month-detail-header span {
  flex: 0 0 auto;
  color: var(--color-muted);
  font-size: 12px;
}

.month-empty {
  color: var(--color-muted);
  padding: 28px 18px;
  font-size: 13px;
}

@media (max-width: 760px) {
  .archive {
    grid-template-columns: 1fr;
  }

  .month-index {
    max-height: 210px;
  }

  .month-button {
    min-height: 38px;
  }
}

@media (max-width: 480px) {
  .archive {
    gap: 12px;
  }

  .archive-empty {
    flex-direction: column;
    text-align: center;
  }

  .month-index {
    max-height: 180px;
  }

  .month-detail-header {
    align-items: flex-start;
    flex-direction: column;
    padding: 14px 14px 0;
  }
}
</style>
