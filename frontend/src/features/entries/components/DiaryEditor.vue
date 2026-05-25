<script setup lang="ts">
import {
  Camera,
  Ellipsis,
  Maximize2,
  Minimize2,
  RotateCcw,
  X,
} from "lucide-vue-next";
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { useI18n } from "vue-i18n";
import IconButton from "../../../shared/components/IconButton.vue";
import {
  formatDateLabel,
  nextLocalDayISO,
  previousLocalDayISO,
  todayISO,
} from "../../../shared/utils/date";
import {
  IMAGE_UPLOAD_MAX_BYTES,
  SUPPORTED_IMAGE_TYPES,
  type UploadImageRequest,
} from "../api";
import type {
  Entry,
  EntryImage,
  EntryInput,
  MoodKey,
  SaveStatus,
} from "../types";
import EntryImageAttachment from "./EntryImageAttachment.vue";
import MarkdownEditor from "./MarkdownEditor.vue";
import MoodSelector from "./MoodSelector.vue";
import TagInput from "./TagInput.vue";

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

type UploadStatus = "preparing" | "uploading" | "failed";
type EntryChromePanel = "actions" | "shortcuts" | null;

type UploadImageItem = {
  id: string;
  file: File;
  signature: string;
  previewUrl: string;
  objectUrl: string;
  status: UploadStatus;
  progress: number;
  error: string;
  persisted: EntryImage | null;
  started: boolean;
  active: boolean;
  upload: UploadImageRequest | null;
  insertionPosition: number | null;
};

const form = reactive<EntryInput>({
  entryDate: props.date || todayISO(),
  title: "",
  body: "",
  mood: "calm",
  tags: [],
});
const { locale, t } = useI18n();

const uploadImages = ref<UploadImageItem[]>([]);
const editorReady = ref(false);
const focusMode = ref(false);
const openEntryChromePanel = ref<EntryChromePanel>(null);
const entryChromeActions = ref<HTMLElement | null>(null);
const pendingFileInsertionPosition = ref<number | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
const markdownEditor = ref<{
  insertImage: (image: { url: string; fileName?: string }) => void;
  insertImageAtPosition: (
    position: number,
    image: { url: string; fileName?: string },
  ) => number | null;
  currentPosition: () => number | null;
} | null>(null);
const autosaveTimer = ref<number | null>(null);
const syncingFromProps = ref(false);
const localDirty = ref(false);
const activeEntryKey = ref("");
const baselineInput = ref<EntryInput>(cloneEntryInput(form));
const draftStorageKey = computed(() => `nikki:draft:${activeEntryKey.value}`);

const heading = computed(() => formatDateLabel(form.entryDate, locale.value));
const navigationDisabled = computed(
  () => props.saving || props.saveStatus === "saving",
);
const localTextForMerge = computed(() => {
  const parts = [form.title.trim(), form.body.trim()].filter(Boolean);
  return parts.join("\n\n");
});
const imageSlotsLeft = computed(() =>
  Math.max(
    0,
    3 - (props.entry?.images.length || 0) - uploadImages.value.length,
  ),
);
const persistedImages = computed(() => props.entry?.images || []);
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
    const draftPromoted = Boolean(
      entry && previousKey === `date:${entry.entryDate}`,
    );
    const changedEntry =
      previousKey !== "" && previousKey !== nextKey && !draftPromoted;
    activeEntryKey.value = nextKey;

    clearAutosaveTimer();
    syncingFromProps.value = true;
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
    if (status === "saved") {
      baselineInput.value = cloneEntryInput(form);
      localDirty.value = false;
      clearLocalDraft();
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
  window.removeEventListener("keydown", onKeydown);
  document.removeEventListener("pointerdown", onDocumentPointerDown);
  clearAutosaveTimer();
  clearUploadImages();
});

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
  document.addEventListener("pointerdown", onDocumentPointerDown);
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
  emit("reloadEntry");
}

