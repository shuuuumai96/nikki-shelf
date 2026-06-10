<script setup lang="ts">
import { ChevronDown, ChevronUp, Image } from "lucide-vue-next";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { listEntryMemories } from "../api";
import { readMemoryPreferences } from "../memory-preferences";
import { moodSpecs } from "../moods";
import type { EntryMemory } from "../types";

const props = defineProps<{
  activeDate: string;
  embedded?: boolean;
}>();

const emit = defineEmits<{
  selectDate: [date: string];
}>();

const { t } = useI18n();
const preferences = ref(readMemoryPreferences());
const memories = ref<EntryMemory[]>([]);
const loading = ref(false);
const error = ref("");
const collapsed = ref(false);

let controller: AbortController | null = null;
let requestID = 0;

const excludedMoods = computed(() => preferences.value.excludedMoods.join(","));
const visible = computed(
  () =>
    preferences.value.enabled &&
    (loading.value || memories.value.length > 0 || error.value),
);

watch(
  () => [props.activeDate, preferences.value.enabled, excludedMoods.value],
  () => {
    void loadMemories();
  },
  { immediate: true },
);

onMounted(() => {
  window.addEventListener("storage", refreshPreferences);
});

onBeforeUnmount(() => {
  controller?.abort();
  window.removeEventListener("storage", refreshPreferences);
});

async function loadMemories() {
  controller?.abort();
  memories.value = [];
  error.value = "";

  if (!preferences.value.enabled) {
    loading.value = false;
    return;
  }

  const currentRequest = requestID + 1;
  requestID = currentRequest;
  controller = new AbortController();
  loading.value = true;

  try {
    const response = await listEntryMemories(
      {
        date: props.activeDate,
        excludeMoods: excludedMoods.value,
        limit: "3",
      },
      controller.signal,
    );
    if (requestID === currentRequest) {
      memories.value = response.items;
    }
  } catch (err) {
    if (isAbortError(err)) {
      return;
    }
    if (requestID === currentRequest) {
      error.value = t("entries.memoryLoadFailed");
    }
  } finally {
    if (requestID === currentRequest) {
      loading.value = false;
    }
  }
}

function refreshPreferences() {
  preferences.value = readMemoryPreferences();
}

function toggleCollapsed() {
  collapsed.value = !collapsed.value;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}
</script>

<template>
  <section
    v-if="visible"
    class="memory-shelf"
    :class="{
      'ui-page': !props.embedded,
      'memory-shelf--embedded': props.embedded,
    }"
  >
    <header class="memory-header">
      <div>
        <p>{{ t("entries.memoryEyebrow") }}</p>
        <h2>{{ t("entries.memoryTitle") }}</h2>
      </div>
      <div class="memory-actions">
        <button
          class="memory-collapse"
          type="button"
          :aria-label="
            collapsed ? t('entries.memoryExpand') : t('entries.memoryCollapse')
          "
          :title="
            collapsed ? t('entries.memoryExpand') : t('entries.memoryCollapse')
          "
          :aria-expanded="!collapsed"
          @click="toggleCollapsed"
        >
          <ChevronDown v-if="collapsed" :size="15" stroke-width="1.8" />
          <ChevronUp v-else :size="15" stroke-width="1.8" />
        </button>
      </div>
    </header>

    <template v-if="!collapsed">
      <p v-if="loading" class="memory-status">
        {{ t("entries.memoryLoading") }}
      </p>
      <p v-else-if="error" class="memory-status memory-status--error">
        {{ error }}
      </p>

      <div v-else class="memory-list">
        <button
          v-for="memory in memories"
          :key="memory.id"
          type="button"
          class="memory-card"
          @click="emit('selectDate', memory.entryDate)"
        >
          <span class="memory-date">{{ memory.entryDate }}</span>
          <strong>{{ memory.title || t("common.untitled") }}</strong>
          <span class="memory-meta">
            <span class="mood-chip">
              <component :is="moodSpecs[memory.mood].icon" :size="13" />
              {{ t(moodSpecs[memory.mood].labelKey) }}
            </span>
            <span v-for="tag in memory.tags" :key="tag" class="tag-chip">{{
              tag
            }}</span>
            <span v-if="memory.hasImage" class="image-chip">
              <Image :size="12" stroke-width="1.8" />
              {{ memory.imageCount }}
            </span>
          </span>
          <span v-if="memory.preview" class="memory-preview">{{
            memory.preview
          }}</span>
        </button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.memory-shelf {
  --page-max: 760px;
  min-height: auto;
  padding-top: 0;
  padding-bottom: 54px;
}

.memory-shelf--embedded {
  margin-top: 18px;
  padding-bottom: 0;
}

.memory-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--border-editor);
  padding-top: 18px;
  color: var(--color-muted);
}

.memory-header > div:first-child {
  min-width: 0;
}

.memory-header p {
  margin: 0 0 3px;
  font-size: 12px;
  line-height: 1.35;
}

.memory-header h2 {
  margin: 0;
  color: var(--color-text);
  font-size: 18px;
  font-weight: 730;
  letter-spacing: 0;
  line-height: 1.35;
}

.memory-actions {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  color: var(--color-muted);
}

.memory-collapse {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-muted);
  padding: 0;
}

.memory-collapse:hover {
  border-color: var(--border-active);
  background: var(--surface-hover);
  color: var(--color-text);
}

.memory-collapse:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 42%, #fff);
  outline-offset: 2px;
}

.memory-list {
  display: grid;
  gap: 8px;
  margin-top: 12px;
}

.memory-card {
  display: grid;
  gap: 5px;
  width: 100%;
  min-width: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 12px 13px;
  text-align: left;
}

.memory-card:hover {
  border-color: var(--border-active);
  background: var(--surface-hover);
}

.memory-card:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--color-accent) 42%, #fff);
  outline-offset: 2px;
}

.memory-date {
  color: var(--color-muted);
  font-size: 11px;
  line-height: 1.25;
}

.memory-card strong {
  overflow: hidden;
  color: var(--color-text);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.memory-preview {
  display: -webkit-box;
  overflow: hidden;
  color: var(--color-text-soft);
  font-size: 13px;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow-wrap: anywhere;
}

.memory-meta {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 5px;
  color: var(--color-muted);
  font-size: 11px;
  line-height: 1.35;
}

.mood-chip,
.tag-chip,
.image-chip {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 4px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--surface-glass-soft);
  padding: 2px 6px;
  overflow-wrap: anywhere;
}

.memory-status {
  margin: 10px 0 0;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1.45;
}

.memory-status--error {
  color: var(--color-danger);
}

@media (max-width: 480px) {
  .memory-shelf {
    padding-bottom: calc(118px + env(safe-area-inset-bottom));
  }

  .memory-shelf--embedded {
    padding-bottom: 0;
  }

  .memory-card strong {
    white-space: normal;
    overflow-wrap: anywhere;
  }
}
</style>
