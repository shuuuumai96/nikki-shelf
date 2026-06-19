# AWS EC2 Public Production Deployment

This runbook describes the minimum operator-managed public production shape for Nikki on one Amazon Linux 2023 EC2 instance. It does not use ECS, App Runner, RDS, ALB, NAT Gateway, high-availability infrastructure, managed database failover, or S3-backed uploads.

## Target Architecture

- One Amazon Linux 2023 EC2 instance with encrypted EBS storage.
- Docker Engine and Docker Compose on the instance.
- `postgres` container with the `nikki_postgres_data` Docker volume on encrypted EBS-backed storage.
- Go `backend` container reachable only on the Docker network.
- `frontend` nginx container bound to `127.0.0.1:8089` for the host reverse proxy.
- Uploaded images in the `nikki_uploads` Docker volume on encrypted EBS-backed storage.
- Caddy installed on the host as the public HTTPS reverse proxy.
- Public access only through Caddy on `443/tcp`, with `80/tcp` open only when needed for ACME HTTP-01 or HTTP-to-HTTPS redirect.
- EC2 management through AWS Systems Manager Session Manager. Do not open public SSH.
- Optional private S3 bucket for encrypted backup copies; uploads are not served from S3.

## Upload Routing Invariant

Do not serve `/uploads/` directly as static files from Caddy, host nginx, frontend nginx, or any other reverse proxy. `/uploads/` must be proxied through the application path that performs metadata lookup and ownership verification.

The upload volume must not be mounted into a public webroot. Caddy should proxy only to the frontend container endpoint, and frontend nginx should proxy `/api/` and `/uploads/` to the backend according to `frontend/nginx/default.conf`.

## AWS Resources

- EC2 instance: Amazon Linux 2023, encrypted EBS root volume, public IP or Elastic IP, no public SSH access.
- Security Group: inbound `443/tcp` required, `80/tcp` only if needed, `22/tcp` closed, `8080/tcp` closed, `5432/tcp` closed.
- IAM role: attach an instance profile that permits AWS SSM Session Manager. Use the AWS-managed `AmazonSSMManagedInstanceCore` policy unless a narrower equivalent policy is maintained.
- Route 53 or external DNS: create an `A` record for the real diary domain pointing to the EC2 public IP or Elastic IP.
- Optional S3 backup bucket: block all public access, enable encryption, enable lifecycle retention, and grant the EC2 role write/read access only to the chosen backup prefix.

## Domain and DNS

Use `diary.example.com` as the placeholder in examples. Replace it with the real domain only in local production configuration on the EC2 host.

1. Point the real domain's `A` record to the EC2 public IP or Elastic IP.
2. Wait for DNS propagation before starting Caddy certificate issuance.
3. Set `NIKKI_CORS_ALLOWED_ORIGINS=https://your-real-domain.example` exactly. Do not use `*`.

## Security Group

Required inbound rules:

- `443/tcp`: open to intended public clients; required for HTTPS.
- `80/tcp`: open only if needed for ACME HTTP-01 or HTTP-to-HTTPS redirect.
- `22/tcp`: must remain closed. Use SSM Session Manager.
- `8080/tcp`: must remain closed. The backend is private to Docker.
- `5432/tcp`: must remain closed. PostgreSQL is private to Docker.

## Instance Setup

Connect with SSM Session Manager, then install Docker:

```bash
sudo dnf update -y
sudo dnf install -y docker
sudo systemctl enable --now docker
sudo usermod -aG docker ec2-user
```

Start a new SSM session after adding the Docker group. Install the Docker Compose plugin if it is not already available:

```bash
docker compose version
```

If that command fails, install the Compose plugin using the current Docker documentation for Amazon Linux 2023, then rerun `docker compose version`.

Install Caddy from the official Caddy repository for Fedora/RHEL-compatible systems, then enable it:

```bash
sudo dnf install -y 'dnf-command(copr)'
sudo dnf copr enable -y @caddy/caddy
sudo dnf install -y caddy
sudo systemctl enable --now caddy
```

Copy `deploy/Caddyfile.production.example` to `/etc/caddy/Caddyfile`, replace `diary.example.com` with the real domain, validate, and reload:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## Production Environment

Create `.env.production` on the EC2 instance beside the Compose files:

```bash
cp .env.production.example .env.production
chmod 600 .env.production
```

Replace every placeholder in `.env.production` before starting the stack. Do not commit `.env.production`.

Production requirements are summarized here; see `docs/CONFIGURATION.md` for purpose, safe defaults, and misconfiguration risks for each production-relevant environment variable.

