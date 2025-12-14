package app

import (
	"testing"
	"time"
)

func TestAppConstants(t *testing.T) {
	// Verify constants are set
	if Name == "" {
		t.Error("Name should not be empty")
	}
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Build == "" {
		t.Error("Build should not be empty")
	}
	if License == "" {
		t.Error("License should not be empty")
	}
	if GitHub == "" {
		t.Error("GitHub should not be empty")
	}
	if GitHubPR == "" {
		t.Error("GitHubPR should not be empty")
	}
	if GitHubIssues == "" {
		t.Error("GitHubIssues should not be empty")
	}
	if DevName == "" {
		t.Error("DevName should not be empty")
	}
	if DevEmail == "" {
		t.Error("DevEmail should not be empty")
	}
}

func TestCurrentYear(t *testing.T) {
	year := CurrentYear()
	expected := time.Now().Year()

	if year != expected {
		t.Errorf("CurrentYear() = %d, want %d", year, expected)
	}

	// Should be reasonable year
	if year < 2024 || year > 2100 {
		t.Errorf("CurrentYear() = %d, seems unreasonable", year)
	}
}

func TestGetConfigDir(t *testing.T) {
	dir := GetConfigDir()

	if dir == "" {
		t.Error("GetConfigDir() should not return empty string")
	}
}

func TestGitHubURLs(t *testing.T) {
	// GitHubPR and GitHubIssues should be based on GitHub
	if len(GitHubPR) <= len(GitHub) {
		t.Error("GitHubPR should be longer than GitHub base URL")
	}
	if len(GitHubIssues) <= len(GitHub) {
		t.Error("GitHubIssues should be longer than GitHub base URL")
	}
}
