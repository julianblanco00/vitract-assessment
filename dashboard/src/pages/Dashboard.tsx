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

const cardClass = "mb-5 rounded-xl border border-slate-200 bg-white px-6 py-5";
const thClass = "border-b border-slate-200 px-2.5 py-2 text-left text-xs font-semibold tracking-wide text-gray-500 uppercase";
const tdClass = "border-b border-slate-200 px-2.5 py-2 text-left";
const tbodyClass = "[&>tr:last-child>td]:border-b-0";

export default function Dashboard() {
	const patientsQuery = useQuery({ queryKey: ["patients"], queryFn: getPatients });
	const samplesQuery = useQuery({ queryKey: ["samples"], queryFn: getSamples });

	if (patientsQuery.isPending || samplesQuery.isPending) {
		return <p className="text-gray-500">Loading…</p>;
	}
	if (patientsQuery.isError || samplesQuery.isError) {
		const message = (patientsQuery.error ?? samplesQuery.error) as Error;
		return <p className="text-red-700">Failed to load dashboard: {message.message}</p>;
	}

	const { groups, unmatched } = groupByPatient(patientsQuery.data, samplesQuery.data);

	return (
		<div>
			<h1 className="mb-6 text-2xl font-semibold">Patients &amp; Samples</h1>

			{groups.map(({ patient, samples }) => (
				<section key={patient.id} className={cardClass}>
					<h2 className="mb-1 text-lg font-semibold">{patient.name}</h2>
					<p className="mb-4 text-sm text-gray-500">
						{patient.patient_id} · {patient.email} · practitioner: {patient.practitioner_name}
					</p>

					{samples.length === 0 ? (
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
								{samples.map((sample) => (
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

			{unmatched.length > 0 && (
				<section className={cardClass}>
					<h2 className="mb-1 text-lg font-semibold">Unmatched processing jobs</h2>
					<p className="mb-4 text-sm text-gray-500">Files that couldn't be linked to a patient in the roster.</p>
					<table className="w-full border-collapse text-sm">
						<thead>
							<tr>
								<th className={thClass}>File</th>
								<th className={thClass}>Status</th>
								<th className={thClass}>Error</th>
								<th className={thClass}></th>
							</tr>
						</thead>
						<tbody className={tbodyClass}>
							{unmatched.map((sample) => (
								<tr key={sample.job_id}>
									<td className={tdClass}>{sample.source_ref}</td>
									<td className={tdClass}>
										<StatusBadge status={sample.status} />
									</td>
									<td className={tdClass}>{sample.error ?? "—"}</td>
									<td className={tdClass}>
										<Link to={`/samples/${sample.job_id}`} className="text-blue-600 hover:underline">
											View
										</Link>
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
