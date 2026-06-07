<script setup lang="ts">
import { useI18n } from "vue-i18n";

const { t } = useI18n();
</script>

<template>
  <section class="editor-loading ui-page">
    <div class="editor-loading-header">
      <div class="loading-line loading-date"></div>
      <div class="loading-line loading-status"></div>
    </div>
    <div class="loading-line loading-title"></div>
    <div class="loading-moods">
      <div v-for="item in 5" :key="item" class="loading-pill"></div>
    </div>
    <div class="loading-line loading-tags"></div>
    <div class="editor-loading-surface">
      <p class="loading-placeholder">{{ t("entries.bodyPlaceholder") }}</p>
      <div class="loading-line loading-paragraph"></div>
      <div class="loading-line loading-paragraph short"></div>
    </div>
  </section>
</template>

<style scoped>
.editor-loading {
  --page-max: 760px;
  min-height: 100%;
  padding-top: 38px;
}

.editor-loading-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 18px;
}

.loading-line,
.loading-pill,
.editor-loading-surface {
  background: var(--color-surface-soft);
}

.loading-date {
  width: 128px;
  height: 28px;
}

.loading-status {
  width: 72px;
  height: 34px;
}

.loading-title {
  width: 62%;
  height: 36px;
}

.loading-moods {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 18px 0 12px;
}

.loading-pill {
  width: 88px;
  height: 34px;
}

.loading-tags {
  width: 180px;
  height: 34px;
}

.editor-loading-surface {
  display: grid;
  align-content: start;
  gap: 12px;
  min-height: 421px;
  margin-top: 22px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 22px;
}

.loading-placeholder {
  margin: 0 0 4px;
  color: var(--color-muted);
  font-size: 15px;
  line-height: 1.7;
}

.loading-paragraph {
  width: min(520px, 88%);
  height: 16px;
}

.loading-paragraph.short {
  width: min(360px, 62%);
}

@media (max-width: 720px) {
  .editor-loading {
    padding-top: 26px;
  }

  .editor-loading-header {
    flex-direction: column;
  }
}

@media (max-width: 480px) {
  .editor-loading {
    padding-top: 32px;
    padding-bottom: calc(112px + env(safe-area-inset-bottom));
  }

  .editor-loading-header {
    gap: 14px;
    margin-bottom: 18px;
  }

  .loading-status {
    margin-left: auto;
  }

  .loading-title {
    width: 74%;
    height: 28px;
  }

  .loading-moods {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    margin: 16px 0 12px;
  }

  .loading-pill {
    width: auto;
    height: 42px;
  }

  .loading-tags {
    width: 100%;
    height: 44px;
  }

  .editor-loading-surface {
    min-height: 445px;
    margin-top: 16px;
  }
}
</style>
