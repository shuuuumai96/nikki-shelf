import { ref, type Ref } from "vue";
import type { SaveStatus } from "../types";

export type DiaryEditorHandle = {
  flushPendingAutosave: () => boolean;
  toggleFocusMode: () => void;
};

type EntryStoreLike = {
  saving: boolean;
  saveStatus: SaveStatus;
  waitForAutosaveIdle: () => Promise<void>;
};

type Translate = (
  key: string,
  params?: Record<string, string | number>,
) => string;

export function useDiaryNavigation(options: {
  store: EntryStoreLike;
  diaryEditor: Ref<DiaryEditorHandle | null>;
  t: Translate;
}) {
  const navigationMessage = ref("");

  async function prepareDiaryNavigation(source: string) {
    const { diaryEditor, store, t } = options;

    // Failed and conflicting saves are blockers because leaving the date can
    // hide unsaved local text behind a different entry.
    if (hasBlockingSaveStatus(store.saveStatus)) {
      navigationMessage.value = blockedNavigationMessage(store.saveStatus, t);
      return false;
    }

    if (store.saving) {
      navigationMessage.value = t("entries.navigationSaving");
      return false;
    }

    // Date changes must flush the editor before loading another entry; otherwise
    // the store can accept a late autosave for the date the user just left.
    diaryEditor.value?.flushPendingAutosave();
    await store.waitForAutosaveIdle();
    if (hasBlockingSaveStatus(store.saveStatus)) {
      navigationMessage.value = blockedNavigationMessage(store.saveStatus, t);
      return false;
    }

    if (store.saveStatus === "saving") {
      navigationMessage.value = t("entries.navigationRetryAfterSave", {
        source,
      });
      return false;
    }

    navigationMessage.value = "";
    return true;
  }

  return {
    navigationMessage,
    prepareDiaryNavigation,
  };
}

function hasBlockingSaveStatus(status: SaveStatus) {
  return status === "failed" || status === "conflict";
}

function blockedNavigationMessage(status: SaveStatus, t: Translate) {
  if (status === "conflict") {
    return t("entries.navigationConflictBlocked");
  }

  return t("entries.navigationFailedBlocked");
}
