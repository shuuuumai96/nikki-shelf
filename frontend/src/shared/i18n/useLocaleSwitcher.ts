import { computed, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  fallbackLocale,
  isSupportedLocale,
  localeOptions,
  localeRegistry,
} from "./locale-registry";
import { writeStoredLocale } from "./locale-storage";
import type { SupportedLocale } from "./locale-types";

export function useLocaleSwitcher() {
  const { locale } = useI18n({ useScope: "global" });

  const currentLocale = computed<SupportedLocale>(() => {
    return isSupportedLocale(locale.value) ? locale.value : fallbackLocale;
  });

  const currentLocaleOption = computed(
    () => localeRegistry[currentLocale.value],
  );

  function setLocale(value: string) {
    const nextLocale = isSupportedLocale(value) ? value : fallbackLocale;
    locale.value = nextLocale;
    writeStoredLocale(nextLocale);
  }

  watch(
    currentLocaleOption,
    (option) => {
      document.documentElement.lang = option.htmlLang;
    },
    { immediate: true },
  );

  return {
    currentLocale,
    currentLocaleOption,
    localeOptions,
    setLocale,
  };
}
