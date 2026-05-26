import { i18n } from "../i18n";

export class ApiError extends Error {
  status: number;
  kind: string;

  constructor(status: number, message: string, kind = "") {
    super(message);
    this.status = status;
    this.kind = kind;
  }
}

let csrfToken = "";

export function setCSRFToken(token: string) {
  csrfToken = token;
}

export function clearCSRFToken() {
  csrfToken = "";
}

export function currentCSRFToken(): string {
  return csrfToken;
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const method = (init?.method || "GET").toUpperCase();
  const headers = new Headers(init?.headers);
  // Do not set JSON content type for FormData; the browser must add the
  // multipart boundary or Echo cannot parse uploads.
  if (!(init?.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (
    csrfToken &&
    !["GET", "HEAD", "OPTIONS"].includes(method) &&
    !headers.has("X-CSRF-Token")
  ) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const response = await fetch(path, {
    ...init,
    credentials: "same-origin",
    headers,
  });

  if (!response.ok) {
    const error = await readError(response);
    throw new ApiError(response.status, error.message, error.kind);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

async function readError(
  response: Response,
): Promise<{ message: string; kind: string }> {
  const text = await response.text();
  try {
    const data = JSON.parse(text) as { error?: string; kind?: string };
    return { message: data.error || text, kind: data.kind || "" };
  } catch {
    return { message: text || i18n.global.t("errors.api"), kind: "" };
  }
}
