<script setup lang="ts">
import { RotateCcw, X } from "lucide-vue-next";
import { useI18n } from "vue-i18n";
import type { UploadImageItem } from "../composables/useEntryImageUploads";
import type { EntryImage } from "../types";
import EntryImageAttachment from "./EntryImageAttachment.vue";

defineProps<{
  entryDate: string;
  focusMode: boolean;
  persistedImages: EntryImage[];
  uploadImages: UploadImageItem[];
}>();

const emit = defineEmits<{
  deletePersistedImage: [image: EntryImage];
  removeUpload: [item: UploadImageItem];
  retryUpload: [item: UploadImageItem];
}>();

const { t } = useI18n();
</script>

<template>
  <div
    class="image-grid"
    :class="{ 'image-grid--focus': focusMode }"
    aria-live="polite"
  >
    <EntryImageAttachment
      v-for="image in persistedImages"
      :key="`persisted-${image.id}`"
      :image="image"
      :entry-date="entryDate"
      can-delete
      @delete="emit('deletePersistedImage', image)"
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
          @click="emit('retryUpload', image)"
        >
          <RotateCcw :size="13" />
          <span>{{ t("common.retry") }}</span>
        </button>
      </div>
      <button
        v-if="image.status !== 'preparing' && image.status !== 'uploading'"
        type="button"
        :aria-label="t('common.removeNamed', { name: image.file.name })"
        @click="emit('removeUpload', image)"
      >
        <X :size="15" />
      </button>
    </figure>
  </div>
</template>

<style scoped>
.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.image-grid--focus {
  display: none;
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
  .image-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .image-grid {
    gap: 8px;
  }

  .image-grid--focus {
    display: grid;
  }
}
</style>
