# Vitract Technical Assignment — Context & Architecture Decisions

This document summarizes the design decisions made so far for the Vitract microbiome lab platform assignment, so work can continue from here (e.g. in Claude Code) without re-deriving the reasoning.

## Assignment recap

Build a slice of a microbiome lab platform: lab result files for patient samples are ingested by a background job, scored (Shannon diversity index), stored, and surfaced to a practitioner, who is notified by email when a result is ready.

- **Backend:** Go
- **Frontend:** React
- **DB:** PostgreSQL (via GORM)
- **Stack constraint:** ingestion must be asynchronous — a background job, not inline in the request that receives the file.
- Provided input: a **patient roster** file + a **set of lab result files** (one JSON per sample, see format below).

## Sample lab result file format

```json
{
  "sample_id": "S1001",
  "patient_id": "P001",
  "sequencing_type": "16S",
  "collected_date": "2026-05-02",
  "taxa": [
    { "name": "Bacteroides", "abundance": 0.40 },
    { "name": "Faecalibacterium", "abundance": 0.25 },
    { "name": "Prevotella", "abundance": 0.20 },
    { "name": "Akkermansia", "abundance": 0.10 },
    { "name": "Roseburia", "abundance": 0.05 }
  ]
}
```

Format of the patient roster is not yet confirmed — need to check whether it includes practitioner info per patient, or whether practitioner assignment needs to be decided as an assumption.

## Data model (GORM / Postgres)

