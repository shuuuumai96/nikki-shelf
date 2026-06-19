<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import type {
  AuthUser,
  ChangePasswordInput,
  DeleteAccountInput,
} from "../../features/auth/types";
import type { UploadImageRequest } from "../../features/entries/api";
import TodayView from "../../features/entries/components/TodayView.vue";
import {
  useDiaryNavigation,
  type DiaryEditorHandle,
} from "../../features/entries/composables/useDiaryNavigation";
import {
  useEntrySurfaceMode,
  type EntryOpenSource,
} from "../../features/entries/composables/useEntrySurfaceMode";
import type { useEntryStore } from "../../features/entries/store";
import type { Entry, EntryInput } from "../../features/entries/types";
import { listAuditEvents } from "../../features/settings/api";
import type { AuditEvent } from "../../features/settings/types";
import { localizedErrorMessage } from "../api/client";
import { useWritingShortcuts } from "../composables/useWritingShortcuts";
import { todayISO } from "../utils/date";
import AppSidebar from "./AppSidebar.vue";

const loadCalendarView = () =>
  import("../../features/calendar/components/CalendarView.vue");
const loadEntriesView = () =>
  import("../../features/entries/components/EntriesView.vue");
const loadSettingsPanel = () =>
  import("../../features/settings/components/SettingsPanel.vue");

const CalendarView = defineAsyncComponent(loadCalendarView);
const EntriesView = defineAsyncComponent(loadEntriesView);
const SettingsPanel = defineAsyncComponent(loadSettingsPanel);

const props = defineProps<{
  authError?: string;
  authLoading: boolean;
  store: ReturnType<typeof useEntryStore>;
  user: AuthUser;
}>();

const emit = defineEmits<{
  changePassword: [input: ChangePasswordInput];
  deleteAccount: [input: DeleteAccountInput];
  logout: [];
}>();

const { t } = useI18n();
const view = ref("today");
const entriesMode = ref<"list" | "archive">("list");
const todayView = ref<DiaryEditorHandle | null>(null);
const memoryJourneyActive = ref(false);
const securityEvents = ref<AuditEvent[]>([]);
const securityEventsLoaded = ref(false);
const securityEventsLoading = ref(false);
const securityEventsError = ref("");
const selectedId = computed(() => props.store.activeEntry?.id);
const showReturnToToday = computed(
  () => memoryJourneyActive.value && props.store.activeDate !== todayISO(),
);
const canLoadSecurityEvents = computed(() => props.user.role === "owner");

const { entrySurfaceMode, resolveEntrySurfaceMode, setEntrySurfaceMode } =
  useEntrySurfaceMode();
const { navigationMessage, prepareDiaryNavigation } = useDiaryNavigation({
  diaryEditor: todayView,
  store: props.store,
  t,
});

onMounted(() => {
  setEntrySurfaceMode(
    props.store.activeDate,
    { entry: props.store.activeEntry },
    "app-start",
  );
  preloadNavigationViews();
});

watch(
  () => [view.value, props.user.id] as const,
  () => {
    if (view.value === "settings" && canLoadSecurityEvents.value) {
      void loadSecurityEvents();
    }
  },
);

useWritingShortcuts({
  canUseShortcuts: () => true,
  getActiveDate: () => props.store.activeDate,
  getFocusModeController: () => todayView.value,
  isWritingView: () => view.value === "today",
  navigateToDate: navigateDiaryDay,
  selectToday,
});

async function selectDate(date: string, source: EntryOpenSource = "calendar") {
  if (!(await prepareDiaryNavigation(navigationSourceLabel(source)))) {
    return;
  }

  const lookup = await props.store.loadEntryByDate(date);
  if (lookup && !props.store.error) {
    setEntrySurfaceMode(props.store.activeDate, lookup, source);
    updateMemoryJourney(source);
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
    props.store.activeDate,
    { entry: props.store.activeEntry },
    "edit",
  );
}

function autosaveDiary(input: EntryInput) {
  void props.store.autosave(input);
}

function uploadDiaryImage(payload: {
  input: EntryInput;
  file: File;
  onProgress: (progress: number) => void;
}): UploadImageRequest {
  return props.store.uploadImage(
    payload.input,
    payload.file,
    payload.onProgress,
  );
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
    memory: t("entries.sourceMemory"),
    edit: t("entries.sourceEdit"),
  };

  return labels[source];
}

