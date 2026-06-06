# Nikki AWS EC2 Private Test Runbook

This runbook is for the first private AWS EC2 Docker Compose test only. It assumes a Debian or Ubuntu EC2 host and is not a broader public production launch checklist.

## 1. Scope

- Use one EC2 instance with Docker Compose.
- Run the frontend nginx container bound to `127.0.0.1:8089`.
- Keep the Go backend container reachable only on the Docker network.
- Run PostgreSQL in the Compose stack.
- Store uploads in the Docker volume on encrypted EBS-backed storage.
- Terminate HTTPS on the EC2 host and proxy to the frontend only.
- Do not expose backend `8080`.
- Do not serve `/uploads/` directly as static files from Caddy, host nginx, frontend nginx, or any other reverse proxy.
- Proxy `/uploads/` through the application path that performs metadata lookup and ownership verification.
- Do not mount the upload volume into a public webroot.
- Do not add RDS, ECS, App Runner, ALB, NAT Gateway, or S3-backed uploads.
- Optional S3 use is limited to encrypted backup artifact upload.

## 2. AWS Preflight

Confirm before provisioning or starting the test:

- AWS root account MFA is enabled.
- AWS Budgets is configured for the account.
- Region is selected and documented.
- Public IPv4 hourly cost is acknowledged.
- EBS encryption by default is enabled, or the instance volume is explicitly encrypted.
- If S3 backup copies are planned, the bucket plan is documented before upload.
- S3 Block Public Access is required for any backup bucket.
- S3 encryption is required for any backup bucket.
- IAM permissions are least-privilege; the instance role, if used for S3 backup upload, only has access to the intended backup prefix.

## 3. EC2 Access Model

Preferred:

- Use AWS Systems Manager Session Manager.
- Do not open inbound SSH.
- Attach only the IAM permissions needed for SSM and optional backup upload.

Fallback:

- Open SSH `22/tcp` only to the operator's current global IP.
- Use key-based auth only.
- Disable password auth.
- Close SSH again after the private test if Session Manager becomes available.

## 4. Security Group

Recommended inbound rules:

- `443/tcp`: allow only the intended source IPs if possible.
- `80/tcp`: allow only for ACME HTTP-01 challenge or HTTP to HTTPS redirect.
- `22/tcp`: closed when SSM works; otherwise restricted to the operator's global IP only.
- `8080/tcp`: never open.
- `5432/tcp`: never open.

Outbound can remain default for package installs, TLS certificate issuance, image pulls, and optional encrypted backup upload.

## 5. EC2 Host Setup Commands

Debian or Ubuntu baseline:

```bash
sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get install -y ca-certificates curl git age

sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

sudo systemctl enable --now docker
docker --version
docker compose version
```

Create the app directory and place the repository there:

```bash
sudo mkdir -p /opt/nikki
sudo chown "$USER":"$USER" /opt/nikki
cd /opt/nikki
git clone <repository-url> .
```

Create production configuration:

```bash
cp .env.production.example .env.production
chmod 600 .env.production
```

Generate a strong PostgreSQL password:

```bash
openssl rand -base64 36
```

Edit `.env.production`:

```bash
nano .env.production
```

Required values are shown for the private test. See `docs/CONFIGURATION.md` for the production configuration reference, including purpose, safe defaults, and misconfiguration risks.

```dotenv
POSTGRES_DB=nikki
POSTGRES_USER=nikki
POSTGRES_PASSWORD=<strong-generated-password>
NIKKI_DATABASE_URL=postgres://nikki:<same-strong-generated-password>@postgres:5432/nikki?sslmode=disable
NIKKI_ADDR=:8080
NIKKI_UPLOAD_DIR=/uploads
NIKKI_PUBLIC_UPLOAD_BASE=/uploads
NIKKI_COOKIE_SECURE=true
NIKKI_SIGNUP_ENABLED=false
NIKKI_FIRST_USER_SETUP_ENABLED=false
NIKKI_FIRST_USER_BOOTSTRAP_TOKEN=<long-random-bootstrap-token>
NIKKI_CORS_ALLOWED_ORIGINS=https://<your-exact-hostname>
NIKKI_IP_EXTRACTOR_MODE=direct
NIKKI_TRUSTED_PROXY_CIDRS=
NIKKI_STRIP_IMAGE_METADATA=true
```

`NIKKI_FIRST_USER_BOOTSTRAP_TOKEN` is required for the operator-controlled first setup path when the database is empty. Keep `NIKKI_FIRST_USER_SETUP_ENABLED=false`; browser setup at `/setup` still requires the token for owner creation and operational backup restore and does not depend on that flag. Keep the token long, random, secret, and out of tickets, screenshots, shared terminal logs, and chat. `NIKKI_SIGNUP_ENABLED=false` keeps additional signup closed after the first user exists.

For the first private EC2 test, `direct` IP extraction is the safest default. If per-client IP handling through proxy headers is needed later, set `NIKKI_IP_EXTRACTOR_MODE=x-real-ip` and set `NIKKI_TRUSTED_PROXY_CIDRS` only after verifying the frontend Docker network CIDR:

