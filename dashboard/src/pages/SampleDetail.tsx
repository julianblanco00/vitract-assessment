import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { getSample } from "../api";
import type { SampleDetail as SampleDetailType } from "../types";
import StatusBadge from "../components/StatusBadge";

export default function SampleDetail() {
  const { jobId } = useParams<{ jobId: string }>();
  const [sample, setSample] = useState<SampleDetailType | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!jobId) return;
    setLoading(true);
    getSample(jobId)
      .then(setSample)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [jobId]);

  return (
    <div>
      <Link to="/" className="back-link">
        ← Back to dashboard
      </Link>

      {loading && <p className="state-message">Loading…</p>}
      {error && <p className="state-message error">Failed to load sample: {error}</p>}

      {sample && (
        <div className="patient-card">
          <h1>{sample.sample_id ?? sample.source_ref}</h1>
          <p className="patient-meta">
            <StatusBadge status={sample.status} /> · source file: {sample.source_ref}
          </p>

          {sample.status === "failed" && (
            <p className="state-message error">{sample.error}</p>
          )}

          {sample.status === "success" && (
            <>
              <dl className="detail-grid">
                <dt>Patient</dt>
                <dd>
                  {sample.patient_name} ({sample.patient_id})
                </dd>
                <dt>Sequencing type</dt>
                <dd>{sample.sequencing_type}</dd>
                <dt>Collected</dt>
                <dd>{sample.collected_date}</dd>
                <dt>Diversity score (Shannon)</dt>
                <dd className="score">{sample.diversity_score?.toFixed(4)}</dd>
              </dl>

              <h2>Taxa breakdown</h2>
              <table>
                <thead>
                  <tr>
                    <th>Taxon</th>
                    <th>Relative abundance</th>
                  </tr>
                </thead>
                <tbody>
                  {sample.taxa?.map((taxon) => (
                    <tr key={taxon.name}>
                      <td>{taxon.name}</td>
                      <td>{(taxon.abundance * 100).toFixed(1)}%</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}

          {(sample.status === "pending" || sample.status === "processing") && (
            <p className="state-message">This sample hasn't finished processing yet.</p>
          )}
        </div>
      )}
    </div>
  );
}
