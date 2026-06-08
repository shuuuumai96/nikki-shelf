import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import type { UploadImageRequest } from "../api";
import type { Entry, EntryImage, EntryInput, SaveStatus } from "../types";
import DiaryEditor from "./DiaryEditor.vue";

const markdownEditorMocks = vi.hoisted(() => ({
  currentPosition: vi.fn(() => null),
  insertImage: vi.fn(),
  insertImageAtPosition: vi.fn(),
}));

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    locale: { value: "ja" },
    t: (key: string, params?: Record<string, string | number>) =>
      params?.progress !== undefined ? `${key}:${params.progress}` : key,
  }),
}));

vi.mock("../../../shared/api/client", () => ({
  localizedErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "error",
}));

vi.mock("../../../shared/i18n", () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}));

vi.mock("./MarkdownEditor.vue", async () => {
  const { defineComponent, h, onMounted } =
    await vi.importActual<typeof import("vue")>("vue");

  return {
    __esModule: true,
    default: defineComponent({
      name: "MarkdownEditor",
      props: {
        modelValue: {
          required: true,
          type: String,
        },
      },
      emits: ["imageDrop", "ready", "update:modelValue"],
      setup(props, { emit, expose }) {
        expose({
          currentPosition: markdownEditorMocks.currentPosition,
          insertImage: (image: { url: string; fileName?: string }) => {
            markdownEditorMocks.insertImage(image);
            emit(
              "update:modelValue",
              `${props.modelValue}\n![${image.fileName || ""}](${image.url})`,
            );
          },
          insertImageAtPosition: (
            position: number,
            image: { url: string; fileName?: string },
          ) => {
            markdownEditorMocks.insertImageAtPosition(position, image);
            emit(
              "update:modelValue",
              `${props.modelValue}\n![${image.fileName || ""}](${image.url})`,
            );
            return position;
          },
        });

        onMounted(() => emit("ready"));

        return () =>
          h(
            "div",
            {
              class: "markdown-editor-stub",
              "data-body": props.modelValue,
            },
            props.modelValue,
          );
      },
    }),
  };
});

describe("DiaryEditor image uploads", () => {
  beforeEach(() => {
    markdownEditorMocks.currentPosition.mockReturnValue(null);
    markdownEditorMocks.insertImage.mockClear();
    markdownEditorMocks.insertImageAtPosition.mockClear();
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      callback(0);
      return 0;
    });
    Object.defineProperty(URL, "createObjectURL", {
      configurable: true,
      value: vi.fn(() => "blob:preview"),
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      configurable: true,
      value: vi.fn(),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("keeps uploaded images out of the diary body", async () => {
    const uploadImage = vi.fn(() => ({
      abort: vi.fn(),
      promise: Promise.resolve(image()),
    }));
    const wrapper = mountEditor({ uploadImage });
    await waitForMarkdownEditor(wrapper);

    await wrapper.find(".image-bar").trigger("drop", {
      dataTransfer: {
        files: [new File(["image"], "photo.png", { type: "image/png" })],
      },
    });
    await flushPromises();
    await nextTick();

    expect(uploadImage).toHaveBeenCalledTimes(1);
    expect(markdownEditorMocks.insertImage).not.toHaveBeenCalled();
    expect(markdownEditorMocks.insertImageAtPosition).not.toHaveBeenCalled();
    expect(wrapper.find(".markdown-editor-stub").attributes("data-body")).toBe(
      "Body",
    );
  });
});

async function waitForMarkdownEditor(wrapper: ReturnType<typeof mountEditor>) {
  for (let attempt = 0; attempt < 5; attempt += 1) {
    await vi.dynamicImportSettled();
    await flushPromises();
    await nextTick();
    if (wrapper.find(".markdown-editor-stub").exists()) {
      return;
    }
  }
  throw new Error("Markdown editor stub did not render");
}

function mountEditor(overrides: {
  uploadImage: (payload: {
    input: EntryInput;
    file: File;
    onProgress: (progress: number) => void;
  }) => UploadImageRequest;
}) {
  return mount(DiaryEditor, {
    props: {
      date: "2026-06-07",
      entry: entry(),
      navigationMessage: "",
      saveError: "",
      saveStatus: "saved" as SaveStatus,
      saving: false,
      tags: [],
      ...overrides,
    },
    global: {
      stubs: {
        EntryImageAttachment: true,
        IconButton: true,
        MoodSelector: true,
        TagInput: true,
      },
    },
  });
}

function entry(): Entry {
  return {
    body: "Body",
    createdAt: "2026-06-07T00:00:00Z",
    entryDate: "2026-06-07",
    id: 1,
    images: [],
    mood: "calm",
    tags: [],
    title: "Title",
    updatedAt: "2026-06-07T00:00:00Z",
    version: 1,
  };
}

function image(): EntryImage {
  return {
    createdAt: "2026-06-07T00:00:00Z",
    entryId: 1,
    fileName: "photo.png",
    id: 10,
    mimeType: "image/png",
    size: 5,
    url: "/api/images/10/content",
  };
}
