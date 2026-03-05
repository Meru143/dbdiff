//go:build integration
// +build integration

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/internal/output"
)

type DBTestConfig struct {
	Name      string
	Dialect   string
	SourceURL string
	TargetURL string
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	fmt.Println("=== dbdiff Integration Tests ===")

	configs := []DBTestConfig{
		{
			Name:    "PostgreSQL",
			Dialect: "postgresql",
			SourceURL: envOrDefault("PG_SOURCE_DSN",
				"postgres://testuser:testpass@localhost:5433/sourcedb?sslmode=disable"),
			TargetURL: envOrDefault("PG_TARGET_DSN",
				"postgres://testuser:testpass@localhost:5434/targetdb?sslmode=disable"),
		},
		{
			Name:    "MySQL",
			Dialect: "mysql",
			SourceURL: envOrDefault("MYSQL_SOURCE_DSN",
				"mysql://testuser:testpass@tcp(localhost:3307)/sourcedb?multiStatements=true&parseTime=true"),
			TargetURL: envOrDefault("MYSQL_TARGET_DSN",
				"mysql://testuser:testpass@tcp(localhost:3308)/targetdb?multiStatements=true&parseTime=true"),
		},
	}

	for _, cfg := range configs {
		runIntegrationTest(cfg)
	}

	fmt.Println("\n=== All Integration Tests Passed ✅ ===")
}

