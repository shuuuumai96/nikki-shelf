import { defineStore } from "pinia";
import {
  ApiError,
  clearCSRFToken,
  setCSRFToken,
} from "../../shared/api/client";
import { i18n } from "../../shared/i18n";
import { login, logout, me, signup } from "./api";
import type { AuthCredentials, AuthUser } from "./types";

type AuthState = {
  user: AuthUser | null;
  ready: boolean;
  loading: boolean;
  error: string;
};

export const useAuthStore = defineStore("auth", {
  state: (): AuthState => ({
    user: null,
    ready: false,
    loading: false,
    error: "",
  }),
  actions: {
    async bootstrap() {
      this.loading = true;
      this.error = "";
      try {
        this.user = await me();
        rememberCSRF(this.user);
      } catch (error) {
        if (!(error instanceof ApiError && error.status === 401)) {
          this.error = errorMessage(error);
        }
        this.user = null;
      } finally {
        this.ready = true;
        this.loading = false;
      }
    },
    async signup(input: AuthCredentials) {
      await this.start(() => signup(input));
    },
    async login(input: AuthCredentials) {
      await this.start(() => login(input));
    },
    async logout() {
      this.loading = true;
      this.error = "";
      try {
        await logout();
      } finally {
        this.user = null;
        clearCSRFToken();
        this.loading = false;
      }
    },
    async start(action: () => Promise<AuthUser>) {
      this.loading = true;
      this.error = "";
      try {
        this.user = await action();
        rememberCSRF(this.user);
      } catch (error) {
        this.error = errorMessage(error);
        throw error;
      } finally {
        this.ready = true;
        this.loading = false;
      }
    },
  },
});

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return i18n.global.t("errors.generic");
}

function rememberCSRF(user: AuthUser | null) {
  if (user?.csrfToken) {
    setCSRFToken(user.csrfToken);
  }
}
