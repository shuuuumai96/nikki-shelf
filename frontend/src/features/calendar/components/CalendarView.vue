<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { Entry } from "../../entries/types";
import CalendarMonth from "./CalendarMonth.vue";

defineProps<{
  entries: Entry[];
  hasMore: boolean;
  loading: boolean;
  selectedDate: string;
}>();

const emit = defineEmits<{
  loadMore: [];
  selectDate: [date: string];
}>();

const { t } = useI18n();
</script>

<template>
  <div class="calendar-view">
    <CalendarMonth
      :entries="entries"
      :selected-date="selectedDate"
      @select="emit('selectDate', $event)"
    />
    <div v-if="hasMore" class="calendar-pagination">
      <button
        class="ui-action"
        type="button"
        :disabled="loading"
        @click="emit('loadMore')"
      >
        {{ t("entries.loadMore") }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.calendar-view {
  min-height: 100%;
}

.calendar-pagination {
  display: flex;
  justify-content: center;
  width: min(880px, calc(100% - 32px));
  margin: 0 auto 32px;
}
</style>
