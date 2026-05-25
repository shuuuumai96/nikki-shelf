<script setup lang="ts">
import ImageExtension from "@tiptap/extension-image";
import LinkExtension from "@tiptap/extension-link";
import Placeholder from "@tiptap/extension-placeholder";
import StarterKit from "@tiptap/starter-kit";
import { EditorContent, useEditor } from "@tiptap/vue-3";
import { BubbleMenu } from "@tiptap/vue-3/menus";
import {
  Bold,
  Code2,
  Heading1,
  Heading2,
  Italic,
  Link,
  List,
  ListOrdered,
  Pilcrow,
  Quote,
  Strikethrough,
} from "lucide-vue-next";
import TurndownService from "turndown";
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  normalizeMarkdown,
  renderEditorMarkdown,
  restoreMarkdownSourceMarkers,
} from "../markdown";

const props = defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
  imageDrop: [payload: { files: File[]; position: number }];
  ready: [];
}>();

type InlineImage = {
  url: string;
  fileName?: string;
};

type ResolvedEditorPosition = {
  depth: number;
  node: (depth: number) => { isTextblock: boolean };
};

const dropIndicator = reactive({
  visible: false,
  top: 0,
});
const editorReady = ref(false);
const { t } = useI18n();

const dropIndicatorStyle = computed(() => ({
  transform: `translateY(${dropIndicator.top}px)`,
}));

const turndown = new TurndownService({
  bulletListMarker: "-",
  codeBlockStyle: "fenced",
  headingStyle: "atx",
});

turndown.addRule("strikethrough", {
  filter: ["s", "del"],
  replacement: (content) => `~~${content}~~`,
});

turndown.addRule("image", {
  filter: "img",
  replacement: (_content, node) => {
    if (!(node instanceof HTMLImageElement) || !node.src) {
      return "";
    }

    const alt = node.alt || "";
    return `![${escapeMarkdownImageText(alt)}](${node.getAttribute("src") || node.src})`;
  },
});

const editor = useEditor({
  content: renderEditorMarkdown(props.modelValue),
  extensions: [
    StarterKit.configure({
      heading: {
        levels: [1, 2, 3],
      },
      link: false,
    }),
    LinkExtension.configure({
      autolink: true,
      linkOnPaste: true,
      openOnClick: false,
      HTMLAttributes: {
        rel: "noopener noreferrer",
        target: null,
      },
    }),
    ImageExtension.configure({
      allowBase64: false,
      inline: false,
    }),
    Placeholder.configure({
      placeholder: () => t("entries.bodyPlaceholder"),
    }),
  ],
  editorProps: {
    attributes: {
      "aria-label": t("entries.bodyLabel"),
      "aria-multiline": "true",
      role: "textbox",
      class: "document-editor-surface entry-markdown-body",
    },
    handlePaste: (_view, event) => handleMarkdownPaste(event),
    handleScrollToSelection: () => {
      // The page owns scrolling. Letting ProseMirror scroll on selection changes
      // fights the fixed/sticky editor chrome in focus mode.
      return true;
    },
  },
  onCreate: () => {
    // Wait for Vue and ProseMirror to paint before revealing the editor; the
    // first layout pass otherwise flashes an unmeasured document surface.
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        editorReady.value = true;
        emit("ready");
      });
    });
  },
  onUpdate: ({ editor }) => {
    emit("update:modelValue", htmlToMarkdown(editor.getHTML()));
  },
});

watch(
  () => props.modelValue,
  (value) => {
    const instance = editor.value;
    if (!instance) {
      return;
    }

    if (htmlToMarkdown(instance.getHTML()) === normalizeMarkdown(value)) {
      return;
    }

    // Prop updates come from server reloads or draft restores. Suppress
    // ProseMirror's update event so the parent does not autosave the echo.
    instance.commands.setContent(renderEditorMarkdown(value), {
      emitUpdate: false,
    });
  },
);

onBeforeUnmount(() => {
  editor.value?.destroy();
});

defineExpose({
  insertImage,
  insertImageAtPosition,
  currentPosition,
});

function htmlToMarkdown(html: string) {
  return normalizeMarkdown(
    restoreMarkdownSourceMarkers(turndown.turndown(html)),
  );
}

