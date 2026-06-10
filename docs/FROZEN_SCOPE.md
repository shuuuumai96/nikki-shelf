# Frozen Scope

Nikki is released for personal daily use as a single-tab, self-hosted diary app with recoverable data, basic search/retrieval, a bounded random memory shelf, and desktop-supported safe image attachments.

The following areas are frozen until explicitly approved. Do not add, expand, or redesign these areas while stabilization, autosave safety, image lifecycle safety, backup/restore, and OSS readiness are still being clarified.

## Frozen Areas

- multi-tab robust conflict recovery
- mobile image upload, retry, and remove flows
- rich inline image editing
- full offline-first PWA behavior
- offline writing, offline sync, background recovery, or push notifications
- photo library features
- visual redesign
- advanced drag-to-line image insertion
- advanced Markdown editor behavior
- statistics expansion
- sharing
- AI features
- public SaaS or multi-tenant behavior

## PWA Boundary

Full offline-first PWA support remains frozen. The existing service worker is limited to static app-shell caching for installability. Do not add offline writing, offline sync, background recovery, background sync, push notifications, or caching of authenticated diary data without a separate product and reliability decision.

Simple installability is limited to web app manifest metadata, icons, theme color, standalone display, mobile web app meta tags, and basic app-shell caching. It must not be bundled with offline-first writing behavior.

## Roadmap Candidate Boundary

Further lightweight reflection, search expansion, archive/reading mode expansion, and retrieval improvements remain roadmap candidates. They should not be implemented until explicitly approved.

The current Today-screen memory shelf is an approved bounded retrieval feature. It may show older entries from the signed-in user's own diary, supports mood exclusions, and can be collapsed locally. Do not expand it into analytics, recommendations, coaching, AI summaries, reminders, photo-library behavior, or statistics without a separate approval.

## Allowed Work

- Fixing build or test failures.
- Removing or disabling fragile behavior when needed for reliability.
- One-time token-gated first-user setup, because it replaces fragile bootstrap-token curl operation with a safer empty-database setup flow.
- One-time token-gated operational backup restore during first setup, because it supports server migration and disaster recovery while preserving the empty-database and setup-token gates.
- Improving autosave safety.
- Improving image lifecycle safety.
- Improving backup and restore.
- Improving PC-browser daily writing reliability.
- Maintaining the approved basic search and memory shelf behavior without broadening their scope.
- Updating documentation to keep the frozen scope explicit.
- OSS readiness documentation and process preparation.

## Temporarily Disabled During Stabilization

- Rich inline image rendering in the Tiptap editor is disabled while build stabilization and image lifecycle safety are prioritized. Uploaded images remain attached to diary entries and visible in the reader and editor attachment grids. If an attached image file cannot be loaded, those grids show a missing-image state for recovery investigation.
- Drag-to-line image insertion may insert Markdown image text, but precise rich inline placement is not a release requirement.
- Image upload controls are hidden on phone-sized viewports because mobile image upload, retry, and remove flows are not release-supported.

## Not Allowed For Now

- New product features.
- Quick Capture or quick-note capture surfaces.
- Settings backup visibility UI, restore checklist UI links, backup dashboards, or other recoverability visibility features.
- General backup dashboards, normal Settings restore UI, and content-backup zip import remain frozen.
- New visual systems or broad UI redesign.
- Advanced editor controls beyond the existing daily diary needs.
- Photo management beyond safe attachment, deletion, backup, and restore.
- Statistics, AI, sharing, reminders, search/retrieval expansion, reflection beyond the current bounded memory shelf, or offline-first PWA expansion.
