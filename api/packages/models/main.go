package models

import (
	"time"

	"gorm.io/gorm"
)

type SampleProcessingStatus string

const (
	StatusPending    SampleProcessingStatus = "pending"
	StatusProcessing SampleProcessingStatus = "processing"
	StatusSuccess    SampleProcessingStatus = "success"
	StatusFailed     SampleProcessingStatus = "failed"
)

type Patient struct {
	gorm.Model
	PatientID      string `gorm:"uniqueIndex;not null"`
	Name           string `gorm:"not null"`
	Email          string `gorm:"uniqueIndex;not null"`
	Birthday       time.Time
	PractitionerID uint         `gorm:"not null;index"`
	Practitioner   Practitioner `gorm:"foreignKey:PractitionerID"`

	Samples []Sample `gorm:"foreignKey:PatientID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

type Practitioner struct {
	gorm.Model
	Name  string `gorm:"not null"`
	Email string `gorm:"uniqueIndex;not null"`

	Patients []Patient `gorm:"foreignKey:PractitionerID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

type Sample struct {
	gorm.Model
	SampleID       string    `gorm:"index;not null"`
	PatientID      uint      `gorm:"not null;index"`
	SequencingType string    `gorm:"not null"`
	CollectedDate  time.Time `gorm:"not null"`
	DiversityScore float64

	Taxa          []Taxon             `gorm:"foreignKey:SampleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ProcessingJob SampleProcessingJob `gorm:"foreignKey:SampleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type Taxon struct {
	gorm.Model
	Name      string  `gorm:"not null"`
	Abundance float32 `gorm:"not null;check:abundance >= 0"`
	SampleID  uint    `gorm:"not null;index"`
}

type SampleProcessingJob struct {
	gorm.Model
	SampleID   *uint  `gorm:"uniqueIndex"`
	Status     string `gorm:"type:varchar(20);not null;index;default:'pending';check:status IN ('pending','processing','success','failed')"`
	Error      string
	FinishedAt time.Time
	SourceRef  string `gorm:"not null"`
	Source     string `gorm:"type:varchar(20);not null;check:source IN ('local_file')"`
}

type IngestionFailure struct {
	gorm.Model
	Source       string `gorm:"type:varchar(20);not null;check:source IN ('local_file')"`
	SourceRef    string `gorm:"not null"`
	RawSampleID  string `gorm:"index"`
	RawPatientID string `gorm:"index"`
	Reason       string `gorm:"not null"`
}
