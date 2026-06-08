<script setup lang="ts">
import { Camera } from "lucide-vue-next";
import {
  computed,
  defineAsyncComponent,
  nextTick,
  onBeforeUnmount,
  reactive,
  ref,
  watch,
} from "vue";
import { useI18n } from "vue-i18n";
import {
  formatDateLabel,
  nextLocalDayISO,
  previousLocalDayISO,
  todayISO,
} from "../../../shared/utils/date";
import type { UploadImageRequest } from "../api";
import { useEntryImageUploads } from "../composables/useEntryImageUploads";
import type { Entry, EntryInput, MoodKey, SaveStatus } from "../types";
import EntryEditorChrome from "./EntryEditorChrome.vue";
import EntryEditorMessages from "./EntryEditorMessages.vue";
import EntryImageUploadGrid from "./EntryImageUploadGrid.vue";
import MarkdownEditorLoadingSurface from "./MarkdownEditorLoadingSurface.vue";
import MoodSelector from "./MoodSelector.vue";
import TagInput from "./TagInput.vue";

const MarkdownEditor = defineAsyncComponent({
  delay: 0,
  loader: async () => {
    await afterNextPaint();
    return import("./MarkdownEditor.vue");
  },
  loadingComponent: MarkdownEditorLoadingSurface,
});

const props = defineProps<{
  entry: Entry | null;
  date: string;
  tags: string[];
  saving: boolean;
  saveStatus: SaveStatus;
  saveError: string;
  navigationMessage: string;
  uploadImage: (payload: {
    input: EntryInput;
    file: File;
    onProgress: (progress: number) => void;
  }) => UploadImageRequest;
}>();

const emit = defineEmits<{
  autosave: [input: EntryInput];
  delete: [];
  deleteImage: [imageId: number];
  navigateDate: [date: string];
  reloadEntry: [];
}>();

type RecoveryCopyStatus = "idle" | "copied" | "failed";

type ConflictRecoveryDraft = EntryInput & {
  baseVersion: number;
  capturedAt: number;
};

const form = reactive<EntryInput>({
  entryDate: props.date || todayISO(),
  title: "",
  body: "",
  mood: "calm",
  tags: [],
});
const { locale, t } = useI18n();

const editorReady = ref(false);
const focusMode = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);
const autosaveTimer = ref<number | null>(null);
const syncingFromProps = ref(false);
const localDirty = ref(false);
const activeEntryKey = ref("");
const baselineInput = ref<EntryInput>(cloneEntryInput(form));
const conflictRecoveryDraft = ref<ConflictRecoveryDraft | null>(null);
const recoveryCopyStatus = ref<RecoveryCopyStatus>("idle");
const recoveryDraftRestored = ref(false);
const draftStorageKey = computed(() => `nikki:draft:${activeEntryKey.value}`);
const {
  clearUploadImages,
  hasSupportedImageFiles,
  imageSlotsLeft,
  persistedImages,
  queueFiles,
  removePersistedImage,
  removeUpload,
  retryUpload,
  uploadImages,
} = useEntryImageUploads({
  deleteImage: (imageId) => emit("deleteImage", imageId),
  entry: computed(() => props.entry),
  form,
  t: (key) => t(key),
  uploadImage: (payload) => props.uploadImage(payload),
});

