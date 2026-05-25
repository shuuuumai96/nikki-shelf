import { fallbackLocale, isSupportedLocale } from "./locale-registry";
import { readStoredLocale } from "./locale-storage";
import type { SupportedLocale } from "./locale-types";

type LocaleStrategy = () => SupportedLocale | null;

const localeStrategies: LocaleStrategy[] = [
  savedPreferenceLocale,
  browserPreferenceLocale,
  fallbackPreferenceLocale,
];

export function resolveInitialLocale(): SupportedLocale {
  return (
    localeStrategies.map((strategy) => strategy()).find(isSupportedLocale) ??
    fallbackLocale
  );
}

function savedPreferenceLocale(): SupportedLocale | null {
  return readStoredLocale();
}

function browserPreferenceLocale(): SupportedLocale | null {
  const browserLocale = window.navigator.language.toLowerCase();
  return browserLocale.startsWith("ja") ? "ja" : null;
}

function fallbackPreferenceLocale(): SupportedLocale {
  return fallbackLocale;
}
