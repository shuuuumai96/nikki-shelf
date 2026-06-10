import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import {
  readMemoryPreferences,
  writeMemoryPreferences,
} from "../memory-preferences";
import type { EntryMemory } from "../types";
import MemoryShelf from "./MemoryShelf.vue";

const mocks = vi.hoisted(() => ({
  listEntryMemories: vi.fn(),
}));

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock("../api", () => ({
  listEntryMemories: mocks.listEntryMemories,
}));

describe("MemoryShelf", () => {
  beforeEach(() => {
    window.localStorage.clear();
    mocks.listEntryMemories.mockReset();
    mocks.listEntryMemories.mockResolvedValue({ items: [memory()] });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("loads memories with the active local preferences", async () => {
    writeMemoryPreferences({
      enabled: true,
      excludedMoods: ["sad", "tired"],
    });

    const wrapper = mount(MemoryShelf, {
      props: { activeDate: "2026-06-10" },
      global: {
        stubs: {
          ChevronDown: true,
          ChevronUp: true,
          Image: true,
        },
      },
    });
    await flushPromises();
    await nextTick();

    expect(mocks.listEntryMemories).toHaveBeenCalledWith(
      {
        date: "2026-06-10",
        excludeMoods: "sad,tired",
        limit: "3",
      },
      expect.any(AbortSignal),
    );
    expect(wrapper.text()).toContain("2026-06-01");
    expect(wrapper.text()).toContain("Past day");
  });

  it("emits the memory date when a card is selected", async () => {
    const wrapper = mount(MemoryShelf, {
      props: { activeDate: "2026-06-10" },
      global: {
        stubs: {
          ChevronDown: true,
          ChevronUp: true,
          Image: true,
        },
      },
    });
    await flushPromises();
    await nextTick();

    await wrapper.find(".memory-card").trigger("click");

    expect(wrapper.emitted("selectDate")).toEqual([[memory().entryDate]]);
  });

  it("can collapse and expand memories from the shelf", async () => {
    const wrapper = mount(MemoryShelf, {
      props: { activeDate: "2026-06-10" },
      global: {
        stubs: {
          ChevronDown: true,
          ChevronUp: true,
          Image: true,
        },
      },
    });
    await flushPromises();
    await nextTick();

    await wrapper.find(".memory-collapse").trigger("click");
    await nextTick();

    expect(readMemoryPreferences()).toEqual({
      enabled: true,
      excludedMoods: ["tired", "sad"],
    });
    expect(wrapper.find(".memory-shelf").exists()).toBe(true);
    expect(wrapper.find(".memory-list").exists()).toBe(false);

    await wrapper.find(".memory-collapse").trigger("click");
    await nextTick();

    expect(wrapper.find(".memory-list").exists()).toBe(true);
  });

  it("stays hidden when memories are disabled", async () => {
    writeMemoryPreferences({
      enabled: false,
      excludedMoods: [],
    });

    const wrapper = mount(MemoryShelf, {
      props: { activeDate: "2026-06-10" },
    });
    await flushPromises();

    expect(mocks.listEntryMemories).not.toHaveBeenCalled();
    expect(wrapper.find(".memory-shelf").exists()).toBe(false);
  });
});

function memory(): EntryMemory {
  return {
    entryDate: "2026-06-01",
    hasImage: true,
    id: 30,
    imageCount: 1,
    mood: "calm",
    preview: "A quiet earlier entry",
    tags: ["home"],
    title: "Past day",
    updatedAt: "2026-06-01T00:00:00Z",
  };
}
