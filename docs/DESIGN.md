# Nikki Design Document

## 1. Product Definition

Nikki is a self-hosted writing cockpit for daily personal records.

The primary experience is PC-browser diary writing: text-first daily records, modest metadata, desktop-supported image attachments, and recoverable self-hosted data. Nikki is for one person or a small trusted household, not a public SaaS or multi-tenant product.

Smartphone support is secondary: reading, light edits, and possible future installability. Nikki should not be shaped around mobile-first capture or photo-library workflows.

## 2. Release Scope

Supported:

- single-user or small trusted household use
- single-tab writing
- PC-browser daily writing
- text diary
- tags
- moods
- normal autosave
- backup and restore
- desktop-supported image attachments
- missing-image UI
- `cleanup-images` command

Unsupported:

- robust multi-tab editing
- automatic merge/conflict resolution
- mobile image upload support
- full mobile photo diary workflow
- inline rich image placement
- full offline-first PWA behavior
- service workers or offline writing
- photo library management
- public SaaS or multi-tenant hosting
- sharing
- AI features

## 3. Design Principles

- PC writing experience over feature breadth
- diary scope over PKM scope
- recoverability over convenience
- self-host clarity over SaaS complexity
- cautious installability over offline-first complexity
- data safety over feature richness
- explicit unsupported scope over ambiguous promises
- desktop-supported safe attachments over mobile photo workflow
- manual recovery over complex automatic merge

## 4. Product Boundaries

### Core Now

- PC-browser writing for daily diary records
- single-tab autosave with stale-version fallback
- tags, moods, and simple date-based organization
- desktop-supported image attachments
- missing-image visibility and image consistency cleanup
- app-level backup archive
- operational PostgreSQL and uploads backup/restore documentation
- Docker Compose self-host operation

### Candidate Next

These are roadmap candidates only and require explicit approval before implementation:

- archive and reading mode improvements
- search and retrieval
- lightweight reflection that stays close to diary review
- public OSS readiness work
- manifest-only installable web app support

### Explicitly Not a Goal

- general-purpose workspace, knowledge-base, or broad PKM replacement
- photo library or mobile photo journal
- mobile-first writing product
- public SaaS or multi-tenant service
- social sharing platform
- offline-first editor
- AI-first diary or coaching product

## 5. System Overview

Nikki has a Vue frontend, a Go/Echo backend, a PostgreSQL database, and local uploaded image storage. The default runtime is Docker Compose.

```text
Browser
|
| HTTP
v
Frontend / Nginx
|
| API
v
Backend
|
+--> PostgreSQL
|
+--> Uploads directory
```

The frontend is built from `frontend/` and served by nginx in the frontend container. nginx proxies `/api/` and `/uploads/` to the backend service. The backend stores diary data in PostgreSQL and image files in the configured upload directory.

## 6. Core User Flows

- Create diary entry: the user creates a date-based entry with title, body, mood, and tags.
- Edit diary entry: the user edits an existing entry and sends the current entry version.
- Normal autosave: the frontend saves routine single-tab edits through the entry update API.
- Conflict detection fallback: stale updates return `409`; the user can reload the server version or preserve local text manually.
- Desktop image attachment: the user attaches supported image files to an entry from a desktop-supported flow.
- Missing image display: if image metadata exists but the file is missing, the UI surfaces a missing-image state.
- Backup creation: the application can produce a backup archive through Settings or `GET /api/export/backup`.
- Restore verification: operators verify restored entries, images, counts, hashes, and sample image serving in an isolated environment.
- `cleanup-images` dry-run: operators inspect file/database mismatches without deleting data.
- `cleanup-images` destructive mode: operators remove reviewed orphan files and image rows linked to missing entries.

## 7. Autosave and Conflict Model

Nikki supports normal single-tab autosave. Each entry has a `version` column. Updates include the client version and increment the stored version only when the row still matches.

A stale update returns `409` with the `entries.stale_version` error kind. This is a fallback, not robust multi-tab editing. Automatic merge is unsupported.

When a conflict occurs, the user can reload the server version or copy local text before reloading. Users should avoid editing the same entry in multiple tabs.

## 8. Image Attachment Model

Images are diary entry attachments. They are not full photo library items.

Desktop-supported image upload is in scope. Mobile image upload is unsupported for this release.

Image metadata is stored in PostgreSQL in the `images` table. Image files are stored on disk in the configured upload directory. The backend generates stored file names and records public URLs. The UI has a missing-image state when a database row points to a file that is not present.

`cleanup-images` detects inconsistencies between database metadata, entries, and files.

Known failure states:

- database row exists but file is missing
- file exists but database row is missing
- image row is linked to a missing entry
- file deletion fails

## 9. Backup and Restore Model

Nikki has an app-level backup archive for entries, images, manifest data, and restore notes. Operational recovery uses PostgreSQL dump/restore and a matching uploads directory restore.

Restore verification should be isolated from live volumes. Verification should include alternate port startup, entry and image counts, content hash checks where practical, and sample image HTTP checks.

See [BACKUP_RESTORE.md](BACKUP_RESTORE.md).

## 10. Security and Operational Boundaries

Nikki assumes a trusted operator-controlled environment. It is not designed as a public multi-tenant service.

Access to diary data is authenticated. File upload handling validates detected image content types, applies size limits, stores generated file names, and serves uploads by basename to avoid path traversal through the upload URL.

Operators are responsible for deployment security, backups, restore testing, and access control around the host. Nikki does not claim enterprise security, hardened SaaS security, or complete protection.

## 11. Known Limitations

- no robust multi-tab editing
- no automatic merge
- no release-supported mobile image upload
- no full offline-first PWA behavior
- no service workers or offline writing
- no photo library features
- local draft is a convenience, not a backup
- desktop image attachments only

## 12. Future Work Policy

Frozen features must not be resumed without a new release decision.

Future work must be justified by diary value, data safety, operational simplicity, and the PC-browser writing experience. Mobile image upload, offline-first PWA behavior, rich inline image editing, robust multi-tab editing, search, and lightweight reflection require separate release gates.

See [FROZEN_SCOPE.md](FROZEN_SCOPE.md) and [ROADMAP.md](ROADMAP.md).