- Database variables must contain real production values and `NIKKI_DATABASE_URL` must point to `postgres:5432` inside Compose.
- `NIKKI_COOKIE_SECURE=true` is required for HTTPS production.
- `NIKKI_CORS_ALLOWED_ORIGINS` must match the exact public HTTPS origin.
- Signup should stay closed for public production. First setup is a separate token-gated empty-database flow for owner creation or operational backup restore.
- `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN` must be a long random secret before an empty database is exposed publicly.
- `NIKKI_AUDIT_RETENTION_DAYS` must be a positive bounded value.
- `NIKKI_STRIP_IMAGE_METADATA=true` is expected for production.
- `.env.production` must not be committed.

`NIKKI_SIGNUP_ENABLED=false` does not allow unsafe unauthenticated first signup. When the database is empty, `/setup` is available in the browser, but owner creation and operational backup restore both require the setup token to match `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN`. `NIKKI_FIRST_USER_SETUP_ENABLED` should remain `false` in public production and does not remove the token requirement. After a user exists or restore succeeds, setup execution is rejected and keeping `NIKKI_SIGNUP_ENABLED=false` closes additional signup unless the operator explicitly changes this setting.

The bootstrap token should be long, random, secret, and never pasted into tickets, chat logs, screenshots, or application logs. Operational backup archives are also sensitive because they contain private diary data and password hashes. If possible, complete first setup while access is still restricted by Security Group, VPN, SSM port forwarding, Tailscale, or another operator-only path. If you test setup with curl, use your real token only in your private shell:

```sh
curl -i \
  -H 'Content-Type: application/json' \
  -d '{"setupToken":"replace-with-long-random-bootstrap-token","username":"owner","password":"replace-with-a-long-password"}' \
  https://diary.example.com/api/setup/owner
```

The token and password in this example are placeholders.

`NIKKI_CORS_ALLOWED_ORIGINS` is a comma-separated list. Same-origin production deployment normally should not need permissive CORS, but the public HTTPS origin must be exact if cross-origin browser access is intentionally required. Do not use `*` in production.

For the initial single-host Caddy-to-frontend-to-backend chain, use `NIKKI_IP_EXTRACTOR_MODE=direct` and leave `NIKKI_TRUSTED_PROXY_CIDRS=` empty. If per-client IP rate limiting is needed later, prefer `NIKKI_IP_EXTRACTOR_MODE=x-real-ip` and set `NIKKI_TRUSTED_PROXY_CIDRS` only to the verified frontend container network CIDR after `docker network inspect nikki_default`; do not include broad public CIDRs.

## Build and Deploy

Validate the production Compose configuration before starting containers:

```bash
docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml config
docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml config | ./scripts/check-production-config.sh
```

Build and start:

```bash
docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml build
docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml up -d
```

Confirm health:

```bash
docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml ps
curl -fsS http://127.0.0.1:8089/
curl -fsS http://127.0.0.1:8089/api/health
curl -fsS https://your-real-domain.example/api/health
```

The backend `8080` port must not be reachable from the internet, and PostgreSQL `5432` must not be reachable from the internet.

## Smoke Tests

First setup succeeds using the token-gated setup flow. On an empty production database, verify setup status first:

```bash
curl -fsS https://your-real-domain.example/api/setup/status
```

Expected result: JSON with `needsSetup: true`, `canCreateOwner: true`, `canRestoreBackup: true`, and `requiresSetupToken: true`. The response must not reveal whether the configured token is present or correct.

Then verify the negative case:

```bash
curl -i \
  -H 'Content-Type: application/json' \
  -d '{"username":"owner","password":"replace-with-a-long-password"}' \
  https://your-real-domain.example/api/setup/owner
```

Expected result: `403`.

Then create the owner user through `/setup` in the browser, restore a valid Nikki operational backup archive through `/setup`, or use the setup owner API:

```bash
curl -i \
  -H 'Content-Type: application/json' \
  -d '{"setupToken":"replace-with-long-random-bootstrap-token","username":"owner","password":"replace-with-a-long-password"}' \
  https://your-real-domain.example/api/setup/owner
```

Expected result for owner creation: `200`, a session cookie, and creation of the owner user. Expected result for restore: verified archive, restored entries/images, setup lock, and return to login. A second call to `/api/setup/owner` or `/api/setup/restore` after this must return `409`. The token and password in this example are placeholders only. Do not paste real tokens, passwords, cookies, operational backup archives, screenshots containing them, or shared shell logs into tickets or chat.

After the owner user exists, run the automated smoke test from the deployment checkout:

