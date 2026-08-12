package notification_test

import (
	"context"
	"testing"

	"vitract-assessment/packages/notification"
)

func TestLogNotifier_NotifyResultReady(t *testing.T) {
	n := &notification.LogNotifier{}
	err := n.NotifyResultReady(context.Background(), "practitioner@example.com", notification.SampleSummary{
		SampleID:       "S1001",
		PatientName:    "Jane Doe",
		DiversityScore: 1.415,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
