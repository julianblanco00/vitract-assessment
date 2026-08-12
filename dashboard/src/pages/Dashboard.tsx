import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getPatients, getSamples } from "../api";
import type { Patient, SampleListItem } from "../types";
import StatusBadge from "../components/StatusBadge";

interface PatientGroup {
  patient: Patient;
  samples: SampleListItem[];
}

function groupByPatient(patients: Patient[], samples: SampleListItem[]) {
  const byPatientId = new Map<string, PatientGroup>();
  for (const patient of patients) {
    byPatientId.set(patient.patient_id, { patient, samples: [] });
  }

  const unmatched: SampleListItem[] = [];
  for (const sample of samples) {
    const group = sample.patient_id ? byPatientId.get(sample.patient_id) : undefined;
    if (group) {
      group.samples.push(sample);
    } else {
      unmatched.push(sample);
    }
  }

  for (const group of byPatientId.values()) {
    group.samples.sort((a, b) => b.created_at.localeCompare(a.created_at));
  }

  return { groups: [...byPatientId.values()], unmatched };
}

function isSupersededDelivery(samples: SampleListItem[], index: number): boolean {
  const current = samples[index];
  if (!current.sample_id) return false;
  return samples.some((s, i) => i < index && s.sample_id === current.sample_id);
}

export default function Dashboard() {
  const [patients, setPatients] = useState<Patient[]>([]);
  const [samples, setSamples] = useState<SampleListItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([getPatients(), getSamples()])
      .then(([p, s]) => {
        setPatients(p);
        setSamples(s);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <p className="state-message">Loading…</p>;
  if (error) return <p className="state-message error">Failed to load dashboard: {error}</p>;

  const { groups, unmatched } = groupByPatient(patients, samples);

  return (
    <div>
      <h1>Patients &amp; Samples</h1>

      {groups.map(({ patient, samples }) => (
        <section key={patient.id} className="patient-card">
          <h2>{patient.name}</h2>
          <p className="patient-meta">
            {patient.patient_id} · {patient.email} · practitioner: {patient.practitioner_name}
          </p>

          {samples.length === 0 ? (
            <p className="state-message">No samples yet.</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Sample</th>
                  <th>Status</th>
                  <th>Collected</th>
                  <th>Diversity score</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {samples.map((sample, i) => (
                  <tr key={sample.job_id}>
                    <td>{sample.sample_id ?? sample.source_ref}</td>
                    <td>
                      <StatusBadge status={sample.status} />
                      {isSupersededDelivery(samples, i) && (
                        <span className="previous-version-tag">previous version</span>
                      )}
                    </td>
                    <td>{sample.collected_date ?? "—"}</td>
                    <td>{sample.diversity_score?.toFixed(4) ?? "—"}</td>
                    <td>
                      <Link to={`/samples/${sample.job_id}`}>View</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      ))}

      {unmatched.length > 0 && (
        <section className="patient-card">
          <h2>Unmatched processing jobs</h2>
          <p className="patient-meta">Files that couldn't be linked to a patient in the roster.</p>
          <table>
            <thead>
              <tr>
                <th>File</th>
                <th>Status</th>
                <th>Error</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {unmatched.map((sample) => (
                <tr key={sample.job_id}>
                  <td>{sample.source_ref}</td>
                  <td>
                    <StatusBadge status={sample.status} />
                  </td>
                  <td>{sample.error ?? "—"}</td>
                  <td>
                    <Link to={`/samples/${sample.job_id}`}>View</Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  );
}
