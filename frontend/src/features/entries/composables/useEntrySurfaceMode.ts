import { ref } from "vue";
import { todayISO } from "../../../shared/utils/date";
import type { EntryDateLookup } from "../types";

export type EntrySurfaceMode = "reader" | "editor";

export type EntryOpenSource =
  | "app-start"
  | "today"
  | "adjacent"
  | "calendar"
  | "list"
  | "archive"
  | "search"
  | "memory"
  | "edit";

export function useEntrySurfaceMode() {
  const entrySurfaceMode = ref<EntrySurfaceMode>("editor");

  function resolveEntrySurfaceMode(
    targetDate: string,
    lookupResult: Pick<EntryDateLookup, "entry">,
    source: EntryOpenSource,
  ): EntrySurfaceMode {
    if (source === "edit") {
      return "editor";
    }

    if (source === "search" && lookupResult.entry !== null) {
      return "reader";
    }

    if (targetDate === todayISO()) {
      return "editor";
    }

    if (lookupResult.entry === null) {
      return "editor";
    }

    return "reader";
  }

  function setEntrySurfaceMode(
    targetDate: string,
    lookupResult: Pick<EntryDateLookup, "entry">,
    source: EntryOpenSource,
  ) {
    entrySurfaceMode.value = resolveEntrySurfaceMode(
      targetDate,
      lookupResult,
      source,
    );
  }

  return {
    entrySurfaceMode,
    resolveEntrySurfaceMode,
    setEntrySurfaceMode,
  };
}
