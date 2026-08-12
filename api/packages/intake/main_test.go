package intake_test

import (
	"os"
	"path/filepath"
	"testing"

	"vitract-assessment/packages/dbtest"
	"vitract-assessment/packages/intake"
	"vitract-assessment/packages/models"
)

func TestEnqueueSamples(t *testing.T) {
	db := dbtest.New(t)
	dir := t.TempDir()

	for _, name := range []string{"a.json", "b.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	enqueued, skipped, err := intake.EnqueueSamples(db, "local_file", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueued != 2 || skipped != 0 {
		t.Fatalf("got enqueued=%d skipped=%d, want 2/0", enqueued, skipped)
	}

	var count int64
	db.Model(&models.SampleProcessingJob{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 jobs in db, got %d", count)
	}

	enqueued, skipped, err = intake.EnqueueSamples(db, "local_file", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueued != 0 || skipped != 2 {
		t.Fatalf("re-run: got enqueued=%d skipped=%d, want 0/2", enqueued, skipped)
	}
}

func TestEnqueueSamples_NewFileAfterRerun(t *testing.T) {
	db := dbtest.New(t)
	dir := t.TempDir()

	writeSample := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeSample("a.json")
	if _, _, err := intake.EnqueueSamples(db, "local_file", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeSample("b.json")
	enqueued, skipped, err := intake.EnqueueSamples(db, "local_file", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueued != 1 || skipped != 1 {
		t.Fatalf("got enqueued=%d skipped=%d, want 1/1", enqueued, skipped)
	}
}
