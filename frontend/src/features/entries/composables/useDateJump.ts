import { ref } from "vue";
import { i18n } from "../../../shared/i18n";

type DateJumpOptions = {
  selectDate: (date: string) => void | Promise<void>;
};

export function useDateJump(options: DateJumpOptions) {
  const dateJumpValue = ref("");
  const dateJumpError = ref("");

  async function jumpToDate() {
    const date = dateJumpValue.value;

    if (!date) {
      dateJumpError.value = "";
      return;
    }

    if (!isValidDateInput(date)) {
      dateJumpError.value = i18n.global.t("errors.invalidDate");
      return;
    }

    dateJumpError.value = "";
    await options.selectDate(date);
  }

  function clearDateJumpError() {
    dateJumpError.value = "";
  }

  return {
    clearDateJumpError,
    dateJumpError,
    dateJumpValue,
    jumpToDate,
  };
}

function isValidDateInput(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return false;
  }

  const date = new Date(`${value}T00:00:00`);
  if (Number.isNaN(date.getTime())) {
    return false;
  }

  const year = String(date.getFullYear()).padStart(4, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}` === value;
}
