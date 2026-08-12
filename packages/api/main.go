package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"vitract-assessment/packages/models"
)

type handler struct {
	db *gorm.DB
}

func NewRouter(db *gorm.DB) http.Handler {
	h := &handler{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /patients", h.listPatients)
	mux.HandleFunc("GET /samples", h.listSamples)
	mux.HandleFunc("GET /samples/{id}", h.getSample)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type PatientResponse struct {
	ID                uint   `json:"id"`
	PatientID         string `json:"patient_id"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	Birthday          string `json:"birthday"`
	PractitionerName  string `json:"practitioner_name"`
	PractitionerEmail string `json:"practitioner_email"`
}

func (h *handler) listPatients(w http.ResponseWriter, r *http.Request) {
	var patients []models.Patient
	if err := h.db.Preload("Practitioner").Order("patients.id").Find(&patients).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]PatientResponse, len(patients))
	for i, p := range patients {
		resp[i] = PatientResponse{
			ID:                p.ID,
			PatientID:         p.PatientID,
			Name:              p.Name,
			Email:             p.Email,
			Birthday:          p.Birthday.UTC().Format("2006-01-02"),
			PractitionerName:  p.Practitioner.Name,
			PractitionerEmail: p.Practitioner.Email,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type SampleListItem struct {
	JobID          uint      `json:"job_id"`
	SourceRef      string    `json:"source_ref"`
	Status         string    `json:"status"`
	Error          string    `json:"error,omitempty"`
	SampleID       *string   `json:"sample_id,omitempty"`
	PatientID      *string   `json:"patient_id,omitempty"`
	PatientName    *string   `json:"patient_name,omitempty"`
	DiversityScore *float64  `json:"diversity_score,omitempty"`
	CollectedDate  *string   `json:"collected_date,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (h *handler) listSamples(w http.ResponseWriter, r *http.Request) {
	var rows []struct {
		JobID          uint
		SourceRef      string
		Status         string
		Error          string
		SampleExtID    *string
		CollectedDate  *time.Time
		DiversityScore *float64
		PatientExtID   *string
		PatientName    *string
		CreatedAt      time.Time
	}

	err := h.db.Table("sample_processing_jobs AS job").
		Select(`job.id AS job_id, job.source_ref, job.status, job.error, job.created_at,
			sample.sample_id AS sample_ext_id, sample.collected_date, sample.diversity_score,
			patient.patient_id AS patient_ext_id, patient.name AS patient_name`).
		Joins("LEFT JOIN samples sample ON sample.id = job.sample_id AND sample.deleted_at IS NULL").
		Joins("LEFT JOIN patients patient ON patient.id = sample.patient_id AND patient.deleted_at IS NULL").
		Where("job.deleted_at IS NULL").
		Order("job.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]SampleListItem, len(rows))
	for i, row := range rows {
		item := SampleListItem{
			JobID:          row.JobID,
			SourceRef:      row.SourceRef,
			Status:         row.Status,
			Error:          row.Error,
			SampleID:       row.SampleExtID,
			PatientID:      row.PatientExtID,
			PatientName:    row.PatientName,
			DiversityScore: row.DiversityScore,
			CreatedAt:      row.CreatedAt,
		}
		if row.CollectedDate != nil {
			s := row.CollectedDate.UTC().Format("2006-01-02")
			item.CollectedDate = &s
		}
		resp[i] = item
	}
	writeJSON(w, http.StatusOK, resp)
}

type TaxonResponse struct {
	Name      string  `json:"name"`
	Abundance float32 `json:"abundance"`
}

type SampleDetailResponse struct {
	JobID          uint            `json:"job_id"`
	SourceRef      string          `json:"source_ref"`
	Status         string          `json:"status"`
	Error          string          `json:"error,omitempty"`
	SampleID       string          `json:"sample_id,omitempty"`
	PatientID      string          `json:"patient_id,omitempty"`
	PatientName    string          `json:"patient_name,omitempty"`
	SequencingType string          `json:"sequencing_type,omitempty"`
	CollectedDate  string          `json:"collected_date,omitempty"`
	DiversityScore *float64        `json:"diversity_score,omitempty"`
	Taxa           []TaxonResponse `json:"taxa,omitempty"`
}

func (h *handler) getSample(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var job models.SampleProcessingJob
	if err := h.db.First(&job, uint(id)).Error; err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	resp := SampleDetailResponse{
		JobID:     job.ID,
		SourceRef: job.SourceRef,
		Status:    job.Status,
		Error:     job.Error,
	}

	if job.SampleID != nil {
		var sample models.Sample
		if err := h.db.Preload("Taxa").First(&sample, *job.SampleID).Error; err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var patient models.Patient
		if err := h.db.First(&patient, sample.PatientID).Error; err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp.SampleID = sample.SampleID
		resp.PatientID = patient.PatientID
		resp.PatientName = patient.Name
		resp.SequencingType = sample.SequencingType
		resp.CollectedDate = sample.CollectedDate.UTC().Format("2006-01-02")
		resp.DiversityScore = &sample.DiversityScore
		resp.Taxa = make([]TaxonResponse, len(sample.Taxa))
		for i, t := range sample.Taxa {
			resp.Taxa[i] = TaxonResponse{Name: t.Name, Abundance: t.Abundance}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
