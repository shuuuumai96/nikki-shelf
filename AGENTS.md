# AGENTS.md

## Project

- Frontend: Vue + Vite + pnpm.
- Backend: Go + Echo + PostgreSQL via pgx.
- Runtime: Docker Compose.

## Commands

- Frontend install/build: `cd frontend && corepack pnpm install --frozen-lockfile && corepack pnpm build`
- Frontend format: `cd frontend && corepack pnpm format`
- Frontend format check: `cd frontend && corepack pnpm format:check`
- Repository format: `python3 scripts/format.py`
- Repository format check: `python3 scripts/format.py --check`
- Repository format on Windows if `python3` is unavailable: `python .\scripts\format.py`
- Repository format check on Windows if `python3` is unavailable: `python .\scripts\format.py --check`
- Backend test: `cd backend && go test ./...`
- Docker build: `docker compose build`
- Run app: `docker compose up -d`

## Rules

- Do not use npm for frontend dependency changes. Keep `pnpm-lock.yaml` updated.
- Backend module path is `github.com/shuuuumai96/nikki-shelf/backend`.
- Backend formatting uses `goimports` with `github.com/shuuuumai96/nikki-shelf/backend` as the local import group.
- Keep backend code data-oriented. Avoid large state structs and excessive branching; prefer small functions and maps/strategies where practical.
- Keep UI simple, calm, document-oriented, and low-clutter.
- Do not commit generated data, uploads, `node_modules`, or build output.

## Local Git Hooks

- Local hooks are optional developer safeguards.
- Enable them with: `git config core.hooksPath .githooks`
- Disable them with: `git config --unset core.hooksPath`
- The current pre-push hook blocks pushes to master and main.
- Work should normally happen on feature branches.
- The hook does not replace GitHub Actions CI.
- The hook does not replace server-side branch protection.
- Keep hooks lightweight.
- Do not add build, test, deploy, Docker, dependency installation, network calls, or secret access to pre-push hooks.

## GitHub Actions Rules

- Do not use floating runner labels such as `ubuntu-latest`, `windows-latest`, or `macos-latest`. Use explicit runner labels such as `ubuntu-24.04`, and make runner version changes explicit in workflow diffs.
- Pin external GitHub Actions to full-length commit SHAs. Do not use mutable refs such as `actions/checkout@v4` directly in workflow files. Resolve SHAs from official upstream action repositories, add a short YAML source-tag comment such as `# actions/checkout v4`, and do not pin to forks unless explicitly approved.
- Keep baseline CI least-privilege. Use `permissions: contents: read` unless a workflow truly needs more, and do not add write permissions without explicit justification.
- Keep baseline CI non-deploying. Do not add secrets, deployment, Docker publishing, release automation, AWS, S3, SSH, EC2, Caddy, or production integration.
- Do not use `pull_request_target` without a separate security review. Use `pull_request` for normal PR validation.
- Avoid `latest` in validation commands. Do not use commands such as `go run ...@latest` for validation unless explicitly approved; prefer installed tools, pinned versions, or documented project-local commands.
- Do not expand CI scope casually. Do not add CodeQL, OpenSSF Scorecard, dependency bots, license scanning, secret scanning, E2E tests, or Docker builds without a separate, explicit change.
