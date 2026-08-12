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
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /patients", h.listPatients)
	mux.HandleFunc("GET /samples", h.listSamples)
	mux.HandleFunc("GET /samples/{id}", h.getSample)
	mux.HandleFunc("GET /samples/{id}/versions", h.getSampleVersions)
	return withCORS(mux)
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := h.db.DB()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}
	if err := sqlDB.PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		next.ServeHTTP(w, r)
	})
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
	ID                uint             `json:"id"`
	PatientID         string           `json:"patient_id"`
	Name              string           `json:"name"`
	Email             string           `json:"email"`
	Birthday          string           `json:"birthday"`
	PractitionerName  string           `json:"practitioner_name"`
	PractitionerEmail string           `json:"practitioner_email"`
	Samples           []SampleListItem `json:"samples"`
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type PaginatedResponse[T any] struct {
	Items      []T   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

func parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}

	pageSize = defaultPageSize
	if v, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && v > 0 && v <= maxPageSize {
		pageSize = v
	}

	return page, pageSize
}

func totalPages(totalItems int64, pageSize int) int {
	if totalItems == 0 {
		return 1
	}
	return int((totalItems + int64(pageSize) - 1) / int64(pageSize))
}

func (h *handler) listPatients(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)

	var total int64
	if err := h.db.Model(&models.Patient{}).Count(&total).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var patients []models.Patient
	err := h.db.Preload("Practitioner").
		Order("patients.id").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&patients).Error
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	patientDBIDs := make([]uint, len(patients))
	for i, p := range patients {
		patientDBIDs[i] = p.ID
	}
	samplesByPatient, err := h.samplesForPatients(patientDBIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]PatientResponse, len(patients))
	for i, p := range patients {
		items[i] = PatientResponse{
			ID:                p.ID,
			PatientID:         p.PatientID,
			Name:              p.Name,
			Email:             p.Email,
			Birthday:          p.Birthday.UTC().Format("2006-01-02"),
			PractitionerName:  p.Practitioner.Name,
			PractitionerEmail: p.Practitioner.Email,
			Samples:           samplesByPatient[p.PatientID],
		}
	}

	writeJSON(w, http.StatusOK, PaginatedResponse[PatientResponse]{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages(total, pageSize),
	})
}

func (h *handler) samplesForPatients(patientDBIDs []uint) (map[string][]SampleListItem, error) {
	byPatient := make(map[string][]SampleListItem)
	if len(patientDBIDs) == 0 {
		return byPatient, nil
	}

	var rows []sampleRow
	err := sampleRowsQuery(h.db).
		Where("sample.patient_id IN ?", patientDBIDs).
		Where(`job.id IN (
			SELECT DISTINCT ON (s2.sample_id) j2.id
			FROM sample_processing_jobs j2
			JOIN samples s2 ON s2.id = j2.sample_id AND s2.deleted_at IS NULL
			WHERE j2.deleted_at IS NULL
			ORDER BY s2.sample_id, j2.created_at DESC
		)`).
		Order("job.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, item := range toSampleListItems(rows) {
		if item.PatientID == nil {
			continue
		}
		byPatient[*item.PatientID] = append(byPatient[*item.PatientID], item)
	}
	return byPatient, nil
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

type sampleRow struct {
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

func sampleRowsQuery(db *gorm.DB) *gorm.DB {
	return db.Table("sample_processing_jobs AS job").
		Select(`job.id AS job_id, job.source_ref, job.status, job.error, job.created_at,
			sample.sample_id AS sample_ext_id, sample.collected_date, sample.diversity_score,
			patient.patient_id AS patient_ext_id, patient.name AS patient_name`).
		Joins("LEFT JOIN samples sample ON sample.id = job.sample_id AND sample.deleted_at IS NULL").
		Joins("LEFT JOIN patients patient ON patient.id = sample.patient_id AND patient.deleted_at IS NULL").
		Where("job.deleted_at IS NULL")
}

func toSampleListItems(rows []sampleRow) []SampleListItem {
	items := make([]SampleListItem, len(rows))
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
		items[i] = item
	}
	return items
}

func (h *handler) listSamples(w http.ResponseWriter, r *http.Request) {
	var rows []sampleRow

	err := sampleRowsQuery(h.db).
		Where(`sample.sample_id IS NULL OR job.id IN (
			SELECT DISTINCT ON (s2.sample_id) j2.id
			FROM sample_processing_jobs j2
			JOIN samples s2 ON s2.id = j2.sample_id AND s2.deleted_at IS NULL
			WHERE j2.deleted_at IS NULL
			ORDER BY s2.sample_id, j2.created_at DESC
		)`).
		Order("job.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toSampleListItems(rows))
}

func (h *handler) getSampleVersions(w http.ResponseWriter, r *http.Request) {
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

	if job.SampleID == nil {
		writeJSON(w, http.StatusOK, []SampleListItem{})
		return
	}

	var sample models.Sample
	if err := h.db.First(&sample, *job.SampleID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var rows []sampleRow
	err = sampleRowsQuery(h.db).
		Where("sample.sample_id = ? AND job.created_at < ?", sample.SampleID, job.CreatedAt).
		Order("job.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toSampleListItems(rows))
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
	LatestJobID    *uint           `json:"latest_job_id,omitempty"`
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

		var latest struct{ ID uint }
		err := h.db.Table("sample_processing_jobs AS j").
			Select("j.id").
			Joins("JOIN samples s ON s.id = j.sample_id AND s.deleted_at IS NULL").
			Where("s.sample_id = ? AND j.deleted_at IS NULL", sample.SampleID).
			Order("j.created_at DESC").
			Limit(1).
			Scan(&latest).Error
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if latest.ID != job.ID {
			resp.LatestJobID = &latest.ID
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
