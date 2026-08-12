import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { getSample, getSampleVersions } from "../api";
import StatusBadge from "../components/StatusBadge";

const thClass = "border-b border-slate-200 px-2.5 py-2 text-left text-xs font-semibold tracking-wide text-gray-500 uppercase";
const tdClass = "border-b border-slate-200 px-2.5 py-2 text-left";
const tbodyClass = "[&>tr:last-child>td]:border-b-0";

export default function SampleDetail() {
	const { jobId } = useParams<{ jobId: string }>();

	const sampleQuery = useQuery({
		queryKey: ["samples", jobId],
		queryFn: () => getSample(jobId!),
		enabled: !!jobId,
	});

	const versionsQuery = useQuery({
		queryKey: ["samples", jobId, "versions"],
		queryFn: () => getSampleVersions(jobId!),
		enabled: !!jobId,
	});

	const previousVersions = versionsQuery.data ?? [];

	return (
		<div>
			<Link to="/" className="mb-5 inline-block text-sm text-blue-600 hover:underline">
				← Back to dashboard
			</Link>

			{sampleQuery.isPending && <p className="text-gray-500">Loading…</p>}
			{sampleQuery.isError && <p className="text-red-700">Failed to load sample: {sampleQuery.error.message}</p>}

			{sampleQuery.data && (
				<div className="rounded-xl border border-slate-200 bg-white px-6 py-5">
					<h1 className="text-2xl font-semibold">{sampleQuery.data.sample_id ?? sampleQuery.data.source_ref}</h1>
					<p className="mt-1 mb-4 text-sm text-gray-500">
						<StatusBadge status={sampleQuery.data.status} /> · source file: {sampleQuery.data.source_ref}
					</p>

					{sampleQuery.data.latest_job_id && (
						<p className="mb-4 text-sm text-gray-500">
							This is a previous version.{" "}
							<Link to={`/samples/${sampleQuery.data.latest_job_id}`} className="text-blue-600 hover:underline">
								View the latest version →
							</Link>
						</p>
					)}

					{sampleQuery.data.status === "failed" && <p className="text-red-700">{sampleQuery.data.error}</p>}

					{sampleQuery.data.status === "success" && (
						<>
							<dl className="mb-6 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1.5">
								<dt className="text-sm text-gray-500">Patient</dt>
								<dd>
									{sampleQuery.data.patient_name} ({sampleQuery.data.patient_id})
								</dd>
								<dt className="text-sm text-gray-500">Sequencing type</dt>
								<dd>{sampleQuery.data.sequencing_type}</dd>
								<dt className="text-sm text-gray-500">Collected</dt>
								<dd>{sampleQuery.data.collected_date}</dd>
								<dt className="text-sm text-gray-500">Diversity score (Shannon)</dt>
								<dd className="text-lg font-bold">{sampleQuery.data.diversity_score?.toFixed(4)}</dd>
							</dl>

							<h2 className="mb-1 text-lg font-semibold">Taxa breakdown</h2>
							<table className="w-full border-collapse text-sm">
								<thead>
									<tr>
										<th className={thClass}>Taxon</th>
										<th className={thClass}>Relative abundance</th>
									</tr>
								</thead>
								<tbody className={tbodyClass}>
									{sampleQuery.data.taxa?.map((taxon) => (
										<tr key={taxon.name}>
											<td className={tdClass}>{taxon.name}</td>
											<td className={tdClass}>{(taxon.abundance * 100).toFixed(1)}%</td>
										</tr>
									))}
								</tbody>
							</table>
						</>
					)}

					{(sampleQuery.data.status === "pending" || sampleQuery.data.status === "processing") && (
						<p className="text-gray-500">This sample hasn't finished processing yet.</p>
					)}

					{previousVersions.length > 0 && (
						<>
							<h2 className="mt-6 mb-1 text-lg font-semibold">Previous versions</h2>
							<table className="w-full border-collapse text-sm">
								<thead>
									<tr>
										<th className={thClass}>Status</th>
										<th className={thClass}>Collected</th>
										<th className={thClass}>Diversity score</th>
										<th className={thClass}></th>
									</tr>
								</thead>
								<tbody className={tbodyClass}>
									{previousVersions.map((version) => (
										<tr key={version.job_id}>
											<td className={tdClass}>
												<StatusBadge status={version.status} />
											</td>
											<td className={tdClass}>{version.collected_date ?? "—"}</td>
											<td className={tdClass}>{version.diversity_score?.toFixed(4) ?? "—"}</td>
											<td className={tdClass}>
												<Link to={`/samples/${version.job_id}`} className="text-blue-600 hover:underline">
													View
												</Link>
											</td>
										</tr>
									))}
								</tbody>
							</table>
						</>
					)}
				</div>
			)}
		</div>
	);
}