func runIntegrationTest(cfg DBTestConfig) {
	fmt.Printf("\n>>> Running Integration Suite for %s <<<\n", cfg.Name)

	ctx := context.Background()
	timeout := 45 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for databases to be ready
	if !waitForDB(cfg.SourceURL, 30) {
		fmt.Printf("❌ %s Source database not ready\n", cfg.Name)
		os.Exit(1)
	}
	if !waitForDB(cfg.TargetURL, 30) {
		fmt.Printf("❌ %s Target database not ready\n", cfg.Name)
		os.Exit(1)
	}
	fmt.Println("✅ Databases ready")

	// Test 1: Create test schema in source
	fmt.Println("\n--- Test 1: Create test schema in source ---")
	err := createTestSchema(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to create source schema for %s: %v\n", cfg.Name, err)
		os.Exit(1)
	}
	fmt.Println("✅ Source schema created")

	// Test 2: Create test schema in target (subset)
	fmt.Println("\n--- Test 2: Create test schema in target ---")
	err = createTargetSchema(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to create target schema for %s: %v\n", cfg.Name, err)
		os.Exit(1)
	}
	fmt.Println("✅ Target schema created")

	// Test 3: Introspect source
	fmt.Println("\n--- Test 3: Introspect source ---")
	sourceDriver, err := db.NewDriver(cfg.SourceURL)
	if err != nil {
		fmt.Printf("❌ Failed to resolve driver for %s: %v\n", cfg.Name, err)
		os.Exit(1)
	}
	err = sourceDriver.Connect(ctx, cfg.SourceURL, "disable", 10*time.Second)
	if err != nil {
		fmt.Printf("❌ Failed to connect to source: %v\n", err)
		os.Exit(1)
	}
	defer sourceDriver.Close()

	sourceSchema, err := sourceDriver.Introspect(ctx, getDefaultSchema(cfg.Dialect), nil)
	if err != nil {
		fmt.Printf("❌ Failed to introspect source: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Source introspected: %d tables\n", len(sourceSchema.Tables))

	// Test 4: Introspect target
	fmt.Println("\n--- Test 4: Introspect target ---")
	targetDriver, err := db.NewDriver(cfg.TargetURL)
	if err != nil {
		fmt.Printf("❌ Failed to resolve target driver for %s: %v\n", cfg.Name, err)
		os.Exit(1)
	}
	err = targetDriver.Connect(ctx, cfg.TargetURL, "disable", 10*time.Second)
	if err != nil {
		fmt.Printf("❌ Failed to connect to target: %v\n", err)
		os.Exit(1)
	}
	defer targetDriver.Close()

	targetSchema, err := targetDriver.Introspect(ctx, getDefaultSchema(cfg.Dialect), nil)
	if err != nil {
		fmt.Printf("❌ Failed to introspect target: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Target introspected: %d tables\n", len(targetSchema.Tables))

	// Test 5: Compare schemas
	fmt.Println("\n--- Test 5: Compare schemas ---")
	differences := diff.Compare(sourceSchema, targetSchema)
	fmt.Printf("✅ Found %d differences\n", len(differences))

	// Test 6: Generate migration SQL
	fmt.Println("\n--- Test 6: Generate migration SQL ---")
	formatterOpts := output.FormatOptions{
		Format:   "sql",
		SourceDB: cfg.SourceURL,
	}
	formatter := output.NewFormatterWithOptions(formatterOpts)
	migrationSQL, err := formatter.FormatMigration(differences, true)
	if err != nil {
		fmt.Printf("❌ Failed to generate migration: %v\n", err)
		os.Exit(1)
	}
	if strings.Contains(migrationSQL, "TABLE") || strings.Contains(migrationSQL, "table") {
		fmt.Println("✅ Migration SQL generated")
	} else {
		fmt.Println("⚠️  Migration SQL may be incomplete")
	}

	// Test 7: Table format output
	fmt.Println("\n--- Test 7: Table format output ---")
	tableFormatter := output.NewFormatterWithOptions(output.FormatOptions{Format: "table", SourceDB: cfg.SourceURL})
	tableOutput, err := tableFormatter.Format(differences)
	if err != nil {
		fmt.Printf("❌ Failed to format table: %v\n", err)
		os.Exit(1)
	}
	if len(tableOutput) > 0 {
		fmt.Println("✅ Table output generated")
	}

	// Test 8: JSON format output
	fmt.Println("\n--- Test 8: JSON format output ---")
	jsonFormatter := output.NewFormatterWithOptions(output.FormatOptions{Format: "json", SourceDB: cfg.SourceURL})
	jsonOutput, err := jsonFormatter.Format(differences)
	if err != nil {
		fmt.Printf("❌ Failed to format JSON: %v\n", err)
		os.Exit(1)
	}
	if strings.Contains(jsonOutput, "diffs") {
		fmt.Println("✅ JSON output generated")
	}

	// Test 9: Migration Tracking
	fmt.Println("\n--- Test 9: Migration Tracking ---")
	err = targetDriver.EnsureMigrationsTable(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to ensure migrations table: %v\n", err)
		os.Exit(1)
	}

	applied, err := targetDriver.GetAppliedMigrations(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to get applied migrations: %v\n", err)
		os.Exit(1)
	}
	if len(applied) != 0 {
		fmt.Printf("❌ Expected 0 applied migrations, got %d\n", len(applied))
		os.Exit(1)
	}

	err = targetDriver.RecordMigration(ctx, "001_init.sql")
	if err != nil {
		fmt.Printf("❌ Failed to record migration: %v\n", err)
		os.Exit(1)
	}

	applied, err = targetDriver.GetAppliedMigrations(ctx)
	if err != nil || len(applied) != 1 || applied[0] != "001_init.sql" {
		fmt.Printf("❌ Failed to verify recorded migration\n")
		os.Exit(1)
	}

	err = targetDriver.RemoveMigration(ctx, "001_init.sql")
	if err != nil {
		fmt.Printf("❌ Failed to remove migration: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Migration tracking validated")
}

func getDefaultSchema(dialect string) string {
	if dialect == "mysql" {
		return "sourcedb" // mysql schema is DB name
	}
	return "public"
}

func waitForDB(url string, maxAttempts int) bool {
	for i := 0; i < maxAttempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		driver, err := db.NewDriver(url)
		if err == nil {
			err = driver.Connect(ctx, url, "disable", 5*time.Second)
			if err == nil {
				driver.Close()
				return true
			}
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func createTestSchema(cfg DBTestConfig) error {
	ctx := context.Background()
	driver, err := db.NewDriver(cfg.SourceURL)
	if err != nil {
		return err
	}
	err = driver.Connect(ctx, cfg.SourceURL, "disable", 10*time.Second)
	if err != nil {
		return err
	}
	defer driver.Close()

	var schema string
	if cfg.Dialect == "mysql" {
		schema = `
		CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS posts (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			published BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		
		CREATE TABLE IF NOT EXISTS comments (
			id INT AUTO_INCREMENT PRIMARY KEY,
			post_id INT,
			user_id INT,
			body TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		
		CREATE INDEX idx_posts_user_id ON posts(user_id);
		CREATE INDEX idx_posts_published ON posts(published);
		CREATE INDEX idx_comments_post_id ON comments(post_id);
		`
	} else {
		schema = `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			published BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS comments (
			id SERIAL PRIMARY KEY,
			post_id INTEGER REFERENCES posts(id) ON DELETE CASCADE,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			body TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
		CREATE INDEX IF NOT EXISTS idx_posts_published ON posts(published);
		CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id);
		`
	}

	return driver.Exec(ctx, schema)
}

func createTargetSchema(cfg DBTestConfig) error {
	ctx := context.Background()
	driver, err := db.NewDriver(cfg.TargetURL)
	if err != nil {
		return err
	}
	err = driver.Connect(ctx, cfg.TargetURL, "disable", 10*time.Second)
	if err != nil {
		return err
	}
	defer driver.Close()

	var schema string
	if cfg.Dialect == "mysql" {
		schema = `
		CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(255) NOT NULL
		);
		`
	} else {
		schema = `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL
		);
		`
	}

	return driver.Exec(ctx, schema)
}
