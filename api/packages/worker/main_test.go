package worker

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"vitract-assessment/packages/dbtest"
	"vitract-assessment/packages/ingestion"
	"vitract-assessment/packages/models"
	"vitract-assessment/packages/notification"
)

type fakeSource struct {
	data   map[string]*ingestion.SampleData
	err    map[string]error
	panics map[string]any
}

func (f *fakeSource) Fetch(ctx context.Context, ref string) (*ingestion.SampleData, error) {
	if p, ok := f.panics[ref]; ok {
		panic(p)
	}
	if err, ok := f.err[ref]; ok {
		return nil, err
	}
	if d, ok := f.data[ref]; ok {
		return d, nil
	}
	return nil, errors.New("unexpected ref: " + ref)
}

type fakeNotifier struct {
	calls []notification.SampleSummary
}

func (f *fakeNotifier) NotifyResultReady(ctx context.Context, to string, s notification.SampleSummary) error {
	f.calls = append(f.calls, s)
	return nil
}

func newRegistry(source ingestion.Source) *ingestion.Registry {
	r := ingestion.NewRegistry()
	r.Register(ingestion.SourceLocalFile, source)
	return r
}

func TestProcessJob_Success(t *testing.T) {
	db := dbtest.New(t)

	practitioner := models.Practitioner{Name: "Dr. Test", Email: "dr@test.local"}
	if err := db.Create(&practitioner).Error; err != nil {
		t.Fatal(err)
	}
	patient := models.Patient{PatientID: "P001", Name: "Jane Doe", Email: "jane@test.local", PractitionerID: practitioner.ID}
	if err := db.Create(&patient).Error; err != nil {
		t.Fatal(err)
	}

	job := models.SampleProcessingJob{Status: string(models.StatusPending), Source: "local_file", SourceRef: "s1.json"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	source := &fakeSource{data: map[string]*ingestion.SampleData{
		"s1.json": {
			SampleID:       "S1001",
			PatientID:      "P001",
			SequencingType: "16S",
			CollectedDate:  "2026-05-02",
			Taxa: []ingestion.TaxonData{
				{Name: "Bacteroides", Abundance: 0.5},
				{Name: "Prevotella", Abundance: 0.5},
			},
		},
	}}
	notifier := &fakeNotifier{}
	w := New(db, newRegistry(source), notifier)

	w.processJob(&job)

	var updated models.SampleProcessingJob
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != string(models.StatusSuccess) {
		t.Fatalf("status = %q, want success (error: %s)", updated.Status, updated.Error)
	}
	if updated.SampleID == nil {
		t.Fatal("expected SampleID to be set")
	}

	var sample models.Sample
	if err := db.Preload("Taxa").First(&sample, *updated.SampleID).Error; err != nil {
		t.Fatal(err)
	}
	if sample.SampleID != "S1001" {
		t.Errorf("sample.SampleID = %q, want S1001", sample.SampleID)
	}
	if len(sample.Taxa) != 2 {
		t.Errorf("got %d taxa, want 2", len(sample.Taxa))
	}
	wantScore := math.Log(2)
	if math.Abs(sample.DiversityScore-wantScore) > 1e-9 {
		t.Errorf("score = %v, want %v", sample.DiversityScore, wantScore)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.calls))
	}
	if notifier.calls[0].SampleID != "S1001" {
		t.Errorf("notified sample id = %q, want S1001", notifier.calls[0].SampleID)
	}
}

func TestProcessJob_UnmatchedPatient(t *testing.T) {
	db := dbtest.New(t)

	job := models.SampleProcessingJob{Status: string(models.StatusPending), Source: "local_file", SourceRef: "bad.json"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	source := &fakeSource{data: map[string]*ingestion.SampleData{
		"bad.json": {
			SampleID:      "S999",
			PatientID:     "P999",
			CollectedDate: "2026-01-01",
			Taxa:          []ingestion.TaxonData{{Name: "X", Abundance: 1}},
		},
	}}
	w := New(db, newRegistry(source), &fakeNotifier{})

	w.processJob(&job)

	var updated models.SampleProcessingJob
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != string(models.StatusFailed) {
		t.Fatalf("status = %q, want failed", updated.Status)
	}
	if updated.SampleID != nil {
		t.Error("expected SampleID to remain nil")
	}

	var count int64
	db.Model(&models.IngestionFailure{}).Where("raw_sample_id = ?", "S999").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 ingestion_failure row, got %d", count)
	}
}