const heading = computed(() => formatDateLabel(form.entryDate, locale.value));
const navigationDisabled = computed(
  () => props.saving || props.saveStatus === "saving",
);
const showRecoveryPanel = computed(
  () => props.saveStatus === "conflict" || conflictRecoveryDraft.value !== null,
);
const activeRecoveryDraft = computed<EntryInput | null>(
  () =>
    conflictRecoveryDraft.value ||
    (props.saveStatus === "conflict" ? cloneEntryInput(form) : null),
);
const recoveryDraftText = computed(() => {
  if (!activeRecoveryDraft.value) {
    return "";
  }
  return formatRecoveryDraft(activeRecoveryDraft.value);
});
const recoveryHelpText = computed(() =>
  props.saveStatus === "conflict"
    ? t("entries.recoveryConflictHelp")
    : t("entries.recoveryServerLoadedHelp"),
);
const recoveryCopyText = computed(() => {
  if (recoveryCopyStatus.value === "copied") {
    return t("entries.recoveryCopied");
  }
  if (recoveryCopyStatus.value === "failed") {
    return t("entries.recoveryCopyFailed");
  }
  return t("entries.copyRecoveryDraft");
});
const canAutosave = computed(() =>
  Boolean(
    props.entry || form.title.trim() || form.body.trim() || form.tags.length,
  ),
);
const displaySaveStatus = computed<SaveStatus>(() => {
  if (
    localDirty.value &&
    !["saving", "failed", "conflict"].includes(props.saveStatus)
  ) {
    return "dirty";
  }
  return props.saveStatus;
});
const exportURL = computed(() =>
  props.entry ? `/api/export/entries/${props.entry.id}/markdown` : "",
);
const exportDisabled = computed(() =>
  Boolean(
    !props.entry ||
    props.saving ||
    ["dirty", "saving", "failed", "conflict"].includes(displaySaveStatus.value),
  ),
);
const statusText = computed(() => {
  const labels: Partial<Record<SaveStatus, string>> = {
    dirty: t("entries.saveDirty"),
    saving: t("entries.saveSaving"),
    saved: t("entries.saveSaved"),
    failed: t("entries.saveFailed"),
    conflict: t("entries.saveConflict"),
  };
  return labels[displaySaveStatus.value] ?? "";
});

watch(
  () => [props.entry, props.date] as const,
  ([entry, date]) => {
    const previousKey = activeEntryKey.value;
    const nextKey = entry ? `entry:${entry.id}` : `date:${date || todayISO()}`;
    // A new-date draft becomes the same logical entry after the first autosave
    // returns an id; keep local state through that promotion.
    const draftPromoted = Boolean(
      entry && previousKey === `date:${entry.entryDate}`,
    );
    const changedEntry =
      previousKey !== "" && previousKey !== nextKey && !draftPromoted;
    activeEntryKey.value = nextKey;

    clearAutosaveTimer();
    syncingFromProps.value = true;
    // Local drafts preserve failed/offline edits, but readLocalDraft rejects
    // them once the server version has moved beyond their base version.
    const savedDraft = readLocalDraft(nextKey, entry?.version ?? 0);
    const serverInput = entryToInput(entry, date);
    if (savedDraft) {
      applyInput(savedDraft);
      baselineInput.value = cloneEntryInput(serverInput);
      localDirty.value = !isSameEntryInput(savedDraft, serverInput);
    } else if (localDirty.value && !changedEntry) {
      form.entryDate = entry?.entryDate || form.entryDate || date || todayISO();
      baselineInput.value = cloneEntryInput(form);
      localDirty.value = false;
    } else {
      applyInput(serverInput);
      baselineInput.value = cloneEntryInput(serverInput);
      localDirty.value = false;
    }
    if (changedEntry) {
      clearUploadImages();
      clearConflictRecoveryDraft();
    }
    void nextTick(() => {
      syncingFromProps.value = false;
    });
  },
  { immediate: true },
);

watch(
  () => props.saveStatus,
  (status) => {
    if (status === "conflict") {
      captureConflictRecoveryDraft();
      return;
    }

    if (status === "saved") {
      baselineInput.value = cloneEntryInput(form);
      localDirty.value = false;
      clearLocalDraft();
      if (recoveryDraftRestored.value) {
        clearConflictRecoveryDraft();
      }
    }
  },
);

watch(
  form,
  () => {
    if (syncingFromProps.value) {
      return;
    }

    scheduleAutosave();
  },
  { deep: true },
);

onBeforeUnmount(() => {
  clearAutosaveTimer();
  clearUploadImages();
});

defineExpose({
  flushPendingAutosave,
  toggleFocusMode,
});

function flushPendingAutosave() {
  if (!canAutosave.value || !localDirty.value || !hasFormChanged()) {
    localDirty.value = false;
    clearAutosaveTimer();
    return false;
  }

  const hadPendingAutosave = autosaveTimer.value !== null;
  clearAutosaveTimer();

  if (hadPendingAutosave || localDirty.value) {
    emit("autosave", { ...form, tags: [...form.tags] });
    return true;
  }

  return false;
}

function retryAutosave() {
  if (!canAutosave.value || !hasFormChanged()) {
    return;
  }
  clearAutosaveTimer();
  emit("autosave", { ...form, tags: [...form.tags] });
}

function navigatePreviousDay() {
  emit("navigateDate", previousLocalDayISO(form.entryDate));
}

