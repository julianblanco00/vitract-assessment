package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type SourceType string

const (
	SourceLocalFile SourceType = "local_file"
)

type SampleData struct {
	SampleID       string
	PatientID      string
	SequencingType string
	CollectedDate  string
	Taxa           []TaxonData
}

type TaxonData struct {
	Name      string
	Abundance float32
}

type Source interface {
	Fetch(ctx context.Context, ref string) (*SampleData, error)
}

type Registry struct {
	sources map[SourceType]Source
}

func NewRegistry() *Registry {
	return &Registry{
		sources: make(map[SourceType]Source),
	}
}

func (r *Registry) Register(t SourceType, s Source) {
	r.sources[t] = s
}

func (r *Registry) Get(t SourceType) (Source, error) {
	s, ok := r.sources[t]

	if !ok {
		return nil, fmt.Errorf("unknown source type: %s", t)
	}

	return s, nil
}

type LocalFileSource struct {
	BaseDir string
}

type localFileFormat struct {
	SampleID       string  `json:"sample_id"`
	PatientID      string  `json:"patient_id"`
	SequencingType string  `json:"sequencing_type"`
	CollectedDate  string  `json:"collected_date"`
	Taxa           []taxon `json:"taxa"`
}

type taxon struct {
	Name      string  `json:"name"`
	Abundance float32 `json:"abundance"`
}

func (s *LocalFileSource) Fetch(ctx context.Context, ref string) (*SampleData, error) {
	f, err := os.Open(filepath.Join(s.BaseDir, ref))
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var raw localFileFormat
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	if raw.SampleID == "" {
		return nil, fmt.Errorf("missing sample_id")
	}
	if len(raw.Taxa) == 0 {
		return nil, fmt.Errorf("no taxa found")
	}

	taxa := make([]TaxonData, 0, len(raw.Taxa))
	for i, t := range raw.Taxa {
		if t.Name == "" {
			return nil, fmt.Errorf("taxon %d: empty name", i)
		}
		if t.Abundance < 0 {
			return nil, fmt.Errorf("taxon %d (%s): negative abundance", i, t.Name)
		}
		taxa = append(taxa, TaxonData{Name: t.Name, Abundance: t.Abundance})
	}

	return &SampleData{
		SampleID:       raw.SampleID,
		PatientID:      raw.PatientID,
		SequencingType: raw.SequencingType,
		CollectedDate:  raw.CollectedDate,
		Taxa:           taxa,
	}, nil
}
