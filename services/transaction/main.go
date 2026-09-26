package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/luciana-okorie/orbisflow/internal/database"
)

// loadDotEnvIfPresent reads .env in the current directory and sets any
// variables it defines, without overriding ones already set in the shell.
// Keeps `go run` working the same on PowerShell as on bash, without
// needing a third-party dotenv package.
func loadDotEnvIfPresent() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if os.Getenv(key) == "" {
			os.Setenv(key, strings.TrimSpace(parts[1]))
		}
	}
}

func main() {
	loadDotEnvIfPresent()
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set (check your .env)")
	}

	pool, err := database.NewPool(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	s := &server{db: pool}

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8110"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/v1/transactions", s.createTransactionHandler)

	log.Printf("orbisflow transaction service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
