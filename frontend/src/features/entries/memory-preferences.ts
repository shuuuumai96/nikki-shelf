import { moodOrder } from "./moods";
import type { MoodKey } from "./types";

export type MemoryPreferences = {
  enabled: boolean;
  excludedMoods: MoodKey[];
};

export const defaultMemoryPreferences: MemoryPreferences = {
  enabled: true,
  excludedMoods: ["tired", "sad"],
};

const memoryPreferencesKey = "nikki:memory-preferences";

export function readMemoryPreferences(): MemoryPreferences {
  try {
    return normalizeMemoryPreferences(
      window.localStorage.getItem(memoryPreferencesKey),
    );
  } catch {
    return { ...defaultMemoryPreferences };
  }
}

export function writeMemoryPreferences(preferences: MemoryPreferences) {
  window.localStorage.setItem(
    memoryPreferencesKey,
    JSON.stringify(normalizeMemoryPreferences(preferences)),
  );
}

function normalizeMemoryPreferences(
  value: string | MemoryPreferences | null,
): MemoryPreferences {
  const parsed =
    typeof value === "string" ? (JSON.parse(value || "{}") as unknown) : value;

  if (!parsed || typeof parsed !== "object") {
    return { ...defaultMemoryPreferences };
  }

  const source = parsed as Partial<MemoryPreferences>;
  return {
    enabled:
      typeof source.enabled === "boolean"
        ? source.enabled
        : defaultMemoryPreferences.enabled,
    excludedMoods: normalizeMoods(source.excludedMoods),
  };
}

function normalizeMoods(value: unknown): MoodKey[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const supported = new Set<MoodKey>(moodOrder);
  const seen = new Set<MoodKey>();
  const moods: MoodKey[] = [];

  value.forEach((item) => {
    if (!supported.has(item as MoodKey) || seen.has(item as MoodKey)) {
      return;
    }
    seen.add(item as MoodKey);
    moods.push(item as MoodKey);
  });

  return moods;
}
