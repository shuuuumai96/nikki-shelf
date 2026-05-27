<script setup lang="ts">
import { Filter, Search } from "lucide-vue-next";
import { computed, defineAsyncComponent, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import AuthPanel from "./features/auth/components/AuthPanel.vue";
import { useAuthStore } from "./features/auth/store";
import type { AuthCredentials } from "./features/auth/types";
import type { UploadImageRequest } from "./features/entries/api";
import DiaryEditor from "./features/entries/components/DiaryEditor.vue";
import EntryReader from "./features/entries/components/EntryReader.vue";
import { useDateJump } from "./features/entries/composables/useDateJump";
import { useEntryFilters } from "./features/entries/composables/useEntryFilters";
import { moodOrder, moodSpecs } from "./features/entries/moods";
import { useEntryStore } from "./features/entries/store";
import type {
  Entry,
  EntryDateLookup,
  EntryInput,
  SaveStatus,
} from "./features/entries/types";
import AppSidebar from "./shared/components/AppSidebar.vue";
import { useWritingShortcuts } from "./shared/composables/useWritingShortcuts";
import { todayISO } from "./shared/utils/date";

const CalendarMonth = defineAsyncComponent(
  () => import("./features/calendar/components/CalendarMonth.vue"),
);
const EntryArchiveNav = defineAsyncComponent(
  () => import("./features/entries/components/EntryArchiveNav.vue"),
);
const EntryList = defineAsyncComponent(
  () => import("./features/entries/components/EntryList.vue"),
);
const SettingsPanel = defineAsyncComponent(
  () => import("./features/settings/components/SettingsPanel.vue"),
);

const auth = useAuthStore();
const store = useEntryStore();
const { t } = useI18n();
const view = ref("today");
const entriesMode = ref<"list" | "archive">("list");
type EntrySurfaceMode = "reader" | "editor";
type EntryOpenSource =
  | "app-start"
  | "today"
  | "adjacent"
  | "calendar"
  | "list"
  | "archive"
  | "search"
  | "edit";

const entrySurfaceMode = ref<EntrySurfaceMode>("editor");
const diaryEditor = ref<{
  flushPendingAutosave: () => boolean;
  toggleFocusMode: () => void;
} | null>(null);
const navigationMessage = ref("");
const { clearEntryFilters, filters, hasEntryFilters } = useEntryFilters();

const selectedId = computed(() => store.activeEntry?.id);

onMounted(() => {
  void bootstrap();
});

const { clearDateJumpError, dateJumpError, dateJumpValue, jumpToDate } =
  useDateJump({
    selectDate: (date) => selectDate(date, "archive"),
  });

useWritingShortcuts({
  canUseShortcuts: () => Boolean(auth.user),
  getActiveDate: () => store.activeDate,
  getFocusModeController: () => diaryEditor.value,
  isWritingView: () => view.value === "today",
  navigateToDate: navigateDiaryDay,
  selectToday,
});

watch(
  () => [filters.query, filters.mood, filters.tag],
  () => {
    void store.loadEntries({
      query: filters.query,
      mood: filters.mood,
      tag: filters.tag,
    });
  },
);

async function selectDate(date: string, source: EntryOpenSource = "calendar") {
  if (!(await prepareDiaryNavigation(navigationSourceLabel(source)))) {
    return;
  }

  const lookup = await store.loadEntryByDate(date);
  if (lookup && !store.error) {
    entrySurfaceMode.value = resolveEntrySurfaceMode(
      store.activeDate,
      lookup,
      source,
    );
    view.value = "today";
    navigationMessage.value = "";
  }
}

async function selectEntry(entry: Entry) {
  await selectDate(entry.entryDate, "list");
}

async function selectToday() {
  await selectDate(todayISO(), "today");
}

async function navigateDiaryDay(date: string) {
  await selectDate(date, "adjacent");
}

function editActiveEntry() {
  entrySurfaceMode.value = resolveEntrySurfaceMode(
    store.activeDate,
    { entry: store.activeEntry },
    "edit",
  );
}

async function prepareDiaryNavigation(source: string) {
  if (hasBlockingSaveStatus(store.saveStatus)) {
    navigationMessage.value = blockedNavigationMessage(store.saveStatus);
    return false;
  }

  if (store.saving) {
    navigationMessage.value = t("entries.navigationSaving");
    return false;
  }

  // Date changes must flush the editor before loading another entry; otherwise
  // the store can accept a late autosave for the date the user just left.
  diaryEditor.value?.flushPendingAutosave();
  await store.waitForAutosaveIdle();
  if (hasBlockingSaveStatus(store.saveStatus)) {
    navigationMessage.value = blockedNavigationMessage(store.saveStatus);
    return false;
  }

  if (store.saveStatus === "saving") {
    navigationMessage.value = t("entries.navigationRetryAfterSave", { source });
    return false;
  }

  navigationMessage.value = "";
  return true;
}

function hasBlockingSaveStatus(status: SaveStatus) {
  return status === "failed" || status === "conflict";
}

function blockedNavigationMessage(status: SaveStatus) {
  if (status === "conflict") {
    return t("entries.navigationConflictBlocked");
  }

  return t("entries.navigationFailedBlocked");
}

async function bootstrap() {
  await auth.bootstrap();
  if (auth.user) {
    await store.bootstrap();
    entrySurfaceMode.value = resolveEntrySurfaceMode(
      store.activeDate,
      { entry: store.activeEntry },
      "app-start",
    );
  }
}

async function login(input: AuthCredentials) {
  await auth.login(input);
  await store.bootstrap();
  entrySurfaceMode.value = resolveEntrySurfaceMode(
    store.activeDate,
    { entry: store.activeEntry },
    "app-start",
  );
}

async function signup(input: AuthCredentials) {
  await auth.signup(input);
  await store.bootstrap();
}

async function logout() {
  await auth.logout();
  store.clear();
  view.value = "today";
  entriesMode.value = "list";
}

function autosaveDiary(input: EntryInput) {
  void store.autosave(input);
}

function uploadDiaryImage(payload: {
  input: EntryInput;
  file: File;
  onProgress: (progress: number) => void;
}): UploadImageRequest {
  return store.uploadImage(payload.input, payload.file, payload.onProgress);
}

function resolveEntrySurfaceMode(
  targetDate: string,
  lookupResult: Pick<EntryDateLookup, "entry">,
  source: EntryOpenSource,
): EntrySurfaceMode {
  if (source === "edit") {
    return "editor";
  }

  if (source === "search" && lookupResult.entry !== null) {
    return "reader";
  }

  // Today and empty dates open as writable surfaces; historical entries open
  // read-only until the user explicitly chooses edit.
  if (targetDate === todayISO()) {
    return "editor";
  }

  if (lookupResult.entry === null) {
    return "editor";
  }

  return "reader";
}

function navigationSourceLabel(source: EntryOpenSource) {
  const labels: Record<EntryOpenSource, string> = {
    "app-start": t("entries.sourceDateNavigation"),
    today: t("entries.sourceToday"),
    adjacent: t("entries.sourceDateNavigation"),
    calendar: t("entries.sourceCalendar"),
    list: t("entries.sourceList"),
    archive: t("entries.sourceArchive"),
    search: t("entries.sourceSearch"),
    edit: t("entries.sourceEdit"),
  };

  return labels[source];
}
</script>

<template>
  <div
    v-if="!auth.ready"
    class="app-shell app-shell-loading"
    aria-hidden="true"
  >
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

  <AuthPanel
    v-else-if="!auth.user"
    :error="auth.error"
    :loading="auth.loading"
    @login="login"
    @signup="signup"
  />

  <div v-else class="app-shell">
    <AppSidebar
      v-model="view"
      :active-date="store.activeDate"
      :stats="store.stats"
      :user="auth.user"
      :tags="store.tags"
      @go-today="selectToday"
      @select-date="selectDate($event, 'search')"
      @logout="logout"
    />

    <main>
      <p v-if="store.error" class="error">{{ store.error }}</p>

      <template v-if="view === 'today'">
        <EntryReader
          v-if="entrySurfaceMode === 'reader' && store.activeEntry !== null"
          :entry="store.activeEntry"
          :active-date="store.activeDate"
          :is-navigating="store.saving || store.saveStatus === 'saving'"
          @edit="editActiveEntry"
          @navigate-date="navigateDiaryDay"
        />

        <DiaryEditor
          v-else
          ref="diaryEditor"
          :entry="store.activeEntry"
          :date="store.activeDate"
          :tags="store.tags"
          :saving="store.saving"
          :save-status="store.saveStatus"
          :save-error="store.saveError"
          :navigation-message="navigationMessage"
          :upload-image="uploadDiaryImage"
          @autosave="autosaveDiary"
          @delete="store.removeActive"
          @delete-image="store.removeImage"
          @navigate-date="navigateDiaryDay"
          @reload-entry="store.reloadActive"
        />
      </template>

      <div v-else-if="view === 'calendar'" class="calendar-view">
        <CalendarMonth
          :entries="store.entries"
          :selected-date="store.activeDate"
          @select="selectDate($event, 'calendar')"
        />
        <div v-if="store.entriesHasMore" class="calendar-pagination">
          <button
            class="ui-action"
            type="button"
            :disabled="store.loading"
            @click="store.loadMoreEntries"
          >
            {{ t("entries.loadMore") }}
          </button>
        </div>
      </div>

      <section v-else-if="view === 'entries'" class="entries-view ui-page">
        <div class="entries-inner">
          <header class="entries-header">
            <h1 class="ui-heading">{{ t("entries.diary") }}</h1>
            <div class="entries-header-actions">
              <div class="entries-mode" :aria-label="t('entries.viewMode')">
                <button
                  type="button"
                  :class="{ active: entriesMode === 'list' }"
                  :aria-pressed="entriesMode === 'list'"
                  @click="entriesMode = 'list'"
                >
                  {{ t("entries.list") }}
                </button>
                <button
                  type="button"
                  :class="{ active: entriesMode === 'archive' }"
                  :aria-pressed="entriesMode === 'archive'"
                  @click="entriesMode = 'archive'"
                >
                  {{ t("entries.archive") }}
                </button>
              </div>
              <form
                class="date-jump"
                :aria-label="t('entries.jumpLabel')"
                @submit.prevent="jumpToDate"
              >
                <label class="sr-only" for="date-jump-input">{{
                  t("entries.jumpToDate")
                }}</label>
                <input
                  id="date-jump-input"
                  v-model="dateJumpValue"
                  type="date"
                  @input="clearDateJumpError"
                />
                <button type="submit">{{ t("entries.jump") }}</button>
              </form>
              <div class="search-box ui-search">
                <Search :size="16" stroke-width="1.8" />
                <input
                  v-model="filters.query"
                  :placeholder="t('entries.search')"
                />
              </div>
            </div>
          </header>

          <p v-if="dateJumpError" class="date-jump-error">
            {{ dateJumpError }}
          </p>

          <div class="filters">
            <div class="filter-label">
              <Filter :size="15" stroke-width="1.8" />
            </div>
            <select v-model="filters.mood" class="ui-select">
              <option value="">{{ t("entries.mood") }}</option>
              <option v-for="key in moodOrder" :key="key" :value="key">
                {{ t(moodSpecs[key].labelKey) }}
              </option>
            </select>
            <select v-model="filters.tag" class="ui-select">
              <option value="">{{ t("entries.tag") }}</option>
              <option v-for="tag in store.tags" :key="tag" :value="tag">
                {{ tag }}
              </option>
            </select>
          </div>

          <div class="entries-scroll" :aria-label="t('entries.diaryEntries')">
            <EntryList
              v-if="entriesMode === 'list'"
              :entries="store.entries"
              :selected-id="selectedId"
              :selected-date="store.activeDate"
              :loading="store.loading"
              :query="filters.query"
              :mood="filters.mood"
              :tag="filters.tag"
              :has-filters="hasEntryFilters"
              :has-more="store.entriesHasMore"
              @select="selectEntry"
              @clear-filters="clearEntryFilters"
              @load-more="store.loadMoreEntries"
            />
            <EntryArchiveNav
              v-else
              :entries="store.entries"
              :selected-date="store.activeDate"
              :loading="store.loading"
              :has-filters="hasEntryFilters"
              :has-more="store.entriesHasMore"
              @select-date="selectDate($event, 'archive')"
              @clear-filters="clearEntryFilters"
              @load-more="store.loadMoreEntries"
            />
          </div>
        </div>
      </section>

      <SettingsPanel
        v-else
        :user="auth.user"
        :loading="auth.loading"
        @logout="logout"
      />
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

.error {
  position: sticky;
  z-index: 5;
  top: 12px;
  width: min(760px, calc(100% - 40px));
  margin: 12px auto 0;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-danger-bg);
  color: var(--color-danger);
  padding: 8px 10px;
  font-size: 13px;
}

