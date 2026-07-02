package db

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func Connect() {
	var err error

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	DB, err = pgxpool.New(context.Background(), dsn)

	if err != nil {
		log.Fatal("Cannot connect to database:", err)
	}

	log.Println("Connected to PostgreSQL")
}