function toggleFocusMode() {
  focusMode.value = !focusMode.value;
}

function exitFocusMode() {
  focusMode.value = false;
}

function openActionsMenu() {
  openEntryChromePanel.value = "actions";
}

function openShortcutsPanel() {
  openEntryChromePanel.value = "shortcuts";
}

function closeEntryChromePanel() {
  openEntryChromePanel.value = null;
}

function toggleEntryActions() {
  if (openEntryChromePanel.value === "actions") {
    closeEntryChromePanel();
    return;
  }
  openActionsMenu();
}

function showShortcutsFromMenu() {
  openShortcutsPanel();
}

function deleteEntryFromMenu() {
  closeEntryChromePanel();
  emit("delete");
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && openEntryChromePanel.value !== null) {
    event.preventDefault();
    closeEntryChromePanel();
    return;
  }

  if (!focusMode.value || event.key !== "Escape") {
    return;
  }

  event.preventDefault();
  exitFocusMode();
}

function onDocumentPointerDown(event: PointerEvent) {
  if (openEntryChromePanel.value === null) {
    return;
  }
  const target = event.target;
  if (target instanceof Node && entryChromeActions.value?.contains(target)) {
    return;
  }
  closeEntryChromePanel();
}

async function copyLocalText() {
  const text = localTextForMerge.value;
  if (!text) {
    return;
  }
  await navigator.clipboard?.writeText(text);
}

function scheduleAutosave() {
  clearAutosaveTimer();

  if (!canAutosave.value) {
    return;
  }

  if (!hasFormChanged()) {
    localDirty.value = false;
    clearLocalDraft();
    return;
  }

  localDirty.value = true;
  writeLocalDraft();
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
  pendingFileInsertionPosition.value = currentEditorPosition();
  fileInput.value?.click();
}

function onFilesSelected(event: Event) {
  const target = event.target as HTMLInputElement;
  queueFiles(
    Array.from(target.files || []),
    pendingFileInsertionPosition.value,
  );
  pendingFileInsertionPosition.value = null;
  target.value = "";
}

function onDrop(event: DragEvent) {
  queueFiles(
    Array.from(event.dataTransfer?.files || []),
    currentEditorPosition(),
  );
}

function onEditorImageDrop(payload: { files: File[]; position: number }) {
  queueFiles(payload.files, payload.position);
}

function onPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || []).filter(
    isSupportedImage,
  );
  if (!files.length) {
    return;
  }

  event.preventDefault();
  queueFiles(files, currentEditorPosition());
}

function queueFiles(files: File[], insertionPosition: number | null = null) {
  if (isMobileViewport()) {
    return;
  }

  const available = imageSlotsLeft.value;
  if (available === 0) {
    return;
  }

  const signatures = new Set(
    uploadImages.value.map((image) => image.signature),
  );
  const selected = files
    .filter(isSupportedImage)
    .filter((file) => {
      const signature = fileSignature(file);
      if (signatures.has(signature)) {
        return false;
      }
      signatures.add(signature);
      return true;
    })
    .slice(0, available);

  const items = selected.map((file) =>
    createUploadItem(file, insertionPosition),
  );
  uploadImages.value = [...uploadImages.value, ...items];
  const uploadableItems: UploadImageItem[] = [];
  items.forEach((item) => {
    if (item.file.size > IMAGE_UPLOAD_MAX_BYTES) {
      item.status = "failed";
      item.error = t("images.maxSize");
      return;
    }
    uploadableItems.push(item);
  });

  if (insertionPosition === null) {
    uploadableItems.forEach((item) => {
      void startUpload(item);
    });
    return;
  }

  void startPositionedUploads(uploadableItems, insertionPosition);
}

