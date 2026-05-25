# 0004 Backup Restore as Core Requirement

## Status

Accepted

## Context

Nikki stores personal diary data that cannot be reproduced if lost. The data spans PostgreSQL rows and uploaded image files, so export alone is not enough for operational recovery.

Restore must be verified before the operator relies on a backup.

## Decision

Treat backup and restore as core requirements.

Document PostgreSQL dump/restore, uploads restore, and isolated restore verification.

## Consequences

Operators must back up both the database and uploads storage from the same point in time. Restore verification should compare entries, images, content hashes where practical, and sample image serving.

The README points to the detailed backup and restore document instead of duplicating the full procedure.
