import type { SampleStatus } from "../types";

const statusStyles: Record<SampleStatus, string> = {
	pending: "bg-gray-100 text-gray-600",
	processing: "bg-blue-100 text-blue-700",
	success: "bg-green-100 text-green-700",
	failed: "bg-red-100 text-red-700",
};

export default function StatusBadge({ status }: { status: SampleStatus }) {
	return (
		<span
			className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-semibold capitalize ${statusStyles[status]}`}
		>
			{status}
		</span>
	);
}
