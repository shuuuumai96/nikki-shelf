<script setup lang="ts">
import { ChevronLeft, ChevronRight } from "lucide-vue-next";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import IconButton from "../../../shared/components/IconButton.vue";
import { monthKey } from "../../../shared/utils/date";
import { moodSpecs } from "../../entries/moods";
import type { Entry } from "../../entries/types";

const props = defineProps<{
  entries: Entry[];
  selectedDate: string;
  visibleMonth?: string;
  entryOnly?: boolean;
}>();

const emit = defineEmits<{
  select: [date: string];
  "update:visibleMonth": [month: string];
}>();

const visibleMonth = ref(monthDate(props.visibleMonth || props.selectedDate));
const { locale, tm, t } = useI18n();
const weekdayLabels = computed(() => tm("calendar.weekdays") as string[]);

const entriesByDate = computed(
  () => new Map(props.entries.map((entry) => [entry.entryDate, entry])),
);

const label = computed(() => {
  return new Intl.DateTimeFormat(locale.value, {
    year: "numeric",
    month: "long",
  }).format(visibleMonth.value);
});

const days = computed(() => {
  const year = visibleMonth.value.getFullYear();
  const month = visibleMonth.value.getMonth();
  const first = new Date(year, month, 1);
  const startOffset = (first.getDay() + 6) % 7;
  const start = new Date(year, month, 1 - startOffset);

  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(start);
    date.setDate(start.getDate() + index);
    const iso = toISO(date);
    return {
      iso,
      day: date.getDate(),
      inMonth: monthKey(date) === monthKey(visibleMonth.value),
      entry: entriesByDate.value.get(iso),
    };
  });
});

function changeMonth(amount: number) {
  visibleMonth.value = new Date(
    visibleMonth.value.getFullYear(),
    visibleMonth.value.getMonth() + amount,
    1,
  );
  emit("update:visibleMonth", monthKey(visibleMonth.value));
}

function selectDay(iso: string, hasEntry: boolean) {
  if (props.entryOnly && !hasEntry) {
    return;
  }
  emit("select", iso);
}

function toISO(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 10);
}

function monthDate(value: string): Date {
  return new Date(`${value.slice(0, 7)}-01T00:00:00`);
}

watch(
  () => props.visibleMonth,
  (value) => {
    if (value && monthKey(visibleMonth.value) !== value.slice(0, 7)) {
      visibleMonth.value = monthDate(value);
    }
  },
);
</script>

<template>
  <section class="calendar">
    <header>
      <IconButton
        :icon="ChevronLeft"
        :label="t('calendar.previousMonth')"
        @click="changeMonth(-1)"
      />
      <h2>{{ label }}</h2>
      <IconButton
        :icon="ChevronRight"
        :label="t('calendar.nextMonth')"
        @click="changeMonth(1)"
      />
    </header>

    <div class="weekdays">
      <span v-for="day in weekdayLabels" :key="day">{{ day }}</span>
    </div>

    <div class="grid">
      <button
        v-for="day in days"
        :key="day.iso"
        type="button"
        class="day"
        :class="{
          muted: !day.inMonth,
          selected: selectedDate === day.iso && (!entryOnly || day.entry),
          'has-entry': day.entry,
        }"
        :aria-current="
          selectedDate === day.iso && (!entryOnly || day.entry)
            ? 'date'
            : undefined
        "
        :disabled="entryOnly && !day.entry"
        @click="selectDay(day.iso, !!day.entry)"
      >
        <span>{{ day.day }}</span>
        <i
          v-if="day.entry"
          :style="{ background: moodSpecs[day.entry.mood].color }"
          aria-hidden="true"
        />
      </button>
    </div>
  </section>
</template>

<style scoped>
.calendar {
  width: min(760px, 100%);
  margin: 0 auto;
  padding: 42px;
}

header {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  margin-bottom: 22px;
}

h2 {
  min-width: 180px;
  margin: 0;
  color: var(--color-text);
  font-size: 22px;
  font-weight: 760;
  letter-spacing: 0;
  text-align: center;
}

.weekdays,
.grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
}

.weekdays {
  color: var(--color-muted);
  font-size: 12px;
  text-align: center;
}

.weekdays span {
  padding: 8px 0;
}

.grid {
  overflow: hidden;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}

.day {
  position: relative;
  display: grid;
  min-height: 82px;
  grid-template-rows: auto 1fr;
  border: 0;
  border-right: 1px solid color-mix(in srgb, var(--color-border) 86%, #fff);
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 86%, #fff);
  background: transparent;
  color: var(--color-text);
  padding: 10px;
  text-align: left;
}

.day:nth-child(7n) {
  border-right: 0;
}

.day:nth-last-child(-n + 7) {
  border-bottom: 0;
}

.day:hover {
  background: color-mix(in srgb, var(--color-wash) 46%, #fff);
}

.day:disabled {
  cursor: default;
}

.day:disabled:hover {
  background: transparent;
}

.day.muted {
  color: var(--color-placeholder);
  background: color-mix(in srgb, var(--color-surface-soft) 78%, #fff);
}

.day.muted:disabled:hover {
  background: color-mix(in srgb, var(--color-surface-soft) 78%, #fff);
}

.day.selected {
  background: var(--surface-active);
}

.day span {
  font-size: 13px;
}

.day i {
  align-self: end;
  width: 10px;
  height: 10px;
  border: 1px solid var(--border-on-glass);
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-dot-ring);
}

@media (max-width: 720px) {
  .calendar {
    padding: 26px 20px 48px;
  }

  .day {
    min-height: 56px;
    padding: 7px;
  }
}

@media (max-width: 480px) {
  .calendar {
    padding: 22px 14px calc(96px + env(safe-area-inset-bottom));
  }

  header {
    gap: 8px;
    margin-bottom: 16px;
  }

  h2 {
    min-width: 0;
    flex: 1;
    font-size: 18px;
  }

  .weekdays span {
    padding: 6px 0;
  }

  .day {
    min-height: 48px;
    padding: 5px;
  }

  .day span {
    font-size: 12px;
  }
}
</style>
