import { onBeforeUnmount, onMounted } from "vue";
import { nextLocalDayISO, previousLocalDayISO } from "../utils/date";

type FocusModeController = {
  toggleFocusMode: () => void;
};

type WritingShortcutsOptions = {
  canUseShortcuts: () => boolean;
  getActiveDate: () => string;
  getFocusModeController: () => FocusModeController | null;
  isWritingView: () => boolean;
  navigateToDate: (date: string) => void | Promise<void>;
  selectToday: () => void | Promise<void>;
};

export function useWritingShortcuts(options: WritingShortcutsOptions) {
  function onGlobalKeydown(event: KeyboardEvent) {
    if (
      event.isComposing ||
      !event.ctrlKey ||
      !event.altKey ||
      event.shiftKey ||
      event.metaKey
    ) {
      return;
    }

    if (!options.canUseShortcuts()) {
      return;
    }

    const focusModeController = options.getFocusModeController();
    if (
      event.key.toLowerCase() === "f" &&
      options.isWritingView() &&
      focusModeController
    ) {
      event.preventDefault();
      focusModeController.toggleFocusMode();
      return;
    }

    if (event.key.toLowerCase() === "t") {
      event.preventDefault();
      void options.selectToday();
      return;
    }

    if (event.key === "ArrowLeft") {
      event.preventDefault();
      void options.navigateToDate(previousLocalDayISO(options.getActiveDate()));
      return;
    }

    if (event.key === "ArrowRight") {
      event.preventDefault();
      void options.navigateToDate(nextLocalDayISO(options.getActiveDate()));
    }
  }

  onMounted(() => {
    window.addEventListener("keydown", onGlobalKeydown);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("keydown", onGlobalKeydown);
  });
}
