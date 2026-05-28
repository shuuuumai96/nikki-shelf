<script setup lang="ts">
import { Upload } from "lucide-vue-next";
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { localizedErrorMessage } from "../../../shared/api/client";
import LanguageSelect from "../../../shared/components/LanguageSelect.vue";
import type {
  SetupOwnerInput,
  SetupRestoreFileInput,
  SetupRestoreInput,
  SetupRestoreVerifyResponse,
} from "../types";

const props = defineProps<{
  error?: string;
  loading?: boolean;
  canCreateOwner?: boolean;
  canRestoreBackup?: boolean;
  setupLocked?: boolean;
  verifyRestore: (
    value: SetupRestoreFileInput,
  ) => Promise<SetupRestoreVerifyResponse>;
  restoreBackup: (value: SetupRestoreInput) => Promise<unknown>;
}>();

const emit = defineEmits<{
  createOwner: [value: SetupOwnerInput];
}>();

const { t } = useI18n();
const ownerError = ref("");
const restoreError = ref("");
const restoreLoading = ref<"" | "verify" | "restore">("");
const restoreResult = ref<SetupRestoreVerifyResponse | null>(null);
const ownerForm = reactive({
  setupToken: "",
  username: "owner",
  password: "",
  passwordConfirmation: "",
});
const restoreForm = reactive({
  setupToken: "",
  backupFile: null as File | null,
  confirmRestore: false,
});

const ownerVisibleError = computed(() => ownerError.value || props.error || "");
const restoreVisibleError = computed(() => restoreError.value);
const ownerDescribedBy = computed(() =>
  ownerVisibleError.value ? "setup-owner-error setup-note" : "setup-note",
);
const restoreDescribedBy = computed(() =>
  restoreVisibleError.value
    ? "setup-restore-error setup-restore-note"
    : "setup-restore-note",
);
const backupSize = computed(() => {
  if (!restoreResult.value) {
    return "";
  }
  return formatBytes(restoreResult.value.backupSizeBytes);
});
const selectedBackupLabel = computed(
  () => restoreForm.backupFile?.name || t("setup.backupFilePlaceholder"),
);

watch(
  () => [
    ownerForm.setupToken,
    ownerForm.username,
    ownerForm.password,
    ownerForm.passwordConfirmation,
  ],
  () => {
    ownerError.value = "";
  },
);

watch(
  () => [restoreForm.setupToken, restoreForm.backupFile],
  () => {
    restoreError.value = "";
    restoreResult.value = null;
    restoreForm.confirmRestore = false;
  },
);

function submitOwner() {
  if (props.setupLocked || !props.canCreateOwner) {
    ownerError.value = t("setup.locked");
    return;
  }

  if (ownerForm.password !== ownerForm.passwordConfirmation) {
    ownerError.value = t("setup.passwordMismatch");
    return;
  }

  emit("createOwner", {
    setupToken: ownerForm.setupToken,
    username: ownerForm.username,
    password: ownerForm.password,
  });
}

function selectBackupFile(event: Event) {
  const input = event.target as HTMLInputElement;
  restoreForm.backupFile = input.files?.[0] ?? null;
}

async function verifyBackup() {
  if (props.setupLocked || !props.canRestoreBackup) {
    restoreError.value = t("setup.locked");
    return;
  }
  if (!restoreForm.setupToken || !restoreForm.backupFile) {
    restoreError.value = t("setup.restoreMissingInput");
    return;
  }

  restoreLoading.value = "verify";
  restoreError.value = "";
  try {
    restoreResult.value = await props.verifyRestore({
      setupToken: restoreForm.setupToken,
      backupFile: restoreForm.backupFile,
    });
  } catch (error) {
    restoreResult.value = null;
    restoreError.value = localizedErrorMessage(error);
  } finally {
    restoreLoading.value = "";
  }
}

async function restoreBackup() {
  if (!restoreResult.value || !restoreForm.backupFile) {
    restoreError.value = t("setup.restoreVerifyFirst");
    return;
  }
  if (!restoreForm.confirmRestore) {
    restoreError.value = t("setup.restoreConfirmRequired");
    return;
  }

  restoreLoading.value = "restore";
  restoreError.value = "";
  try {
    await props.restoreBackup({
      setupToken: restoreForm.setupToken,
      backupFile: restoreForm.backupFile,
      confirmRestore: true,
    });
  } catch (error) {
    restoreError.value = localizedErrorMessage(error);
  } finally {
    restoreLoading.value = "";
  }
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}
</script>

