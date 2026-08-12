package api_test

import (
	"encoding/json"
	"fmt"
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

func seedRedelivery(t *testing.T, db *gorm.DB) (jobID uint) {
	t.Helper()

	var patient models.Patient
	if err := db.Where("patient_id = ?", "P001").First(&patient).Error; err != nil {
		t.Fatal(err)
	}

	sample := models.Sample{
		SampleID:       "S1001",
		PatientID:      patient.ID,
		SequencingType: "16S",
		CollectedDate:  time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC),
		DiversityScore: 1.430,
	}
	if err := db.Create(&sample).Error; err != nil {
		t.Fatal(err)
	}

	job := models.SampleProcessingJob{SampleID: &sample.ID, Status: string(models.StatusSuccess), Source: "local_file", SourceRef: "S1001_redelivery.json"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	return job.ID
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

	var got api.PaginatedResponse[api.PatientResponse]
	resp := getJSON(t, srv.URL+"/patients", &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(got.Items) != 1 {
		t.Fatalf("got %d patients, want 1", len(got.Items))
	}
	if got.Items[0].PatientID != "P001" || got.Items[0].Birthday != "1980-01-15" {
		t.Errorf("unexpected patient: %+v", got.Items[0])
	}
	if got.Items[0].PractitionerName != "Dr. Test" {
		t.Errorf("practitioner name = %q, want Dr. Test", got.Items[0].PractitionerName)
	}
	if got.Page != 1 || got.PageSize != 20 || got.TotalItems != 1 || got.TotalPages != 1 {
		t.Errorf("unexpected pagination metadata: %+v", got)
	}
	if len(got.Items[0].Samples) != 1 || got.Items[0].Samples[0].SourceRef != "S1001.json" {
		t.Errorf("unexpected embedded samples: %+v", got.Items[0].Samples)
	}
}

func TestListPatients_SamplesDontLeakAcrossPatients(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db)       // P001, with sample S1001.json
	seedRedelivery(t, db) // another delivery of S1001 for P001

	practitioner := models.Practitioner{Name: "Dr. Test 2", Email: "dr2@test.local"}
	if err := db.Create(&practitioner).Error; err != nil {
		t.Fatal(err)
	}
	otherPatient := models.Patient{
		PatientID:      "P002",
		Name:           "John Roe",
		Email:          "john@test.local",
		Birthday:       time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		PractitionerID: practitioner.ID,
	}
	if err := db.Create(&otherPatient).Error; err != nil {
		t.Fatal(err)
	}
	otherSample := models.Sample{
		SampleID:       "S2001",
		PatientID:      otherPatient.ID,
		SequencingType: "16S",
		CollectedDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DiversityScore: 2.0,
	}
	if err := db.Create(&otherSample).Error; err != nil {
		t.Fatal(err)
	}
	otherJob := models.SampleProcessingJob{SampleID: &otherSample.ID, Status: string(models.StatusSuccess), Source: "local_file", SourceRef: "S2001.json"}
	if err := db.Create(&otherJob).Error; err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var got api.PaginatedResponse[api.PatientResponse]
	resp := getJSON(t, srv.URL+"/patients?page_size=10", &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(got.Items) != 2 {
		t.Fatalf("got %d patients, want 2", len(got.Items))
	}

	byPatientID := map[string]api.PatientResponse{}
	for _, p := range got.Items {
		byPatientID[p.PatientID] = p
	}

	p001 := byPatientID["P001"]
	if len(p001.Samples) != 1 || p001.Samples[0].SourceRef != "S1001_redelivery.json" {
		t.Errorf("P001 samples = %+v, want only the latest S1001 delivery", p001.Samples)
	}

	p002 := byPatientID["P002"]
	if len(p002.Samples) != 1 || p002.Samples[0].SourceRef != "S2001.json" {
		t.Errorf("P002 samples = %+v, want only S2001.json", p002.Samples)
	}
}

func TestListPatients_Pagination(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db) // seeds P001

	practitioner := models.Practitioner{Name: "Dr. Test 2", Email: "dr2@test.local"}
	if err := db.Create(&practitioner).Error; err != nil {
		t.Fatal(err)
	}
	for i, patientID := range []string{"P002", "P003", "P004"} {
		patient := models.Patient{
			PatientID:      patientID,
			Name:           fmt.Sprintf("Patient %d", i+2),
			Email:          fmt.Sprintf("patient%d@test.local", i+2),
			Birthday:       time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			PractitionerID: practitioner.ID,
		}
		if err := db.Create(&patient).Error; err != nil {
			t.Fatal(err)
		}
	}

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var page1 api.PaginatedResponse[api.PatientResponse]
	resp := getJSON(t, srv.URL+"/patients?page=1&page_size=2", &page1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("got %d items on page 1, want 2", len(page1.Items))
	}
	if page1.TotalItems != 4 || page1.TotalPages != 2 || page1.Page != 1 || page1.PageSize != 2 {
		t.Errorf("unexpected pagination metadata on page 1: %+v", page1)
	}

	var page2 api.PaginatedResponse[api.PatientResponse]
	resp = getJSON(t, srv.URL+"/patients?page=2&page_size=2", &page2)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(page2.Items) != 2 {
		t.Fatalf("got %d items on page 2, want 2", len(page2.Items))
	}

	seen := map[string]bool{}
	for _, p := range append(page1.Items, page2.Items...) {
		if seen[p.PatientID] {
			t.Errorf("patient %s appeared on more than one page", p.PatientID)
		}
		seen[p.PatientID] = true
	}
	if len(seen) != 4 {
		t.Errorf("got %d distinct patients across both pages, want 4", len(seen))
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

func TestListSamples_ExcludesSupersededDeliveries(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db)
	redeliveryJobID := seedRedelivery(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var got []api.SampleListItem
	resp := getJSON(t, srv.URL+"/samples", &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (latest S1001 delivery + the unmatched failure)", len(got))
	}

	byRef := map[string]api.SampleListItem{}
	for _, item := range got {
		byRef[item.SourceRef] = item
	}

	if _, ok := byRef["S1001.json"]; ok {
		t.Error("expected the superseded S1001.json delivery to be excluded from the list")
	}

	redelivery, ok := byRef["S1001_redelivery.json"]
	if !ok {
		t.Fatal("missing S1001_redelivery.json row")
	}
	if redelivery.JobID != redeliveryJobID {
		t.Errorf("job_id = %d, want %d", redelivery.JobID, redeliveryJobID)
	}
	if redelivery.CollectedDate == nil || *redelivery.CollectedDate != "2026-05-09" {
		t.Errorf("collected_date = %v, want 2026-05-09", redelivery.CollectedDate)
	}

	if _, ok := byRef["bad.json"]; !ok {
		t.Error("missing bad.json row")
	}
}

func TestGetSampleVersions_ReturnsOlderDeliveries(t *testing.T) {
	db := dbtest.New(t)
	originalJobID := seedData(t, db)
	redeliveryJobID := seedRedelivery(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var versions []api.SampleListItem
	resp := getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(redeliveryJobID))+"/versions", &versions)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(versions) != 1 {
		t.Fatalf("got %d versions, want 1", len(versions))
	}
	if versions[0].JobID != originalJobID {
		t.Errorf("job_id = %d, want %d", versions[0].JobID, originalJobID)
	}
	if versions[0].SourceRef != "S1001.json" {
		t.Errorf("source_ref = %q, want S1001.json", versions[0].SourceRef)
	}
}

func TestGetSampleVersions_ExcludesNewerDeliveries(t *testing.T) {
	db := dbtest.New(t)
	originalJobID := seedData(t, db)
	seedRedelivery(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var versions []api.SampleListItem
	resp := getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(originalJobID))+"/versions", &versions)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(versions) != 0 {
		t.Errorf("got %d versions, want 0 (the original delivery has no earlier version, only a newer one)", len(versions))
	}
}

func TestGetSampleVersions_NoSample(t *testing.T) {
	db := dbtest.New(t)
	seedData(t, db)

	var failedJob models.SampleProcessingJob
	if err := db.Where("source_ref = ?", "bad.json").First(&failedJob).Error; err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var versions []api.SampleListItem
	resp := getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(failedJob.ID))+"/versions", &versions)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(versions) != 0 {
		t.Errorf("got %d versions, want 0", len(versions))
	}
}

