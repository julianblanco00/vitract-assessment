import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import { getPatients } from "../api";
import StatusBadge from "../components/StatusBadge";

const cardClass = "mb-5 rounded-xl border border-slate-200 bg-white px-6 py-5";
const thClass = "border-b border-slate-200 px-2.5 py-2 text-left text-xs font-semibold tracking-wide text-gray-500 uppercase";
const tdClass = "border-b border-slate-200 px-2.5 py-2 text-left";
const tbodyClass = "[&>tr:last-child>td]:border-b-0";

export default function Dashboard() {
  const [page, setPage] = useState(1);

  const patientsQuery = useQuery({ queryKey: ["patients", page], queryFn: () => getPatients(page) });

  if (patientsQuery.isPending) {
    return <p className="text-gray-500">Loading…</p>;
  }
  if (patientsQuery.isError) {
    return <p className="text-red-700">Failed to load dashboard: {patientsQuery.error.message}</p>;
  }

  const { items: patients, total_pages } = patientsQuery.data;

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold">Patients &amp; Samples</h1>

      {patients.map((patient) => (
        <section key={patient.id} className={cardClass}>
          <h2 className="mb-1 text-lg font-semibold">{patient.name}</h2>
          <p className="mb-4 text-sm text-gray-500">
            {patient.patient_id} · {patient.email} · practitioner: {patient.practitioner_name}
          </p>

          {patient.samples.length === 0 ? (
            <p className="text-gray-500">No samples yet.</p>
          ) : (
            <table className="w-full border-collapse text-sm">
              <thead>
                <tr>
                  <th className={thClass}>Sample</th>
                  <th className={thClass}>Status</th>
                  <th className={thClass}>Collected</th>
                  <th className={thClass}>Diversity score</th>
                  <th className={thClass}></th>
                </tr>
              </thead>
              <tbody className={tbodyClass}>
                {patient.samples.map((sample) => (
                  <tr key={sample.job_id}>
                    <td className={tdClass}>{sample.sample_id ?? sample.source_ref}</td>
                    <td className={tdClass}>
                      <StatusBadge status={sample.status} />
                    </td>
                    <td className={tdClass}>{sample.collected_date ?? "—"}</td>
                    <td className={tdClass}>{sample.diversity_score?.toFixed(4) ?? "—"}</td>
                    <td className={tdClass}>
                      <Link to={`/samples/${sample.job_id}`} className="text-blue-600 hover:underline">
                        View
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      ))}

      {total_pages > 1 && (
        <div className="mb-5 flex items-center justify-center gap-4 text-sm">
          <button
            type="button"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
            className="text-blue-600 hover:underline disabled:cursor-not-allowed disabled:text-gray-300 disabled:no-underline"
          >
            ← Previous
          </button>
          <span className="text-gray-500">
            Page {page} of {total_pages}
          </span>
          <button
            type="button"
            disabled={page >= total_pages}
            onClick={() => setPage((p) => p + 1)}
            className="text-blue-600 hover:underline disabled:cursor-not-allowed disabled:text-gray-300 disabled:no-underline"
          >
            Next →
          </button>
        </div>
      )}
    </div>
  );
}
