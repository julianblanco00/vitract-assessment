import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { getSample } from "../api";
import StatusBadge from "../components/StatusBadge";

export default function SampleDetail() {
  const { jobId } = useParams<{ jobId: string }>();

  const sampleQuery = useQuery({
    queryKey: ["samples", jobId],
    queryFn: () => getSample(jobId!),
    enabled: !!jobId,
  });

  return (
    <div>
      <Link to="/" className="back-link">
        ← Back to dashboard
      </Link>

      {sampleQuery.isPending && <p className="state-message">Loading…</p>}
      {sampleQuery.isError && (
        <p className="state-message error">Failed to load sample: {sampleQuery.error.message}</p>
      )}

      {sampleQuery.data && (
        <div className="patient-card">
          <h1>{sampleQuery.data.sample_id ?? sampleQuery.data.source_ref}</h1>
          <p className="patient-meta">
            <StatusBadge status={sampleQuery.data.status} /> · source file: {sampleQuery.data.source_ref}
          </p>

          {sampleQuery.data.status === "failed" && (
            <p className="state-message error">{sampleQuery.data.error}</p>
          )}

          {sampleQuery.data.status === "success" && (
            <>
              <dl className="detail-grid">
                <dt>Patient</dt>
                <dd>
                  {sampleQuery.data.patient_name} ({sampleQuery.data.patient_id})
                </dd>
                <dt>Sequencing type</dt>
                <dd>{sampleQuery.data.sequencing_type}</dd>
                <dt>Collected</dt>
                <dd>{sampleQuery.data.collected_date}</dd>
                <dt>Diversity score (Shannon)</dt>
                <dd className="score">{sampleQuery.data.diversity_score?.toFixed(4)}</dd>
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
                  {sampleQuery.data.taxa?.map((taxon) => (
                    <tr key={taxon.name}>
                      <td>{taxon.name}</td>
                      <td>{(taxon.abundance * 100).toFixed(1)}%</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}

          {(sampleQuery.data.status === "pending" || sampleQuery.data.status === "processing") && (
            <p className="state-message">This sample hasn't finished processing yet.</p>
          )}
        </div>
      )}
    </div>
  );
}