func TestGetSampleVersions_NoOtherDeliveries(t *testing.T) {
	db := dbtest.New(t)
	successJobID := seedData(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var versions []api.SampleListItem
	resp := getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(successJobID))+"/versions", &versions)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(versions) != 0 {
		t.Errorf("got %d versions, want 0", len(versions))
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
	if detail.LatestJobID != nil {
		t.Errorf("latest_job_id = %v, want nil (this is the only, and therefore latest, delivery)", detail.LatestJobID)
	}
}

func TestGetSample_LatestJobID(t *testing.T) {
	db := dbtest.New(t)
	originalJobID := seedData(t, db)
	redeliveryJobID := seedRedelivery(t, db)

	srv := httptest.NewServer(api.NewRouter(db))
	defer srv.Close()

	var original api.SampleDetailResponse
	resp := getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(originalJobID)), &original)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if original.LatestJobID == nil || *original.LatestJobID != redeliveryJobID {
		t.Errorf("original.latest_job_id = %v, want %d", original.LatestJobID, redeliveryJobID)
	}

	var latest api.SampleDetailResponse
	resp = getJSON(t, srv.URL+"/samples/"+strconv.Itoa(int(redeliveryJobID)), &latest)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if latest.LatestJobID != nil {
		t.Errorf("latest.latest_job_id = %v, want nil (this already is the latest delivery)", latest.LatestJobID)
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
