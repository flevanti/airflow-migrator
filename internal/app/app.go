package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/flevanti/airflow-migrator/internal/core"
	"github.com/flevanti/airflow-migrator/internal/secrets"
	"golang.org/x/term"
)

// App information - single source of truth
const (
	Name         = "Airflow Connection Migrator"
	Version      = "1.0.6"
	Build        = "6"
	License      = "Apache 2.0"
	GitHub       = "https://github.com/flevanti/airflow-migrator"
	GitHubPR     = "https://github.com/flevanti/airflow-migrator/pulls"
	GitHubIssues = "https://github.com/flevanti/airflow-migrator/issues"
	DevName      = "Francesco Levanti"
	DevEmail     = "levanti.francesco@gmail.com"
)

// CurrentYear returns the current year for copyright notices
func CurrentYear() int {
	return time.Now().Year()
}

// App holds the initialized application components
type App struct {
	ConfigDir string
	Secrets   *secrets.Store
	Migrator  *core.Migrator
}

// Initialize sets up the application: config dir, password prompt, secrets store
func Initialize() (*App, error) {
	configDir := GetConfigDir()

	// Ensure config dir exists
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	fmt.Printf("Config directory: %s\n", configDir)

	// Get master password
	password, err := getMasterPassword(configDir)
	if err != nil {
		return nil, err
	}

	// Initialize secret store
	store, err := secrets.New(configDir, password)
	if err != nil {
		return nil, fmt.Errorf("failed to open secrets store: %w", err)
	}

	return &App{
		ConfigDir: configDir,
		Secrets:   store,
		Migrator:  core.New(),
	}, nil
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() string {
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