function createUploadItem(
  file: File,
  insertionPosition: number | null,
): UploadImageItem {
  const objectUrl = URL.createObjectURL(file);
  return {
    id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
    file,
    signature: fileSignature(file),
    previewUrl: objectUrl,
    objectUrl,
    status: "preparing",
    progress: 0,
    error: "",
    persisted: null,
    started: false,
    active: true,
    upload: null,
    insertionPosition,
  };
}

async function startPositionedUploads(
  items: UploadImageItem[],
  insertionPosition: number,
) {
  let nextPosition = insertionPosition;
  for (const item of items) {
    item.insertionPosition = nextPosition;
    await startUpload(item);
    if (typeof item.insertionPosition === "number") {
      nextPosition = item.insertionPosition;
    }
  }
}

async function startUpload(item: UploadImageItem) {
  if (item.started) {
    return;
  }

  item.started = true;
  item.active = true;
  item.status = "uploading";
  item.progress = 0;
  item.error = "";

  try {
    const upload = props.uploadImage({
      input: { ...form, tags: [...form.tags] },
      file: item.file,
      onProgress: (progress) => {
        if (!item.active) {
          return;
        }
        item.progress = progress;
      },
    });
    item.upload = upload;
    const image = await upload.promise;

    if (
      !item.active ||
      !uploadImages.value.some((current) => current.id === item.id)
    ) {
      return;
    }

    item.persisted = image;
    item.progress = 100;
    if (item.insertionPosition === null) {
      markdownEditor.value?.insertImage({
        url: image.url,
        fileName: image.fileName,
      });
    } else {
      item.insertionPosition =
        markdownEditor.value?.insertImageAtPosition(item.insertionPosition, {
          url: image.url,
          fileName: image.fileName,
        }) ?? item.insertionPosition;
    }
    removeCompletedUpload(item);
  } catch (error) {
    if (!item.active) {
      return;
    }

    item.status = "failed";
    item.error = errorMessage(error);
    item.started = false;
  } finally {
    if (item.upload) {
      item.upload = null;
    }
  }
}

function retryUpload(item: UploadImageItem) {
  if (item.status !== "failed") {
    return;
  }

  void startUpload(item);
}

function removeUpload(item: UploadImageItem) {
  item.active = false;
  item.upload?.abort();
  revokeObjectUrl(item);
  uploadImages.value = uploadImages.value.filter(
    (image) => image.id !== item.id,
  );
  if (item.persisted) {
    emit("deleteImage", item.persisted.id);
  }
}

function removePersistedImage(image: EntryImage) {
  emit("deleteImage", image.id);
}

function removeCompletedUpload(item: UploadImageItem) {
  item.active = false;
  revokeObjectUrl(item);
  uploadImages.value = uploadImages.value.filter(
    (image) => image.id !== item.id,
  );
}

function clearUploadImages() {
  uploadImages.value.forEach((image) => {
    image.active = false;
    image.upload?.abort();
    revokeObjectUrl(image);
  });
  uploadImages.value = [];
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

function isMobileViewport() {
  return window.matchMedia("(max-width: 480px)").matches;
}

function currentEditorPosition() {
  return markdownEditor.value?.currentPosition() ?? null;
}

function isSupportedImage(file: File) {
  return SUPPORTED_IMAGE_TYPES.includes(file.type);
}

function fileSignature(file: File) {
  return `${file.name}:${file.size}:${file.lastModified}`;
}

function revokeObjectUrl(item: UploadImageItem) {
  if (!item.objectUrl) {
    return;
  }

  URL.revokeObjectURL(item.objectUrl);
  item.objectUrl = "";
}

function errorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }
  return t("images.fallbackUploadFailed");
}
</script>