function navigateNextDay() {
  emit("navigateDate", nextLocalDayISO(form.entryDate));
}

function reloadServerVersion() {
  clearAutosaveTimer();
  captureConflictRecoveryDraft();
  localDirty.value = false;
  emit("reloadEntry");
}

function toggleFocusMode() {
  focusMode.value = !focusMode.value;
}

function exitFocusMode() {
  focusMode.value = false;
}

async function copyRecoveryDraft() {
  const text = recoveryDraftText.value;
  if (!text) {
    return;
  }

  try {
    await navigator.clipboard.writeText(text);
    recoveryCopyStatus.value = "copied";
  } catch {
    recoveryCopyStatus.value = "failed";
  }
}

function restoreRecoveryDraft() {
  if (!conflictRecoveryDraft.value) {
    return;
  }

  const draft = cloneEntryInput(conflictRecoveryDraft.value);
  clearAutosaveTimer();
  syncingFromProps.value = true;
  applyInput(draft);
  baselineInput.value = cloneEntryInput(entryToInput(props.entry, props.date));
  localDirty.value = !isSameEntryInput(draft, baselineInput.value);
  recoveryDraftRestored.value = true;
  recoveryCopyStatus.value = "idle";

  void nextTick(() => {
    syncingFromProps.value = false;
    if (localDirty.value) {
      scheduleAutosave();
    }
  });
}

function dismissRecoveryDraft() {
  clearConflictRecoveryDraft();
}

function scheduleAutosave() {
  clearAutosaveTimer();

  if (!canAutosave.value) {
    return;
  }

  // baselineInput is the last server-accepted form. localDirty is only the
  // observable delta from that baseline, which keeps navigation checks simple.
  if (!hasFormChanged()) {
    localDirty.value = false;
    clearLocalDraft();
    return;
  }

  localDirty.value = true;
  writeLocalDraft();
  if (props.saveStatus === "conflict") {
    captureConflictRecoveryDraft();
    return;
  }

  autosaveTimer.value = window.setTimeout(() => {
    autosaveTimer.value = null;
    emit("autosave", { ...form, tags: [...form.tags] });
  }, 1000);
}

function clearAutosaveTimer() {
  if (autosaveTimer.value === null) {
    return;
  }

  window.clearTimeout(autosaveTimer.value);
  autosaveTimer.value = null;
}

function pickFiles() {
  fileInput.value?.click();
}

function onFilesSelected(event: Event) {
  const target = event.target as HTMLInputElement;
  queueFiles(Array.from(target.files || []));
  target.value = "";
}

function onDrop(event: DragEvent) {
  queueFiles(Array.from(event.dataTransfer?.files || []));
}

function onEditorImageDrop(payload: { files: File[] }) {
  queueFiles(payload.files);
}

function onPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || []);
  if (!hasSupportedImageFiles(files)) {
    return;
  }

  event.preventDefault();
  queueFiles(files);
}

function writeLocalDraft() {
  if (!activeEntryKey.value) {
    return;
  }

  window.localStorage.setItem(
    draftStorageKey.value,
    JSON.stringify({
      ...form,
      tags: [...form.tags],
      baseVersion: props.entry?.version ?? 0,
      savedAt: Date.now(),
    }),
  );
}

function captureConflictRecoveryDraft() {
  conflictRecoveryDraft.value = {
    ...cloneEntryInput(form),
    baseVersion: props.entry?.version ?? 0,
    capturedAt: Date.now(),
  };
  recoveryDraftRestored.value = false;
  recoveryCopyStatus.value = "idle";
}

function clearConflictRecoveryDraft() {
  conflictRecoveryDraft.value = null;
  recoveryCopyStatus.value = "idle";
  recoveryDraftRestored.value = false;
}

function readLocalDraft(
  key: string,
  serverVersion: number,
): (EntryInput & { savedAt: number; baseVersion?: number }) | null {
  try {
    const raw = window.localStorage.getItem(`nikki:draft:${key}`);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as EntryInput & {
      savedAt?: number;
      baseVersion?: number;
    };
    if (
      !parsed.entryDate ||
      typeof parsed.body !== "string" ||
      !Array.isArray(parsed.tags)
    ) {
      return null;
    }
    const baseVersion =
      typeof parsed.baseVersion === "number" ? parsed.baseVersion : 0;
    // Refuse stale drafts instead of merging blindly; newer server text should
    // not be overwritten by a tab with an older base version.
    if (
      serverVersion > 0 &&
      (baseVersion === 0 || serverVersion > baseVersion)
    ) {
      return null;
    }
    return { ...parsed, savedAt: parsed.savedAt || 0 };
  } catch {
    return null;
  }
}

