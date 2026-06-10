import { request } from "../../shared/api/client";
import type {
  AuthConfig,
  AuthCredentials,
  AuthUser,
  DeleteAccountInput,
} from "./types";

export function config(): Promise<AuthConfig> {
  return request<AuthConfig>("/api/auth/config");
}

export function signup(input: AuthCredentials): Promise<AuthUser> {
  return request<AuthUser>("/api/auth/signup", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function login(input: AuthCredentials): Promise<AuthUser> {
  return request<AuthUser>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function logout(): Promise<void> {
  return request<void>("/api/auth/logout", { method: "POST" });
}

export function deleteAccount(input: DeleteAccountInput): Promise<void> {
  return request<void>("/api/auth/me", {
    method: "DELETE",
    body: JSON.stringify(input),
  });
}

export function me(): Promise<AuthUser> {
  return request<AuthUser>("/api/auth/me");
}
