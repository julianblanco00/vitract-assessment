package main

import (
	"flag"
	"log/slog"
	"os"

	"vitract-assessment/packages/database"
	"vitract-assessment/packages/ingestion"
	"vitract-assessment/packages/intake"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	samplesDir := flag.String("samples", "./data/samples", "directory of lab result JSON files")
	flag.Parse()

	godotenv.Load()

	db, err := database.Connect()
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	enqueued, skipped, err := intake.EnqueueSamples(db, string(ingestion.SourceLocalFile), *samplesDir)
	if err != nil {
		slog.Error("sample ingestion failed", "error", err)
		os.Exit(1)
	}
	slog.Info("samples enqueued", "new", enqueued, "skipped", skipped)
}
