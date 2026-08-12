import { useQuery } from "@tanstack/react-query";
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

export default function Dashboard() {
  const patientsQuery = useQuery({ queryKey: ["patients"], queryFn: getPatients });
  const samplesQuery = useQuery({ queryKey: ["samples"], queryFn: getSamples });

  if (patientsQuery.isPending || samplesQuery.isPending) {
    return <p className="state-message">Loading…</p>;
  }
  if (patientsQuery.isError || samplesQuery.isError) {
    const message = (patientsQuery.error ?? samplesQuery.error) as Error;
    return <p className="state-message error">Failed to load dashboard: {message.message}</p>;
  }

  const { groups, unmatched } = groupByPatient(patientsQuery.data, samplesQuery.data);

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
                {samples.map((sample) => (
                  <tr key={sample.job_id}>
                    <td>{sample.sample_id ?? sample.source_ref}</td>
                    <td>
                      <StatusBadge status={sample.status} />
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
