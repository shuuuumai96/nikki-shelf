import { isSupportedLocale } from "./locale-registry";
import type { SupportedLocale } from "./locale-types";

const localeStorageKey = "nikki:locale";

export function readStoredLocale(): SupportedLocale | null {
  return normalizeStoredLocale(window.localStorage.getItem(localeStorageKey));
}

export function writeStoredLocale(locale: SupportedLocale) {
  window.localStorage.setItem(localeStorageKey, locale);
}

function normalizeStoredLocale(value: string | null): SupportedLocale | null {
  return isSupportedLocale(value) ? value : null;
}