<template>
  <main class="setup-screen">
    <section class="setup-panel" :aria-label="t('setup.panelLabel')">
      <div class="setup-chrome">
        <div class="setup-brand">
          <span>{{ t("common.appName") }}</span>
        </div>
        <LanguageSelect />
      </div>

      <div class="setup-copy">
        <h1>{{ t("setup.heading") }}</h1>
        <p>{{ t("setup.lead") }}</p>
        <p id="setup-note" class="setup-note">
          {{ t("setup.publicWarning") }}
        </p>
        <p class="setup-note">
          {{ t("setup.tokenHelp") }}
        </p>
      </div>

      <div class="setup-options">
        <form
          class="setup-option"
          :aria-describedby="ownerDescribedBy"
          @submit.prevent="submitOwner"
        >
          <div class="option-heading">
            <h2>{{ t("setup.startFreshTitle") }}</h2>
            <p>{{ t("setup.startFreshBody") }}</p>
          </div>

          <div class="field">
            <label for="setup-token">{{ t("setup.token") }}</label>
            <input
              id="setup-token"
              v-model="ownerForm.setupToken"
              :aria-invalid="Boolean(ownerVisibleError)"
              autocomplete="off"
              required
              type="password"
            />
          </div>

          <div class="field">
            <label for="setup-username">{{ t("auth.username") }}</label>
            <input
              id="setup-username"
              v-model="ownerForm.username"
              :aria-invalid="Boolean(ownerVisibleError)"
              autocomplete="username"
              :placeholder="t('auth.usernamePlaceholder')"
              required
            />
          </div>

          <div class="field">
            <label for="setup-password">{{ t("auth.password") }}</label>
            <input
              id="setup-password"
              v-model="ownerForm.password"
              :aria-invalid="Boolean(ownerVisibleError)"
              autocomplete="new-password"
              minlength="8"
              :placeholder="t('auth.passwordPlaceholder')"
              required
              type="password"
            />
          </div>

          <div class="field">
            <label for="setup-password-confirmation">{{
              t("setup.passwordConfirmation")
            }}</label>
            <input
              id="setup-password-confirmation"
              v-model="ownerForm.passwordConfirmation"
              :aria-invalid="Boolean(ownerVisibleError)"
              autocomplete="new-password"
              minlength="8"
              :placeholder="t('auth.passwordPlaceholder')"
              required
              type="password"
            />
          </div>

          <p
            v-if="ownerVisibleError"
            id="setup-owner-error"
            class="setup-error"
            role="alert"
          >
            {{ ownerVisibleError }}
          </p>

          <button
            class="setup-submit"
            type="submit"
            :disabled="loading || setupLocked || !canCreateOwner"
          >
            <span
              v-if="loading"
              class="setup-spinner"
              aria-hidden="true"
            ></span>
            <span>{{
              loading ? t("common.loading") : t("setup.createOwner")
            }}</span>
          </button>
        </form>

        <form
          class="setup-option"
          :aria-describedby="restoreDescribedBy"
          @submit.prevent="restoreBackup"
        >
          <div class="option-heading">
            <h2>{{ t("setup.restoreTitle") }}</h2>
            <p id="setup-restore-note">{{ t("setup.restoreBody") }}</p>
          </div>

          <div class="field">
            <label for="setup-restore-token">{{ t("setup.token") }}</label>
            <input
              id="setup-restore-token"
              v-model="restoreForm.setupToken"
              :aria-invalid="Boolean(restoreVisibleError)"
              autocomplete="off"
              required
              type="password"
            />
          </div>

          <div class="field">
            <label for="setup-backup-file">{{ t("setup.backupFile") }}</label>
            <label
              class="file-picker"
              :class="{ selected: restoreForm.backupFile }"
              for="setup-backup-file"
            >
              <input
                id="setup-backup-file"
                class="sr-only"
                :aria-invalid="Boolean(restoreVisibleError)"
                accept=".tar.gz,.tgz,application/gzip"
                required
                type="file"
                @change="selectBackupFile"
              />
              <span class="file-picker-action">
                <Upload :size="15" stroke-width="1.9" />
                <span>{{ t("setup.chooseBackupFile") }}</span>
              </span>
              <span class="file-picker-name">{{ selectedBackupLabel }}</span>
            </label>
          </div>

          <div class="restore-actions">
            <button
              class="setup-secondary"
              type="button"
              :disabled="
                restoreLoading !== '' ||
                setupLocked ||
                !canRestoreBackup ||
                !restoreForm.setupToken ||
                !restoreForm.backupFile
              "
              @click="verifyBackup"
            >
              <span
                v-if="restoreLoading === 'verify'"
                class="setup-spinner secondary"
                aria-hidden="true"
              ></span>
              <span>{{
                restoreLoading === "verify"
                  ? t("common.loading")
                  : t("setup.verifyBackup")
              }}</span>
            </button>
          </div>

          <div v-if="restoreResult" class="restore-result">
            <strong>{{ t("setup.restoreVerified") }}</strong>
            <dl>
              <div>
                <dt>{{ t("setup.backupCreatedAt") }}</dt>
                <dd>{{ restoreResult.backupCreatedAt || "-" }}</dd>
              </div>
              <div>
                <dt>{{ t("setup.nikkiVersion") }}</dt>
                <dd>{{ restoreResult.nikkiVersion || "-" }}</dd>
              </div>
              <div>
                <dt>{{ t("setup.schemaVersion") }}</dt>
                <dd>{{ restoreResult.schemaVersion || "-" }}</dd>
              </div>
              <div>
                <dt>{{ t("setup.entryCount") }}</dt>
                <dd>{{ restoreResult.entryCount }}</dd>
              </div>
              <div>
                <dt>{{ t("setup.imageCount") }}</dt>
                <dd>{{ restoreResult.imageCount }}</dd>
              </div>
              <div>
                <dt>{{ t("setup.backupSize") }}</dt>
                <dd>{{ backupSize }}</dd>
              </div>
            </dl>
            <p v-if="restoreResult.warnings.length" class="setup-note">
              {{ restoreResult.warnings.join(", ") }}
            </p>
          </div>

          <label v-if="restoreResult" class="restore-confirm">
            <input v-model="restoreForm.confirmRestore" type="checkbox" />
            <span>{{ t("setup.restoreConfirm") }}</span>
          </label>

          <p
            v-if="restoreVisibleError"
            id="setup-restore-error"
            class="setup-error"
            role="alert"
          >
            {{ restoreVisibleError }}
          </p>

          <button
            class="setup-submit"
            type="submit"
            :disabled="
              restoreLoading !== '' ||
              setupLocked ||
              !canRestoreBackup ||
              !restoreResult ||
              !restoreForm.confirmRestore
            "
          >
            <span
              v-if="restoreLoading === 'restore'"
              class="setup-spinner"
              aria-hidden="true"
            ></span>
            <span>{{
              restoreLoading === "restore"
                ? t("common.loading")
                : t("setup.restoreBackup")
            }}</span>
          </button>
        </form>
      </div>
    </section>
  </main>
