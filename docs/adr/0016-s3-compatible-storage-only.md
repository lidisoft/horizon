# 0016. S3-compatible object storage only

- **Status**: Accepted
- **Date**: 2026-10-27

## Context

Issues carry attachments and pasted images.

## Decision

Files are stored only in **S3-compatible** object storage (RustFS for
development and self-hosting). Uploads use presigned PUT URLs; downloads go
through a permission-checked redirect to a short-lived presigned GET.

## Consequences

- One storage code path. Self-hosted installs must run RustFS or another
  S3-compatible service.
- File bytes never pass through the Go server for upload or download.
