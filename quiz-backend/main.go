package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"quiz-backend/db"
	"quiz-backend/routes"
)

func main() {
	_ = godotenv.Load()

	db.Connect()

	router := routes.SetupRoutes()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Println("Server started on :" + port)

	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatal(err)
	}
}