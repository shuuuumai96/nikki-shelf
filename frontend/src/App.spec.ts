import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick, reactive } from "vue";
import App from "./App.vue";
import type { AuthUser } from "./features/auth/types";
import type { Entry, Stats } from "./features/entries/types";
import type { SetupStatus } from "./features/setup/types";

const mocks = vi.hoisted(() => ({
  authStore: null as any,
  entryStore: null as any,
  setupStatus: vi.fn(),
  createOwner: vi.fn(),
  restoreBackup: vi.fn(),
  verifyRestore: vi.fn(),
}));

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    locale: { value: "ja" },
    t: (key: string, params?: Record<string, string>) =>
      params?.source ? `${key}:${params.source}` : key,
  }),
}));

vi.mock("./shared/i18n", () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}));

vi.mock("./shared/api/client", () => ({
  localizedErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "error",
}));

vi.mock("./features/auth/store", () => ({
  useAuthStore: () => mocks.authStore,
}));

vi.mock("./features/entries/store", () => ({
  useEntryStore: () => mocks.entryStore,
}));

vi.mock("./features/setup/api", () => ({
  createOwner: mocks.createOwner,
  restoreBackup: mocks.restoreBackup,
  status: mocks.setupStatus,
  verifyRestore: mocks.verifyRestore,
}));

