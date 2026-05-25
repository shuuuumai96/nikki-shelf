<script setup lang="ts">
import { Download, LogOut } from "lucide-vue-next";
import { useI18n } from "vue-i18n";
import LanguageSelect from "../../../shared/components/LanguageSelect.vue";
import type { AuthUser } from "../../auth/types";

defineProps<{
  user: AuthUser;
  loading?: boolean;
}>();

const emit = defineEmits<{
  logout: [];
}>();

const { t } = useI18n();
</script>

<template>
  <section class="settings ui-page">
    <h1 class="ui-heading">{{ t("settings.title") }}</h1>

    <div class="settings-block">
      <h2>{{ t("settings.account") }}</h2>
      <div class="account-card">
        <span>{{ user.username }}</span>
        <button
          class="ui-action"
          type="button"
          :disabled="loading"
          @click="emit('logout')"
        >
          <LogOut :size="16" stroke-width="1.8" />
          <span>{{ t("nav.logout") }}</span>
        </button>
      </div>
    </div>

    <div class="settings-block">
      <h2>{{ t("settings.language") }}</h2>
      <LanguageSelect />
    </div>

    <div class="settings-block">
      <h2>{{ t("settings.export") }}</h2>
      <div class="export-actions">
        <a class="export-link ui-action" href="/api/export/json">
          <Download :size="16" stroke-width="1.8" />
          <span>{{ t("export.json") }}</span>
        </a>
        <a class="export-link ui-action" href="/api/export/markdown">
          <Download :size="16" stroke-width="1.8" />
          <span>{{ t("export.markdown") }}</span>
        </a>
        <a class="export-link ui-action" href="/api/export/backup">
          <Download :size="16" stroke-width="1.8" />
          <span>{{ t("export.backup") }}</span>
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.settings {
  --page-max: 760px;
}

h1 {
  margin: 0 0 26px;
  font-size: 28px;
}

.settings-block {
  border-top: 1px solid var(--border-editor);
  padding-top: 20px;
}

.settings-block + .settings-block {
  margin-top: 22px;
}

h2 {
  margin: 0 0 12px;
  color: var(--color-muted);
  font-size: 14px;
  font-weight: 650;
  letter-spacing: 0;
}

.export-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.account-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.account-card > span {
  overflow: hidden;
  color: var(--color-text);
  font-size: 15px;
  font-weight: 680;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.export-link {
  min-height: 36px;
  text-decoration: none;
}

@media (max-width: 480px) {
  h1 {
    margin-bottom: 22px;
    font-size: 24px;
  }

  .export-actions {
    display: grid;
    grid-template-columns: 1fr;
  }

  .account-card {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