<template>
  <section
    class="editor entry-surface ui-page"
    :class="{ 'editor-ready': editorReady, 'focus-mode': focusMode }"
    @paste="onPaste"
  >
    <header class="entry-surface__chrome">
      <div
        class="entry-surface__date-nav"
        :aria-label="t('entries.dateNavigation')"
      >
        <button
          type="button"
          class="entry-surface__chrome-action"
          :disabled="navigationDisabled"
          @click="navigatePreviousDay"
        >
          <span aria-hidden="true">‹</span>
          <span>{{ t("common.previousDay") }}</span>
        </button>
        <h1 class="ui-heading entry-surface__date">
          <span>{{ heading }}</span>
        </h1>
        <button
          type="button"
          class="entry-surface__chrome-action"
          :disabled="navigationDisabled"
          @click="navigateNextDay"
        >
          <span>{{ t("common.nextDay") }}</span>
          <span aria-hidden="true">›</span>
        </button>
      </div>

      <div class="actions entry-surface__chrome-side">
        <button
          v-if="focusMode"
          type="button"
          class="focus-exit"
          @click="exitFocusMode"
        >
          <Minimize2 :size="15" stroke-width="1.8" />
          <span>{{ t("common.exitFocusMode") }}</span>
        </button>
        <IconButton
          v-else
          :icon="Maximize2"
          :label="t('common.focusMode')"
          @click="toggleFocusMode"
        />
        <div v-if="entry" ref="entryChromeActions" class="entry-actions">
          <button
            type="button"
            class="entry-actions__trigger"
            :aria-label="t('common.entryActions')"
            :title="t('common.entryActions')"
            :aria-expanded="openEntryChromePanel === 'actions'"
            aria-controls="editor-entry-actions"
            @click="toggleEntryActions"
            @keydown.esc="closeEntryChromePanel"
          >
            <Ellipsis :size="17" stroke-width="1.8" />
          </button>
          <div
            v-if="openEntryChromePanel === 'actions'"
            id="editor-entry-actions"
            class="entry-actions__menu"
            role="menu"
            @keydown.esc="closeEntryChromePanel"
          >
            <a
              v-if="!exportDisabled"
              class="entry-actions__item"
              role="menuitem"
              :href="exportURL"
              @click="closeEntryChromePanel"
            >
              {{ t("common.exportMarkdown") }}
            </a>
            <button
              v-else
              type="button"
              class="entry-actions__item"
              role="menuitem"
              disabled
              :title="t('common.saveBeforeExport')"
            >
              {{ t("common.exportMarkdown") }}
            </button>
            <button
              type="button"
              class="entry-actions__item"
              role="menuitem"
              @click="showShortcutsFromMenu"
            >
              {{ t("common.keyboardShortcuts") }}
            </button>
            <div class="entry-actions__separator" role="separator"></div>
            <button
              type="button"
              class="entry-actions__item entry-actions__item--danger"
              role="menuitem"
              :disabled="saving"
              @click="deleteEntryFromMenu"
            >
              {{ t("common.delete") }}
            </button>
          </div>
          <div
            v-if="openEntryChromePanel === 'shortcuts'"
            id="writing-shortcuts-help"
            class="entry-shortcuts__panel"
            role="region"
            :aria-label="t('common.keyboardShortcuts')"
            @keydown.esc="closeEntryChromePanel"
          >
            <p>{{ t("common.keyboardShortcuts") }}</p>
            <dl>
              <div>
                <dt>
                  <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                  ><kbd>F</kbd>
                </dt>
                <dd>{{ t("common.focusMode") }}</dd>
              </div>
              <div>
                <dt><kbd>Esc</kbd></dt>
                <dd>{{ t("common.exitFocusMode") }}</dd>
              </div>
              <div>
                <dt>
                  <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                  ><kbd>T</kbd>
                </dt>
                <dd>{{ t("common.today") }}</dd>
              </div>
              <div>
                <dt>
                  <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                  ><kbd>ArrowLeft</kbd>
                </dt>
                <dd>{{ t("common.previousDay") }}</dd>
              </div>
              <div>
                <dt>
                  <kbd>Ctrl</kbd><span>+</span><kbd>Alt</kbd><span>+</span
                  ><kbd>ArrowRight</kbd>
                </dt>
                <dd>{{ t("common.nextDay") }}</dd>
              </div>
            </dl>
          </div>
        </div>
        <div
          class="save-status"
          :class="`status-${displaySaveStatus}`"
          role="status"
          aria-live="polite"
          aria-atomic="true"
        >
          <span v-if="statusText">{{ statusText }}</span>
          <button
            v-if="saveStatus === 'failed'"
            type="button"
            :disabled="!canAutosave"
            @click="retryAutosave"
          >
            {{ t("common.retry") }}
          </button>
        </div>
      </div>
    </header>

    <div class="entry-surface__messages">
      <p v-if="saveStatus === 'failed' && saveError" class="save-error">
        {{ saveError }}
      </p>
      <p v-if="navigationMessage" class="navigation-warning" aria-live="polite">
        {{ navigationMessage }}
      </p>
      <div
        v-if="saveStatus === 'conflict'"
        class="recovery-panel"
        aria-live="polite"
      >
        <p>{{ saveError }}</p>
        <div>
          <button type="button" @click="reloadServerVersion">
            {{ t("entries.loadServerVersion") }}
          </button>
          <button
            type="button"
            :disabled="!localTextForMerge"
            @click="copyLocalText"
          >
            {{ t("entries.copyLocalBody") }}
          </button>
        </div>
      </div>
    </div>

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
        ref="markdownEditor"
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
    </div>

    <div
      v-if="persistedImages.length || uploadImages.length"
      class="image-grid"
      aria-live="polite"
    >
      <EntryImageAttachment
        v-for="image in persistedImages"
        :key="`persisted-${image.id}`"
        :image="image"
        :entry-date="form.entryDate"
        can-delete
        @delete="removePersistedImage"
      />

      <figure
        v-for="image in uploadImages"
        :key="image.id"
        class="image-tile"
        :class="`upload-${image.status}`"
      >
        <img :src="image.previewUrl" :alt="image.file.name" />
        <div class="upload-state" aria-live="polite">
          <span v-if="image.status === 'preparing'">{{
            t("images.preparing")
          }}</span>
          <span v-else-if="image.status === 'uploading'">{{
            t("images.uploading", { progress: image.progress })
          }}</span>
          <span v-else>{{ image.error || t("images.uploadFailed") }}</span>
          <button
            v-if="image.status === 'failed'"
            type="button"
            @click="retryUpload(image)"
          >
            <RotateCcw :size="13" />
            <span>{{ t("common.retry") }}</span>
          </button>
        </div>
        <button
          v-if="image.status !== 'preparing' && image.status !== 'uploading'"
          type="button"
          :aria-label="t('common.removeNamed', { name: image.file.name })"
          @click="removeUpload(image)"
        >
          <X :size="15" />
        </button>
      </figure>
    </div>
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

