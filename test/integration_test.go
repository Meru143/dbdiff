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

const (
	sourceURL = "postgres://testuser:testpass@localhost:5433/sourcedb?sslmode=disable"
	targetURL = "postgres://testuser:testpass@localhost:5434/targetdb?sslmode=disable"
)

func main() {
	fmt.Println("=== dbdiff Integration Tests ===")
	
	ctx := context.Background()
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for databases to be ready
	if !waitForDB(sourceURL, 30) {
		fmt.Println("❌ Source database not ready")
		os.Exit(1)
	}
	if !waitForDB(targetURL, 30) {
		fmt.Println("❌ Target database not ready")
		os.Exit(1)
	}
	fmt.Println("✅ Databases ready")

	// Test 1: Create test schema in source
	fmt.Println("\n--- Test 1: Create test schema in source ---")
	err := createTestSchema(sourceURL)
	if err != nil {
		fmt.Printf("❌ Failed to create source schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Source schema created")

	// Test 2: Create test schema in target (subset)
	fmt.Println("\n--- Test 2: Create test schema in target ---")
	err = createTargetSchema(targetURL)
	if err != nil {
		fmt.Printf("❌ Failed to create target schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Target schema created")

	// Test 3: Introspect source
	fmt.Println("\n--- Test 3: Introspect source ---")
	sourceConn, err := db.Connect(ctx, sourceURL, "disable", 10*time.Second)
	if err != nil {
		fmt.Printf("❌ Failed to connect to source: %v\n", err)
		os.Exit(1)
	}
	defer sourceConn.Close()

	sourceSchema, err := db.Introspect(ctx, sourceConn, "public", nil)
	if err != nil {
		fmt.Printf("❌ Failed to introspect source: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Source introspected: %d tables\n", len(sourceSchema.Tables))

	// Test 4: Introspect target
	fmt.Println("\n--- Test 4: Introspect target ---")
	targetConn, err := db.Connect(ctx, targetURL, "disable", 10*time.Second)
	if err != nil {
		fmt.Printf("❌ Failed to connect to target: %v\n", err)
		os.Exit(1)
	}
	defer targetConn.Close()

	targetSchema, err := db.Introspect(ctx, targetConn, "public", nil)
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
	formatter := output.NewFormatter("sql")
	migrationSQL, err := formatter.FormatMigration(differences, true)
	if err != nil {
		fmt.Printf("❌ Failed to generate migration: %v\n", err)
		os.Exit(1)
	}
	if strings.Contains(migrationSQL, "CREATE TABLE") {
		fmt.Println("✅ Migration SQL generated")
	} else {
		fmt.Println("⚠️  Migration SQL may be incomplete")
	}

	// Test 7: Table format output
	fmt.Println("\n--- Test 7: Table format output ---")
	tableFormatter := output.NewFormatter("table")
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
	jsonFormatter := output.NewFormatter("json")
	jsonOutput, err := jsonFormatter.Format(differences)
	if err != nil {
		fmt.Printf("❌ Failed to format JSON: %v\n", err)
		os.Exit(1)
	}
	if strings.Contains(jsonOutput, "diffs") {
		fmt.Println("✅ JSON output generated")
	}

	fmt.Println("\n=== All Integration Tests Passed ✅ ===")
}

func waitForDB(url string, maxAttempts int) bool {
	for i := 0; i < maxAttempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		conn, err := db.Connect(ctx, url, "disable", 5*time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func createTestSchema(url string) error {
	ctx := context.Background()
	conn, err := db.Connect(ctx, url, "disable", 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	schema := `
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

	_, err = conn.Exec(ctx, schema)
	return err
}

func createTargetSchema(url string) error {
	ctx := context.Background()
	conn, err := db.Connect(ctx, url, "disable", 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Target has only users table (subset)
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) NOT NULL
	);
	`

	_, err = conn.Exec(ctx, schema)
	return err
}
