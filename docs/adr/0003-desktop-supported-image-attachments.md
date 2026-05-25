# 0003 Desktop-Supported Image Attachments

## Status

Accepted

## Context

Image attachments are useful for diary entries, but a full mobile photo diary workflow adds risk around mobile upload controls, retries, removal, network interruption, layout, and recovery.

Nikki should support safe attachments without becoming a photo library or mobile photo diary.

## Decision

Support desktop image attachments.

Do not release mobile image upload or a full mobile photo diary workflow.

## Consequences

Image files are treated as entry attachments with database metadata and files on disk. Missing-image states and cleanup tooling are part of the release.

Mobile upload controls and full photo diary behavior remain out of scope until a separate release decision.
