# Vitract Lab API

Backend for a microbiome lab platform: lab results for patient samples are ingested from files, scored with the Shannon diversity index, stored, and surfaced through a small REST API — with a practitioner notified once a result is ready.

## What it does

A lab produces one JSON file per sample (taxa + their relative abundances). This service:

1. **Registers** each new file as a job (`pending`), without reading its contents.
2. **Processes** that job asynchronously: parses the file, matches the sample to a patient from the roster, computes the Shannon diversity index over the taxa abundances, persists the result, and notifies the patient's practitioner.
3. **Serves** the results (patients, samples with status, per-sample taxa breakdown) over HTTP for the dashboard.

Ingestion never runs inline in a request — a file being registered and a file being processed are two separate steps, so a slow or failing computation can never block whatever triggered the ingestion.

## How it works

**Intake vs. processing.** `packages/intake` only looks at a directory of lab result files and creates a `pending` `SampleProcessingJob` per file it hasn't seen before (tracked by filename, so re-running it is safe). It never opens a file. All of the actual work — reading the file, validating it, matching the patient, scoring, and persisting — happens in `packages/worker`, picked up asynchronously. A job claims itself with `SELECT ... FOR UPDATE SKIP LOCKED`, so more than one worker could run concurrently without double-processing anything.

**Data model.** A `Patient` belongs to a `Practitioner`; a `Sample` belongs to a `Patient` and has many `Taxon` rows plus one `SampleProcessingJob`. A sample's external `sample_id` (from the file) is *not* a unique key — if the same `sample_id` is delivered again in a different file, it becomes its own `Sample` row with its own score, rather than overwriting the earlier one. That keeps every delivery in history instead of losing the previous result.

**Unmatched patients.** If a file's `patient_id` doesn't exist in the roster, no `Sample` is created — the job is marked `failed` and an `IngestionFailure` row records what the file claimed, so the roster can be fixed and the file dealt with without silently dropping data.

**Setup vs. ongoing ingestion.** The patient roster and a single practitioner (the roster has no practitioner column, so every patient is assigned to one seeded practitioner) are created by versioned migrations (`packages/migrations`, via `gormigrate`) — a one-time setup step for a fresh database. Sample files, on the other hand, keep arriving over time, so they go through the repeatable intake step instead.

**Notifications** are a logged stub (`packages/notification`) standing in for a real email provider. A failed notification doesn't undo an already-computed result — it's just logged.

## Project layout

```
main.go              starts the worker (background goroutine) + HTTP API
cmd/migrate           runs the schema + roster migrations, once
cmd/ingest             scans data/samples for new files and registers jobs
packages/
  models               GORM models
  database             DB connection
  migrations           schema creation + roster/practitioner seed
  ingestion            reads and parses a lab result file
  intake               registers new files as pending jobs
  worker               claims jobs, validates/scores/stores/notifies
  scoring              Shannon diversity index (pure function)
  notification         notifier interface + logging stub
  api                  HTTP handlers for the dashboard
  dbtest               Postgres-backed test database helper
data/                  the provided patient roster + sample files
```

## Running it

**Requirements:** Go 1.25+, Docker.

1. Start Postgres:
   ```
   docker compose up -d
   ```
2. Create `.env` in this directory (matches `docker-compose.yml`):
   ```
   DB_HOST=localhost
   DB_USER=vitract
   DB_PASSWORD=vitract
   DB_NAME=vitract
   DB_PORT=5433
   ```
3. Run the migrations (creates the schema, seeds the practitioner and patient roster):
   ```
   go run cmd/migrate/main.go
   ```
4. Register the provided sample files as jobs:
   ```
   go run cmd/ingest/main.go
   ```
5. Start the server (processes the queued jobs and serves the API on `:8080`):
   ```
   go run main.go
   ```

### Tests

```
go test ./...
```

Tests that touch the database run against a real Postgres (`vitract_test`, created automatically alongside the dev database) so behavior like `SKIP LOCKED` is exercised for real — each test runs inside a transaction that's rolled back afterwards, so they don't need Docker to be freshly reset between runs.
