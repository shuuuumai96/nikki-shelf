import MarkdownIt from "markdown-it";

const entryMarkdown = new MarkdownIt({
  breaks: true,
  html: false,
  linkify: true,
  typographer: true,
});

export function renderEntryMarkdown(value: string) {
  const normalized = normalizeMarkdownForRender(value);
  return normalized ? entryMarkdown.render(normalized) : "";
}

export function renderEditorMarkdown(value: string) {
  const normalized = normalizeMarkdownForRender(value);
  return normalized ? entryMarkdown.render(normalized) : "<p></p>";
}

export function normalizeMarkdown(value: string) {
  return value.replace(/\n{3,}/g, "\n\n").trim();
}

export function normalizeMarkdownForRender(value: string) {
  const normalized = normalizeMarkdown(value);
  // Turndown escapes leading Markdown markers when converting editor HTML back
  // to text. Restore only when the source looks like repeatedly escaped blocks.
  return shouldRestoreEscapedMarkdownMarkers(normalized)
    ? restoreMarkdownSourceMarkers(normalized)
    : normalized;
}

export function restoreMarkdownSourceMarkers(value: string) {
  return value
    .replace(/^\\`\\`\\`/gm, "```")
    .replace(/^\\~\\~\\~/gm, "~~~")
    .replace(/^\\(#{1,6})(?=\s)/gm, "$1")
    .replace(/^\\([-*+])(?=\s)/gm, "$1")
    .replace(/^\\(>)(?=\s)/gm, "$1")
    .replace(/^\\((?:```|~~~))/gm, "$1");
}

function shouldRestoreEscapedMarkdownMarkers(value: string) {
  const markers =
    value.match(/^\\(?:#{1,6}|[-*+]|>|(?:```|~~~))(?=\s|$|[a-zA-Z])/gm) ?? [];
  const escapedFenceMarkers = value.match(/^\\(?:`\\`\\`|~\\~\\~)/gm) ?? [];
  return (
    markers.length + escapedFenceMarkers.length >= 2 ||
    /^\\(?:```|~~~|`\\`\\`|~\\~\\~)/m.test(value)
  );
}