function clearLocalDraft() {
  if (!activeEntryKey.value) {
    return;
  }
  window.localStorage.removeItem(draftStorageKey.value);
}

function entryToInput(entry: Entry | null, date: string): EntryInput {
  return {
    entryDate: entry?.entryDate || date || todayISO(),
    title: entry?.title || "",
    body: entry?.body || "",
    mood: (entry?.mood || "calm") as MoodKey,
    tags: [...(entry?.tags || [])],
  };
}

function applyInput(input: EntryInput) {
  form.entryDate = input.entryDate;
  form.title = input.title;
  form.body = input.body;
  form.mood = input.mood;
  form.tags = [...input.tags];
}

function hasFormChanged() {
  return !isSameEntryInput(form, baselineInput.value);
}

function isSameEntryInput(left: EntryInput, right: EntryInput) {
  return (
    left.entryDate === right.entryDate &&
    left.title === right.title &&
    left.body === right.body &&
    left.mood === right.mood &&
    sameTags(left.tags, right.tags)
  );
}

function sameTags(left: string[], right: string[]) {
  return (
    left.length === right.length &&
    left.every((tag, index) => tag === right[index])
  );
}

function cloneEntryInput(input: EntryInput): EntryInput {
  return { ...input, tags: [...input.tags] };
}

function formatRecoveryDraft(input: EntryInput) {
  const title = input.title.trim() || t("entries.recoveryEmpty");
  const body = input.body.trim() || t("entries.recoveryEmpty");
  const tags = input.tags.length
    ? input.tags.join(", ")
    : t("entries.recoveryEmpty");

  return [
    `${t("entries.recoveryDate")}: ${input.entryDate}`,
    `${t("entries.recoveryTitle")}: ${title}`,
    `${t("entries.recoveryMood")}: ${moodLabel(input.mood)}`,
    `${t("entries.recoveryTags")}: ${tags}`,
    "",
    `${t("entries.recoveryBody")}:`,
    body,
  ].join("\n");
}

function moodLabel(mood: MoodKey) {
  const labels: Record<MoodKey, string> = {
    happy: t("entries.moodHappy"),
    calm: t("entries.moodCalm"),
    tired: t("entries.moodTired"),
    sad: t("entries.moodSad"),
    excited: t("entries.moodExcited"),
  };

  return labels[mood];
}

function afterNextPaint() {
  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => resolve());
    });
  });
}
</script>

<template>
  <section
    class="editor entry-surface ui-page"
    :class="{ 'editor-ready': editorReady, 'focus-mode': focusMode }"
    @paste="onPaste"
  >
    <EntryEditorChrome
      :can-autosave="canAutosave"
      :display-save-status="displaySaveStatus"
      :entry="entry"
      :export-disabled="exportDisabled"
      :export-url="exportURL"
      :focus-mode="focusMode"
      :heading="heading"
      :image-slots-left="imageSlotsLeft"
      :navigation-disabled="navigationDisabled"
      :save-status="saveStatus"
      :saving="saving"
      :status-text="statusText"
      @delete-entry="emit('delete')"
      @exit-focus-mode="exitFocusMode"
      @next-day="navigateNextDay"
      @pick-files="pickFiles"
      @previous-day="navigatePreviousDay"
      @retry-autosave="retryAutosave"
      @toggle-focus-mode="toggleFocusMode"
    />

    <EntryEditorMessages
      :navigation-message="navigationMessage"
      :recovery-copy-text="recoveryCopyText"
      :recovery-draft-text="recoveryDraftText"
      :recovery-help-text="recoveryHelpText"
      :save-error="saveError"
      :save-status="saveStatus"
      :show-recovery-panel="showRecoveryPanel"
      @copy-recovery-draft="copyRecoveryDraft"
      @dismiss-recovery-draft="dismissRecoveryDraft"
      @reload-server-version="reloadServerVersion"
      @restore-recovery-draft="restoreRecoveryDraft"
    />

    <input
      v-model="form.title"
      class="title-input entry-surface__title"
      :placeholder="t('entries.titlePlaceholder')"
      maxlength="80"
    />

    <div class="meta-row entry-surface__meta">
      <MoodSelector v-model="form.mood" />
      <TagInput v-model="form.tags" :suggestions="tags" />
    </div>

    <div class="entry-surface__body">
      <MarkdownEditor
        v-model="form.body"
        @image-drop="onEditorImageDrop"
        @ready="editorReady = true"
      />
    </div>

    <div
      class="image-bar entry-surface__attachments"
      @dragover.prevent
      @drop.prevent="onDrop"
    >
      <button
        class="ui-action"
        type="button"
        :disabled="imageSlotsLeft === 0"
        @click="pickFiles"
      >
        <Camera :size="16" stroke-width="1.8" />
        <span>{{ t("images.photo") }}</span>
      </button>
    </div>
    <label class="sr-only" for="entry-image-input">{{
      t("images.addPhotos")
    }}</label>
    <input
      id="entry-image-input"
      ref="fileInput"
      class="sr-only"
      type="file"
      accept="image/jpeg,image/png,image/gif,image/webp"
      multiple
      @change="onFilesSelected"
    />

    <EntryImageUploadGrid
      v-if="persistedImages.length || uploadImages.length"
      :entry-date="form.entryDate"
      :focus-mode="focusMode"
      :persisted-images="persistedImages"
      :upload-images="uploadImages"
      @delete-persisted-image="removePersistedImage"
      @remove-upload="removeUpload"
      @retry-upload="retryUpload"
    />
  </section>
