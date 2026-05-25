import type { LocaleOption, SupportedLocale } from "./locale-types";

export const fallbackLocale: SupportedLocale = "en";

export const localeRegistry: Record<SupportedLocale, LocaleOption> = {
  ja: {
    code: "ja",
    label: "Japanese",
    nativeLabel: "日本語",
    htmlLang: "ja",
  },
  en: {
    code: "en",
    label: "English",
    nativeLabel: "English",
    htmlLang: "en",
  },
};

export const localeOptions = Object.values(localeRegistry);

export function isSupportedLocale(
  value: string | null | undefined,
): value is SupportedLocale {
  return Boolean(value && value in localeRegistry);
}