function updateMemoryJourney(source: EntryOpenSource) {
  // Memory navigation keeps a temporary return path only while the user walks
  // adjacent days from a memory result.
  const viewingToday = props.store.activeDate === todayISO();
  if (source === "memory") {
    memoryJourneyActive.value = !viewingToday;
    return;
  }

  if (source === "adjacent") {
    memoryJourneyActive.value = memoryJourneyActive.value && !viewingToday;
    return;
  }

  memoryJourneyActive.value = false;
}

function preloadNavigationViews() {
  void Promise.all([
    loadCalendarView(),
    loadEntriesView(),
    loadSettingsPanel(),
  ]).catch(() => {});
}

async function loadSecurityEvents(force = false) {
  if (!canLoadSecurityEvents.value || securityEventsLoading.value) {
    return;
  }
  if (securityEventsLoaded.value && !force) {
    return;
  }

  securityEventsLoading.value = true;
  securityEventsError.value = "";
  try {
    securityEvents.value = (await listAuditEvents()).items;
    securityEventsLoaded.value = true;
  } catch (error) {
    securityEventsError.value = localizedErrorMessage(error);
  } finally {
    securityEventsLoading.value = false;
  }
}
</script>

<template>
  <div class="app-shell">
    <AppSidebar
      v-model="view"
      :active-date="store.activeDate"
      :stats="store.stats"
      :user="user"
      :tags="store.tags"
      @go-today="selectToday"
      @select-date="selectDate($event, 'search')"
      @logout="emit('logout')"
    />

    <main>
      <p v-if="store.error" class="error">{{ store.error }}</p>

      <TodayView
        v-if="view === 'today'"
        ref="todayView"
        :active-date="store.activeDate"
        :entry="store.activeEntry"
        :entry-surface-mode="entrySurfaceMode"
        :navigation-message="navigationMessage"
        :save-error="store.saveError"
        :save-status="store.saveStatus"
        :saving="store.saving"
        :show-return-today="showReturnToToday"
        :tags="store.tags"
        :upload-image="uploadDiaryImage"
        @autosave="autosaveDiary"
        @delete="store.removeActive"
        @delete-image="store.removeImage"
        @edit="editActiveEntry"
        @open-memory="selectDate($event, 'memory')"
        @navigate-date="navigateDiaryDay"
        @return-today="selectToday"
        @reload-entry="store.reloadActive"
      />

      <CalendarView
        v-else-if="view === 'calendar'"
        :entries="store.entries"
        :has-more="store.entriesHasMore"
        :loading="store.loading"
        :selected-date="store.activeDate"
        @load-more="store.loadMoreEntries"
        @select-date="selectDate($event, 'calendar')"
      />

      <EntriesView
        v-else-if="view === 'entries'"
        v-model:mode="entriesMode"
        :entries="store.entries"
        :has-more="store.entriesHasMore"
        :loading="store.loading"
        :selected-date="store.activeDate"
        :selected-id="selectedId"
        :tags="store.tags"
        @load-entries="store.loadEntries"
        @load-more="store.loadMoreEntries"
        @select-date="selectDate"
        @select-entry="selectEntry"
      />

      <SettingsPanel
        v-else
        :error="authError"
        :user="user"
        :loading="authLoading"
        :security-events="securityEvents"
        :security-events-error="securityEventsError"
        :security-events-loading="securityEventsLoading"
        @change-password="emit('changePassword', $event)"
        @delete-account="emit('deleteAccount', $event)"
        @logout="emit('logout')"
        @refresh-security-events="loadSecurityEvents(true)"
      />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: grid;
  height: 100dvh;
  min-height: 100dvh;
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

@media (max-width: 820px) {
  .app-shell {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(0, 1fr);
  }

  main {
    height: auto;
  }
}

@media (max-width: 640px) {
  .app-shell {
    display: block;
    height: 100dvh;
    min-height: 100dvh;
  }

  main {
    height: 100dvh;
    min-height: 0;
    padding-bottom: calc(84px + env(safe-area-inset-bottom));
    -webkit-overflow-scrolling: touch;
  }
}

@media (max-width: 480px) {
  main {
    padding-bottom: calc(84px + env(safe-area-inset-bottom));
  }
}
</style>
