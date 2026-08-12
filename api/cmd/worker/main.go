package main

import (
	"log/slog"
	"os"
	"time"
	"vitract-assessment/packages/database"
	"vitract-assessment/packages/ingestion"
	"vitract-assessment/packages/notification"
	"vitract-assessment/packages/worker"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	godotenv.Load()

	db, err := database.Connect()
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	registry := ingestion.NewRegistry()
	registry.Register(ingestion.SourceLocalFile, &ingestion.LocalFileSource{BaseDir: "./data/samples"})

	w := worker.New(db, registry, &notification.LogNotifier{})
	w.Run(2 * time.Second)
}
