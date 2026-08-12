# Vitract Lab Dashboard

A small dashboard for the Vitract Lab API: a list of patients and their samples with processing status, and a detail view with the diversity score and per-taxon breakdown.

## What it does

- **Dashboard** (`/`) — every patient with their samples, each showing its processing status (`pending`/`processing`/`success`/`failed`), collected date, and diversity score once available. If a sample was delivered more than once (same `sample_id`, a newer file), older deliveries are shown tagged as a previous version instead of being hidden. Files that couldn't be matched to any patient (e.g. an unknown `patient_id`) are listed separately, with their error.
- **Sample detail** (`/samples/:jobId`) — patient, sequencing type, collected date, the diversity score, and the full taxa/abundance breakdown for a successfully processed sample; the failure reason for one that isn't.

## How it works

It's a plain client-rendered React app — no login, no write operations, just reading from the API. `src/api.ts` is a small typed fetch client for the three read endpoints (`/patients`, `/samples`, `/samples/:id`); `src/types.ts` mirrors the API's JSON response shapes. The dashboard page fetches patients and samples separately and groups samples under their patient client-side by matching `patient_id`, rather than the API doing that shaping server-side.

## Running it

**Requirements:** Node 18+, and the [API](../api) running locally on `:8080`.

```
npm install
npm run dev
```

Opens on `http://localhost:5173`. If the API isn't on `http://localhost:8080`, point the dashboard at it with a `.env` file:

```
VITE_API_URL=http://your-api-host:port
```

### Build

```
npm run build
```
