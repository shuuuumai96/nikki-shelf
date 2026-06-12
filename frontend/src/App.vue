<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref } from "vue";
import { useAuthStore } from "./features/auth/store";
import type {
  AuthCredentials,
  ChangePasswordInput,
  DeleteAccountInput,
} from "./features/auth/types";
import { useEntryStore } from "./features/entries/store";
import {
  createOwner as createSetupOwner,
  status as fetchSetupStatus,
  restoreBackup as restoreSetupBackup,
  verifyRestore as verifySetupRestore,
} from "./features/setup/api";
import type {
  SetupOwnerInput,
  SetupRestoreFileInput,
  SetupRestoreInput,
  SetupStatus,
} from "./features/setup/types";
import { localizedErrorMessage } from "./shared/api/client";
import AppAuthenticatedShell from "./shared/components/AppAuthenticatedShell.vue";
import AppLoadingShell from "./shared/components/AppLoadingShell.vue";
import OfflineBanner from "./shared/components/OfflineBanner.vue";
import { useOnlineStatus } from "./shared/composables/useOnlineStatus";

const AuthPanel = defineAsyncComponent(
  () => import("./features/auth/components/AuthPanel.vue"),
);
const SetupPanel = defineAsyncComponent(
  () => import("./features/setup/components/SetupPanel.vue"),
);

const auth = useAuthStore();
const store = useEntryStore();
const setupStatus = ref<SetupStatus | null>(null);
const setupReady = ref(false);
const setupError = ref("");

const { isOffline } = useOnlineStatus();

const appReady = computed(() => auth.ready && setupReady.value);
const showSetup = computed(
  () => !auth.user && Boolean(setupStatus.value?.needsSetup),
);
const authPanelError = computed(() => auth.error || setupError.value);

onMounted(() => {
  void bootstrap();
});

async function bootstrap() {
  await auth.bootstrap();
  if (auth.user) {
    setupReady.value = true;
    if (window.location.pathname === "/setup") {
      replacePath("/");
    }
    await store.bootstrap();
    return;
  }

  await refreshSetupStatus();
  if (setupStatus.value?.needsSetup) {
    replacePath("/setup");
  } else if (window.location.pathname === "/setup") {
    replacePath("/");
  }
}

async function login(input: AuthCredentials) {
  await auth.login(input);
  replacePath("/");
  await store.bootstrap();
}

async function signup(input: AuthCredentials) {
  await auth.signup(input);
  replacePath("/");
  await store.bootstrap();
}

async function createOwner(input: SetupOwnerInput) {
  await auth.start(() => createSetupOwner(input));
  setupStatus.value = null;
  replacePath("/");
  await store.bootstrap();
}

async function verifyRestore(input: SetupRestoreFileInput) {
  return verifySetupRestore(input);
}

async function restoreBackup(input: SetupRestoreInput) {
  await restoreSetupBackup(input);
  auth.user = null;
  setupStatus.value = {
    needsSetup: false,
    setupLocked: true,
    canCreateOwner: false,
    canRestoreBackup: false,
    requiresSetupToken: true,
    restoreInProgress: false,
  };
  setupError.value = "";
  replacePath("/");
}

async function logout() {
  await auth.logout();
  store.clear();
  await refreshSetupStatus();
}

async function deleteAccount(input: DeleteAccountInput) {
  await auth.deleteAccount(input);
  store.clear();
  // Deleting the final user reopens setup, so route only after refreshing it.
  await refreshSetupStatus();
  if (setupStatus.value?.needsSetup) {
    replacePath("/setup");
  } else {
    replacePath("/");
  }
}

async function changePassword(input: ChangePasswordInput) {
  await auth.changePassword(input);
  store.clear();
  replacePath("/");
}

async function refreshSetupStatus() {
  setupReady.value = false;
  setupError.value = "";
  try {
    setupStatus.value = await fetchSetupStatus();
  } catch (error) {
    setupStatus.value = null;
    setupError.value = localizedErrorMessage(error);
  } finally {
    setupReady.value = true;
  }
}

function replacePath(path: string) {
  if (window.location.pathname !== path) {
    window.history.replaceState({}, "", path);
  }
}
</script>

<template>
  <AppLoadingShell v-if="!appReady" />

  <SetupPanel
    v-else-if="showSetup"
    :can-create-owner="setupStatus?.canCreateOwner"
    :can-restore-backup="setupStatus?.canRestoreBackup"
    :error="authPanelError"
    :loading="auth.loading"
    :restore-backup="restoreBackup"
    :setup-locked="setupStatus?.setupLocked"
    :verify-restore="verifyRestore"
    @create-owner="createOwner"
  />

  <AuthPanel
    v-else-if="!auth.user"
    :error="authPanelError"
    :loading="auth.loading"
    @login="login"
    @signup="signup"
  />

  <AppAuthenticatedShell
    v-else
    :auth-loading="auth.loading"
    :auth-error="auth.error"
    :store="store"
    :user="auth.user"
    @change-password="changePassword"
    @delete-account="deleteAccount"
    @logout="logout"
  />

  <OfflineBanner v-if="isOffline" />
</template>