```bash
docker network inspect nikki_default
```

## 6. Production Compose Verification

Run from the repository root:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env.production config
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env.production config | ./scripts/check-production-config.sh
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env.production build
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env.production up -d
docker compose ps
```

Verify:

- Backend has no published host port.
- Frontend publishes only `127.0.0.1:8089->80`.
- PostgreSQL has no published host port.

## 7. HTTPS Reverse Proxy

Caddy-first setup:

```bash
sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt-get update
sudo apt-get install -y caddy
```

Example `/etc/caddy/Caddyfile`:

```caddyfile
your-domain.example {
	encode gzip zstd

	request_body {
		max_size 512MB
	}

	reverse_proxy 127.0.0.1:8089 {
		header_up Host {host}
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}
}
```

Do not proxy directly to backend `8080`.

Caddy should proxy only to the frontend container endpoint. Frontend nginx should proxy `/api/` and `/uploads/` to the backend according to `frontend/nginx/default.conf`; do not replace this with public static serving for uploads.

Validate and reload:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
sudo systemctl status caddy --no-pager
```

Add HSTS only after HTTPS is confirmed and rollback is understood:

```caddyfile
header Strict-Transport-Security "max-age=31536000; includeSubDomains"
```

## 8. First Functional Test

Use the HTTPS origin only.

First setup succeeds using the token-gated setup flow. On an empty database, verify setup status:

```bash
curl -fsS https://<your-domain>/api/setup/status
```

Expected result: JSON with `needsSetup: true`, `canCreateOwner: true`, `canRestoreBackup: true`, and `requiresSetupToken: true`. This response must not reveal whether the configured token is present or correct.

Verify setup without the setup token is rejected:

```bash
curl -i \
  -H 'Content-Type: application/json' \
  -d '{"username":"owner","password":"replace-with-a-long-password"}' \
  https://<your-domain>/api/setup/owner
```

Expected result: `403`.

Then create the owner user through `/setup` in the browser, restore a valid Nikki operational backup archive through `/setup`, or use the setup owner API:

```bash
curl -i \
  -H 'Content-Type: application/json' \
  -d '{"setupToken":"replace-with-long-random-bootstrap-token","username":"owner","password":"replace-with-a-long-password"}' \
  https://<your-domain>/api/setup/owner
```

Expected result for owner creation: `200`, a session cookie, and creation of the owner user. Expected result for restore: verified archive, restored entries/images, setup lock, and return to login. A second call to `/api/setup/owner` or `/api/setup/restore` must return `409`. The token and password in this example are placeholders only. Do not paste real tokens, passwords, cookies, operational backup archives, screenshots containing them, or shared shell logs into tickets or chat.

Browser checks:

- HTTPS page loads.
- Logout works.
- Login works.
- Create an entry.
- Update the entry.
- Upload an image.
- Load the uploaded image while authenticated.
- Delete the image.

API checks:

```bash
curl -i https://<your-domain>/api/entries
curl -i https://<your-domain>/api/images/<known-image-id>/content
```

Expected unauthenticated results:

- `/api/entries` returns `401`.
- `/api/images/.../content` returns `401`.

Check second signup is blocked after first user exists:

```bash
curl -i \
  -H 'Content-Type: application/json' \
  -d '{"username":"seconduser","password":"not-the-real-password"}' \
  https://<your-domain>/api/auth/signup
```

Expected result: `403`.

Check mutating API without CSRF fails:

```bash
curl -i \
  -b 'nikki_session=<valid-session-cookie-value>' \
  -H 'Content-Type: application/json' \
  -d '{"entryDate":"2026-05-19","title":"csrf test","body":"test","mood":"calm","tags":[]}' \
  https://<your-domain>/api/entries
```

Expected result: `403`. Do not paste real session cookies into shared logs or tickets.

## 9. Backup Test

Positive backup:

```bash
ENV_FILE=.env.production ./scripts/backup-production.sh
ls -lh backups/*/
```

Verify the newest backup directory contains:

- `nikki-operational-backup-<timestamp>.tar.gz`.
- The archive contains `manifest.json`, `db/postgres.dump`, `uploads/uploads.tar`, and `SHA256SUMS` when checksum tooling is available.

Negative uploads-volume test:

```bash
UPLOADS_VOLUME=nikki_missing_uploads_volume_for_test ENV_FILE=.env.production ./scripts/backup-production.sh
```

Expected result: non-zero failure before `tar`.

Age encryption test:

```bash
age-keygen -o ~/nikki-backup-age-key.txt
chmod 600 ~/nikki-backup-age-key.txt
grep '^# public key:' ~/nikki-backup-age-key.txt
AGE_RECIPIENT='<public-age-recipient>' ENV_FILE=.env.production ./scripts/backup-production.sh
ls -lh backups/*/*.age backups/*/SHA256SUMS.age
```