Key decisions:
- IDs are custom strings coming from the source JSON (`sample_id`, `patient_id`), not GORM's default `uint`. No `gorm.Model` embedding on `Patient`/`Sample`/`Practitioner`/`SampleProcessingJob`/`IngestionFailure` — declared fields explicitly instead, to avoid the `ID uint` vs `ID string` conflict.
- `Taxon` (singular of "taxa") uses `gorm.Model` since it has no external ID — GORM autogenerates `uint`.
- Relationships are declared explicitly with `foreignKey` tags even where GORM's naming convention would infer them, for readability given the custom string IDs.
- `DiversityScore` is `float64` — **never `uint`**, since Shannon's index is a non-integer, non-negative float.
- FK delete behavior:
  - `Patient → Sample`: `OnDelete:RESTRICT` (samples have standalone clinical value, shouldn't vanish silently).
  - `Sample → Taxon`: `OnDelete:CASCADE` (taxa have no meaning outside their sample).
  - `Sample → SampleProcessingJob`: `OnDelete:CASCADE`, and `SampleID` has a `uniqueIndex` (one job per sample, retries update the same row via an `Attempts` counter rather than creating new rows — a documented simplification for this assignment).
  - `Practitioner → Patient`: `OnDelete:RESTRICT`.
- Soft delete (`gorm.DeletedAt`) used on `Patient`/`Sample`/`Practitioner`. Note: **Postgres `ON DELETE CASCADE` does NOT fire on GORM soft deletes** (they're just `UPDATE deleted_at`, not real `DELETE`s). If cascading soft-delete of `Taxon` when a `Sample` is soft-deleted is desired, needs a `BeforeDelete` hook that manually propagates it. Still an open TODO.
- `Sample.Status`-equivalent lives on `SampleProcessingJob.Status`, not on `Sample` itself: `pending | processing | success | failed`, enforced via a Postgres `CHECK` constraint (not a native Postgres `ENUM` — simpler to alter later, documented tradeoff).
- `SampleProcessingJob.Source` (`local_file`, extensible) also uses a `CHECK` constraint, paired with `SourceRef` (the opaque reference within that source — a file path for `local_file`, would be an S3 key/URL for other sources later).

```go
type Patient struct {
    ID             string `gorm:"primaryKey"`
    Name           string `gorm:"not null"`
    Email          string `gorm:"uniqueIndex;not null"`
    Birthday       time.Time
    PractitionerID string `gorm:"not null;index"`
    Practitioner   Practitioner `gorm:"foreignKey:PractitionerID"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      gorm.DeletedAt `gorm:"index"`

    Samples []Sample `gorm:"foreignKey:PatientID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

type Practitioner struct {
    ID        string `gorm:"primaryKey"`
    Name      string `gorm:"not null"`
    Email     string `gorm:"uniqueIndex;not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`

    Patients []Patient `gorm:"foreignKey:PractitionerID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

type Sample struct {
    ID             string    `gorm:"primaryKey"`
    PatientID      string    `gorm:"not null;index"`
    SequencingType string    `gorm:"not null"`
    CollectedDate  time.Time `gorm:"not null"`
    DiversityScore float64
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      gorm.DeletedAt `gorm:"index"`

    Taxa          []Taxon             `gorm:"foreignKey:SampleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
    ProcessingJob SampleProcessingJob `gorm:"foreignKey:SampleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type Taxon struct {
    gorm.Model
    Name      string  `gorm:"not null"`
    Abundance float32 `gorm:"not null;check:abundance >= 0"`
    SampleID  string  `gorm:"not null;index"`
}

type SampleProcessingJob struct {
    ID         string `gorm:"primaryKey"`
    SampleID   string `gorm:"not null;uniqueIndex"`
    Status     string `gorm:"type:varchar(20);not null;index;default:'pending';check:status IN ('pending','processing','success','failed')"`
    Attempts   int    `gorm:"not null;default:0"`
    Error      string
    FinishedAt time.Time
    Source     string `gorm:"type:varchar(20);not null;check:source IN ('local_file')"` // extensible: s3, presigned_url, ...
    SourceRef  string
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

type IngestionFailure struct {
    ID           string `gorm:"primaryKey"`
    Source       string `gorm:"type:varchar(20);not null;check:source IN ('local_file')"`
    SourceRef    string `gorm:"not null"`
    RawSampleID  string `gorm:"index"`
    RawPatientID string `gorm:"index"`
    Reason       string `gorm:"not null"`
    CreatedAt    time.Time
}
```

**Enum creation caveat:** GORM doesn't manage native Postgres `ENUM` types via `AutoMigrate`. We opted for `varchar` + `CHECK` constraint instead (simpler to alter, avoids raw-SQL enum bootstrapping). If a native enum is ever wanted, it needs `db.Exec("CREATE TYPE ... AS ENUM (...)")` run before `AutoMigrate`.

**UPDATE (superseded above):** the schema actually implemented diverges from the string-PK design above — see "Implementation notes" at the end of this document for what's actually in `packages/models`.

## Ingestion & processing pipeline architecture

### Core principle: "not inline in the request"

The brief requires that receiving a file and processing it (parsing, scoring, persisting) are decoupled. Whatever receives the file (HTTP endpoint or ingestion script) only creates a `Sample` + a `pending` `SampleProcessingJob` and returns immediately. A separate **worker** process picks up jobs asynchronously.

### How files arrive (design discussion, not yet built)

Considered options: direct HTTP upload, directory/bucket polling, presigned-URL-via-webhook (how this would likely work in a real system, since two companies wouldn't share a bucket directly), and a CLI ingestion script for this assignment's fixed dataset.

**Decision:** Build an HTTP upload endpoint as the architecturally "real" entry point (documented in README as standing in for what would be an S3 `ObjectCreated` event / webhook in production), AND a CLI ingestion script that processes the provided sample dataset through the same internal logic. **Upload endpoint work was paused** to focus on the processing pipeline first — still TODO.

**UPDATE:** given the brief says "the **provided** lab result files" (a fixed dataset, not client uploads), the HTTP upload endpoint was deprioritized. The CLI intake path is what's actually built; see "Implementation notes."

### `ingestion` package — abstracts "where files come from" AND "how to parse them"

Decision: the `Source` interface owns both fetching bytes and parsing them into a generic `SampleData` shape, since different sources will have fundamentally different formats (local JSON file vs. whatever an S3 provider sends). This keeps the worker fully agnostic of file format.

**Deliberate non-reuse of `models.Sample`/`models.Taxon` for JSON parsing:** the on-disk/wire format (`SampleData`/`TaxonData`, JSON tags) is kept separate from the persistence model (`gorm` tags, `time.Time` fields, relations). They'll diverge over time and mixing concerns (serialization vs. persistence) in one struct causes friction (e.g. `CollectedDate` is a `string` on the wire but `time.Time` in the DB).

### `scoring` package — pure function, decoupled from GORM models

`Shannon(abundances []float32) float64` computes `H = -sum(p_i * ln(p_i))`, normalizing internally so callers don't need pre-normalized input. Kept in its own package (no `models` dependency) specifically so it's trivially unit-testable with plain `[]float32` inputs/outputs and reusable outside the worker if needed later.

### `notification` package

`Notifier` interface + `LogNotifier` — the documented stub in place of a real email provider (SES/SendGrid/etc.), per the assignment's allowance for "a clearly documented stub... where a real account would be needed."

**Design decision:** if notification fails, the job is still marked `success` (only logged) — a failed email shouldn't invalidate an already-correctly-computed and persisted result. Noted as a place to improve later (e.g. a `NotifiedAt` field + retry) — documented as a future improvement, not built.

### `worker` package

`claimNextJob` atomically claims the oldest pending job using `SELECT ... FOR UPDATE SKIP LOCKED`, so multiple worker instances/goroutines can run concurrently without double-processing the same job. Returns `*models.SampleProcessingJob` (pointer) so `nil` can unambiguously signal "no pending job," and so mutations (`Status`, `Error`, etc.) persist back through `Save` without copy semantics surprises.

**UPDATE (major change from the original design below):** `processJob` now owns *all* of validate/score/store — see "Implementation notes."

### Ingestion of sample files + patient/practitioner matching

**Decision on unmatched `patient_id`:** rather than silently skipping (log-only) or relaxing the FK to allow orphan samples (`PatientID` nullable + `RawPatientID`, considered but rejected as unnecessary complexity for this assignment), we record an `IngestionFailure` row and do **not** create the `Sample`. This preserves strict referential integrity while still giving visibility (dashboard-able) into what failed and why. If the roster is later corrected, re-running ingestion on the same file succeeds normally (no uniqueness constraint blocks retry).

**Directory layout to avoid the server auto-reading unprocessed files on startup:**

The concern raised: registering `LocalFileSource{BaseDir: "./data"}` at startup does NOT itself trigger reading — only `Worker.processJob` calling `source.Fetch` does, and that only happens for jobs that were explicitly created. The originally proposed `incoming/`/`processed/` directory split (moving files after ingestion) was **not** used in the final implementation — idempotency is DB-driven instead (see "Implementation notes"), so the sample files stay in place in `data/samples/`.

## Still TODO (not built yet, only designed/discussed)

1. ~~`cmd/ingest` CLI command~~ — **built**, see below.
2. ~~Patient roster ingestion~~ — **built** as a migration, see below. Practitioner assignment: single seeded "house" practitioner, since the roster has no practitioner column and the dataset only has 3 patients (documented assumption).
3. ~~`database.Connect()`~~ — **built**, env-var based DSN via `.env` (loaded with `godotenv` in each `main()`), returns `(*gorm.DB, error)`.
4. **HTTP upload endpoint** — deprioritized (see UPDATE above), not built.
5. **M3: Results API** — **built**: `GET /patients`, `GET /samples`, `GET /samples/{id}`. See below.
6. **Ingestion failures endpoint** — `GET /ingestion-failures` to list `IngestionFailure` rows for the dashboard. Not built yet.
7. **M4: React dashboard** — not started at all. Needs: patient/sample list with status, sample detail view (score + taxa breakdown, including grouping redeliveries of the same `sample_id` as "previous versions"), and an ingestion-failures panel.
8. **Operability**: health check endpoint, structured logging around the pipeline (partially covered by `log.Printf` calls sprinkled through the worker/ingestion code — could be formalized), and README section on deployment/monitoring approach.
9. **Soft-delete cascade gap**: not applicable in the current schema in the same way — see "Implementation notes" (soft delete is still present via `gorm.Model` on every table, but the `BeforeDelete` cascade-to-`Taxon` hook is still an open TODO).
10. **Retry policy**: no `Attempts` field/retry logic was implemented — a failure is terminal. Worth deciding: auto-retry N times before giving up, or leave manual/out of scope with a README note.

## Key architectural principles applied throughout

- Keep wire/file formats (`SampleData`, JSON-tagged structs) separate from persistence models (`gorm`-tagged structs) — they'll diverge over time.
- Prefer explicit `foreignKey` tags over relying on GORM's naming convention, given custom string IDs.
- Interfaces (`Source`, `Notifier`) over concrete types at integration boundaries, with a documented in-memory/local stub for anything that would need real infrastructure/credentials in production (per the assignment's explicit allowance for this).
- Pure, dependency-free functions (`scoring.Shannon`) separated from orchestration code (`worker`) for testability.
- Favor a DB-backed job queue (`SampleProcessingJob` table + `SKIP LOCKED`) over an in-memory channel-based queue, specifically because the brief emphasizes "operability" and "graceful handling of failures" — an in-memory queue loses all pending work on a crash/restart, which is unacceptable for a system founders depend on in production.

## Implementation notes (what's actually built, supersedes some decisions above)

The design evolved during implementation. The points below are the current, actual state — where they conflict with earlier sections in this document, these win.

**Primary keys are standardized `uint` (via `gorm.Model`) on every model**, not the custom string PKs originally planned. Models sourced from an external file (`Patient`, `Sample`) additionally carry that file's own identifier as a separate, plain indexed column (`Patient.PatientID`, `Sample.SampleID`), kept apart from the primary key:
- `Patient.PatientID` — `uniqueIndex` (the roster guarantees uniqueness).
- `Sample.SampleID` — **plain index, deliberately NOT unique.** A redelivered file for the same `sample_id` (e.g. `S1001_redelivery.json`) becomes its own `Sample` row with its own `uint` ID, rather than being rejected or overwriting the prior delivery. This keeps full history — every delivery is queryable, and a dashboard can show older ones as "previous version" of the same `sample_id`.
- `Practitioner` has no external ID column at all — it isn't sourced from any external file in this assignment (it's synthesized), so it's just a `gorm.Model` uint PK, same as `Taxon`, `SampleProcessingJob`, `IngestionFailure`.

All foreign keys point at the internal `uint` ID, not the external string (e.g. `Sample.PatientID uint`, `Taxon.SampleID uint`).

**`SampleProcessingJob.SampleID` is `*uint` (nullable), not a required FK.** This is the biggest behavioral change from the original design: a job is created by intake *before* its file is ever opened. Intake (`packages/intake.EnqueueSamples`) only knows a `SourceRef` (filename) exists — it does zero parsing. Everything the assignment describes as one unit of background work — **validate the file, compute its diversity score, and store the result with a processing status** — happens entirely inside `worker.processJob`: fetch+parse (validate), look up the patient, compute the Shannon score, create the `Sample` + `Taxon` rows, and only then set `job.SampleID` and flip `Status` to `success`. This was a deliberate re-read of the brief: "not inline in the request" was judged to mean intake should do *no* file I/O at all, not just skip the scoring step.

`IngestionFailure` is still recorded from inside the worker (not intake) when a file parses fine but its `patient_id` doesn't match any `Patient` — same rationale as originally documented (preserve referential integrity, keep visibility into what failed and why), just triggered from a different place.

**Roster ingestion is two migrations run via `gormigrate`** (`packages/migrations`), not a runtime CLI step: `202608130001_create_schema` (wraps `AutoMigrate` for all 6 models) and `202608130002_seed_roster` (seeds the single house `Practitioner`, then creates every `Patient` row from `data/patients.csv`). Migrations are a one-time, versioned, tracked-in-DB operation — appropriate for the roster/practitioner, which is setup data for a fresh DB, unlike sample files which keep arriving over time. `cmd/migrate` runs them explicitly; they are **not** run implicitly by `cmd/ingest` or the server, by design (migrations should be a deliberate, separate step).

**Package layout (as built):**
- `packages/database` — `Connect()` only (DSN from env vars). No `Migrate()` — see migrations above.
- `packages/migrations` — the two `gormigrate` migrations described above.
- `packages/models` — the 6 GORM models.
- `packages/ingestion` — `Source` interface, `Registry`, `LocalFileSource` (fetch + parse a lab result JSON file into `SampleData`). Unchanged from the original design.
- `packages/intake` — (originally sketched as `packages/ingest`, renamed to avoid confusion with `packages/ingestion`, which actually does the fetching). `EnqueueSamples(db, sourceType, dir)`: scans a directory, creates a `pending` `SampleProcessingJob` per new file (by `SourceRef`), skips files that already have a job. Never opens/parses files.
- `packages/scoring` — `Shannon()`, unchanged.
- `packages/notification` — `Notifier`/`LogNotifier`, unchanged.
- `packages/worker` — `Worker.Run(interval)`; `processJob` does 100% of validate+score+store+notify, as described above.
- `packages/api` — `NewRouter(db) http.Handler`, the M3 Results API (`GET /patients`, `GET /samples`, `GET /samples/{id}`), using Go 1.22+'s stdlib `http.ServeMux` method+path routing (no external router dependency). Response DTOs are hand-written structs, not raw GORM models. **Gotcha fixed:** dates (`Birthday`, `CollectedDate`) must be formatted with `.UTC()` before `.Format("2006-01-02")` — without it, Postgres/pgx returns the value converted to the local session timezone, which can shift the displayed calendar date by a day.
- `cmd/migrate` — runs `migrations.Run(db)`, explicitly, once.
- `cmd/ingest` — runs `intake.EnqueueSamples(db, ...)` only (no migrations bundled in).
- `main.go` (root) — the server: starts the worker in a goroutine (`go w.Run(...)`), then blocks on `http.ListenAndServe(":8080", api.NewRouter(db))`.

**Local dev setup:** `docker-compose.yml` runs Postgres on host port **5433** (not 5432 — a native Postgres install was already bound to 5432 on the dev machine, so the container was remapped to avoid the conflict). `.env` (gitignored) holds `DB_HOST=localhost`, `DB_USER=vitract`, `DB_PASSWORD=vitract`, `DB_NAME=vitract`, `DB_PORT=5433` — these are dummy local dev credentials matching `docker-compose.yml`, hardcoded there deliberately (not a real secret) so a reviewer can run the project with zero config. Loaded via `godotenv.Load()` called explicitly at the top of each `main()` (not inside `database.Connect()` — env loading is a composition-root concern, not something a `Connect()` function should do as a side effect).

**Verified end-to-end** against the provided dataset: `cmd/migrate` → `cmd/ingest` → server (worker + API). Of the 8 sample files, 6 processed successfully (including both `S1001.json` and `S1001_redelivery.json`, each producing its own `Sample` row/score as designed), `S1006.json` failed on invalid JSON, and `S1007.json` failed on an unmatched `patient_id` (`P999`) and correctly produced an `IngestionFailure` row.