function handleMarkdownPaste(event: ClipboardEvent) {
  const text = event.clipboardData?.getData("text/plain") ?? "";
  if (!looksLikeMarkdown(text)) {
    return false;
  }

  const instance = editor.value;
  if (!instance) {
    return false;
  }

  const source = normalizeMarkdown(text);
  if (!source) {
    return false;
  }

  // Plain text that looks like Markdown should stay Markdown. Browser paste
  // would otherwise insert literal syntax into the rich editor document.
  event.preventDefault();
  if (instance.isEmpty) {
    instance.commands.setContent(renderEditorMarkdown(source), {
      emitUpdate: false,
    });
    emit("update:modelValue", source);
    return true;
  }

  instance.chain().focus().insertContent(renderEditorMarkdown(source)).run();
  return true;
}

function looksLikeMarkdown(value: string) {
  const text = value.trim();
  if (!text) {
    return false;
  }

  return [
    /^#{1,6}\s/m,
    /^[-*+]\s/m,
    /^\d+[.)]\s/m,
    /^>\s/m,
    /^(```|~~~)/m,
    /`[^`\n]+`/,
    /!\[[^\]]*]\([^)]+\)/,
    /\[[^\]]+]\([^)]+\)/,
  ].some((pattern) => pattern.test(text));
}

function insertImage(image: InlineImage) {
  const instance = editor.value;
  if (!instance || !image.url) {
    return;
  }

  instance.chain().focus().insertContent(imageContent(image)).run();
}

function insertImageAtPosition(position: number, image: InlineImage) {
  const instance = editor.value;
  if (!instance || !image.url) {
    return null;
  }

  // Return the post-insert selection so callers inserting several uploaded
  // images can chain the next insertion after the image paragraph.
  instance
    .chain()
    .focus()
    .insertContentAt(position, imageContent(image), { updateSelection: true })
    .run();

  return instance.state.selection.to;
}

function currentPosition() {
  return editor.value?.state.selection.from ?? null;
}

function escapeMarkdownImageText(value: string) {
  return value.replace(/\\/g, "\\\\").replace(/\]/g, "\\]");
}

function imageContent(image: InlineImage) {
  return [
    {
      type: "image",
      attrs: {
        src: image.url,
        alt: image.fileName ?? "",
      },
    },
    {
      type: "paragraph",
    },
  ];
}

function onEditorDragOver(event: DragEvent) {
  if (!isExternalFileDrag(event)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = hasImageFile(event.dataTransfer)
      ? "copy"
      : "none";
  }

  if (!hasImageFile(event.dataTransfer)) {
    hideDropIndicator();
    return;
  }

  updateDropIndicator(event);
}

function onEditorDragLeave(event: DragEvent) {
  const current = event.currentTarget as HTMLElement | null;
  const next = event.relatedTarget as Node | null;
  if (!current || !next || !current.contains(next)) {
    hideDropIndicator();
  }
}

