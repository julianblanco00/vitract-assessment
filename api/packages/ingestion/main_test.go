package ingestion_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vitract-assessment/packages/ingestion"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalFileSource_Fetch_Valid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1.json", `{
		"sample_id": "S1001",
		"patient_id": "P001",
		"sequencing_type": "16S",
		"collected_date": "2026-05-02",
		"taxa": [
			{"name": "Bacteroides", "abundance": 0.4},
			{"name": "Prevotella", "abundance": 0.6}
		]
	}`)

	source := &ingestion.LocalFileSource{BaseDir: dir}
	data, err := source.Fetch(context.Background(), "s1.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.SampleID != "S1001" || data.PatientID != "P001" || data.SequencingType != "16S" || data.CollectedDate != "2026-05-02" {
		t.Errorf("unexpected data: %+v", data)
	}
	if len(data.Taxa) != 2 {
		t.Fatalf("got %d taxa, want 2", len(data.Taxa))
	}
	if data.Taxa[0].Name != "Bacteroides" || data.Taxa[0].Abundance != 0.4 {
		t.Errorf("unexpected taxon: %+v", data.Taxa[0])
	}
}

func TestLocalFileSource_Fetch_Errors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"missing sample_id", `{"patient_id": "P001", "taxa": [{"name": "X", "abundance": 1}]}`},
		{"no taxa", `{"sample_id": "S1001", "patient_id": "P001", "taxa": []}`},
		{"empty taxon name", `{"sample_id": "S1001", "patient_id": "P001", "taxa": [{"name": "", "abundance": 1}]}`},
		{"negative abundance", `{"sample_id": "S1001", "patient_id": "P001", "taxa": [{"name": "X", "abundance": -1}]}`},
		{"invalid json", `{not valid json`},
	}

	dir := t.TempDir()
	source := &ingestion.LocalFileSource{BaseDir: dir}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeFile(t, dir, "s.json", tt.content)
			if _, err := source.Fetch(context.Background(), "s.json"); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestLocalFileSource_Fetch_FileNotFound(t *testing.T) {
	source := &ingestion.LocalFileSource{BaseDir: t.TempDir()}
	if _, err := source.Fetch(context.Background(), "missing.json"); err == nil {
		t.Error("expected an error, got nil")
	}
}
