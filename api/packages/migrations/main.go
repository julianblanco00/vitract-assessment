package migrations

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"vitract-assessment/packages/models"
)

const housePractitionerEmail = "house-practitioner@vitract.local"

func Run(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, migrations())
	return m.Migrate()
}

func migrations() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			ID: "202608130001_create_schema",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&models.Practitioner{},
					&models.Patient{},
					&models.Sample{},
					&models.Taxon{},
					&models.SampleProcessingJob{},
					&models.IngestionFailure{},
				)
			},
		},
		{
			ID: "202608130002_seed_roster",
			Migrate: func(tx *gorm.DB) error {
				practitioner := models.Practitioner{Email: housePractitionerEmail, Name: "House Practitioner"}
				if err := tx.Where(models.Practitioner{Email: housePractitionerEmail}).FirstOrCreate(&practitioner).Error; err != nil {
					return fmt.Errorf("failed to seed house practitioner: %w", err)
				}
				return seedPatients(tx, practitioner.ID)
			},
		},
	}
}

func seedPatients(tx *gorm.DB, practitionerID uint) error {
	f, err := os.Open("./data/patients.csv")
	if err != nil {
		return fmt.Errorf("failed to open roster: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		return fmt.Errorf("failed to read roster header: %w", err)
	}

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read roster row: %w", err)
		}

		patientID, name, dob, email := row[0], row[1], row[2], row[3]

		birthday, err := time.Parse("2006-01-02", dob)
		if err != nil {
			return fmt.Errorf("patient %s: invalid date_of_birth: %w", patientID, err)
		}

		patient := models.Patient{
			PatientID:      patientID,
			Name:           name,
			Email:          email,
			Birthday:       birthday,
			PractitionerID: practitionerID,
		}
		if err := tx.Create(&patient).Error; err != nil {
			return fmt.Errorf("patient %s: failed to create: %w", patientID, err)
		}
	}

	return nil
}