function onEditorDrop(event: DragEvent) {
  if (!isExternalFileDrag(event)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();

  const files = Array.from(event.dataTransfer?.files || []);
  const position = dropPositionFromEvent(event);
  hideDropIndicator();

  if (!files.length || position === null) {
    return;
  }

  emit("imageDrop", { files, position });
}

function updateDropIndicator(event: DragEvent) {
  const position = dropPositionFromEvent(event);
  const instance = editor.value;
  if (!instance || position === null) {
    hideDropIndicator();
    return;
  }

  const coords = instance.view.coordsAtPos(position);
  const editorRect = instance.view.dom.getBoundingClientRect();
  dropIndicator.top = Math.max(0, coords.top - editorRect.top);
  dropIndicator.visible = true;
}

function dropPositionFromEvent(event: DragEvent) {
  const instance = editor.value;
  if (!instance) {
    return null;
  }

  const result = instance.view.posAtCoords({
    left: event.clientX,
    top: event.clientY,
  });
  if (!result) {
    return null;
  }

  // Convert ProseMirror's nearest character position into a block boundary so
  // dropped images land between paragraphs instead of splitting text nodes.
  return blockBoundaryPosition(result.pos, event.clientY);
}

function blockBoundaryPosition(position: number, clientY: number) {
  const instance = editor.value;
  if (!instance) {
    return position;
  }

  const { doc } = instance.state;
  const resolved = doc.resolve(position);
  const depth = textBlockDepth(resolved);
  if (depth === null) {
    return position;
  }

  const blockPosition = resolved.before(depth);
  const blockNode = instance.view.nodeDOM(blockPosition);
  if (!(blockNode instanceof HTMLElement)) {
    return clientY <
      midpoint(
        instance.view.coordsAtPos(blockPosition),
        instance.view.coordsAtPos(resolved.after(depth)),
      )
      ? blockPosition
      : resolved.after(depth);
  }

  const rect = blockNode.getBoundingClientRect();
  return clientY < rect.top + rect.height / 2
    ? blockPosition
    : resolved.after(depth);
}

function textBlockDepth(resolved: ResolvedEditorPosition) {
  for (let depth = resolved.depth; depth > 0; depth -= 1) {
    if (resolved.node(depth).isTextblock) {
      return depth;
    }
  }
  return null;
}

function midpoint(start: { top: number }, end: { top: number }) {
  return start.top + (end.top - start.top) / 2;
}

function isExternalFileDrag(event: DragEvent) {
  return Array.from(event.dataTransfer?.types || []).includes("Files");
}

function hasImageFile(dataTransfer: DataTransfer | null) {
  return Array.from(dataTransfer?.items || []).some(
    (item) => item.kind === "file" && item.type.startsWith("image/"),
  );
}

function hideDropIndicator() {
  dropIndicator.visible = false;
}

function toggleHeading(level: 1 | 2) {
  editor.value?.chain().focus().toggleHeading({ level }).run();
}

function setParagraph() {
  editor.value?.chain().focus().setParagraph().run();
}

function toggleBold() {
  editor.value?.chain().focus().toggleBold().run();
}

function toggleItalic() {
  editor.value?.chain().focus().toggleItalic().run();
}

function toggleStrike() {
  editor.value?.chain().focus().toggleStrike().run();
}

function toggleBlockquote() {
  editor.value?.chain().focus().toggleBlockquote().run();
}

function toggleBulletList() {
  editor.value?.chain().focus().toggleBulletList().run();
}

function toggleOrderedList() {
  editor.value?.chain().focus().toggleOrderedList().run();
}

function toggleCodeBlock() {
  editor.value?.chain().focus().toggleCodeBlock().run();
}

function shouldShowBubble({
  editor,
  state,
}: {
  editor: { isEditable: boolean };
  state: { selection: { empty: boolean } };
}) {
  return editor.isEditable && !state.selection.empty;
}

function setLink() {
  const instance = editor.value;
  if (!instance) {
    return;
  }

  const current = instance.getAttributes("link").href as string | undefined;
  const href = window.prompt("URL", current || "https://");
  if (href === null) {
    return;
  }

  const next = href.trim();
  if (next === "") {
    instance.chain().focus().extendMarkRange("link").unsetLink().run();
    return;
  }

  instance
    .chain()
    .focus()
    .extendMarkRange("link")
    .setLink({ href: next })
    .run();
}
</script>

<template>
  <div class="markdown-editor">
    <BubbleMenu
      v-if="editor"
      :editor="editor"
      :should-show="shouldShowBubble"
      :options="{ placement: 'top', strategy: 'fixed', offset: 8 }"
    >
      <div class="bubble-toolbar" :aria-label="t('entries.textStyleMenu')">
        <button
          type="button"
          :class="{ active: editor.isActive('paragraph') }"
          :title="t('entries.paragraph')"
          :aria-label="t('entries.paragraph')"
          @mousedown.prevent
          @click="setParagraph"
        >
          <Pilcrow :size="16" stroke-width="1.9" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('heading', { level: 1 }) }"
          :title="t('entries.heading1')"
          :aria-label="t('entries.heading1')"
          @mousedown.prevent
          @click="toggleHeading(1)"
        >
          <Heading1 :size="17" stroke-width="1.9" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('heading', { level: 2 }) }"
          :title="t('entries.heading2')"
          :aria-label="t('entries.heading2')"
          @mousedown.prevent
          @click="toggleHeading(2)"
        >
          <Heading2 :size="17" stroke-width="1.9" />
        </button>
        <span aria-hidden="true" class="bubble-separator"></span>
        <button
          type="button"
          :class="{ active: editor.isActive('bold') }"
          :title="t('entries.bold')"
          :aria-label="t('entries.bold')"
          @mousedown.prevent
          @click="toggleBold"
        >
          <Bold :size="16" stroke-width="2.15" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('italic') }"
          :title="t('entries.italic')"
          :aria-label="t('entries.italic')"
          @mousedown.prevent
          @click="toggleItalic"
        >
          <Italic :size="16" stroke-width="2.05" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('strike') }"
          :title="t('entries.strike')"
          :aria-label="t('entries.strike')"
          @mousedown.prevent
          @click="toggleStrike"
        >
          <Strikethrough :size="16" stroke-width="2" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('link') }"
          :title="t('entries.link')"
          :aria-label="t('entries.link')"
          @mousedown.prevent
          @click="setLink"
        >
          <Link :size="16" stroke-width="1.9" />
        </button>
        <span aria-hidden="true" class="bubble-separator"></span>
        <button
          type="button"
          :class="{ active: editor.isActive('blockquote') }"
          :title="t('entries.quote')"
          :aria-label="t('entries.quote')"
          @mousedown.prevent
          @click="toggleBlockquote"
        >
          <Quote :size="16" stroke-width="1.9" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('bulletList') }"
          :title="t('entries.bulletList')"
          :aria-label="t('entries.bulletList')"
          @mousedown.prevent
          @click="toggleBulletList"
        >
          <List :size="16" stroke-width="1.9" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('orderedList') }"
          :title="t('entries.orderedList')"
          :aria-label="t('entries.orderedList')"
          @mousedown.prevent
          @click="toggleOrderedList"
        >
          <ListOrdered :size="16" stroke-width="1.9" />
        </button>
        <button
          type="button"
          :class="{ active: editor.isActive('codeBlock') }"
          :title="t('entries.code')"
          :aria-label="t('entries.code')"
          @mousedown.prevent
          @click="toggleCodeBlock"
        >
          <Code2 :size="16" stroke-width="1.9" />
        </button>
      </div>
    </BubbleMenu>

    <div
      class="document-editor-wrap"
      @dragover="onEditorDragOver"
      @dragleave="onEditorDragLeave"
      @drop="onEditorDrop"
    >
      <div
        v-if="dropIndicator.visible"
        class="drop-indicator"
        :style="dropIndicatorStyle"
        aria-hidden="true"
      ></div>
      <div
        v-if="!editorReady"
        class="document-editor document-editor-static"
        aria-hidden="true"
      >
        <div
          class="document-editor-surface entry-markdown-body"
          v-html="renderEditorMarkdown(modelValue)"
        ></div>
      </div>
      <EditorContent
        v-if="editor"
        :editor="editor"
        class="document-editor"
        :class="{ ready: editorReady }"
      />
    </div>
  </div>