</template>

<style scoped>
.setup-screen {
  display: grid;
  min-height: 100dvh;
  place-items: center;
  background: var(--color-wash);
  padding: 42px 18px;
}

.setup-panel {
  width: min(860px, 100%);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 28px;
}

.setup-chrome {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.setup-brand {
  display: inline-flex;
  align-items: center;
  color: var(--color-text);
}

.setup-brand span {
  font-size: 18px;
  font-weight: 760;
}

.setup-copy {
  margin: 28px 0 20px;
}

.setup-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.setup-option {
  display: grid;
  align-content: start;
  gap: 15px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: 18px;
}

.option-heading {
  min-height: 84px;
}

h1,
h2 {
  margin: 0;
  color: var(--color-text);
  letter-spacing: 0;
  line-height: 1.25;
}

h1 {
  font-size: 27px;
  font-weight: 720;
}

h2 {
  font-size: 18px;
  font-weight: 720;
}

p {
  margin: 9px 0 0;
  color: var(--color-muted);
  font-size: 14px;
  line-height: 1.6;
}

.setup-note {
  color: var(--color-text-note);
  font-size: 12px;
}

.field {
  display: grid;
  gap: 7px;
}

label,
dt {
  color: var(--color-muted);
  font-size: 12px;
  font-weight: 680;
}

input {
  width: 100%;
  min-height: 46px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-glass);
  color: var(--color-text);
  padding: 0 12px;
  outline: 0;
  transition:
    border-color 140ms ease,
    box-shadow 140ms ease,
    background-color 140ms ease;
}

