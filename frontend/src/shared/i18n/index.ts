import { createI18n } from "vue-i18n";
import { fallbackLocale } from "./locale-registry";
import { resolveInitialLocale } from "./locale-strategies";
import en from "./messages/en";
import ja from "./messages/ja";

export const i18n = createI18n({
  legacy: false,
  locale: resolveInitialLocale(),
  fallbackLocale,
  messages: {
    en,
    ja,
  },
});
