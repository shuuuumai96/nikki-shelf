<script setup lang="ts">
import {
  BookOpen,
  CalendarDays,
  Home,
  LogOut,
  Search,
  Settings,
} from "lucide-vue-next";
import { defineAsyncComponent } from "vue";
import { useI18n } from "vue-i18n";
import type { AuthUser } from "../../features/auth/types";
import EntryMemorySearchLoading from "../../features/entries/components/EntryMemorySearchLoading.vue";
import type { Stats } from "../../features/entries/types";
import { todayISO } from "../utils/date";

const EntryMemorySearch = defineAsyncComponent({
  delay: 0,
  loader: async () => {
    await afterNextPaint();
    return import("../../features/entries/components/EntryMemorySearch.vue");
  },
  loadingComponent: EntryMemorySearchLoading,
});

const props = defineProps<{
  modelValue: string;
  activeDate: string;
  stats: Stats | null;
  user: AuthUser;
  tags: string[];
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
  goToday: [];
  selectDate: [date: string];
  logout: [];
}>();

const { t } = useI18n();

const navItems = [
  { key: "today", labelKey: "nav.today", icon: Home },
  { key: "calendar", labelKey: "nav.calendar", icon: CalendarDays },
  { key: "entries", labelKey: "nav.entries", icon: Search },
  { key: "settings", labelKey: "nav.settings", icon: Settings },
];

function isNavItemActive(key: string) {
  if (key === "today") {
    return props.modelValue === "today" && props.activeDate === todayISO();
  }

  return props.modelValue === key;
}

function afterNextPaint() {
  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => resolve());
    });
  });
}
</script>

<template>
  <aside class="sidebar">
    <div class="brand">
      <BookOpen :size="19" stroke-width="1.8" />
      <span>{{ t("common.appName") }}</span>
    </div>

    <EntryMemorySearch
      class="memory-rail"
      :tags="tags"
      :selected-date="activeDate"
      @select-date="emit('selectDate', $event)"
    />

    <nav class="nav" :aria-label="t('nav.main')">
      <button
        v-for="item in navItems"
        :key="item.key"
        class="nav-item"
        :class="{ active: isNavItemActive(item.key) }"
        @click="
          item.key === 'today'
            ? emit('goToday')
            : emit('update:modelValue', item.key)
        "
      >
        <component :is="item.icon" :size="17" stroke-width="1.8" />
        <span>{{ t(item.labelKey) }}</span>
      </button>
    </nav>

    <div class="sidebar-footer">
      <div class="account-row">
        <div>
          <span>{{ t("nav.signedIn") }}</span>
          <strong>{{ user.username }}</strong>
        </div>
        <button
          type="button"
          :title="t('nav.logout')"
          :aria-label="t('nav.logout')"
          @click="emit('logout')"
        >
          <LogOut :size="16" stroke-width="1.8" />
        </button>
      </div>
      <div class="mini-stat">
        <span>{{ t("nav.entriesCount") }}</span>
        <strong>{{ stats?.totalEntries ?? 0 }}</strong>
      </div>
      <div class="mini-stat">
        <span>{{ t("nav.streak") }}</span>
        <strong>{{ stats?.currentStreak ?? 0 }}</strong>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  display: flex;
  height: 100dvh;
  min-height: 0;
  flex-direction: column;
  overflow-y: auto;
  border-right: 1px solid var(--color-border);
  background: var(--color-sidebar);
  padding: 16px 12px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 9px;
  height: 38px;
  border: 0;
  border-radius: var(--radius-md);
  background: transparent;
  padding: 0 10px;
  color: var(--color-text);
  font-size: 16px;
  font-weight: 760;
}

.brand svg {
  color: var(--color-text);
}

.nav {
  display: grid;
  gap: 6px;
  margin-top: 16px;
}

.nav-item {
  display: grid;
  grid-template-columns: 22px 1fr;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  padding: 0 8px;
  text-align: left;
  transition:
    background-color 140ms ease,
    border-color 140ms ease,
    color 140ms ease;
}

.nav-item:hover {
  background: var(--surface-glass-soft);
  color: var(--color-text);
}

.nav-item.active {
  border-color: var(--border-active);
  background: var(--surface-active);
  color: var(--color-text);
}

.nav-item span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-footer {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: auto;
}

.account-row {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: 1fr 34px;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  padding: 8px;
}

.account-row span {
  display: block;
  color: var(--color-muted);
  font-size: 11px;
}

.account-row strong {
  display: block;
  overflow: hidden;
  color: var(--color-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-row button {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
}

.account-row button:hover {
  border-color: var(--border-active);
  background: var(--surface-hover);
  color: var(--color-text);
}

.mini-stat {
  min-width: 0;
}

.mini-stat {
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  padding: 8px;
}

.mini-stat span {
  display: block;
  color: var(--color-muted);
  font-size: 11px;
}

.mini-stat strong {
  display: block;
  margin-top: 2px;
  color: var(--color-text);
  font-size: 18px;
}

@media (max-width: 820px) {
  .sidebar {
    height: auto;
    min-height: auto;
    border-right: 0;
    border-bottom: 1px solid var(--color-border);
  }

  .nav {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .memory-rail,
  .sidebar :deep(.memory-rail) {
    display: none;
  }

  .sidebar-footer {
    display: none;
  }
}

@media (max-width: 640px) {
  .sidebar {
    position: fixed;
    z-index: 20;
    right: 0;
    bottom: 0;
    left: 0;
    display: block;
    height: auto;
    border-top: 1px solid var(--border-control);
    border-bottom: 0;
    background: color-mix(in srgb, var(--color-sidebar) 96%, transparent);
    box-shadow: 0 -10px 24px rgba(20, 20, 20, 0.08);
    padding: 6px max(12px, env(safe-area-inset-right))
      calc(6px + env(safe-area-inset-bottom))
      max(12px, env(safe-area-inset-left));
    backdrop-filter: blur(12px);
  }

  .brand {
    display: none;
  }

  .nav {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 4px;
    margin-top: 0;
  }

  .nav-item {
    min-height: 48px;
    grid-template-columns: 1fr;
    justify-items: center;
    gap: 3px;
    padding: 5px 2px 4px;
    font-size: 11px;
    line-height: 1.2;
    text-align: center;
  }
  .nav-item span {
    max-width: 100%;
  }
}
</style>
