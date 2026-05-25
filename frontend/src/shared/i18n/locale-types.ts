export const supportedLocaleCodes = ["ja", "en"] as const;

export type SupportedLocale = (typeof supportedLocaleCodes)[number];

export type LocaleOption = {
  code: SupportedLocale;
  label: string;
  nativeLabel: string;
  htmlLang: string;
};
