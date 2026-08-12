package notification

import (
	"context"
	"log"
)

type Notifier interface {
	NotifyResultReady(ctx context.Context, to string, sample SampleSummary) error
}

type SampleSummary struct {
	SampleID       string
	PatientName    string
	DiversityScore float64
}

type LogNotifier struct{}

func (n *LogNotifier) NotifyResultReady(ctx context.Context, to string, sample SampleSummary) error {
	log.Printf(
		"[EMAIL STUB] To: %s | Subject: Result ready for sample %s | Patient: %s | Diversity Score: %.4f",
		to, sample.SampleID, sample.PatientName, sample.DiversityScore,
	)
	return nil
}
