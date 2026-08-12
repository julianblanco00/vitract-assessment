# Vitract Lab Platform

A slice of a microbiome lab platform: lab result files for patient samples are ingested asynchronously, scored with the Shannon diversity index, stored, and surfaced to a practitioner through a small dashboard.

This repo has two parts:

- **[`api/`](api)** — the Go backend: async ingestion pipeline, Postgres via GORM, and the REST API the dashboard reads from.
- **[`dashboard/`](dashboard)** — a React + Vite dashboard: patients and their samples with processing status, and a detail view with the score and per-taxon breakdown.

Each has its own README with the full picture (architecture, project layout, running it standalone, tests). This one is just the map and the quickstart.

## Quickstart

```
./dev.sh
```

Requires Docker, Go 1.25+, Node.js, and [pnpm](https://pnpm.io). The script checks for all four up front and fails with a clear message if one's missing. This one script:

1. Starts Postgres in Docker (creating `api/.env` with local dev defaults if it isn't there yet).
2. Waits for it to accept connections.
3. Runs the database migrations (schema + patient roster).
4. Registers the provided lab result files for processing.
5. Starts the API (`http://localhost:8080`), the worker, and the dashboard (`http://localhost:5173`) together.

Ctrl+C stops all three. It's safe to re-run — migrations and file registration are idempotent, so nothing gets duplicated.

## Running things individually

See [`api/README.md`](api/README.md) and [`dashboard/README.md`](dashboard/README.md) for what `dev.sh` automates, broken into steps, plus how to run each project's tests.

## Deployment

- **API** and **worker**: one Docker image, two ECS Fargate services (`main.go` and `cmd/worker`), so each scales independently. The API service sits behind an ALB, with `GET /health` as the target group's health check.
- **Migrations**: `cmd/migrate` runs automatically in a GitHub Action as part of the deploy pipeline, before the new API version rolls out.
- **Ingest**: `cmd/ingest` is only used to load this assignment's sample dataset — not a production concern.
- **Dashboard**: a static build (`pnpm run build`), deployed separately to S3 + CloudFront, pointed at the API via `VITE_API_URL`.
- **Database**: managed Postgres (RDS) instead of self-hosted; config/secrets as env vars, never `.env`.
- **Infrastructure**: Terraform, in an `infra/` directory in this repo, with state in a remote backend (S3 + lock table) — it defines the VPC, RDS, ECS cluster/services, ALB, ECR, the S3+CloudFront setup for the dashboard, and IAM/Secrets Manager. Infra changes (`terraform plan`/`apply`) run as their own gated CI job, separate from the app deploy pipeline that builds/pushes the image and rolls out a new ECS task definition — so shipping a new app version doesn't require a full Terraform run each time.

## Monitoring

Nothing here needs infrastructure beyond what the deployment above already sets up — Fargate ships every container's stdout to CloudWatch Logs automatically, and our logs are already structured JSON (`log/slog`), so CloudWatch can act on them directly without a separate monitoring stack:

- **API up/down**: the ALB's target group already tracks `HealthyHostCount` against `GET /health` (which itself checks DB connectivity, not just that the process is running). An alarm on that metric covers "the API is down or can't reach the database."
- **Worker alive**: the worker has no HTTP endpoint of its own to poll (it's a pure background loop), so it emits a `"msg":"heartbeat"` log line every 30s instead. A CloudWatch Logs metric filter turns those lines into a metric, and an alarm with `TreatMissingData: breaching` fires if no heartbeat shows up for a few minutes — that's how a wedged or crashed worker gets caught, since the API's own health check can't see it (they're separate ECS services).
- **Failure rate**: `"level":"WARN","msg":"job failed"` and `"level":"ERROR","msg":"job panicked"` are both metric-filterable the same way — alarm when either crosses a threshold in a 5-minute window.
- **Alerting**: all of the above alarms publish to an SNS topic, fanned out to email/Slack for the on-call.

What's deliberately *not* built: a `/metrics` endpoint or a dedicated heartbeat table. The structured logs already carry everything CloudWatch needs, so adding either would be duplicating state that already exists, just to have a nicer query interface than log-based metric filters — a reasonable thing to add later if CloudWatch's query ergonomics become the bottleneck, not before.

## What's not finished, and why

- **Authentication.** There's no login — the dashboard is open by design for this assignment ("something simple, no logins" was an explicit scope decision, not an oversight). For a real deployment I'd add **AWS Cognito**, to stay consistent with the rest of the AWS-based stack.
- **Multiple practitioners.** Every patient is assigned to a single seeded "house" practitioner today (a documented assumption, since the provided roster has no practitioner column). Real support for multiple practitioners — assigning patients to specific ones, and scoping what each practitioner can see — isn't built yet.
- **Filters on the patients view.** The dashboard lists all patients (paginated) with no way to search or narrow the list yet.
