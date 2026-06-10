import { beforeEach, describe, expect, it } from "vitest";
import {
  readMemoryPreferences,
  writeMemoryPreferences,
} from "./memory-preferences";

describe("memory preferences", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("defaults to enabled with sensitive moods excluded", () => {
    expect(readMemoryPreferences()).toEqual({
      enabled: true,
      excludedMoods: ["tired", "sad"],
    });
  });

  it("keeps only supported mood exclusions", () => {
    window.localStorage.setItem(
      "nikki:memory-preferences",
      JSON.stringify({
        enabled: false,
        excludedMoods: ["sad", "sleepy", "sad", "tired"],
      }),
    );

    expect(readMemoryPreferences()).toEqual({
      enabled: false,
      excludedMoods: ["sad", "tired"],
    });
  });

  it("writes normalized preferences", () => {
    writeMemoryPreferences({
      enabled: true,
      excludedMoods: ["sad", "sad", "calm"],
    });

    expect(readMemoryPreferences()).toEqual({
      enabled: true,
      excludedMoods: ["sad", "calm"],
    });
  });
});