func TestProcessJob_FetchError(t *testing.T) {
	db := dbtest.New(t)

	job := models.SampleProcessingJob{Status: string(models.StatusPending), Source: "local_file", SourceRef: "broken.json"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	source := &fakeSource{err: map[string]error{"broken.json": errors.New("invalid json: unexpected EOF")}}
	w := New(db, newRegistry(source), &fakeNotifier{})

	w.processJob(&job)

	var updated models.SampleProcessingJob
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != string(models.StatusFailed) {
		t.Fatalf("status = %q, want failed", updated.Status)
	}
	if updated.SampleID != nil {
		t.Error("expected SampleID to remain nil")
	}

	var count int64
	db.Model(&models.IngestionFailure{}).Count(&count)
	if count != 0 {
		t.Errorf("expected no ingestion_failure rows for a technical failure, got %d", count)
	}
}

func TestProcessJobSafely_RecoversFromPanic(t *testing.T) {
	db := dbtest.New(t)

	job := models.SampleProcessingJob{Status: string(models.StatusPending), Source: "local_file", SourceRef: "panics.json"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	source := &fakeSource{panics: map[string]any{"panics.json": "boom"}}
	w := New(db, newRegistry(source), &fakeNotifier{})

	w.processJobSafely(&job)

	var updated models.SampleProcessingJob
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != string(models.StatusFailed) {
		t.Fatalf("status = %q, want failed", updated.Status)
	}
	if updated.SampleID != nil {
		t.Error("expected SampleID to remain nil")
	}
	if !strings.Contains(updated.Error, "boom") {
		t.Errorf("error = %q, want it to mention the panic value", updated.Error)
	}
}

func TestProcessJobSafely_ContinuesAfterPanic(t *testing.T) {
	db := dbtest.New(t)

	practitioner := models.Practitioner{Name: "Dr. Test", Email: "dr2@test.local"}
	if err := db.Create(&practitioner).Error; err != nil {
		t.Fatal(err)
	}
	patient := models.Patient{PatientID: "P002", Name: "John Roe", Email: "john@test.local", PractitionerID: practitioner.ID}
	if err := db.Create(&patient).Error; err != nil {
		t.Fatal(err)
	}

	panicking := models.SampleProcessingJob{Status: string(models.StatusPending), Source: "local_file", SourceRef: "panics.json"}
	if err := db.Create(&panicking).Error; err != nil {
		t.Fatal(err)
	}
	ok := models.SampleProcessingJob{Status: string(models.StatusPending), Source: "local_file", SourceRef: "ok.json"}
	if err := db.Create(&ok).Error; err != nil {
		t.Fatal(err)
	}

	source := &fakeSource{
		panics: map[string]any{"panics.json": "boom"},
		data: map[string]*ingestion.SampleData{
			"ok.json": {
				SampleID:      "S2001",
				PatientID:     "P002",
				CollectedDate: "2026-01-02",
				Taxa:          []ingestion.TaxonData{{Name: "X", Abundance: 1}},
			},
		},
	}
	w := New(db, newRegistry(source), &fakeNotifier{})

	w.processJobSafely(&panicking)
	w.processJobSafely(&ok)

	var updatedPanicking, updatedOK models.SampleProcessingJob
	if err := db.First(&updatedPanicking, panicking.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&updatedOK, ok.ID).Error; err != nil {
		t.Fatal(err)
	}

	if updatedPanicking.Status != string(models.StatusFailed) {
		t.Errorf("panicking job status = %q, want failed", updatedPanicking.Status)
	}
	if updatedOK.Status != string(models.StatusSuccess) {
		t.Errorf("second job status = %q, want success (error: %s)", updatedOK.Status, updatedOK.Error)
	}
}

func TestHeartbeat_Throttled(t *testing.T) {
	w := New(nil, nil, nil)

	w.heartbeat()
	first := w.lastHeartbeat
	if first.IsZero() {
		t.Fatal("expected lastHeartbeat to be set after the first call")
	}

	w.heartbeat()
	if w.lastHeartbeat != first {
		t.Error("expected a second call within heartbeatInterval to be throttled")
	}
}

func TestClaimNextJob(t *testing.T) {
	db := dbtest.New(t)
	job := models.SampleProcessingJob{Status: string(models.StatusPending), Source: "local_file", SourceRef: "x.json"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	w := New(db, ingestion.NewRegistry(), &fakeNotifier{})

	claimed, err := w.claimNextJob()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected a job to be claimed")
	}
	if claimed.Status != string(models.StatusProcessing) {
		t.Errorf("status = %q, want processing", claimed.Status)
	}

	none, err := w.claimNextJob()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if none != nil {
		t.Error("expected no job to be claimed (none left pending)")
	}
}
