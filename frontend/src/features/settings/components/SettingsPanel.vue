<script setup lang="ts">
import {
  Download,
  KeyRound,
  LogOut,
  RefreshCw,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-vue-next";
import { computed, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import LanguageSelect from "../../../shared/components/LanguageSelect.vue";
import type {
  AuthUser,
  ChangePasswordInput,
  DeleteAccountInput,
} from "../../auth/types";
import {
  readMemoryPreferences,
  writeMemoryPreferences,
} from "../../entries/memory-preferences";
import { moodOrder, moodSpecs } from "../../entries/moods";
import type { MoodKey } from "../../entries/types";
import type { AuditEvent } from "../types";

const props = defineProps<{
  error?: string;
  user: AuthUser;
  loading?: boolean;
  securityEvents?: AuditEvent[];
  securityEventsLoading?: boolean;
  securityEventsError?: string;
}>();

const emit = defineEmits<{
  changePassword: [input: ChangePasswordInput];
  deleteAccount: [input: DeleteAccountInput];
  logout: [];
  refreshSecurityEvents: [];
}>();

const { t } = useI18n();
const memoryPreferences = ref(readMemoryPreferences());
const accountErrorSource = ref<"change" | "delete" | null>(null);
const changePanelOpen = ref(false);
const changeError = ref("");
const changeForm = reactive({
  currentPassword: "",
  newPassword: "",
  confirmPassword: "",
});
const deletePanelOpen = ref(false);
const deleteError = ref("");
const deleteForm = reactive({
  username: "",
  password: "",
});
const canViewSecurityEvents = computed(() => props.user.role === "owner");
const visibleSecurityEvents = computed(() => props.securityEvents ?? []);

const deleteUsernameMatches = computed(
  () =>
    deleteForm.username.trim().toLowerCase() ===
    props.user.username.toLowerCase(),
);
const canSubmitDelete = computed(
  () =>
    deleteUsernameMatches.value &&
    deleteForm.password.trim().length > 0 &&
    !props.loading,
);
const canSubmitChange = computed(
  () =>
    changeForm.currentPassword.trim().length > 0 &&
    changeForm.newPassword.trim().length > 0 &&
    changeForm.confirmPassword.trim().length > 0 &&
    !props.loading,
);
const changeVisibleError = computed(
  () =>
    changeError.value ||
    (accountErrorSource.value === "change" ? props.error : "") ||
    "",
);
const deleteVisibleError = computed(
  () =>
    deleteError.value ||
    (accountErrorSource.value === "delete" ? props.error : "") ||
    "",
);

function setMemoryEnabled(event: Event) {
  const target = event.target as HTMLInputElement;
  updateMemoryPreferences({ enabled: target.checked });
}

function toggleExcludedMood(mood: MoodKey, event: Event) {
  const target = event.target as HTMLInputElement;
  const excluded = new Set(memoryPreferences.value.excludedMoods);
  if (target.checked) {
    excluded.add(mood);
  } else {
    excluded.delete(mood);
  }
  updateMemoryPreferences({
    excludedMoods: moodOrder.filter((key) => excluded.has(key)),
  });
}

function updateMemoryPreferences(
  next: Partial<typeof memoryPreferences.value>,
) {
  memoryPreferences.value = {
    ...memoryPreferences.value,
    ...next,
  };
  writeMemoryPreferences(memoryPreferences.value);
}

function openDeletePanel() {
  changePanelOpen.value = false;
  resetChangeForm();
  accountErrorSource.value = null;
  deletePanelOpen.value = true;
  deleteError.value = "";
  deleteForm.username = "";
  deleteForm.password = "";
}

function closeDeletePanel() {
  deletePanelOpen.value = false;
  accountErrorSource.value = null;
  resetDeleteForm();
}

function openChangePanel() {
  deletePanelOpen.value = false;
  resetDeleteForm();
  accountErrorSource.value = null;
  changePanelOpen.value = true;
  resetChangeForm();
}

function closeChangePanel() {
  changePanelOpen.value = false;
  accountErrorSource.value = null;
  resetChangeForm();
}

function resetChangeForm() {
  changeError.value = "";
  changeForm.currentPassword = "";
  changeForm.newPassword = "";
  changeForm.confirmPassword = "";
}

function resetDeleteForm() {
  deleteError.value = "";
  deleteForm.username = "";
  deleteForm.password = "";
}

function submitChangePassword() {
  accountErrorSource.value = "change";
  if (!changeForm.currentPassword.trim()) {
    changeError.value = t("settings.changePasswordCurrentRequired");
    return;
  }
  if (!validPasswordLength(changeForm.newPassword)) {
    changeError.value = t("settings.changePasswordLength");
    return;
  }
  if (changeForm.newPassword.trim() === changeForm.currentPassword.trim()) {
    changeError.value = t("settings.changePasswordSame");
    return;
  }
  if (changeForm.newPassword.trim() !== changeForm.confirmPassword.trim()) {
    changeError.value = t("settings.changePasswordMismatch");
    return;
  }

  changeError.value = "";
  emit("changePassword", {
    currentPassword: changeForm.currentPassword,
    newPassword: changeForm.newPassword,
  });
}

function submitDeleteAccount() {
  accountErrorSource.value = "delete";
  if (!deleteUsernameMatches.value) {
    deleteError.value = t("settings.deleteAccountUsernameMismatch");
    return;
  }
  if (!deleteForm.password.trim()) {
    deleteError.value = t("settings.deleteAccountPasswordRequired");
    return;
  }

  deleteError.value = "";
  emit("deleteAccount", {
    username: deleteForm.username,
    password: deleteForm.password,
  });
}

function validPasswordLength(value: string) {
  const length = value.trim().length;
  return length >= 8 && length <= 200;
}

function auditEventLabel(event: AuditEvent) {
  const key = auditEventLabelKeys[event.eventType];
  return key ? t(key) : event.eventType;
}

function auditOutcomeLabel(event: AuditEvent) {
  return t(
    event.outcome === "failed"
      ? "settings.securityOutcomeFailed"
      : "settings.securityOutcomeSucceeded",
  );
}

function auditEventDetail(event: AuditEvent) {
  const details = [
    actorDetail(event),
    event.remoteIp
      ? t("settings.securityRemoteIp", { ip: event.remoteIp })
      : "",
    event.reasonKind
      ? t("settings.securityReason", { reason: event.reasonKind })
      : "",
    targetDetail(event),
    metadataDetail(event),
  ].filter(Boolean);
  return details.join(" / ");
}

function actorDetail(event: AuditEvent) {
  if (event.actorUsername) {
    return t("settings.securityActor", { actor: event.actorUsername });
  }
  if (event.actorUserId) {
    return t("settings.securityActor", { actor: `#${event.actorUserId}` });
  }
  return "";
}

function targetDetail(event: AuditEvent) {
  if (!event.targetType || !event.targetId) {
    return "";
  }
  return t("settings.securityTarget", {
    target: `${event.targetType} #${event.targetId}`,
  });
}

function metadataDetail(event: AuditEvent) {
  const metadata = event.metadata ?? {};
  const keys = [
    "format",
    "entry_count",
    "image_count",
    "remaining_users",
    "bytes_out",
    "backup_size_bytes",
  ];
  return keys
    .filter((key) => metadata[key])
    .map((key) => `${metadataLabel(key)} ${metadata[key]}`)
    .join(", ");
}

function metadataLabel(key: string) {
  return t(auditMetadataLabelKeys[key] ?? "settings.securityMetadataValue");
}

function formatAuditTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

const auditEventLabelKeys: Record<string, string> = {
  "auth.signup_succeeded": "settings.auditEvents.authSignupSucceeded",
  "auth.signup_failed": "settings.auditEvents.authSignupFailed",
  "auth.login_succeeded": "settings.auditEvents.authLoginSucceeded",
  "auth.login_failed": "settings.auditEvents.authLoginFailed",
  "auth.logout_succeeded": "settings.auditEvents.authLogoutSucceeded",
  "auth.password_changed": "settings.auditEvents.authPasswordChanged",
  "auth.password_change_failed":
    "settings.auditEvents.authPasswordChangeFailed",
  "auth.account_deleted": "settings.auditEvents.authAccountDeleted",
  "auth.account_delete_failed": "settings.auditEvents.authAccountDeleteFailed",
  "auth.csrf_failed": "settings.auditEvents.authCsrfFailed",
  "auth.rate_limited": "settings.auditEvents.authRateLimited",
  "setup.owner_created": "settings.auditEvents.setupOwnerCreated",
  "setup.owner_create_failed": "settings.auditEvents.setupOwnerCreateFailed",
  "setup.restore_verified": "settings.auditEvents.setupRestoreVerified",
  "setup.restore_completed": "settings.auditEvents.setupRestoreCompleted",
  "setup.restore_failed": "settings.auditEvents.setupRestoreFailed",
  "export.completed": "settings.auditEvents.exportCompleted",
  "export.entry_markdown.completed":
    "settings.auditEvents.exportEntryMarkdownCompleted",
  "entries.deleted": "settings.auditEvents.entriesDeleted",
  "images.deleted": "settings.auditEvents.imagesDeleted",
};

const auditMetadataLabelKeys: Record<string, string> = {
  format: "settings.securityMetadataFormat",
  entry_count: "settings.securityMetadataEntries",
  image_count: "settings.securityMetadataImages",
  remaining_users: "settings.securityMetadataRemainingUsers",
  bytes_out: "settings.securityMetadataBytes",
  backup_size_bytes: "settings.securityMetadataBackupSize",
};
</script>

<template>
  <section class="settings ui-page">
    <h1 class="ui-heading">{{ t("settings.title") }}</h1>

    <div class="settings-block">
      <h2>{{ t("settings.account") }}</h2>
      <div class="account-card">
        <div class="account-summary">
          <span class="account-name">{{ user.username }}</span>
          <div class="account-actions">
            <button
              class="ui-action"
              data-testid="open-change-password"
              type="button"
              :disabled="loading"
              @click="openChangePanel"
            >
              <KeyRound :size="16" stroke-width="1.8" />
              <span>{{ t("settings.changePassword") }}</span>
            </button>
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
        <div class="account-danger-actions">
          <button
            class="ui-action danger-action"
            data-testid="open-delete-account"
            type="button"
            :disabled="loading"
            @click="openDeletePanel"
          >
            <Trash2 :size="16" stroke-width="1.8" />
            <span>{{ t("settings.deleteAccount") }}</span>
          </button>
        </div>
      </div>
      <form
        v-if="changePanelOpen"
        class="account-panel"
        data-testid="change-password-panel"
        @submit.prevent="submitChangePassword"
      >
        <div class="account-panel__head">
          <h3>{{ t("settings.changePassword") }}</h3>
          <button
            class="icon-button"
            type="button"
            :aria-label="t('common.close')"
            :disabled="loading"
            @click="closeChangePanel"
          >
            <X :size="16" stroke-width="1.8" />
          </button>
        </div>
        <p>{{ t("settings.changePasswordBody") }}</p>
        <label>
          <span>{{ t("settings.currentPassword") }}</span>
          <input
            v-model="changeForm.currentPassword"
            autocomplete="current-password"
            data-testid="change-password-current"
            type="password"
            :disabled="loading"
            :aria-invalid="Boolean(changeVisibleError)"
          />
        </label>
        <label>
          <span>{{ t("settings.newPassword") }}</span>
          <input
            v-model="changeForm.newPassword"
            autocomplete="new-password"
            data-testid="change-password-new"
            type="password"
            :disabled="loading"
            :aria-invalid="Boolean(changeVisibleError)"
          />
        </label>
        <label>
          <span>{{ t("settings.confirmNewPassword") }}</span>
          <input
            v-model="changeForm.confirmPassword"
            autocomplete="new-password"
            data-testid="change-password-confirm"
            type="password"
            :disabled="loading"
            :aria-invalid="Boolean(changeVisibleError)"
          />
        </label>
        <p v-if="changeVisibleError" class="account-panel__error">
          {{ changeVisibleError }}
        </p>
        <div class="account-panel__actions">
          <button
            class="ui-action"
            type="button"
            :disabled="loading"
            @click="closeChangePanel"
          >
            <X :size="16" stroke-width="1.8" />
            <span>{{ t("settings.changePasswordCancel") }}</span>
          </button>
          <button
            class="ui-action"
            data-testid="change-password-submit"
            type="submit"
            :disabled="!canSubmitChange"
          >
            <KeyRound :size="16" stroke-width="1.8" />
            <span>{{ t("settings.changePasswordSubmit") }}</span>
          </button>
        </div>
      </form>
      <form
        v-if="deletePanelOpen"
        class="danger-panel"
        data-testid="delete-account-panel"
        @submit.prevent="submitDeleteAccount"
      >
        <div class="danger-panel__head">
          <h3>{{ t("settings.deleteAccount") }}</h3>
          <button
            class="icon-button"
            type="button"
            :aria-label="t('common.close')"
            :disabled="loading"
            @click="closeDeletePanel"
          >
            <X :size="16" stroke-width="1.8" />
          </button>
        </div>
        <p>{{ t("settings.deleteAccountBody") }}</p>
        <label>
          <span>{{ t("settings.deleteAccountUsernameConfirm") }}</span>
          <input
            v-model="deleteForm.username"
            autocomplete="username"
            data-testid="delete-account-username"
            :disabled="loading"
            :aria-invalid="Boolean(deleteVisibleError)"
          />
        </label>
        <label>
          <span>{{ t("settings.deleteAccountPasswordConfirm") }}</span>
          <input
            v-model="deleteForm.password"
            autocomplete="current-password"
            data-testid="delete-account-password"
            type="password"
            :disabled="loading"
            :aria-invalid="Boolean(deleteVisibleError)"
          />
        </label>
        <p v-if="deleteVisibleError" class="danger-panel__error">
          {{ deleteVisibleError }}
        </p>
        <div class="danger-panel__actions">
          <button
            class="ui-action"
            type="button"
            :disabled="loading"
            @click="closeDeletePanel"
          >
            <X :size="16" stroke-width="1.8" />
            <span>{{ t("settings.deleteAccountCancel") }}</span>
          </button>
          <button
            class="ui-action danger-action"
            data-testid="delete-account-submit"
            type="submit"
            :disabled="!canSubmitDelete"
          >
            <Trash2 :size="16" stroke-width="1.8" />
            <span>{{ t("settings.deleteAccountSubmit") }}</span>
          </button>
        </div>
      </form>
    </div>

    <div v-if="canViewSecurityEvents" class="settings-block">
      <div class="security-head">
        <h2>{{ t("settings.securityHistory") }}</h2>
        <button
          class="icon-button"
          type="button"
          :aria-label="t('settings.securityRefresh')"
          :disabled="securityEventsLoading"
          @click="emit('refreshSecurityEvents')"
        >
          <RefreshCw :size="16" stroke-width="1.8" />
        </button>
      </div>
      <p v-if="securityEventsLoading" class="security-state">
        {{ t("settings.securityLoading") }}
      </p>
      <p v-else-if="securityEventsError" class="security-error">
        {{ securityEventsError }}
      </p>
      <p v-else-if="visibleSecurityEvents.length === 0" class="security-state">
        {{ t("settings.securityEmpty") }}
      </p>
      <ol v-else class="security-list">
        <li v-for="event in visibleSecurityEvents" :key="event.id">
          <div class="security-event-main">
            <ShieldCheck :size="15" stroke-width="1.8" />
            <strong>{{ auditEventLabel(event) }}</strong>
            <span
              class="security-outcome"
              :class="{ failed: event.outcome === 'failed' }"
            >
              {{ auditOutcomeLabel(event) }}
            </span>
          </div>
          <time :datetime="event.createdAt">
            {{ formatAuditTime(event.createdAt) }}
          </time>
          <p v-if="auditEventDetail(event)">
            {{ auditEventDetail(event) }}
          </p>
        </li>
      </ol>
    </div>

    <div class="settings-block">
      <h2>{{ t("settings.language") }}</h2>
      <LanguageSelect />
    </div>

    <div class="settings-block">
      <h2>{{ t("settings.memories") }}</h2>
      <label class="toggle-row">
        <input
          type="checkbox"
          :checked="memoryPreferences.enabled"
          @change="setMemoryEnabled"
        />
        <span>{{ t("settings.memoriesEnabled") }}</span>
      </label>
      <div class="mood-filter-group">
        <p>{{ t("settings.memoriesExcludedMoods") }}</p>
        <div class="mood-filter-list">
          <label v-for="mood in moodOrder" :key="mood" class="mood-filter">
            <input
              type="checkbox"
              :checked="memoryPreferences.excludedMoods.includes(mood)"
              @change="toggleExcludedMood(mood, $event)"
            />
            <span>
              <component :is="moodSpecs[mood].icon" :size="14" />
              {{ t(moodSpecs[mood].labelKey) }}
            </span>
          </label>
        </div>
      </div>
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
  display: grid;
  gap: 14px;
}

.account-summary {
  display: grid;
  gap: 12px;
}

.account-name {
  overflow: hidden;
  color: var(--color-text);
  font-size: 15px;
  font-weight: 680;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-start;
}

.account-danger-actions {
  display: flex;
  border-top: 1px solid var(--border-subtle);
  padding-top: 14px;
}

.danger-action {
  border-color: var(--color-danger-border);
  color: var(--color-danger);
}

.danger-action:hover:not(:disabled),
.danger-action:focus-visible {
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

.account-panel {
  display: grid;
  gap: 12px;
  margin-top: 14px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  padding: 14px;
}

.account-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.account-panel h3 {
  margin: 0;
  color: var(--color-text);
  font-size: 15px;
}

.account-panel p {
  margin: 0;
  color: var(--color-text-soft);
  font-size: 13px;
  line-height: 1.55;
}

.account-panel label {
  display: grid;
  gap: 6px;
  color: var(--color-muted);
  font-size: 12px;
}

.account-panel input {
  min-height: 38px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-bg);
  color: var(--color-text);
  padding: 0 10px;
  font: inherit;
}

.account-panel input:focus {
  border-color: var(--border-active);
  outline: 0;
}

.account-panel input[aria-invalid="true"] {
  border-color: var(--color-danger-border);
}

.account-panel__error {
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-danger-bg);
  color: var(--color-danger) !important;
  padding: 8px 10px;
}

