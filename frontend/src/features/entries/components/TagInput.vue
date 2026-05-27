<script setup lang="ts">
import { X } from "lucide-vue-next";
import { ref } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  modelValue: string[];
  suggestions?: string[];
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string[]];
}>();

const draft = ref("");
const { t } = useI18n();

function addTag(value = draft.value) {
  const tag = value.trim();
  if (!tag || props.modelValue.includes(tag)) {
    draft.value = "";
    return;
  }
  emit("update:modelValue", [...props.modelValue, tag]);
  draft.value = "";
}

function removeTag(tag: string) {
  emit(
    "update:modelValue",
    props.modelValue.filter((item) => item !== tag),
  );
}
</script>

<template>
  <div class="tag-input ui-control">
    <span v-for="tag in modelValue" :key="tag" class="tag">
      {{ tag }}
      <button
        type="button"
        :aria-label="t('common.removeNamed', { name: tag })"
        @click="removeTag(tag)"
      >
        <X :size="13" />
      </button>
    </span>

    <input
      v-model="draft"
      list="tag-suggestions"
      :placeholder="t('entries.tag')"
      @keydown.enter.prevent="addTag()"
      @blur="addTag()"
    />
    <datalist id="tag-suggestions">
      <option v-for="tag in suggestions" :key="tag" :value="tag" />
    </datalist>
  </div>
</template>

<style scoped>
.tag-input {
  display: flex;
  min-height: 38px;
  flex-wrap: wrap;
  align-items: center;
  gap: 7px;
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 5px 7px;
}

.tag {
  display: inline-flex;
  height: 26px;
  align-items: center;
  gap: 4px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  background: var(--color-surface-soft);
  color: var(--color-text-tag);
  padding: 0 8px;
  font-size: 12px;
  font-weight: 500;
}

.tag button {
  display: grid;
  width: 18px;
  height: 18px;
  place-items: center;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: currentColor;
  padding: 0;
}

.tag button:hover {
  background: var(--surface-chip-remove-hover);
}

input {
  min-width: 90px;
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--color-text);
}

input::placeholder {
  color: var(--color-placeholder);
}

@media (max-width: 480px) {
  .tag-input {
    min-height: 44px;
    gap: 8px;
    border-radius: var(--radius-sm);
    padding: 7px 10px;
  }

  .tag {
    min-height: 30px;
    padding: 0 9px;
  }

  .tag button {
    width: 24px;
    height: 24px;
  }

  input {
    min-height: 30px;
    font-size: 16px;
  }
}
</style>
