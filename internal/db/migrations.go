package db

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB, scriptPaths ...string) error {
	for _, scriptPath := range scriptPaths {
		log.Printf("Running migration: %s", scriptPath)

		file, err := os.Open(scriptPath)
		if err != nil {
			return fmt.Errorf("cannot open %s: %w", scriptPath, err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", scriptPath, err)
		}

		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			_, err := db.Exec(stmt)
			if err != nil {
				if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate key") {
					log.Printf("Skipping error in %s: %v", scriptPath, err)
					continue
				}
				return fmt.Errorf("error executing statement in %s: %w", scriptPath, err)
			}
		}

		log.Printf("Migration %s applied", scriptPath)
	}

	return nil
}
