# Vitract Lab Dashboard

A small dashboard for the Vitract Lab API: a list of patients and their samples with processing status, and a detail view with the diversity score and per-taxon breakdown.

## What it does

- **Dashboard** (`/`) — every patient with their samples, each showing its processing status (`pending`/`processing`/`success`/`failed`), collected date, and diversity score once available. Patients are paginated (Previous/Next), matching how `GET /patients` returns them.
- **Sample detail** (`/samples/:jobId`) — patient, sequencing type, collected date, the diversity score, and the full taxa/abundance breakdown for a successfully processed sample; the failure reason for one that isn't. If a sample was delivered more than once (same `sample_id`, a later file), the detail view lists earlier deliveries under "Previous versions", and an older delivery's page links back to the latest one.

## How it works

A client-rendered React app — no login, no write operations, just reading from the API — using [TanStack Query](https://tanstack.com/query) for data fetching/caching and [Tailwind CSS](https://tailwindcss.com) for styling. `src/api.ts` is a small typed fetch client; `src/types.ts` mirrors the API's JSON response shapes.

The dashboard page fetches one page of patients from `GET /patients` and renders each patient's `samples` array as-is — it does no client-side grouping or matching of samples to patients. That shaping happens entirely on the backend (see `api/README.md`'s "How it works"), so the dashboard only ever needs the page of patients it's currently displaying, not the full list of samples in the system.

## Running it

**Requirements:** Node 18+, [pnpm](https://pnpm.io), and the [API](../api) running locally on `:8080`.

```
pnpm install
pnpm run dev
```

Opens on `http://localhost:5173`. If the API isn't on `http://localhost:8080`, point the dashboard at it with a `.env` file:

```
VITE_API_URL=http://your-api-host:port
```

### Build

```
pnpm run build
```
