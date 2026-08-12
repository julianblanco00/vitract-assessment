package dbtest

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"vitract-assessment/packages/models"
)

const testDBName = "vitract_test"

var (
	once     sync.Once
	baseDB   *gorm.DB
	setupErr error
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func dsn(dbname string) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		getenv("DB_HOST", "localhost"),
		getenv("DB_USER", "vitract"),
		getenv("DB_PASSWORD", "vitract"),
		dbname,
		getenv("DB_PORT", "5433"),
	)
}

func setup() (*gorm.DB, error) {
	admin, err := gorm.Open(postgres.Open(dsn("postgres")), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	if err := admin.Exec("CREATE DATABASE " + testDBName).Error; err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}
	if sqlDB, err := admin.DB(); err == nil {
		sqlDB.Close()
	}

	db, err := gorm.Open(postgres.Open(dsn(testDBName)), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	if err := db.AutoMigrate(
		&models.Practitioner{},
		&models.Patient{},
		&models.Sample{},
		&models.Taxon{},
		&models.SampleProcessingJob{},
		&models.IngestionFailure{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate test database: %w", err)
	}

	return db, nil
}

func New(t *testing.T) *gorm.DB {
	t.Helper()

	once.Do(func() {
		baseDB, setupErr = setup()
	})
	if setupErr != nil {
		t.Fatalf("failed to set up test database: %v", setupErr)
	}

	tx := baseDB.Begin()
	t.Cleanup(func() { tx.Rollback() })
	return tx
}
