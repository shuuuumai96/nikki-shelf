<script setup lang="ts">
import { useI18n } from "vue-i18n";

const { t } = useI18n();
</script>

<template>
  <div class="markdown-editor-loading">
    <div class="document-editor-wrap">
      <div class="document-editor">
        <div class="document-editor-surface entry-markdown-body">
          <p class="placeholder">{{ t("entries.bodyPlaceholder") }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.markdown-editor-loading {
  margin-top: 22px;
}

.document-editor {
  color: var(--color-text);
}

.document-editor-wrap {
  position: relative;
  min-height: 421px;
}

.document-editor-surface {
  min-height: 360px;
  outline: none;
  border-top: 1px solid var(--border-subtle);
  padding: 22px 0 38px;
}

.placeholder {
  margin: 0;
  color: var(--color-placeholder);
  font-size: 15px;
  line-height: 1.7;
}

@media (max-width: 480px) {
  .markdown-editor-loading {
    margin-top: 16px;
  }

  .document-editor-surface {
    min-height: 300px;
    padding: 4px 0 28px;
  }

  .document-editor-wrap {
    min-height: 445px;
  }
}
</style>
