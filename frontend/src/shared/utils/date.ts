export function todayISO(): string {
  const date = new Date();
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 10);
}

export function addLocalDaysISO(value: string, days: number): string {
  const date = new Date(`${value}T00:00:00`);
  date.setDate(date.getDate() + days);
  return localDateISO(date);
}

export function previousLocalDayISO(value: string): string {
  return addLocalDaysISO(value, -1);
}

export function nextLocalDayISO(value: string): string {
  return addLocalDaysISO(value, 1);
}

export function formatDateLabel(value: string, locale = "en"): string {
  const date = new Date(`${value}T00:00:00`);
  return new Intl.DateTimeFormat(locale, {
    month: "long",
    day: "numeric",
    weekday: "short",
  }).format(date);
}

export function monthKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function localDateISO(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}