.entry-surface__chrome {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 12px;
  min-height: 34px;
  margin-bottom: 24px;
  color: var(--color-muted);
  font-size: 13px;
  line-height: 1.4;
}

.entry-surface__date-nav {
  display: inline-flex;
  grid-column: 2;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 14px;
}

.entry-surface__chrome-action {
  appearance: none;
  display: inline-flex;
  min-height: auto;
  align-items: center;
  gap: 3px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  box-shadow: none;
  color: var(--color-muted);
  cursor: pointer;
  font: inherit;
  letter-spacing: 0;
  line-height: 1.4;
  padding: 2px 4px;
  text-decoration: none;
}

.entry-surface__chrome-action:hover:not(:disabled) {
  color: var(--color-text);
  text-decoration: underline;
  text-underline-offset: 0.18em;
}

.entry-surface__chrome-action:focus-visible {
  outline: 1px solid color-mix(in srgb, var(--color-accent) 58%, transparent);
  outline-offset: 2px;
}

.entry-surface__chrome-action:disabled {
  cursor: not-allowed;
  color: var(--color-placeholder);
  opacity: 0.55;
  text-decoration: none;
}

.entry-surface__date {
  margin: 0;
  color: var(--color-text);
  font-size: 17px;
  font-weight: 680;
  letter-spacing: 0;
  line-height: 1.35;
}

