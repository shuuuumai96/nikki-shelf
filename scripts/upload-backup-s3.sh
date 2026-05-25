#!/usr/bin/env sh
set -eu

BACKUP_RUN_DIR="${BACKUP_RUN_DIR:?BACKUP_RUN_DIR is required}"
S3_BUCKET="${S3_BUCKET:?S3_BUCKET is required}"
S3_PREFIX="${S3_PREFIX:?S3_PREFIX is required}"
AWS_REGION="${AWS_REGION:?AWS_REGION is required}"
ALLOW_PLAINTEXT_UPLOAD="${ALLOW_PLAINTEXT_UPLOAD:-false}"
S3_KMS_KEY_ID="${S3_KMS_KEY_ID:-}"

if ! command -v aws >/dev/null 2>&1; then
  printf '%s\n' "ERROR: aws CLI is required." >&2
  exit 1
fi

if [ ! -d "${BACKUP_RUN_DIR}" ]; then
  printf '%s\n' "ERROR: backup directory does not exist: ${BACKUP_RUN_DIR}" >&2
  exit 1
fi

set -- "${BACKUP_RUN_DIR}"/*.age
if [ "$1" = "${BACKUP_RUN_DIR}/*.age" ]; then
  if [ "${ALLOW_PLAINTEXT_UPLOAD}" != "true" ]; then
    printf '%s\n' "ERROR: no encrypted .age artifacts found. Set ALLOW_PLAINTEXT_UPLOAD=true only for an intentional plaintext upload." >&2
    exit 1
  fi
  set -- "${BACKUP_RUN_DIR}"/*
fi

sse_args="--sse AES256"
if [ -n "${S3_KMS_KEY_ID}" ]; then
  sse_args="--sse aws:kms --sse-kms-key-id ${S3_KMS_KEY_ID}"
fi

for artifact in "$@"; do
  [ -f "${artifact}" ] || continue
  name="$(basename "${artifact}")"
  aws s3 cp "${artifact}" "s3://${S3_BUCKET}/${S3_PREFIX%/}/${name}" --region "${AWS_REGION}" ${sse_args}
done

printf '%s\n' "Upload complete. Confirm the bucket blocks public access, enforces encryption, and has lifecycle retention configured."