.account-panel__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.security-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.security-head h2 {
  margin-bottom: 0;
}

.security-list {
  display: grid;
  gap: 0;
  margin: 0;
  padding: 0;
  list-style: none;
}

.security-list li {
  display: grid;
  gap: 5px;
  border-top: 1px solid var(--border-subtle);
  padding: 12px 0;
}

.security-list li:first-child {
  border-top: 0;
}

.security-event-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
}

.security-event-main strong {
  min-width: 0;
  overflow: hidden;
  color: var(--color-text);
  font-size: 13px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.security-event-main svg {
  flex: 0 0 auto;
  color: var(--color-muted);
}

.security-outcome {
  flex: 0 0 auto;
  margin-left: auto;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--color-muted);
  padding: 2px 6px;
  font-size: 11px;
}

.security-outcome.failed {
  border-color: var(--color-danger-border);
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

.security-list time,
.security-list p,
.security-state,
.security-error {
  margin: 0;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1.45;
}

.security-error {
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-danger-bg);
  color: var(--color-danger);
  padding: 8px 10px;
}

.danger-panel {
  display: grid;
  gap: 12px;
  margin-top: 14px;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-danger-bg);
  padding: 14px;
}

.danger-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.danger-panel h3 {
  margin: 0;
  color: var(--color-danger);
  font-size: 15px;
}

