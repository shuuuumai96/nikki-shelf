import { computed, reactive } from "vue";

export function useEntryFilters() {
  const filters = reactive({
    query: "",
    mood: "",
    tag: "",
  });

  const hasEntryFilters = computed(() =>
    Boolean(filters.query || filters.mood || filters.tag),
  );

  function clearEntryFilters() {
    filters.query = "";
    filters.mood = "";
    filters.tag = "";
  }

  return {
    clearEntryFilters,
    filters,
    hasEntryFilters,
  };
}
