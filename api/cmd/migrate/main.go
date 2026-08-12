package main

import (
	"log"

	"vitract-assessment/packages/database"
	"vitract-assessment/packages/migrations"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := migrations.Run(db); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	log.Println("migrations applied")
}
