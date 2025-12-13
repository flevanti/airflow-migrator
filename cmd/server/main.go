package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/flevanti/airflow-migrator/api"
	"github.com/flevanti/airflow-migrator/internal/core"
	"github.com/flevanti/airflow-migrator/internal/secrets"
	"golang.org/x/term"
)

func main() {
	configDir := getConfigDir()
	fmt.Printf("Config directory: %s\n", configDir)

	// Get master password
	password, err := getMasterPassword(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize secret store
	store, err := secrets.New(configDir, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize core migrator
	migrator := core.New()

	// Start HTTP server
	server := api.NewServer(migrator, store, configDir)

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

func getConfigDir() string {
	// Check env var first
	if dir := os.Getenv("AIRFLOW_MIGRATOR_CONFIG"); dir != "" {
		return dir
	}

	// Default to ~/.config/airflow-migrator
	home, err := os.UserHomeDir()
	if err != nil {
		return ".airflow-migrator"
	}
	return filepath.Join(home, ".config", "airflow-migrator")
}

func getMasterPassword(configDir string) (string, error) {
	isNew := !secrets.Exists(configDir)

	if isNew {
		fmt.Println("First run - create a master password to encrypt your credentials.")
		fmt.Print("Enter new master password: ")
	} else {
		fmt.Print("Enter master password: ")
	}

	// Read password without echo
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // newline after password

	if err != nil {
		// Fallback for non-terminal (e.g., piped input)
		reader := bufio.NewReader(os.Stdin)
		password, _ := reader.ReadString('\n')
		return strings.TrimSpace(password), nil
	}

	password := string(passwordBytes)

	// Confirm on first run
	if isNew {
		fmt.Print("Confirm master password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()

		if err != nil {
			return "", fmt.Errorf("failed to read confirmation")
		}

		if string(confirmBytes) != password {
			return "", fmt.Errorf("passwords do not match")
		}
	}

	return password, nil
}