.entry-surface__chrome-side {
  justify-self: end;
}

.focus-exit {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 10px;
  font-size: 13px;
  line-height: 1;
  white-space: nowrap;
}

.focus-exit:hover {
  background: var(--surface-hover);
}

.focus-exit:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 64%, transparent);
  outline-offset: 2px;
}

.entry-actions {
  position: relative;
  display: inline-flex;
}

.entry-actions__trigger {
  appearance: none;
  display: inline-grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  cursor: pointer;
}

.entry-actions__trigger:hover,
.entry-actions__trigger[aria-expanded="true"] {
  background: var(--surface-hover);
  color: var(--color-text-soft);
}

.entry-actions__trigger:focus-visible {
  outline: 1px solid color-mix(in srgb, var(--color-accent) 58%, transparent);
  outline-offset: 2px;
}

.entry-actions__menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 6;
  min-width: 172px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  box-shadow: var(--shadow-soft);
  padding: 6px;
}

.entry-actions__item {
  appearance: none;
  display: flex;
  width: 100%;
  align-items: center;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--color-text-soft);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  letter-spacing: 0;
  line-height: 1.3;
  padding: 8px 9px;
  text-align: left;
  text-decoration: none;
  white-space: nowrap;
}

.entry-actions__item:hover:not(:disabled),
.entry-actions__item:focus-visible {
  background: var(--surface-hover);
  color: var(--color-text);
  outline: none;
}

.entry-actions__item:disabled {
  cursor: not-allowed;
  opacity: 0.52;
}

.entry-actions__item--danger {
  color: var(--color-danger);
}