Plaintext artifacts are kept by default. If plaintext deletion is desired later, use `DELETE_PLAINTEXT_AFTER_ENCRYPT=true` only after decrypting and restore-testing encrypted artifacts.

Backups must be stored outside git. The repository ignores `backups/`, but operators should still treat backup directories as sensitive.

## 10. Optional S3 Backup Upload

Only use S3 after encrypted `.age` artifacts exist.

Requirements:

- Bucket already exists.
- Block Public Access is enabled.
- Bucket encryption is enabled.
- Lifecycle retention is configured.
- IAM permissions are least-privilege for the intended bucket prefix.
- The upload script does not create buckets or IAM policies.
- The upload script refuses plaintext by default.

Example:

```bash
BACKUP_RUN_DIR=backups/<timestamp> \
S3_BUCKET=<existing-private-bucket> \
S3_PREFIX=nikki/ec2-private-test \
AWS_REGION=<region> \
./scripts/upload-backup-s3.sh
```

Use SSE-KMS only if the key and IAM permissions already exist:

```bash
S3_KMS_KEY_ID=<kms-key-id> BACKUP_RUN_DIR=backups/<timestamp> S3_BUCKET=<bucket> S3_PREFIX=nikki AWS_REGION=<region> ./scripts/upload-backup-s3.sh
```

## 11. Restore Rehearsal

Run restore rehearsal in an isolated Compose project. Do not restore into the live stack.

```bash
docker compose -p nikki_restore -f docker-compose.yml up -d postgres
```

Extract the operational archive:

```bash
mkdir -p /tmp/nikki-restore
tar -xzf backups/<timestamp>/nikki-operational-backup-<timestamp>.tar.gz -C /tmp/nikki-restore
```

Restore database:

```bash
cat /tmp/nikki-restore/db/postgres.dump | docker compose -p nikki_restore exec -T postgres pg_restore --data-only --no-owner --disable-triggers -U nikki -d nikki
```

Restore uploads:

```bash
docker run --rm \
  -v nikki_restore_nikki_uploads:/uploads \
  -v /tmp/nikki-restore:/backup:ro \
  alpine sh -c 'cd /uploads && tar -xf /backup/uploads/uploads.tar'
```

Start an isolated backend/frontend using an alternate project name and non-conflicting ports, or follow the isolated backend procedure in `docs/BACKUP_RESTORE.md`.

Verify:

- Login works.
- Entry count matches source.
- Image count matches source.
- A sample image loads while authenticated.

Destroy isolated restore stack only after verification:

```bash
docker compose -p nikki_restore down
```

Do not delete the backup artifacts just because the restore rehearsal passed.

## 12. Rollback

- Keep existing source data untouched.
- If DNS was changed, revert it to the previous target.
- Stop the EC2 stack:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env.production down
```

- Preserve EBS snapshots and backup artifacts.
- Never delete local data before AWS restore is verified.
- If the test is abandoned, keep enough artifacts to investigate safely, then destroy AWS resources deliberately to stop cost.

## 13. Manual Schema-Change Runbook

Versioned migrations are not implemented in this pass. Do not add a migration framework unless an explicit task requests it.

Any private-test release that changes `backend/internal/db/schema.sql` or requires production SQL must be handled manually:

1. Back up production data, including PostgreSQL and the matching uploads volume.
2. Verify the backup with an isolated restore.
3. Review the SQL manually before applying it.
4. Apply the SQL in a controlled maintenance window.
5. Deploy the application.
6. Verify health, login, entry read/write, image read/write, and backup creation.

Do not add a migration library or migration implementation as part of this runbook.

## 14. Cost Watch

Track these cost drivers:

- EC2 instance.
- Public IPv4 hourly charge.
- EBS volume size.
- EBS snapshots.
- S3 backup storage.
- S3 requests.
- Data transfer out, especially uploaded image serving.
- Route 53 hosted zone and DNS queries.
- CloudWatch logs if enabled.
- Domain registration or renewal.

Avoid NAT Gateway, ALB, RDS, ECS, and App Runner for this first test unless requirements change.

## 15. Final Go/No-Go Checklist

The private EC2 test may start only if all items are true:

- AWS Budget exists.
- Security Group is minimal.
- SSM works, or SSH is key-only and IP-restricted.
- `.env.production` has no placeholders.
- Production Compose config passes.
- `scripts/check-production-config.sh` passes.
- Backend has no public host ports.
- PostgreSQL has no public host ports.
- HTTPS reverse proxy config is ready.
- `NIKKI_COOKIE_SECURE=true`.
- `NIKKI_SIGNUP_ENABLED=false`.
- `NIKKI_FIRST_USER_SETUP_ENABLED=false`.
- `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN` is set to a long random secret and is not a placeholder.
- `NIKKI_CORS_ALLOWED_ORIGINS` is the exact HTTPS origin.
- Backup positive test passes.
- Invalid uploads volume backup test fails safely.
- Age encryption positive test passes on EC2.
- Restore rehearsal plan exists.
- No one has pasted real setup tokens, cookies, passwords, backup contents, or env files into logs or tickets.
