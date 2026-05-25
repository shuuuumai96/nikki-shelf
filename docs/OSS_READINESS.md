# OSS Readiness

Nikki is preparing for a future public self-hosted OSS direction. This checklist is about project readiness, not new product features.

## License

Nikki is licensed under Apache-2.0. The top-level `LICENSE` file contains the full Apache License, Version 2.0 text.

The top-level `NOTICE` file contains project notice information. `THIRD_PARTY_NOTICES.md` contains a conservative informational summary of third-party dependency notices.

## Dependency License Audit

A bounded manual/local dependency license review was performed on 2026-05-22 for the current dependency set. This was a manual current-tree review only, not continuous compliance automation or a claim of full legal compliance.

Review inputs:

- frontend dependencies from `frontend/package.json`, `frontend/pnpm-lock.yaml`, and `corepack pnpm licenses list`
- backend modules from `backend/go.mod`, `backend/go.sum`, `go list -m all`, and upstream license files in the local Go module cache

Review result:

- No blocking dependency license issue was found in this bounded review.
- Frontend dependency licenses reported by pnpm were MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, and Python-2.0.
- `argparse@2.0.1`, a transitive frontend dependency of `markdown-it`, reports `Python-2.0`; treat it as a notice/redistribution item to preserve in release materials rather than ignoring it.
- Backend Go module license files reviewed as MIT, ISC, or BSD-style licenses. The `golang.org/x/*` modules also include the standard Go project `PATENTS` file.

This dependency license review is still bounded and must be repeated before formal public releases and after dependency changes. Do not treat this note as a substitute for legal review where one is required.

## Security Policy

`SECURITY.md` exists and should stay conservative. It should state that Nikki is not a hardened multi-user SaaS and should explain how to report vulnerabilities without exposing private diary data or secrets.

## Contribution Guide

`CONTRIBUTING.md` exists and should stay short. It should explain project scope, required verification, documentation expectations, and the need to discuss frozen areas before expanding them.

## Changelog and Release Notes

`CHANGELOG.md` exists and should be updated for release-significant changes. Nikki has not yet made a stable public release.

Release notes should be honest about compatibility and support status:

- document user-visible changes
- call out breaking deployment, schema, authentication, backup/restore, or data-layout changes clearly
- document security fixes without disclosing exploit details prematurely
- keep release notes honest about unsupported areas

## Versioning Policy

Nikki has not yet made a stable public release. Pre-1.0 releases may include breaking deployment, schema, authentication, backup/restore, or data-layout changes.

Once public releases begin, GitHub Releases and tags are the intended release publication mechanism. Release notes for tagged releases should point operators to any required backup, restore, migration, or configuration steps before upgrade.

Until stable release support is defined, security support should be described as current main branch / current release only.

## Secret Scanning

A manual local review of the current tracked repository was performed on 2026-05-22. The review covered `git ls-files`, tracked filenames, tracked text content, documentation examples, and `.gitignore` coverage for common local secret and data-artifact paths.

Review result:

- No real committed secrets were found in the current tracked files.
- No tracked backup, diary export, upload, database dump, SQLite, zip/tar backup, private key, cookie jar, or `.age` backup artifact files were found. The only tracked `.sql` file matching artifact-style extension checks is `backend/internal/db/schema.sql`, which is application schema source.
- `.env.production` and common local `.env` variants are ignored; `.env.production.example` remains tracked and uses placeholder values.
- The development Compose file and isolated restore documentation contain local-only development database credentials; production docs instruct operators to replace placeholders and not commit `.env.production`.
- This was a manual/local current-tree review only. It is not continuous GitHub secret scanning automation, a full history scan, or a guarantee that future commits cannot introduce secrets.

Before a public release, repeat this review and consider repository-hosted secret scanning separately. Keep dependency license auditing as a separate future task.

## GitHub Actions Safety

Do not add GitHub Actions in this task.

Before adding CI, review workflow safety:

- avoid printing secrets or environment files
- avoid uploading diary data, backups, or build artifacts that may contain private data
- pin or review third-party actions
- keep permissions minimal
- avoid workflows that run untrusted pull request code with write tokens or secrets

## Optional Future Checks

OpenSSF Scorecard may be useful later as a public repository hygiene check. Treat it as advisory, not as a replacement for project-specific review.

An ASVS-lite security checklist may be useful later for authentication, sessions, upload handling, deployment settings, and backup safety. Nikki should not claim full ASVS compliance unless that work is explicitly performed and maintained.
