export type SampleStatus = "pending" | "processing" | "success" | "failed";

export interface SampleListItem {
  job_id: number;
  source_ref: string;
  status: SampleStatus;
  error?: string;
  sample_id?: string;
  patient_id?: string;
  patient_name?: string;
  diversity_score?: number;
  collected_date?: string;
  created_at: string;
}

export interface Patient {
  id: number;
  patient_id: string;
  name: string;
  email: string;
  birthday: string;
  practitioner_name: string;
  practitioner_email: string;
  samples: SampleListItem[];
}

export interface PaginatedResponse<T> {
  items: T[];
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface Taxon {
  name: string;
  abundance: number;
}

export interface SampleDetail {
  job_id: number;
  source_ref: string;
  status: SampleStatus;
  error?: string;
  sample_id?: string;
  patient_id?: string;
  patient_name?: string;
  sequencing_type?: string;
  collected_date?: string;
  diversity_score?: number;
  taxa?: Taxon[];
  latest_job_id?: number;
}
