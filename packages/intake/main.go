package intake

import (
	"errors"
	"fmt"
	"os"

	"gorm.io/gorm"

	"vitract-assessment/packages/models"
)

func EnqueueSamples(db *gorm.DB, sourceType string, dir string) (enqueued, skipped int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read samples dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		sourceRef := entry.Name()

		var existing models.SampleProcessingJob
		err := db.Where("source = ? AND source_ref = ?", sourceType, sourceRef).First(&existing).Error
		if err == nil {
			skipped++
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return enqueued, skipped, fmt.Errorf("%s: failed to check existing job: %w", sourceRef, err)
		}

		job := models.SampleProcessingJob{
			Status:    string(models.StatusPending),
			Source:    sourceType,
			SourceRef: sourceRef,
		}
		if err := db.Create(&job).Error; err != nil {
			return enqueued, skipped, fmt.Errorf("%s: failed to create job: %w", sourceRef, err)
		}
		enqueued++
	}

	return enqueued, skipped, nil
}
