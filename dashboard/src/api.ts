import type { PaginatedResponse, Patient, SampleDetail, SampleListItem } from "./types";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

async function get<T>(path: string): Promise<T> {
	const res = await fetch(`${API_URL}${path}`);
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error(body.error ?? `request to ${path} failed with status ${res.status}`);
	}
	return res.json();
}

export function getPatients(page = 1, pageSize = 20): Promise<PaginatedResponse<Patient>> {
	return get(`/patients?page=${page}&page_size=${pageSize}`);
}

export function getSample(jobId: number | string): Promise<SampleDetail> {
	return get(`/samples/${jobId}`);
}

export function getSampleVersions(jobId: number | string): Promise<SampleListItem[]> {
	return get(`/samples/${jobId}/versions`);
}