</template>

<style scoped>
.entry-surface {
  --page-max: 760px;
  min-height: 100%;
  padding-top: 38px;
}

.entry-surface.focus-mode {
  --page-max: 820px;
  position: fixed;
  inset: 0;
  z-index: 30;
  width: 100%;
  overflow: auto;
  background: var(--color-bg);
  margin: 0;
  padding-right: max(var(--page-x), calc((100% - var(--page-max)) / 2));
  padding-left: max(var(--page-x), calc((100% - var(--page-max)) / 2));
  padding-top: 34px;
  padding-bottom: 58px;
}

.editor:not(.editor-ready) {
  visibility: hidden;
}

.entry-surface__title {
  margin: 0 0 16px;
  min-height: 39px;
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

.entry-surface__body {
  margin: 0;
  font-size: 16px;
  line-height: 1.85;
}

.entry-surface__body :deep(.markdown-editor) {
  margin-top: 0;
}

.entry-surface__body :deep(.document-editor-surface) {
  border-top: 0;
  padding-top: 0;
}

.entry-surface__attachments {
  margin-top: 24px;
}

.focus-mode .entry-surface__meta,
.focus-mode .entry-surface__attachments {
  display: none;
}

.focus-mode .entry-surface__title {
  margin-top: 22px;
  margin-bottom: 18px;
}

.focus-mode .entry-surface__body :deep(.document-editor-wrap) {
  min-height: 58vh;
}

.focus-mode .entry-surface__body :deep(.document-editor-surface) {
  min-height: 58vh;
  font-size: 18px;
  line-height: 1.82;
  padding-bottom: 68px;
}

.title-input {
  width: 100%;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--color-text);
  font-size: 28px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.32;
  padding: 0 0 2px;
}

.title-input::placeholder {
  color: var(--color-placeholder);
}

.meta-row {
  margin: 0 0 24px;
}

.image-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 24px;
  border: 1px dashed color-mix(in srgb, var(--color-line) 72%, transparent);
  border-radius: var(--radius-md);
  padding: 12px;
  background: var(--color-surface);
}

.image-bar .ui-action {
  min-height: 38px;
  gap: 8px;
  padding: 0 14px;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .entry-surface {
    padding-top: 26px;
  }
}

@media (max-width: 480px) {
  .entry-surface {
    padding-top: 18px;
    padding-bottom: calc(112px + env(safe-area-inset-bottom));
  }

  .title-input {
    font-size: 21px;
  }

  .entry-surface__title {
    margin-bottom: 12px;
    min-height: 34px;
  }

  .meta-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 10px;
    margin-bottom: 16px;
    min-height: 0;
    width: 100%;
  }

  .meta-row :deep(.moods),
  .meta-row :deep(.tag-input) {
    width: 100%;
  }

  .image-bar .ui-action {
    min-height: 44px;
    width: 100%;
  }

  .image-bar {
    display: none;
  }
}
</style>
