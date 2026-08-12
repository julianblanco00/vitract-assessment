import type { SampleStatus } from "../types";

export default function StatusBadge({ status }: { status: SampleStatus }) {
  return <span className={`status-badge status-${status}`}>{status}</span>;
}
