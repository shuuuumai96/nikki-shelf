# 0002 Single-Tab Writing Assumption

## Status

Accepted

## Context

Diary writing is usually a focused single-user workflow. Robust multi-tab editing and automatic merge would require more complex conflict resolution, user interaction design, testing, and recovery behavior.

The current release needs predictable autosave and clear conflict fallback rather than complex merge behavior.

## Decision

Assume single-tab writing.

Do not support robust multi-tab editing or automatic merge in this release.

## Consequences

Normal autosave is supported for single-tab use. Entry versioning detects stale updates and returns a conflict instead of attempting to merge.

Users should avoid editing the same entry in multiple tabs. If a conflict occurs, the user can reload the server version or manually preserve local text.