.danger-panel p {
  margin: 0;
  color: var(--color-text-soft);
  font-size: 13px;
  line-height: 1.55;
}

.danger-panel label {
  display: grid;
  gap: 6px;
  color: var(--color-muted);
  font-size: 12px;
}

.danger-panel input {
  min-height: 38px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 10px;
  font: inherit;
}

.danger-panel input:focus {
  border-color: var(--border-active);
  outline: 0;
}

.danger-panel input[aria-invalid="true"] {
  border-color: var(--color-danger-border);
}

.danger-panel__error {
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-danger) !important;
  padding: 8px 10px;
}

.danger-panel__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.icon-button {
  display: inline-grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
}

.icon-button:hover:not(:disabled),
.icon-button:focus-visible {
  border-color: var(--border-subtle);
  background: var(--color-surface);
  color: var(--color-text);
  outline: 0;
}

.export-link {
  min-height: 36px;
  text-decoration: none;
}

.toggle-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text);
  font-size: 14px;
}

.toggle-row input,
.mood-filter input {
  width: 15px;
  height: 15px;
}

.mood-filter-group {
  display: grid;
  gap: 8px;
  margin-top: 14px;
}

.mood-filter-group p {
  margin: 0;
  color: var(--color-muted);
  font-size: 12px;
  line-height: 1.45;
}

.mood-filter-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.mood-filter {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text-soft);
  padding: 7px 9px;
  font-size: 13px;
}

.mood-filter span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
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

  .account-actions,
  .account-danger-actions,
  .account-panel__actions,
  .danger-panel__actions {
    display: grid;
    grid-template-columns: 1fr;
  }

  .mood-filter-list {
    display: grid;
    grid-template-columns: 1fr;
  }
}
</style>
