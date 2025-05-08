package db

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

// NewDB creates a new SQLite database for caching GitHub API calls
func NewDB() (*DB, error) {
	// Create cache directory if it doesn't exist
	cacheDir := filepath.Join(os.Getenv("HOME"), ".changelog-checker")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(cacheDir, "cache.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS github_pr_cache (
			repo_owner TEXT,
			repo_name TEXT,
			pr_number INTEGER,
			title TEXT,
			fetched_at TIMESTAMP,
			PRIMARY KEY (repo_owner, repo_name, pr_number)
		)
	`)
	if err != nil {
		return nil, err
	}
	
	// Create validation cache table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS validation_cache (
			repo_owner TEXT,
			repo_name TEXT,
			pr_number INTEGER,
			changelog_desc TEXT,
			status INTEGER,
			last_validated TIMESTAMP,
			PRIMARY KEY (repo_owner, repo_name, pr_number)
		)
	`)
	if err != nil {
		return nil, err
	}

	return &DB{db: db}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.db.Close()
}

// GetPRInfo retrieves PR information from the cache
func (d *DB) GetPRInfo(repoOwner, repoName string, prNumber int) (string, bool, error) {
	var title string
	var fetchedAt time.Time

	err := d.db.QueryRow(
		"SELECT title, fetched_at FROM github_pr_cache WHERE repo_owner = ? AND repo_name = ? AND pr_number = ?",
		repoOwner, repoName, prNumber,
	).Scan(&title, &fetchedAt)

	if err == sql.ErrNoRows {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	// Check if cache is older than 7 days
	if time.Since(fetchedAt) > 7*24*time.Hour {
		log.Printf("Cache for PR #%d is older than 7 days, will refresh", prNumber)
		return "", false, nil
	}

	return title, true, nil
}

// StorePRInfo stores PR information in the cache
func (d *DB) StorePRInfo(repoOwner, repoName string, prNumber int, title string) error {
	_, err := d.db.Exec(
		"INSERT OR REPLACE INTO github_pr_cache (repo_owner, repo_name, pr_number, title, fetched_at) VALUES (?, ?, ?, ?, ?)",
		repoOwner, repoName, prNumber, title, time.Now(),
	)
	return err
}

// GetValidationResult retrieves validation result from the cache
// Returns status, cached (bool), and error
func (d *DB) GetValidationResult(repoOwner, repoName string, prNumber int, changelogDesc string) (int, bool, error) {
	var status int
	var storedChangelogDesc string
	var lastValidated time.Time

	err := d.db.QueryRow(
		"SELECT changelog_desc, status, last_validated FROM validation_cache WHERE repo_owner = ? AND repo_name = ? AND pr_number = ?",
		repoOwner, repoName, prNumber,
	).Scan(&storedChangelogDesc, &status, &lastValidated)

	if err == sql.ErrNoRows {
		return 0, false, nil
	} else if err != nil {
		return 0, false, err
	}

	// If the changelog description has changed, invalidate the cache
	if storedChangelogDesc != changelogDesc {
		return 0, false, nil
	}

	// Check if cache is older than 7 days (same as PR info cache)
	if time.Since(lastValidated) > 7*24*time.Hour {
		log.Printf("Validation cache for PR #%d is older than 7 days, will refresh", prNumber)
		return 0, false, nil
	}

	return status, true, nil
}

// StoreValidationResult stores validation result in the cache
func (d *DB) StoreValidationResult(repoOwner, repoName string, prNumber int, changelogDesc string, status int) error {
	_, err := d.db.Exec(
		"INSERT OR REPLACE INTO validation_cache (repo_owner, repo_name, pr_number, changelog_desc, status, last_validated) VALUES (?, ?, ?, ?, ?, ?)",
		repoOwner, repoName, prNumber, changelogDesc, status, time.Now(),
	)
	return err
}