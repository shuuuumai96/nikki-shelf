<script setup lang="ts">
import { ImageOff, X } from "lucide-vue-next";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import type { EntryImage } from "../types";

const props = withDefaults(
  defineProps<{
    image: EntryImage;
    entryDate: string;
    canDelete?: boolean;
    showCaption?: boolean;
  }>(),
  {
    canDelete: false,
    showCaption: false,
  },
);

const emit = defineEmits<{
  delete: [image: EntryImage];
}>();

const missing = ref(false);
const { t } = useI18n();

const safeName = computed(() => props.image.fileName || t("images.missingAlt"));
const details = computed(() =>
  [
    { label: t("images.imageId"), value: String(props.image.id) },
    { label: t("images.file"), value: props.image.fileName },
    { label: t("images.url"), value: props.image.url },
    { label: t("images.entryDate"), value: props.entryDate },
  ].filter((item) => item.value),
);

watch(
  () => [props.image.id, props.image.url],
  () => {
    missing.value = false;
  },
);

function markMissing() {
  missing.value = true;
}
</script>

<template>
  <figure
    class="image-attachment"
    :class="{ 'image-attachment--missing': missing }"
  >
    <template v-if="!missing">
      <img :src="image.url" :alt="safeName" @error="markMissing" />
      <figcaption v-if="showCaption && image.fileName">
        {{ image.fileName }}
      </figcaption>
    </template>

    <div v-else class="missing-image" role="note" aria-live="polite">
      <div class="missing-image__heading">
        <ImageOff :size="18" stroke-width="1.8" aria-hidden="true" />
        <span>{{ t("images.missingHeading") }}</span>
      </div>
      <dl>
        <template v-for="item in details" :key="item.label">
          <dt>{{ item.label }}</dt>
          <dd>{{ item.value }}</dd>
        </template>
      </dl>
      <p>{{ t("images.missingHelp") }}</p>
    </div>

    <button
      v-if="canDelete"
      type="button"
      :aria-label="t('common.removeNamed', { name: safeName })"
      @click="emit('delete', image)"
    >
      <X :size="15" />
    </button>
  </figure>
</template>

<style scoped>
.image-attachment {
  position: relative;
  min-width: 0;
  margin: 0;
}

.image-attachment img {
  display: block;
  width: 100%;
  aspect-ratio: 4 / 3;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  object-fit: cover;
}

.image-attachment figcaption {
  margin-top: 6px;
  color: var(--color-muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.image-attachment button {
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

.missing-image {
  display: flex;
  min-height: 100%;
  aspect-ratio: 4 / 3;
  flex-direction: column;
  gap: 8px;
  border: 1px dashed var(--color-danger-border);
  border-radius: var(--radius-md);
  background: var(--color-danger-bg);
  color: var(--color-danger);
  padding: 12px;
  font-size: 12px;
  line-height: 1.35;
  overflow: auto;
}

.missing-image__heading {
  display: flex;
  align-items: center;
  gap: 7px;
  font-weight: 680;
}

.missing-image dl {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 4px 8px;
  margin: 0;
}

.missing-image dt {
  color: var(--color-text-soft);
}

.missing-image dd {
  margin: 0;
  color: var(--color-text);
  overflow-wrap: anywhere;
}

.missing-image p {
  margin: 0;
  color: var(--color-text-soft);
  overflow-wrap: anywhere;
}
</style>
