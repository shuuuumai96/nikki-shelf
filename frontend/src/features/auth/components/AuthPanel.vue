<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import LanguageSelect from "../../../shared/components/LanguageSelect.vue";
import { config as fetchAuthConfig } from "../api";
import type { AuthCredentials, SignupMode } from "../types";

defineProps<{
  error?: string;
  loading?: boolean;
}>();

const emit = defineEmits<{
  login: [value: AuthCredentials];
  signup: [value: AuthCredentials];
}>();

const mode = ref<"login" | "signup">("login");
const signupMode = ref<SignupMode>("closed");
const passwordVisible = ref(false);
const { t } = useI18n();
const form = reactive<AuthCredentials>({
  username: "",
  password: "",
});

const heading = computed(() =>
  mode.value === "login" ? t("auth.login") : signupHeading.value,
);
const subtitle = computed(() =>
  mode.value === "login" ? t("auth.loginSubtitle") : signupSubtitle.value,
);
const actionLabel = computed(() =>
  mode.value === "login" ? t("auth.login") : signupAction.value,
);
const showSignup = computed(() => signupMode.value !== "closed");
const signupHeading = computed(() =>
  signupMode.value === "setup"
    ? t("auth.setupHeading")
    : t("auth.signupHeading"),
);
const signupSubtitle = computed(() =>
  signupMode.value === "setup"
    ? t("auth.setupSubtitle")
    : t("auth.signupSubtitle"),
);
const signupAction = computed(() =>
  signupMode.value === "setup" ? t("auth.setupAction") : t("auth.signupAction"),
);
const signupNote = computed(() =>
  signupMode.value === "setup" ? t("auth.setupNote") : t("auth.signupNote"),
);
const passwordAutocomplete = computed(() =>
  mode.value === "login" ? "current-password" : "new-password",
);
const passwordInputType = computed(() =>
  passwordVisible.value ? "text" : "password",
);

onMounted(async () => {
  try {
    const authConfig = await fetchAuthConfig();
    signupMode.value = authConfig.signupAvailable
      ? authConfig.signupMode
      : "closed";
    if (signupMode.value === "setup") {
      mode.value = "signup";
    }
  } catch {
    signupMode.value = "closed";
  }
});

watch(signupMode, (value) => {
  if (value === "closed") {
    mode.value = "login";
  }
});

function submit() {
  const payload = {
    username: form.username,
    password: form.password,
  };
  if (mode.value === "login") {
    emit("login", payload);
    return;
  }
  if (!showSignup.value) {
    return;
  }
  emit("signup", payload);
}
</script>

<template>
  <main class="auth-screen">
    <section class="auth-panel" :aria-label="t('auth.panelLabel')">
      <div class="auth-chrome">
        <div class="auth-brand">
          <span>{{ t("common.appName") }}</span>
        </div>
        <LanguageSelect />
      </div>

      <div class="auth-copy">
        <h1>{{ heading }}</h1>
        <p>{{ subtitle }}</p>
      </div>

      <div
        v-if="showSignup"
        class="mode-switch"
        role="group"
        :aria-label="t('auth.modeLabel')"
      >
        <button
          type="button"
          :aria-pressed="mode === 'login'"
          :class="{ active: mode === 'login' }"
          @click="mode = 'login'"
        >
          {{ t("auth.login") }}
        </button>
        <button
          type="button"
          :aria-pressed="mode === 'signup'"
          :class="{ active: mode === 'signup' }"
          @click="mode = 'signup'"
        >
          {{ t("auth.signup") }}
        </button>
      </div>

      <form
        :aria-describedby="error ? 'auth-error' : undefined"
        @submit.prevent="submit"
      >
        <div class="field">
          <label for="auth-username">{{ t("auth.username") }}</label>
          <input
            id="auth-username"
            v-model="form.username"
            :aria-invalid="Boolean(error)"
            autocomplete="username"
            :placeholder="t('auth.usernamePlaceholder')"
            required
          />
        </div>

        <div class="field">
          <label for="auth-password">{{ t("auth.password") }}</label>
          <div class="password-field">
            <input
              id="auth-password"
              v-model="form.password"
              :aria-invalid="Boolean(error)"
              :autocomplete="passwordAutocomplete"
              minlength="8"
              :placeholder="t('auth.passwordPlaceholder')"
              required
              :type="passwordInputType"
            />
            <button
              type="button"
              class="password-toggle"
              :aria-label="
                passwordVisible
                  ? t('auth.hidePassword')
                  : t('auth.showPassword')
              "
              @click="passwordVisible = !passwordVisible"
            >
              {{ passwordVisible ? t("auth.hide") : t("auth.show") }}
            </button>
          </div>
        </div>

        <p v-if="mode === 'signup'" class="auth-note">
          {{ signupNote }}
        </p>
        <p v-if="error" id="auth-error" class="auth-error" role="alert">
          {{ error }}
        </p>

        <button class="auth-submit" type="submit" :disabled="loading">
          <span v-if="loading" class="auth-spinner" aria-hidden="true"></span>
          <span>{{ loading ? t("common.loading") : actionLabel }}</span>
        </button>
      </form>
    </section>
  </main>
