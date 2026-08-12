package main

import (
	"log/slog"
	"os"

	"vitract-assessment/packages/database"
	"vitract-assessment/packages/migrations"

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

	if err := migrations.Run(db); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	slog.Info("migrations applied")
}