describe("App", () => {
  beforeEach(() => {
    setNavigatorOnline(true);
    window.history.replaceState({}, "", "/");
    mocks.setupStatus.mockReset();
    mocks.createOwner.mockReset();
    mocks.restoreBackup.mockReset();
    mocks.verifyRestore.mockReset();
    mocks.verifyRestore.mockResolvedValue({ ok: true });

    mocks.authStore = reactive({
      user: null as AuthUser | null,
      ready: false,
      loading: false,
      error: "",
      bootstrap: vi.fn(async () => {
        mocks.authStore.ready = true;
      }),
      login: vi.fn(async () => {
        mocks.authStore.user = user();
        mocks.authStore.ready = true;
      }),
      signup: vi.fn(async () => {
        mocks.authStore.user = user();
        mocks.authStore.ready = true;
      }),
      logout: vi.fn(async () => {
        mocks.authStore.user = null;
      }),
      start: vi.fn(async (action: () => Promise<AuthUser>) => {
        mocks.authStore.user = await action();
        mocks.authStore.ready = true;
      }),
    });

    mocks.entryStore = reactive({
      entries: [] as Entry[],
      entriesHasMore: false,
      activeEntry: null as Entry | null,
      activeDate: "2026-06-07",
      tags: [] as string[],
      stats: stats(),
      loading: false,
      saving: false,
      saveStatus: "idle",
      saveError: "",
      error: "",
      bootstrap: vi.fn(async () => undefined),
      loadEntries: vi.fn(async () => undefined),
      loadMoreEntries: vi.fn(async () => undefined),
      loadEntryByDate: vi.fn(async (date: string) => {
        mocks.entryStore.activeDate = date;
        mocks.entryStore.activeEntry = null;
        return { date, entry: null, exists: false };
      }),
      waitForAutosaveIdle: vi.fn(async () => undefined),
      autosave: vi.fn(async () => undefined),
      uploadImage: vi.fn(),
      removeActive: vi.fn(async () => undefined),
      removeImage: vi.fn(async () => undefined),
      reloadActive: vi.fn(async () => undefined),
      clear: vi.fn(() => {
        mocks.entryStore.activeEntry = null;
      }),
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    setNavigatorOnline(true);
  });

  it("shows the loading shell until auth and setup are ready", () => {
    mocks.authStore.bootstrap = vi.fn(() => new Promise(() => undefined));

    const wrapper = mountApp();

    expect(wrapper.find(".app-shell-loading").exists()).toBe(true);
    expect(wrapper.find('[data-testid="auth-panel"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="setup-panel"]').exists()).toBe(false);
  });

  it("routes unauthenticated users to setup when setup is required", async () => {
    mocks.setupStatus.mockResolvedValue(setupStatus({ needsSetup: true }));

    const wrapper = mountApp();
    await flushPromises();

    expect(wrapper.find('[data-testid="setup-panel"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="auth-panel"]').exists()).toBe(false);
    expect(window.location.pathname).toBe("/setup");
  });

  it("routes unauthenticated users to auth when setup is complete", async () => {
    mocks.setupStatus.mockResolvedValue(setupStatus({ needsSetup: false }));

    const wrapper = mountApp();
    await flushPromises();

    expect(wrapper.find('[data-testid="auth-panel"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="setup-panel"]').exists()).toBe(false);
    expect(window.location.pathname).toBe("/");
  });

  it("shows the authenticated diary shell after bootstrapping a session", async () => {
    mocks.authStore.bootstrap = vi.fn(async () => {
      mocks.authStore.user = user();
      mocks.authStore.ready = true;
    });

    const wrapper = mountApp();
    await flushPromises();

    expect(mocks.entryStore.bootstrap).toHaveBeenCalledTimes(1);
    expect(wrapper.find('[data-testid="sidebar"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="diary-editor"]').exists()).toBe(true);
  });

  it("shows the offline banner when the browser is offline", async () => {
    setNavigatorOnline(false);
    mocks.setupStatus.mockResolvedValue(setupStatus({ needsSetup: false }));

    const wrapper = mountApp();
    await flushPromises();

    expect(wrapper.text()).toContain("common.offline");
  });

  it("blocks date navigation while the active entry has a failed save", async () => {
    mocks.authStore.bootstrap = vi.fn(async () => {
      mocks.authStore.user = user();
      mocks.authStore.ready = true;
    });
    mocks.entryStore.saveStatus = "failed";
    const wrapper = mountApp();
    await flushPromises();
    mocks.entryStore.loadEntryByDate.mockClear();

    await wrapper
      .findComponent({ name: "AppSidebar" })
      .vm.$emit("update:modelValue", "calendar");
    await vi.dynamicImportSettled();
    await flushPromises();
    await nextTick();
    await wrapper
      .findComponent({ name: "CalendarMonth" })
      .vm.$emit("select", "2026-06-08");
    await flushPromises();

    expect(mocks.entryStore.loadEntryByDate).not.toHaveBeenCalled();

    await wrapper
      .findComponent({ name: "AppSidebar" })
      .vm.$emit("update:modelValue", "today");
    await nextTick();

    expect(
      wrapper.findComponent({ name: "DiaryEditor" }).props("navigationMessage"),
    ).toBe("entries.navigationFailedBlocked");
  });

  it("opens a search-selected existing entry in reader mode", async () => {
    mocks.authStore.bootstrap = vi.fn(async () => {
      mocks.authStore.user = user();
      mocks.authStore.ready = true;
    });
    const foundEntry = entry({ entryDate: "2026-06-01" });
    mocks.entryStore.loadEntryByDate.mockImplementation(
      async (date: string) => {
        mocks.entryStore.activeDate = date;
        mocks.entryStore.activeEntry = foundEntry;
        return { date, entry: foundEntry, exists: true };
      },
    );
    const wrapper = mountApp();
    await flushPromises();
    mocks.entryStore.loadEntryByDate.mockClear();

    await wrapper
      .findComponent({ name: "AppSidebar" })
      .vm.$emit("selectDate", "2026-06-01");
    await flushPromises();

    expect(mocks.entryStore.loadEntryByDate).toHaveBeenCalledWith("2026-06-01");
    expect(wrapper.find('[data-testid="entry-reader"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="diary-editor"]').exists()).toBe(false);
  });
});

function mountApp() {
  return mount(App, {
    global: {
      stubs: {
        AppSidebar: {
          name: "AppSidebar",
          props: ["modelValue"],
          emits: ["goToday", "logout", "selectDate", "update:modelValue"],
          template: '<aside data-testid="sidebar"></aside>',
        },
        AuthPanel: {
          name: "AuthPanel",
          emits: ["login", "signup"],
          template: '<section data-testid="auth-panel"></section>',
        },
        CalendarMonth: {
          name: "CalendarMonth",
          emits: ["select"],
          template: '<section data-testid="calendar-month"></section>',
        },
        DiaryEditor: {
          name: "DiaryEditor",
          props: ["navigationMessage"],
          methods: {
            flushPendingAutosave: () => false,
            toggleFocusMode: () => undefined,
          },
          template:
            '<section data-testid="diary-editor">{{ navigationMessage }}</section>',
        },
        EntryArchiveNav: {
          name: "EntryArchiveNav",
          template: '<section data-testid="entry-archive-nav"></section>',
        },
        EntryList: {
          name: "EntryList",
          template: '<section data-testid="entry-list"></section>',
        },
        EntryReader: {
          name: "EntryReader",
          template: '<section data-testid="entry-reader"></section>',
        },
        Filter: true,
        Search: true,
        SettingsPanel: {
          name: "SettingsPanel",
          template: '<section data-testid="settings-panel"></section>',
        },
        SetupPanel: {
          name: "SetupPanel",
          emits: ["createOwner"],
          template: '<section data-testid="setup-panel"></section>',
        },
      },
    },
  });
}

function setNavigatorOnline(value: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    configurable: true,
    value,
  });
}

function setupStatus(overrides: Partial<SetupStatus> = {}): SetupStatus {
  return {
    canCreateOwner: true,
    canRestoreBackup: true,
    needsSetup: false,
    requiresSetupToken: false,
    restoreInProgress: false,
    setupLocked: false,
    ...overrides,
  };
}

function user(): AuthUser {
  return {
    csrfToken: "csrf-token",
    id: 1,
    role: "owner",
    username: "nikki",
  };
}

function stats(): Stats {
  return {
    currentStreak: 0,
    lastEntryDate: "",
    moodCounts: {
      calm: 0,
      excited: 0,
      happy: 0,
      sad: 0,
      tired: 0,
    },
    totalEntries: 0,
  };
}

function entry(overrides: Partial<Entry> = {}): Entry {
  return {
    body: "Body",
    createdAt: "2026-06-01T00:00:00Z",
    entryDate: "2026-06-01",
    id: 1,
    images: [],
    mood: "calm",
    tags: [],
    title: "Title",
    updatedAt: "2026-06-01T00:00:00Z",
    version: 1,
    ...overrides,
  };
}
