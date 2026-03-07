//go:build e2e
// +build e2e

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("=== DBDiff E2E Tests ===")

	tests := []struct {
		name    string
		cmd     string
		args    []string
		wantErr bool
	}{
		{"version", "dbdiff", []string{"--version"}, false},
		{"help", "dbdiff", []string{"--help"}, false},
		{"compare no args", "dbdiff", []string{"compare"}, true},
		{"migrate no args", "dbdiff", []string{"migrate"}, true},
		{"diff no args", "dbdiff", []string{"diff"}, true},
		{"tables no args", "dbdiff", []string{"tables"}, true},
		{"validate no args", "dbdiff", []string{"validate"}, true},
	}

	cwd, _ := os.Getwd()
	// If running from within /test/e2e directory, go up two levels
	if strings.Contains(cwd, "test") {
		// Find the index of "test" and trim from there
		index := strings.LastIndex(cwd, "test")
		if index != -1 {
			cwd = cwd[:index]
		}
	}

	failed := 0
	for _, tt := range tests {
		// Use "go run main.go" from the root
		cmd := exec.Command("go", append([]string{"run", "main.go"}, tt.args...)...)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()

		gotErr := err != nil
		if gotErr != tt.wantErr {
			fmt.Printf("❌ %s: expected error=%v, got error=%v\n", tt.name, tt.wantErr, gotErr)
			fmt.Printf("   Output: %s\n", string(out))
			failed++
		} else {
			fmt.Printf("✅ %s\n", tt.name)
		}
	}

	// Test help output contains expected commands
	cmd := exec.Command("go", "run", "main.go", "--help")
	cmd.Dir = cwd
	out, _ := cmd.Output()
	output := string(out)

	expectedCommands := []string{"compare", "migrate", "diff", "tables", "validate"}
	for _, c := range expectedCommands {
		if !strings.Contains(output, c) {
			fmt.Printf("❌ Help missing command: %s\n", c)
			failed++
		} else {
			fmt.Printf("✅ Help contains: %s\n", c)
		}
	}

	if failed > 0 {
		fmt.Printf("\n❌ %d tests failed\n", failed)
		os.Exit(1)
	}

	fmt.Println("\n=== All E2E Tests Passed ✅ ===")
}
