package main

import (
	"fmt"
	"os"

	"github.com/flevanti/airflow-migrator/api"
	"github.com/flevanti/airflow-migrator/internal/app"
)

func main() {
	// Initialize app (config, password, secrets)
	application, err := app.Initialize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Start HTTP server
	server := api.NewServer(application.Migrator, application.Secrets, application.ConfigDir)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	fmt.Printf("Starting server on http://localhost:%s\n", port)
	if err := server.Start(":" + port); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