```bash
export NIKKI_SMOKE_BASE_URL=https://your-real-domain.example
export NIKKI_SMOKE_USERNAME=owner
read -rs NIKKI_SMOKE_PASSWORD
export NIKKI_SMOKE_PASSWORD
sh ./scripts/smoke-production.sh
```

The script checks production Compose configuration, public health, closed signup, locked setup, login, CSRF rejection, diary create/read/update/delete, search, memory shelf, image upload/display/delete, owner audit history, logout, and unauthenticated access rejection. It creates and deletes one disposable entry at `NIKKI_SMOKE_ENTRY_DATE`, which defaults to `2099-12-31`; set a different unused date if that date already exists.

Password-change smoke is available but intentionally opt-in because it temporarily changes the smoke user's password and then changes it back:

```bash
export NIKKI_SMOKE_RUN_PASSWORD_CHANGE=true
read -rs NIKKI_SMOKE_NEW_PASSWORD
export NIKKI_SMOKE_NEW_PASSWORD
sh ./scripts/smoke-production.sh
```

Operational backup smoke is also opt-in because it creates sensitive backup artifacts:

```bash
NIKKI_SMOKE_RUN_BACKUP=true sh ./scripts/smoke-production.sh
```

From a browser at the public HTTPS origin after the owner user exists:

- Login works.
- Logout works.
- Re-login works.
- Entry create/edit works.
- Image upload/display/delete works.

From the EC2 host:

```bash
curl -fsS https://your-real-domain.example/api/health
curl -i https://your-real-domain.example/api/entries
```

Unauthenticated API access to protected endpoints should return the expected `401`.

## Backups

Nikki data lives in both PostgreSQL and the uploads volume. Back up both from the same backup run:

```bash
ENV_FILE=.env.production ./scripts/backup-production.sh
```

The backup artifact must be a `nikki-operational-backup-YYYYmmdd-HHMMSS.tar.gz` archive containing `manifest.json`, `db/postgres.dump`, `uploads/uploads.tar`, and checksums when checksum tooling is available. Keep it intact because diary text and uploads must be restored as one set.

Set `AGE_RECIPIENT` to create encrypted `.age` copies:

```bash
AGE_RECIPIENT=age1... ENV_FILE=.env.production ./scripts/backup-production.sh
```

If S3 is used, upload only encrypted artifacts unless plaintext upload is deliberately accepted for a private bucket:

```bash
BACKUP_DIR=backups/<timestamp> \
  S3_BUCKET=your-private-backup-bucket \
  S3_PREFIX=nikki \
  AWS_REGION=ap-northeast-1 \
  ./scripts/upload-backup-s3.sh
```

## Restore Verification

Do not test destructive restore on the production instance. Use the isolated restore procedure in `docs/BACKUP_RESTORE.md` and verify:

```bash
# Follow docs/BACKUP_RESTORE.md with isolated volumes and alternate ports.
# Required evidence:
# - restored DB count matches source
# - restored upload hash check passes
# - sample restored image returns 200 when authenticated
```

## Rollback and Stop

Stop the app without deleting volumes:

```bash
docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml stop
```

Roll back code by checking out the previous reviewed revision, then rerun config validation, build, and `up -d`. Take a backup before deploying any release that changes schema.

## Schema Changes

Nikki currently uses automatic idempotent schema setup from `backend/internal/db/schema.sql` through the backend startup path. Versioned migrations are not implemented in this pass.

Do not add a migration framework unless an explicit task requests it.

Every schema-changing release requires a manual runbook:

1. Back up production data, including PostgreSQL and the matching uploads volume.
2. Verify the backup with an isolated restore.
3. Review the SQL manually before applying it.
4. Apply the SQL in a controlled maintenance window.
5. Deploy the application.
6. Verify health, login, entry read/write, image read/write, and backup creation.

Do not introduce a migration library or migration implementation as part of routine schema-change documentation updates.

## Healthchecks

Production containers use `restart: unless-stopped`. The backend healthcheck sends a GET request to `http://127.0.0.1:8080/api/health` from inside the backend container with `wget -q -O -` and verifies the JSON status. Do not use `wget --spider` or a HEAD-style healthcheck for the backend endpoint, because `/api/health` is expected to be checked with GET. The frontend healthcheck calls `http://127.0.0.1/` from inside the nginx container using BusyBox `wget` already present in the Alpine image.

## Cost Guardrails

Configure AWS Budgets before deployment. Include the public IPv4 hourly charge, EBS storage, EBS snapshots, CloudWatch logs, Route 53/domain costs, S3 backup storage if used, and data transfer out.

Avoid NAT Gateway, ALB, RDS, ECS, and App Runner for this initial deployment unless requirements change. They add cost and complexity that the current single-host Compose architecture does not require.
