import { ApiError, currentCSRFToken, request } from "../../shared/api/client";
import { i18n } from "../../shared/i18n";
import type {
  Entry,
  EntryDateLookup,
  EntryFilter,
  EntryImage,
  EntryInput,
  EntryMemoryFilter,
  EntryMemoryResponse,
  EntryPage,
  EntrySearchFilter,
  EntrySearchResponse,
  Stats,
} from "./types";

export const IMAGE_UPLOAD_MAX_BYTES = 8 * 1024 * 1024;
export const SUPPORTED_IMAGE_TYPES = [
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
];

export type UploadImageRequest = {
  promise: Promise<EntryImage>;
  abort: () => void;
};

function queryString(
  filter: EntryFilter | EntrySearchFilter | EntryMemoryFilter,
): string {
  const params = new URLSearchParams();
  Object.entries(filter).forEach(([key, value]) => {
    if (value) {
      params.set(key, String(value));
    }
  });
  const text = params.toString();
  return text ? `?${text}` : "";
}

export function listEntries(filter: EntryFilter = {}): Promise<EntryPage> {
  return request<EntryPage>(`/api/entries${queryString(filter)}`);
}

export function searchEntries(
  filter: EntrySearchFilter = {},
  signal?: AbortSignal,
): Promise<EntrySearchResponse> {
  return request<EntrySearchResponse>(
    `/api/entries/search${queryString(filter)}`,
    { signal },
  );
}

export function listEntryMemories(
  filter: EntryMemoryFilter,
  signal?: AbortSignal,
): Promise<EntryMemoryResponse> {
  return request<EntryMemoryResponse>(
    `/api/entries/memories${queryString(filter)}`,
    { signal },
  );
}

export function getEntry(id: number): Promise<Entry> {
  return request<Entry>(`/api/entries/${id}`);
}

export function getEntryByDate(date: string): Promise<EntryDateLookup> {
  return request<EntryDateLookup>(`/api/entries/date/${date}`);
}

export function createEntry(input: EntryInput): Promise<Entry> {
  return request<Entry>("/api/entries", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateEntry(
  id: number,
  input: EntryInput,
  expectedVersion: number,
): Promise<Entry> {
  return request<Entry>(`/api/entries/${id}`, {
    method: "PUT",
    body: JSON.stringify({ ...input, expectedVersion }),
  });
}

export function deleteEntry(id: number): Promise<void> {
  return request<void>(`/api/entries/${id}`, { method: "DELETE" });
}

export async function uploadImages(
  entryId: number,
  files: File[],
): Promise<EntryImage[]> {
  const form = new FormData();
  files.forEach((file) => form.append("images", file));
  return request<EntryImage[]>(`/api/entries/${entryId}/images`, {
    method: "POST",
    body: form,
  });
}

export function uploadImage(
  entryId: number,
  file: File,
  onProgress: (progress: number) => void,
): UploadImageRequest {
  const form = new FormData();
  form.append("images", file);
  const xhr = new XMLHttpRequest();

  const promise = new Promise<EntryImage>((resolve, reject) => {
    xhr.open("POST", `/api/entries/${entryId}/images`);
    xhr.withCredentials = true;
    // fetch does not expose upload progress; keep this path on XHR and attach
    // the same CSRF token that request() adds to mutating fetch calls.
    const token = currentCSRFToken();
    if (token) {
      xhr.setRequestHeader("X-CSRF-Token", token);
    }

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress(
          Math.min(99, Math.round((event.loaded / event.total) * 100)),
        );
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const images = JSON.parse(xhr.responseText || "[]") as EntryImage[];
          if (!images[0]) {
            reject(
              new ApiError(
                xhr.status,
                i18n.global.t("images.fallbackUploadFailed"),
              ),
            );
            return;
          }
          onProgress(100);
          resolve(images[0]);
        } catch {
          reject(
            new ApiError(
              xhr.status,
              i18n.global.t("images.fallbackUploadFailed"),
            ),
          );
        }
        return;
      }

      const error = readXHRError(xhr);
      reject(new ApiError(xhr.status, error.message, error.kind));
    };

    xhr.onerror = () =>
      reject(new ApiError(0, i18n.global.t("images.fallbackUploadFailed")));
    xhr.onabort = () =>
      reject(new ApiError(0, i18n.global.t("images.fallbackUploadFailed")));
    xhr.send(form);
  });

  return {
    promise,
    abort: () => xhr.abort(),
  };
}

export function deleteImage(id: number): Promise<void> {
  return request<void>(`/api/images/${id}`, { method: "DELETE" });
}

export function getStats(): Promise<Stats> {
  return request<Stats>("/api/stats");
}

export function listTags(): Promise<string[]> {
  return request<string[]>("/api/tags");
}

function readXHRError(xhr: XMLHttpRequest): { message: string; kind: string } {
  try {
    const data = JSON.parse(xhr.responseText || "{}") as {
      error?: string;
      kind?: string;
    };
    return {
      message: data.error || i18n.global.t("images.fallbackUploadFailed"),
      kind: data.kind || "",
    };
  } catch {
    return {
      message: xhr.responseText || i18n.global.t("images.fallbackUploadFailed"),
      kind: "",
    };
  }
}
