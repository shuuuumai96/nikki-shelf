<template>
  <div class="app-shell app-shell-loading" aria-hidden="true">
    <aside class="loading-sidebar">
      <div class="loading-brand">Nikki</div>
    </aside>
    <main>
      <section class="loading-editor ui-page">
        <div class="loading-editor-header">
          <div class="loading-line loading-date"></div>
          <div class="loading-line loading-status"></div>
        </div>
        <div class="loading-line loading-title"></div>
        <div class="loading-moods">
          <div v-for="item in 5" :key="item" class="loading-pill"></div>
        </div>
        <div class="loading-line loading-tags"></div>
        <div class="loading-editor-surface"></div>
      </section>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: grid;
  height: 100dvh;
  grid-template-columns: 230px 1fr;
  overflow: hidden;
}

main {
  min-width: 0;
  min-height: 0;
  height: 100dvh;
  overflow: auto;
  overflow-anchor: none;
  scrollbar-gutter: stable;
  background: var(--color-bg);
}

.app-shell-loading {
  pointer-events: none;
}

.loading-sidebar {
  height: 100dvh;
  border-right: 1px solid var(--color-border);
  background: var(--color-sidebar);
  padding: 16px 12px;
}

.loading-brand {
  height: 38px;
  padding: 0 10px;
  font-size: 16px;
  font-weight: 760;
  line-height: 38px;
}

.loading-editor {
  --page-max: 760px;
  min-height: 100%;
  padding-top: 38px;
}

.loading-editor-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 18px;
}

.loading-line,
.loading-pill,
.loading-editor-surface {
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

.loading-editor-surface {
  min-height: 421px;
  margin-top: 22px;
  border-top: 1px solid var(--border-subtle);
}

@media (max-width: 820px) {
  .app-shell {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(0, 1fr);
  }

  main {
    height: auto;
  }

  .loading-sidebar {
    height: auto;
    border-right: 0;
    border-bottom: 1px solid var(--color-border);
  }
}

@media (max-width: 640px) {
  .app-shell {
    display: block;
    height: 100dvh;
  }

  main {
    height: 100svh;
    padding-bottom: calc(72px + env(safe-area-inset-bottom));
    -webkit-overflow-scrolling: touch;
  }

  .loading-sidebar {
    position: fixed;
    z-index: 20;
    right: 0;
    bottom: 0;
    left: 0;
    height: 65px;
    border-top: 1px solid var(--border-control);
    border-bottom: 0;
  }
}

@media (max-width: 720px) {
  .loading-editor {
    padding-top: 26px;
  }

  .loading-editor-header {
    flex-direction: column;
  }
}

@media (max-width: 480px) {
  main {
    padding-bottom: calc(76px + env(safe-area-inset-bottom));
  }

  .loading-editor {
    padding-top: 32px;
    padding-bottom: calc(112px + env(safe-area-inset-bottom));
  }

  .loading-editor-header {
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

  .loading-editor-surface {
    min-height: 445px;
    margin-top: 16px;
  }
}
</style>
