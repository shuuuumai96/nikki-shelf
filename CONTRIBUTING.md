# Contributing

Nikki is a self-hosted, text-first diary for PC-browser writing, daily records, safe attachments, and recoverable personal data. It is intended for one person or a small trusted household, not public SaaS or multi-tenant hosting.

Before starting larger changes, please read:

- [README.md](README.md)
- [docs/DESIGN.md](docs/DESIGN.md)
- [docs/FROZEN_SCOPE.md](docs/FROZEN_SCOPE.md)
- [docs/OSS_READINESS.md](docs/OSS_READINESS.md)

## Scope

Keep changes aligned with Nikki's current scope: diary writing, recoverability, simple self-host operation, and desktop-supported attachments.

Avoid expanding frozen areas without prior discussion, including offline-first PWA behavior, mobile image upload, sharing, AI features, photo-library behavior, robust multi-tab editing, and broad visual redesign.

## Verification

Run the relevant checks before opening a pull request:

```bash
cd frontend
corepack pnpm build
```

```bash
cd backend
go test ./...
```

```bash
git diff --check
```

If a check is not relevant or cannot be run, explain why.

## Documentation

Update documentation for behavior changes, deployment changes, backup/restore implications, security-relevant changes, or any change that affects supported and unsupported scope.

Do not commit generated data, uploads, `node_modules`, build output, environment files, database dumps, diary exports, or backup artifacts.