</template>

<style scoped>
.auth-screen {
  display: grid;
  min-height: 100dvh;
  place-items: center;
  background: var(--color-wash);
  padding: 48px 18px;
}

.auth-panel {
  width: min(400px, 100%);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 28px;
}

.auth-chrome {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.auth-brand {
  display: inline-flex;
  align-items: center;
  color: var(--color-text);
}

.auth-brand span {
  font-size: 18px;
  font-weight: 760;
}

.auth-copy {
  margin: 28px 0 20px;
}

h1 {
  margin: 0;
  color: var(--color-text);
  font-size: 27px;
  font-weight: 720;
  letter-spacing: 0;
  line-height: 1.25;
}

p {
  margin: 9px 0 0;
  color: var(--color-muted);
  font-size: 14px;
}

.mode-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-glass-soft);
  padding: 3px;
}

.mode-switch button {
  min-height: 38px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  padding: 0 10px;
  font-size: 13px;
  font-weight: 650;
  transition:
    background-color 120ms ease,
    border-color 120ms ease,
    color 140ms ease;
}

.mode-switch button:hover,
.mode-switch button.active {
  color: var(--color-text);
}

.mode-switch button.active {
  border-color: var(--border-subtle);
  background: var(--color-surface);
}

form {
  display: grid;
  gap: 15px;
  margin-top: 22px;
}

.field {
  display: grid;
  gap: 7px;
}

label {
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

input:focus {
  border-color: var(--border-active);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-text) 8%, transparent);
}

.password-field {
  position: relative;
}

.password-field input {
  padding-right: 58px;
}

.password-toggle {
  position: absolute;
  top: 50%;
  right: 7px;
  min-height: 32px;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  padding: 0 8px;
  font-size: 12px;
  font-weight: 650;
  transform: translateY(-50%);
}

.password-toggle:hover {
  background: var(--surface-hover);
  color: var(--color-text);
}

.auth-note {
  margin: -2px 0 0;
  color: var(--color-text-note);
  font-size: 12px;
  line-height: 1.6;
}

.auth-error {
  margin: 0;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-md);
  background: var(--color-danger-bg);
  color: var(--color-danger);
  padding: 10px 11px;
  font-size: 13px;
  line-height: 1.55;
}

.auth-submit {
  display: inline-flex;
  min-height: 46px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid var(--color-text);
  border-radius: var(--radius-md);
  background: var(--color-text);
  color: #ffffff;
  font-weight: 680;
  transition:
    background-color 140ms ease,
    border-color 140ms ease,
    opacity 140ms ease;
}

.auth-submit:hover:not(:disabled) {
  background: var(--color-accent);
}

.auth-submit:disabled {
  opacity: 0.68;
}

.auth-spinner {
  width: 15px;
  height: 15px;
  border: 2px solid rgba(255, 255, 255, 0.38);
  border-top-color: #ffffff;
  border-radius: 999px;
  animation: auth-spin 760ms linear infinite;
}

@keyframes auth-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-spinner {
    animation: none;
  }

  .auth-submit,
  .mode-switch button,
  input {
    transition: none;
  }
}

@media (max-width: 480px) {
  .auth-screen {
    align-items: stretch;
    place-items: start center;
    background: var(--color-bg);
    padding: 36px 18px 28px;
  }

  .auth-panel {
    border-color: transparent;
    background: transparent;
    padding: 0;
  }

  h1 {
    font-size: 25px;
  }

  .auth-copy {
    margin-top: 26px;
  }

  input,
  .auth-submit {
    min-height: 48px;
  }

  input {
    font-size: 16px;
  }
}
</style>
