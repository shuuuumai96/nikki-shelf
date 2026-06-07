<script setup lang="ts">
import { defineAsyncComponent, ref } from "vue";
import type { UploadImageRequest } from "../api";
import type { EntrySurfaceMode } from "../composables/useEntrySurfaceMode";
import type { Entry, EntryInput, SaveStatus } from "../types";
import DiaryEditorLoadingSurface from "./DiaryEditorLoadingSurface.vue";

const DiaryEditor = defineAsyncComponent({
  delay: 0,
  loader: async () => {
    await afterNextPaint();
    return import("./DiaryEditor.vue");
  },
  loadingComponent: DiaryEditorLoadingSurface,
});
const EntryReader = defineAsyncComponent(() => import("./EntryReader.vue"));

defineProps<{
  activeDate: string;
  entry: Entry | null;
  entrySurfaceMode: EntrySurfaceMode;
  navigationMessage: string;
  saveError: string;
  saveStatus: SaveStatus;
  saving: boolean;
  tags: string[];
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
  edit: [];
  navigateDate: [date: string];
  reloadEntry: [];
}>();

const diaryEditor = ref<{
  flushPendingAutosave: () => boolean;
  toggleFocusMode: () => void;
} | null>(null);

function flushPendingAutosave() {
  return diaryEditor.value?.flushPendingAutosave() ?? false;
}

function toggleFocusMode() {
  diaryEditor.value?.toggleFocusMode();
}

defineExpose({
  flushPendingAutosave,
  toggleFocusMode,
});

function afterNextPaint() {
  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => resolve());
    });
  });
}
</script>

<template>
  <EntryReader
    v-if="entrySurfaceMode === 'reader' && entry !== null"
    :entry="entry"
    :active-date="activeDate"
    :is-navigating="saving || saveStatus === 'saving'"
    @edit="emit('edit')"
    @navigate-date="emit('navigateDate', $event)"
  />

  <DiaryEditor
    v-else
    ref="diaryEditor"
    :entry="entry"
    :date="activeDate"
    :tags="tags"
    :saving="saving"
    :save-status="saveStatus"
    :save-error="saveError"
    :navigation-message="navigationMessage"
    :upload-image="uploadImage"
    @autosave="emit('autosave', $event)"
    @delete="emit('delete')"
    @delete-image="emit('deleteImage', $event)"
    @navigate-date="emit('navigateDate', $event)"
    @reload-entry="emit('reloadEntry')"
  />
</template>
