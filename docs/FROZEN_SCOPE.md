# Frozen Scope

Nikki is released for personal daily use as a single-tab, self-hosted diary app with recoverable data and desktop-supported safe image attachments.

The following areas are frozen until explicitly approved. Do not add, expand, or redesign these areas while stabilization, autosave safety, image lifecycle safety, backup/restore, and OSS readiness are still being clarified.

## Frozen Areas

- multi-tab robust conflict recovery
- mobile image upload, retry, and remove flows
- rich inline image editing
- full offline-first PWA behavior
- service workers or offline writing
- photo library features
- visual redesign
- advanced drag-to-line image insertion
- advanced Markdown editor behavior
- statistics expansion
- sharing
- AI features
- public SaaS or multi-tenant behavior

## PWA Boundary

Full offline-first PWA support remains frozen. Do not add service workers, offline writing, offline sync, cache strategies, or background recovery behavior without a separate product and reliability decision.

Simple installability may be reconsidered separately as a narrow manifest-only theme: web app manifest, icons, theme color, and standalone display. That work is not approved by this document and should not be bundled with offline-first behavior.

## Roadmap Candidate Boundary

Lightweight reflection, search, archive/reading mode, and retrieval improvements may be future roadmap candidates. They should not be implemented until explicitly approved.

## Allowed Work

- Fixing build or test failures.
- Removing or disabling fragile behavior when needed for reliability.
- One-time token-gated first-user setup, because it replaces fragile bootstrap-token curl operation with a safer empty-database setup flow.
- One-time token-gated operational backup restore during first setup, because it supports server migration and disaster recovery while preserving the empty-database and setup-token gates.
- Improving autosave safety.
- Improving image lifecycle safety.
- Improving backup and restore.
- Improving PC-browser daily writing reliability.
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
- Statistics, AI, sharing, reminders, search, reflection, or offline-first PWA features.
