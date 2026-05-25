# 0001 Release Scope Feature Cuts

## Status

Accepted

## Context

Nikki contains useful diary functionality, but several adjacent features increase release risk: mobile photo workflows, robust multi-tab editing, PWA behavior, rich inline image editing, photo library management, sharing, AI features, and statistics expansion.

The release needs to protect personal diary data and keep operations understandable.

## Decision

Proceed with feature cuts.

Release Nikki as a personal-use single-tab text diary with recoverable data and desktop-supported safe attachments.

Current project positioning has since moved toward public self-hosted OSS readiness while preserving this release boundary.

## Consequences

The release scope is smaller and clearer. Unsupported features must remain documented and frozen until a new release decision changes the scope.

Future work should prioritize data safety, backup/restore reliability, and simple operator-controlled operation over feature expansion.
