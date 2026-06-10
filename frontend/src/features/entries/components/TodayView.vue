<script setup lang="ts">
import { computed, ref } from "vue";
import { todayISO } from "../../../shared/utils/date";
import type { UploadImageRequest } from "../api";
import type { EntrySurfaceMode } from "../composables/useEntrySurfaceMode";
import type { Entry, EntryInput, SaveStatus } from "../types";
import DiaryEditor from "./DiaryEditor.vue";
import EntryReader from "./EntryReader.vue";
import MemoryShelf from "./MemoryShelf.vue";

const props = defineProps<{
  activeDate: string;
  entry: Entry | null;
  entrySurfaceMode: EntrySurfaceMode;
  navigationMessage: string;
  saveError: string;
  saveStatus: SaveStatus;
  saving: boolean;
  showReturnToday: boolean;
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
  openMemory: [date: string];
  reloadEntry: [];
  returnToday: [];
}>();

const diaryEditor = ref<{
  flushPendingAutosave: () => boolean;
  toggleFocusMode: () => void;
} | null>(null);
const showMemoryShelf = computed(() => props.activeDate === todayISO());

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
</script>

<template>
  <EntryReader
    v-if="entrySurfaceMode === 'reader' && entry !== null"
    :entry="entry"
    :active-date="activeDate"
    :is-navigating="saving || saveStatus === 'saving'"
    :show-return-today="showReturnToday"
    @edit="emit('edit')"
    @navigate-date="emit('navigateDate', $event)"
    @return-today="emit('returnToday')"
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
  >
    <template #after-attachments>
      <MemoryShelf
        v-if="showMemoryShelf"
        embedded
        :active-date="activeDate"
        @select-date="emit('openMemory', $event)"
      />
    </template>
  </DiaryEditor>
</template>
