package main

import (
	"flag"
	"log"

	"vitract-assessment/packages/database"
	"vitract-assessment/packages/ingestion"
	"vitract-assessment/packages/intake"

	"github.com/joho/godotenv"
)

func main() {
	samplesDir := flag.String("samples", "./data/samples", "directory of lab result JSON files")
	flag.Parse()

	godotenv.Load()

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	enqueued, skipped, err := intake.EnqueueSamples(db, string(ingestion.SourceLocalFile), *samplesDir)
	if err != nil {
		log.Fatalf("sample ingestion failed: %v", err)
	}
	log.Printf("samples enqueued: %d new, %d skipped (already known)", enqueued, skipped)
}
