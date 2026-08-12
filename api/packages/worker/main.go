package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"vitract-assessment/packages/ingestion"
	"vitract-assessment/packages/models"
	"vitract-assessment/packages/notification"
	"vitract-assessment/packages/scoring"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Worker struct {
	db       *gorm.DB
	registry *ingestion.Registry
	notifier notification.Notifier
}

func New(db *gorm.DB, registry *ingestion.Registry, notifier notification.Notifier) *Worker {
	return &Worker{
		db:       db,
		registry: registry,
		notifier: notifier,
	}
}

func (w *Worker) Run(interval time.Duration) {
	slog.Info("worker started", "poll_interval", interval.String())

	for {
		job, err := w.claimNextJob()
		if err != nil {
			slog.Error("failed to claim job", "error", err)
			time.Sleep(interval)
			continue
		}

		if job == nil {
			time.Sleep(interval)
			continue
		}

		w.processJobSafely(job)
	}
}

func (w *Worker) claimNextJob() (*models.SampleProcessingJob, error) {
	var job models.SampleProcessingJob

	err := w.db.Transaction(func(tx *gorm.DB) error {
		result := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", string(models.StatusPending)).
			Order("created_at ASC").
			First(&job)

		if result.Error != nil {
			return result.Error
		}

		job.Status = string(models.StatusProcessing)
		return tx.Save(&job).Error
	})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &job, nil
}

func (w *Worker) processJobSafely(job *models.SampleProcessingJob) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("job panicked", "job_id", job.ID, "source_ref", job.SourceRef, "panic", r)
			w.markFailed(job, fmt.Sprintf("internal error: %v", r))
		}
	}()
	w.processJob(job)
}

func (w *Worker) processJob(job *models.SampleProcessingJob) {
	source, err := w.registry.Get(ingestion.SourceType(job.Source))
	if err != nil {
		w.markFailed(job, err.Error())
		return
	}

	data, err := source.Fetch(context.Background(), job.SourceRef)
	if err != nil {
		w.markFailed(job, fmt.Sprintf("invalid file: %v", err))
		return
	}

	var patient models.Patient
	err = w.db.Where("patient_id = ?", data.PatientID).First(&patient).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		w.markIngestionFailure(job, data, "patient_id not found in roster")
		return
	}
	if err != nil {
		w.markFailed(job, fmt.Sprintf("failed to look up patient: %v", err))
		return
	}

	collectedDate, err := time.Parse("2006-01-02", data.CollectedDate)
	if err != nil {
		w.markIngestionFailure(job, data, fmt.Sprintf("invalid collected_date: %v", err))
		return
	}

	abundances := make([]float32, len(data.Taxa))
	for i, t := range data.Taxa {
		abundances[i] = t.Abundance
	}
	score := scoring.Shannon(abundances)

	sample := models.Sample{
		SampleID:       data.SampleID,
		PatientID:      patient.ID,
		SequencingType: data.SequencingType,
		CollectedDate:  collectedDate,
		DiversityScore: score,
	}

	err = w.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sample).Error; err != nil {
			return err
		}

		taxa := make([]models.Taxon, len(data.Taxa))
		for i, t := range data.Taxa {
			taxa[i] = models.Taxon{Name: t.Name, Abundance: t.Abundance, SampleID: sample.ID}
		}
		if err := tx.Create(&taxa).Error; err != nil {
			return err
		}

		job.SampleID = &sample.ID
		job.Status = string(models.StatusSuccess)
		job.FinishedAt = time.Now()
		return tx.Save(job).Error
	})
	if err != nil {
		w.markFailed(job, err.Error())
		return
	}

	slog.Info("job succeeded", "job_id", job.ID, "sample_id", sample.SampleID, "score", sample.DiversityScore)

	w.notifyPractitioner(&sample)
}

func (w *Worker) markFailed(job *models.SampleProcessingJob, reason string) {
	job.Status = string(models.StatusFailed)
	job.Error = reason
	job.FinishedAt = time.Now()
	w.db.Save(job)
	slog.Warn("job failed", "job_id", job.ID, "source_ref", job.SourceRef, "reason", reason)
}

func (w *Worker) markIngestionFailure(job *models.SampleProcessingJob, data *ingestion.SampleData, reason string) {
	failure := models.IngestionFailure{
		Source:       job.Source,
		SourceRef:    job.SourceRef,
		RawSampleID:  data.SampleID,
		RawPatientID: data.PatientID,
		Reason:       reason,
	}
	if err := w.db.Create(&failure).Error; err != nil {
		slog.Error("failed to record ingestion failure", "job_id", job.ID, "error", err)
	}
	w.markFailed(job, reason)
}

func (w *Worker) notifyPractitioner(sample *models.Sample) {
	var patient models.Patient
	if err := w.db.Preload("Practitioner").First(&patient, "id = ?", sample.PatientID).Error; err != nil {
		slog.Error("failed to load patient for notification", "sample_id", sample.SampleID, "error", err)
		return
	}

	err := w.notifier.NotifyResultReady(context.Background(), patient.Practitioner.Email, notification.SampleSummary{
		SampleID:       sample.SampleID,
		PatientName:    patient.Name,
		DiversityScore: sample.DiversityScore,
	})
	if err != nil {
		slog.Error("failed to notify practitioner", "sample_id", sample.SampleID, "error", err)
	}
}