.calendar-view {
  min-height: 100%;
}

.calendar-pagination {
  display: flex;
  justify-content: center;
  width: min(880px, calc(100% - 32px));
  margin: 0 auto 32px;
}

.entries-view {
  --page-max: 900px;
  display: block;
  height: 100%;
  overflow: hidden;
}

.entries-inner {
  display: grid;
  height: 100%;
  min-height: 0;
  width: min(820px, 100%);
  margin: 0 auto;
  min-width: 0;
  grid-template-rows: auto auto minmax(0, 1fr);
}

.entries-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.entries-header h1 {
  font-size: 28px;
}

.entries-header-actions {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.entries-mode {
  display: inline-grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  min-width: 172px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 3px;
}

.entries-mode button {
  min-height: 30px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  padding: 0 10px;
  font-size: 13px;
}

.entries-mode button:hover {
  color: var(--color-text);
}

.entries-mode button.active {
  border-color: var(--border-active);
  background: var(--surface-active);
  color: var(--color-text);
}

.date-jump {
  display: grid;
  grid-template-columns: minmax(124px, 1fr) auto;
  align-items: center;
  gap: 6px;
  min-width: 192px;
}

.date-jump input {
  width: 100%;
  min-width: 0;
  height: 34px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 8px;
  font-size: 12px;
}

.date-jump input:focus {
  border-color: var(--border-active);
  outline: 0;
}

.date-jump button {
  min-height: 34px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-muted);
  padding: 0 10px;
  font-size: 12px;
}

.date-jump button:hover {
  border-color: var(--border-active);
  background: var(--surface-hover);
  color: var(--color-text);
}

.date-jump-error {
  margin: -6px 0 10px;
  color: var(--color-danger);
  font-size: 12px;
}

.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  min-width: 0;
}

.filter-label {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  color: var(--color-muted);
}

.entries-scroll {
  min-height: 0;
  overflow: auto;
  padding: 0 4px 24px 0;
  scrollbar-gutter: stable;
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

@media (max-width: 720px) {
  .entries-header {
    align-items: stretch;
    flex-direction: column;
  }

  .entries-header-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .entries-mode {
    width: 100%;
  }

  .date-jump {
    width: 100%;
  }

  .search-box {
    width: 100%;
  }
}

@media (max-width: 480px) {
  .entries-header {
    gap: 12px;
    margin-bottom: 12px;
  }

  .entries-header h1 {
    font-size: 24px;
  }

  .entries-mode button {
    min-height: 36px;
  }

  .filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    display: grid;
    gap: 8px;
    margin-bottom: 14px;
  }

  .filter-label {
    display: none;
  }

  .filters .ui-select {
    width: 100%;
    min-width: 0;
  }

  .entries-scroll {
    padding-right: 0;
  }
}
</style>
