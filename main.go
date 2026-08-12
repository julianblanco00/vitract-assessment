package main

import (
	"log"
	"net/http"
	"time"
	"vitract-assessment/packages/api"
	"vitract-assessment/packages/database"
	"vitract-assessment/packages/ingestion"
	"vitract-assessment/packages/notification"
	"vitract-assessment/packages/worker"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}

	registry := ingestion.NewRegistry()
	registry.Register(ingestion.SourceLocalFile, &ingestion.LocalFileSource{BaseDir: "./data/samples"})

	w := worker.New(db, registry, &notification.LogNotifier{})
	go w.Run(2 * time.Second)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", api.NewRouter(db)))
}