input[type="checkbox"] {
  width: 16px;
  min-height: 16px;
  margin: 0;
}

input:focus {
  border-color: var(--border-active);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-text) 8%, transparent);
}

.file-picker {
  display: grid;
  min-height: 46px;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-glass);
  color: var(--color-muted);
  padding: 6px;
  cursor: pointer;
  transition:
    border-color 140ms ease,
    box-shadow 140ms ease,
    background-color 140ms ease;
}

.file-picker:hover {
  border-color: var(--border-active);
  background: var(--surface-hover);
}

.file-picker:focus-within {
  border-color: var(--border-active);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-text) 8%, transparent);
}

.file-picker.selected {
  color: var(--color-text);
}

.file-picker-action {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  padding: 0 10px;
  font-size: 12px;
  font-weight: 680;
  white-space: nowrap;
}

.file-picker-name {
  min-width: 0;
  overflow: hidden;
  color: inherit;
  font-size: 13px;
  font-weight: 520;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.setup-error {
  margin: 0;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-md);
  background: var(--color-danger-bg);
  color: var(--color-danger);
  padding: 10px 11px;
  font-size: 13px;
  line-height: 1.55;
}

.setup-submit,
.setup-secondary {
  display: inline-flex;
  min-height: 46px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: var(--radius-md);
  font-weight: 680;
  transition:
    background-color 140ms ease,
    border-color 140ms ease,
    opacity 140ms ease;
}

.setup-submit {
  border: 1px solid var(--color-text);
  background: var(--color-text);
  color: #ffffff;
}

.setup-secondary {
  width: 100%;
  border: 1px solid var(--border-subtle);
  background: var(--color-surface);
  color: var(--color-text);
}

.setup-submit:hover:not(:disabled) {
  background: var(--color-accent);
}

.setup-secondary:hover:not(:disabled) {
  border-color: var(--border-active);
  background: var(--surface-hover);
}

.setup-submit:disabled,
.setup-secondary:disabled {
  opacity: 0.68;
}

.setup-spinner {
  width: 15px;
  height: 15px;
  border: 2px solid rgba(255, 255, 255, 0.38);
  border-top-color: #ffffff;
  border-radius: 999px;
  animation: setup-spin 760ms linear infinite;
}

.setup-spinner.secondary {
  border-color: color-mix(in srgb, var(--color-text) 22%, transparent);
  border-top-color: var(--color-text);
}

.restore-actions {
  display: grid;
}

.restore-result {
  display: grid;
  gap: 10px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-glass);
  padding: 12px;
}

.restore-result strong {
  color: var(--color-text);
  font-size: 13px;
}

.restore-result dl {
  display: grid;
  gap: 7px;
  margin: 0;
}

.restore-result dl div {
  display: flex;
  justify-content: space-between;
  gap: 10px;
}

.restore-result dd {
  margin: 0;
  color: var(--color-text);
  font-size: 12px;
  text-align: right;
}

.restore-confirm {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: start;
  gap: 9px;
  color: var(--color-muted);
  line-height: 1.5;
}

@keyframes setup-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .setup-spinner {
    animation: none;
  }

  .setup-submit,
  .setup-secondary,
  input {
    transition: none;
  }
}

@media (max-width: 760px) {
  .setup-options {
    grid-template-columns: 1fr;
  }

  .option-heading {
    min-height: 0;
  }
}

@media (max-width: 480px) {
  .setup-screen {
    align-items: stretch;
    place-items: start center;
    background: var(--color-bg);
    padding: 32px 18px 28px;
  }

  .setup-panel {
    border-color: transparent;
    background: transparent;
    padding: 0;
  }

  .setup-option {
    padding: 16px;
  }

  h1 {
    font-size: 25px;
  }

  .setup-copy {
    margin-top: 26px;
  }

  input,
  .setup-submit,
  .setup-secondary {
    min-height: 48px;
  }

  input {
    font-size: 16px;
  }
}
</style>
