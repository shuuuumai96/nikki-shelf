import { request } from "../../shared/api/client";
import type { AuthUser } from "../auth/types";
import type {
  SetupOwnerInput,
  SetupRestoreFileInput,
  SetupRestoreInput,
  SetupRestoreResponse,
  SetupRestoreVerifyResponse,
  SetupStatus,
} from "./types";

export function status(): Promise<SetupStatus> {
  return request<SetupStatus>("/api/setup/status");
}

export function createOwner(input: SetupOwnerInput): Promise<AuthUser> {
  return request<AuthUser>("/api/setup/owner", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function verifyRestore(
  input: SetupRestoreFileInput,
): Promise<SetupRestoreVerifyResponse> {
  return request<SetupRestoreVerifyResponse>("/api/setup/restore/verify", {
    method: "POST",
    body: setupRestoreFormData(input),
  });
}

export function restoreBackup(
  input: SetupRestoreInput,
): Promise<SetupRestoreResponse> {
  const data = setupRestoreFormData(input);
  data.set("confirmRestore", input.confirmRestore ? "true" : "false");
  return request<SetupRestoreResponse>("/api/setup/restore", {
    method: "POST",
    body: data,
  });
}

function setupRestoreFormData(input: SetupRestoreFileInput) {
  const data = new FormData();
  data.set("setupToken", input.setupToken);
  data.set("backupFile", input.backupFile);
  return data;
}