</template>

<style scoped>
.markdown-editor {
  margin-top: 22px;
}

.document-editor {
  color: var(--color-text);
}

.document-editor:not(.ready) {
  position: absolute;
  inset: 0;
  visibility: hidden;
  pointer-events: none;
}

.document-editor-static {
  position: static;
  visibility: visible;
}

.document-editor-wrap {
  position: relative;
  min-height: 421px;
}

.drop-indicator {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  z-index: 2;
  height: 2px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--color-accent) 72%, #fff);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 14%, transparent);
  pointer-events: none;
}

.document-editor :deep(.document-editor-surface) {
  min-height: 360px;
  outline: none;
  border-top: 1px solid var(--border-subtle);
  padding: 22px 0 38px;
}

.document-editor
  :deep(.document-editor-surface p.is-editor-empty:first-child::before) {
  float: left;
  height: 0;
  color: var(--color-placeholder);
  content: attr(data-placeholder);
  pointer-events: none;
}

.bubble-toolbar {
  display: flex;
  max-width: min(calc(100vw - 32px), 432px);
  align-items: center;
  gap: 3px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  padding: 5px;
  box-shadow: var(--nikki-shadow-subtle);
  overflow-x: auto;
  scrollbar-width: none;
}

.bubble-toolbar::-webkit-scrollbar {
  display: none;
}

.bubble-toolbar button {
  display: grid;
  flex: 0 0 auto;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-muted);
  padding: 0;
  transition:
    background-color 120ms ease,
    color 120ms ease;
}

.bubble-toolbar button:hover,
.bubble-toolbar button.active {
  background: color-mix(in srgb, var(--color-wash) 66%, #fff);
  color: var(--color-text);
}

.bubble-separator {
  width: 1px;
  height: 20px;
  flex: 0 0 auto;
  background: color-mix(in srgb, var(--color-border) 70%, #fff);
}

@media (max-width: 480px) {
  .markdown-editor {
    margin-top: 16px;
  }

  .document-editor :deep(.document-editor-surface) {
    min-height: 300px;
    padding: 4px 0 28px;
  }

  .document-editor-wrap {
    min-height: 445px;
  }

  .bubble-toolbar {
    max-width: calc(100vw - 24px);
  }
}
</style>
