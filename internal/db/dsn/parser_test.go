package dsn

import (
	"testing"
)

func TestParsePostgres(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDB   string
		wantHost string
		wantPort string
		wantUser string
		wantErr  bool
	}{
		{
			name:     "standard URL",
			input:    "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
			wantDB:   "mydb",
			wantHost: "localhost",
			wantPort: "5432",
			wantUser: "user",
		},
		{
			name:     "postgresql scheme",
			input:    "postgresql://admin:secret@db.example.com:5433/production",
			wantDB:   "production",
			wantHost: "db.example.com",
			wantPort: "5433",
			wantUser: "admin",
		},
		{
			name:     "default port",
			input:    "postgres://user:pass@localhost/mydb",
			wantDB:   "mydb",
			wantHost: "localhost",
			wantPort: "5432",
			wantUser: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if cfg.Database != tt.wantDB {
				t.Errorf("Database = %q, want %q", cfg.Database, tt.wantDB)
			}
			if cfg.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", cfg.Host, tt.wantHost)
			}
			if cfg.Port != tt.wantPort {
				t.Errorf("Port = %q, want %q", cfg.Port, tt.wantPort)
			}
			if cfg.User != tt.wantUser {
				t.Errorf("User = %q, want %q", cfg.User, tt.wantUser)
			}
			if cfg.Driver != "postgres" {
				t.Errorf("Driver = %q, want postgres", cfg.Driver)
			}
		})
	}
}

func TestParseMySQLStandard(t *testing.T) {
	cfg, err := Parse("mysql://testuser:testpass@localhost:3306/mydb?parseTime=true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != "mysql" {
		t.Errorf("Driver = %q, want mysql", cfg.Driver)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", cfg.Host)
	}
	if cfg.Port != "3306" {
		t.Errorf("Port = %q, want 3306", cfg.Port)
	}
	if cfg.Database != "mydb" {
		t.Errorf("Database = %q, want mydb", cfg.Database)
	}
	if cfg.User != "testuser" {
		t.Errorf("User = %q, want testuser", cfg.User)
	}
}

func TestParseMySQLTCP(t *testing.T) {
	cfg, err := Parse("mysql://testuser:testpass@tcp(localhost:3307)/sourcedb?multiStatements=true&parseTime=true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", cfg.Host)
	}
	if cfg.Port != "3307" {
		t.Errorf("Port = %q, want 3307", cfg.Port)
	}
	if cfg.Database != "sourcedb" {
		t.Errorf("Database = %q, want sourcedb", cfg.Database)
	}
	if cfg.Params["multiStatements"] != "true" {
		t.Errorf("missing multiStatements param")
	}
}

func TestParseSQLServer(t *testing.T) {
	cfg, err := Parse("sqlserver://sa:Password123@localhost:1433?database=testdb&encrypt=disable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != "sqlserver" {
		t.Errorf("Driver = %q, want sqlserver", cfg.Driver)
	}
	if cfg.Database != "testdb" {
		t.Errorf("Database = %q, want testdb", cfg.Database)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", cfg.Host)
	}
	if cfg.Port != "1433" {
		t.Errorf("Port = %q, want 1433", cfg.Port)
	}
	if cfg.Params["encrypt"] != "disable" {
		t.Errorf("missing encrypt param")
	}
}

func TestUnsupportedFormat(t *testing.T) {
	_, err := Parse("mongodb://localhost:27017/mydb")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestToNativeDSN_Postgres(t *testing.T) {
	cfg, _ := Parse("postgres://user:pass@localhost:5432/mydb?sslmode=disable")
	dsn := cfg.ToNativeDSN()
	if dsn == "" {
		t.Error("expected non-empty DSN")
	}
	if cfg.Driver != "postgres" {
		t.Errorf("expected postgres driver, got %s", cfg.Driver)
	}
}

func TestToNativeDSN_MySQL(t *testing.T) {
	cfg, _ := Parse("mysql://user:pass@tcp(localhost:3306)/mydb?parseTime=true")
	dsn := cfg.ToNativeDSN()
	if dsn == "" {
		t.Error("expected non-empty DSN")
	}
	// Should be in go-sql-driver format
	if !contains(dsn, "tcp(localhost:3306)") {
		t.Errorf("expected tcp() format in DSN, got: %s", dsn)
	}
}

func TestToNativeDSN_SQLServer(t *testing.T) {
	cfg, _ := Parse("sqlserver://sa:Pass@localhost:1433?database=testdb")
	dsn := cfg.ToNativeDSN()
	if dsn == "" {
		t.Error("expected non-empty DSN")
	}
	if !contains(dsn, "sqlserver://") {
		t.Errorf("expected sqlserver:// scheme in DSN, got: %s", dsn)
	}
}

func TestRoundTrip(t *testing.T) {
	original := "postgres://user:pass@localhost:5432/mydb?sslmode=disable"
	cfg, err := Parse(original)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	dsn := cfg.ToNativeDSN()
	cfg2, err := Parse(dsn)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}

	if cfg.Host != cfg2.Host || cfg.Port != cfg2.Port || cfg.Database != cfg2.Database {
		t.Errorf("round-trip mismatch: %+v vs %+v", cfg, cfg2)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
