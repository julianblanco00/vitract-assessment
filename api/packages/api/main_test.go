package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"gorm.io/gorm"

	"vitract-assessment/packages/api"
	"vitract-assessment/packages/dbtest"
	"vitract-assessment/packages/models"
)

func seedData(t *testing.T, db *gorm.DB) (successJobID uint) {
	t.Helper()

	practitioner := models.Practitioner{Name: "Dr. Test", Email: "dr@test.local"}
	if err := db.Create(&practitioner).Error; err != nil {
		t.Fatal(err)
	}

	patient := models.Patient{
		PatientID:      "P001",
		Name:           "Jane Doe",
		Email:          "jane@test.local",
		Birthday:       time.Date(1980, 1, 15, 0, 0, 0, 0, time.UTC),
		PractitionerID: practitioner.ID,
	}
	if err := db.Create(&patient).Error; err != nil {
		t.Fatal(err)
	}

	sample := models.Sample{
		SampleID:       "S1001",
		PatientID:      patient.ID,
		SequencingType: "16S",
		CollectedDate:  time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		DiversityScore: 1.415,
	}
	if err := db.Create(&sample).Error; err != nil {
		t.Fatal(err)
	}

	taxa := []models.Taxon{{Name: "Bacteroides", Abundance: 0.5, SampleID: sample.ID}}
	if err := db.Create(&taxa).Error; err != nil {
		t.Fatal(err)
	}

	successJob := models.SampleProcessingJob{SampleID: &sample.ID, Status: string(models.StatusSuccess), Source: "local_file", SourceRef: "S1001.json"}
	if err := db.Create(&successJob).Error; err != nil {
		t.Fatal(err)
	}

	failedJob := models.SampleProcessingJob{Status: string(models.StatusFailed), Error: "patient_id not found in roster", Source: "local_file", SourceRef: "bad.json"}
	if err := db.Create(&failedJob).Error; err != nil {
		t.Fatal(err)
	}

	return successJob.ID
}

func getJSON(t *testing.T, url string, v any) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	return resp
}

func TestHealth(t *testing.T) {
	db := dbtest.New(t)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var got map[string]string
	resp := getJSON(t, srv.URL+"/health", &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got["status"] != "ok" {
		t.Errorf("status field = %q, want ok", got["status"])
	}
}

func TestListPatients(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var got []api.PatientResponse
	resp := getJSON(t, srv.URL+"/patients", &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(got) != 1 {
		t.Fatalf("got %d patients, want 1", len(got))
	}
	if got[0].PatientID != "P001" || got[0].Birthday != "1980-01-15" {
		t.Errorf("unexpected patient: %+v", got[0])
	}
	if got[0].PractitionerName != "Dr. Test" {
		t.Errorf("practitioner name = %q, want Dr. Test", got[0].PractitionerName)
	}
}

func TestListSamples(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var got []api.SampleListItem
	resp := getJSON(t, srv.URL+"/samples", &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}

	byRef := map[string]api.SampleListItem{}
	for _, item := range got {
		byRef[item.SourceRef] = item
	}

	success, ok := byRef["S1001.json"]
	if !ok {
		t.Fatal("missing S1001.json row")
	}
	if success.Status != "success" || success.SampleID == nil || *success.SampleID != "S1001" {
		t.Errorf("unexpected success row: %+v", success)
	}
	if success.CollectedDate == nil || *success.CollectedDate != "2026-05-02" {
		t.Errorf("collected_date = %v, want 2026-05-02", success.CollectedDate)
	}
	if success.DiversityScore == nil || *success.DiversityScore != 1.415 {
		t.Errorf("diversity_score = %v, want 1.415", success.DiversityScore)
	}

	failed, ok := byRef["bad.json"]
	if !ok {
		t.Fatal("missing bad.json row")
	}
	if failed.Status != "failed" || failed.Error == "" {
		t.Errorf("unexpected failed row: %+v", failed)
	}
	if failed.SampleID != nil {
		t.Errorf("expected no sample_id on a failed row, got %v", failed.SampleID)
	}
}

func TestGetSample_Success(t *testing.T) {
	db := dbtest.New(t)
	successJobID := seedData(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var detail api.SampleDetailResponse
	resp := getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(successJobID)), &detail)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if detail.SampleID != "S1001" || detail.PatientName != "Jane Doe" {
		t.Errorf("unexpected detail: %+v", detail)
	}
	if len(detail.Taxa) != 1 || detail.Taxa[0].Name != "Bacteroides" {
		t.Errorf("unexpected taxa: %+v", detail.Taxa)
	}
}

func TestGetSample_Failed(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db)

	var failedJob models.SampleProcessingJob
	if err := db.Where("source_ref = ?", "bad.json").First(&failedJob).Error; err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var detail api.SampleDetailResponse
	resp := getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(failedJob.ID)), &detail)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if detail.Status != "failed" || detail.SampleID != "" {
		t.Errorf("unexpected detail: %+v", detail)
	}
}

func TestGetSample_NotFound(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	resp := getJSON(t, srv.URL+"/samples/999999", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetSample_InvalidID(t *testing.T) {
	db := dbtest.New(t)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	resp := getJSON(t, srv.URL+"/samples/not-a-number", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