.entry-actions__item--danger:hover:not(:disabled),
.entry-actions__item--danger:focus-visible {
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

.entry-actions__separator {
  height: 1px;
  background: var(--border-subtle);
  margin: 5px 3px;
}

.entry-surface__messages {
  margin-bottom: 14px;
}

.entry-surface__messages:empty {
  display: none;
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

.focus-mode .entry-surface__chrome {
  position: sticky;
  top: 0;
  z-index: 3;
  border-bottom: 1px solid
    color-mix(in srgb, var(--border-subtle) 54%, transparent);
  background: color-mix(in srgb, var(--color-bg) 94%, transparent);
  padding: 0 0 12px;
}

.focus-mode .entry-surface__date-nav {
  justify-content: flex-start;
}

.focus-mode .entry-surface__meta,
.focus-mode .entry-surface__attachments,
.focus-mode .image-grid {
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

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.entry-shortcuts__panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 6;
  width: min(300px, calc(100vw - 32px));
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  box-shadow: var(--shadow-soft);
  color: var(--color-text-soft);
  padding: 12px;
}

.entry-shortcuts__panel p {
  margin: 0 0 10px;
  color: var(--color-text);
  font-size: 12px;
  font-weight: 650;
  line-height: 1.3;
}

.entry-shortcuts__panel dl {
  display: grid;
  gap: 8px;
  margin: 0;
}

.entry-shortcuts__panel dl div {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(92px, 0.9fr);
  align-items: center;
  gap: 10px;
}

.entry-shortcuts__panel dt {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 3px;
  color: var(--color-muted);
  font-size: 11px;
  line-height: 1.6;
}

.entry-shortcuts__panel dd {
  margin: 0;
  color: var(--color-text-soft);
  font-size: 12px;
  line-height: 1.4;
}

.entry-shortcuts__panel kbd {
  border: 1px solid var(--border-control);
  border-radius: 4px;
  background: color-mix(in srgb, var(--color-bg) 78%, var(--color-surface));
  color: var(--color-text-soft);
  padding: 1px 5px;
  font: inherit;
  font-size: 11px;
  line-height: 1.45;
  white-space: nowrap;
}

.save-status {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 8px;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1;
  white-space: nowrap;
}

.save-status span {
  display: inline-flex;
  align-items: center;
}

.save-status.status-saving {
  color: var(--color-text-soft);
}

.save-status.status-saved {
  color: var(--color-muted);
}

.save-status.status-dirty {
  color: var(--color-text-note);
}

.save-status.status-failed {
  color: var(--color-danger);
}

.save-status.status-conflict {
  color: var(--color-danger);
}

.save-status button {
  min-height: 28px;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: inherit;
  padding: 0 8px;
  font-size: 12px;
}

.save-status button:hover {
  background: var(--color-danger-bg);
}

.save-error {
  max-width: 520px;
  margin: -10px 0 14px;
  color: var(--color-danger);
  font-size: 12px;
  line-height: 1.6;
}

.navigation-warning {
  max-width: 520px;
  margin: -10px 0 14px;
  color: var(--color-danger);
  font-size: 12px;
  line-height: 1.6;
}

.recovery-panel {
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-danger-bg);
  color: var(--color-danger);
  margin: -10px 0 14px;
  padding: 9px 10px;
  font-size: 12px;
  line-height: 1.6;
}

.recovery-panel p {
  margin: 0 0 8px;
}

.recovery-panel div {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.recovery-panel button {
  min-height: 30px;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-danger);
  padding: 0 9px;
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

.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.image-tile {
  position: relative;
  overflow: hidden;
  aspect-ratio: 4 / 3;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  margin: 0;
}

.image-tile img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.image-tile.upload-preparing,
.image-tile.upload-uploading {
  border-style: dashed;
}

.image-tile.upload-failed {
  border-color: var(--color-danger-border);
}

.image-tile button {
  position: absolute;
  top: 8px;
  right: 8px;
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--surface-glass-solid);
  color: var(--color-text);
}

.upload-state {
  position: absolute;
  right: 8px;
  bottom: 8px;
  left: 8px;
  display: flex;
  min-height: 34px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--surface-glass-solid);
  color: var(--color-text-soft);
  padding: 6px 9px;
  font-size: 12px;
  line-height: 1.35;
}

.upload-state span {
  min-width: 0;
  overflow-wrap: anywhere;
}

.upload-failed .upload-state {
  border-color: var(--color-danger-border);
  background: color-mix(in srgb, var(--color-danger-bg) 88%, #fff);
  color: var(--color-danger);
}

.upload-state button {
  position: static;
  display: inline-flex;
  width: auto;
  min-width: max-content;
  height: 28px;
  align-items: center;
  gap: 6px;
  border-color: color-mix(in srgb, currentColor 24%, transparent);
  background: transparent;
  color: inherit;
  padding: 0 9px;
  font-size: 12px;
}

@media (max-width: 720px) {
  .entry-surface {
    padding-top: 26px;
  }

  .entry-surface__chrome {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .entry-surface__date-nav {
    grid-column: 1;
    justify-content: flex-start;
  }

  .image-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .entry-surface {
    padding-top: 22px;
    padding-bottom: calc(154px + env(safe-area-inset-bottom));
  }

  .entry-surface__chrome {
    gap: 10px;
    margin-bottom: 22px;
  }

  .entry-surface__date-nav {
    flex-wrap: wrap;
    gap: 8px;
  }

  .entry-surface__date {
    font-size: 16px;
  }

  .actions {
    justify-content: flex-end;
  }

  .save-status {
    min-height: 30px;
  }

  .title-input {
    font-size: 22px;
  }

  .meta-row {
    margin-bottom: 22px;
  }

  .image-bar .ui-action {
    width: 100%;
  }

  .image-bar {
    display: none;
  }

  .image-grid {
    gap: 8px;
  }
}
</style>
