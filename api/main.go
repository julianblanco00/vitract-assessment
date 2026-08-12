package main

import (
	"log/slog"
	"net/http"
	"os"
	"vitract-assessment/packages/api"
	"vitract-assessment/packages/database"

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

	const addr = ":8080"
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, api.NewRouter(db)); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
