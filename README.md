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

Requires Docker, Go 1.25+, and [pnpm](https://pnpm.io). This one script:

1. Starts Postgres in Docker (creating `api/.env` with local dev defaults if it isn't there yet).
2. Waits for it to accept connections.
3. Runs the database migrations (schema + patient roster).
4. Registers the provided lab result files for processing.
5. Starts the API (`http://localhost:8080`) and the dashboard (`http://localhost:5173`) together.

Ctrl+C stops both. It's safe to re-run — migrations and file registration are idempotent, so nothing gets duplicated.

## Running things individually

See [`api/README.md`](api/README.md) and [`dashboard/README.md`](dashboard/README.md) for what `dev.sh` automates, broken into steps, plus how to run each project's tests.
