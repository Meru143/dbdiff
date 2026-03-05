// Package dsn provides DSN parsing utilities for multiple database backends.
// It converts various URL formats into the native DSN strings expected by
// each database driver.
package dsn

import (
	"fmt"
	"net/url"
	"strings"
)

// Config holds parsed connection parameters
type Config struct {
	Driver   string // "postgres", "mysql", "sqlserver"
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Params   map[string]string
}

// Parse converts a connection string into a Config struct.
// Supported formats:
//   - postgres://user:pass@host:port/dbname?sslmode=disable
//   - mysql://user:pass@tcp(host:port)/dbname?parseTime=true
//   - sqlserver://user:pass@host:port?database=dbname
func Parse(connStr string) (*Config, error) {
	lower := strings.ToLower(connStr)

	switch {
	case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
		return parsePostgres(connStr)
	case strings.HasPrefix(lower, "mysql://"), strings.HasPrefix(lower, "mysql+tcp://"):
		return parseMySQL(connStr)
	case strings.HasPrefix(lower, "sqlserver://"):
		return parseSQLServer(connStr)
	default:
		return nil, fmt.Errorf("unsupported connection string format: %s", connStr)
	}
}

// ToNativeDSN converts a Config back to the native DSN expected by the driver.
func (c *Config) ToNativeDSN() string {
	switch c.Driver {
	case "postgres":
		return c.toPostgresDSN()
	case "mysql":
		return c.toMySQLDSN()
	case "sqlserver":
		return c.toSQLServerDSN()
	default:
		return ""
	}
}

func parsePostgres(connStr string) (*Config, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres URL: %w", err)
	}

	cfg := &Config{
		Driver: "postgres",
		Host:   u.Hostname(),
		Port:   u.Port(),
		Params: make(map[string]string),
	}

	if cfg.Port == "" {
		cfg.Port = "5432"
	}

	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}

	cfg.Database = strings.TrimPrefix(u.Path, "/")

	for k, v := range u.Query() {
		cfg.Params[k] = v[0]
	}

	return cfg, nil
}

func parseMySQL(connStr string) (*Config, error) {
	// MySQL URLs: mysql://user:pass@tcp(host:port)/dbname?params
	// or mysql://user:pass@host:port/dbname?params

	// Strip the mysql:// prefix
	raw := connStr
	if strings.HasPrefix(strings.ToLower(raw), "mysql+tcp://") {
		raw = "mysql://" + raw[12:]
	}

	// Check for tcp() format: mysql://user:pass@tcp(host:port)/dbname
	if idx := strings.Index(raw, "tcp("); idx != -1 {
		return parseMySQLTCP(raw)
	}

	// Standard URL format: mysql://user:pass@host:port/dbname
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid mysql URL: %w", err)
	}

	cfg := &Config{
		Driver: "mysql",
		Host:   u.Hostname(),
		Port:   u.Port(),
		Params: make(map[string]string),
	}

	if cfg.Port == "" {
		cfg.Port = "3306"
	}

	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}

	cfg.Database = strings.TrimPrefix(u.Path, "/")

	for k, v := range u.Query() {
		cfg.Params[k] = v[0]
	}

	return cfg, nil
}

func parseMySQLTCP(raw string) (*Config, error) {
	// Format: mysql://user:pass@tcp(host:port)/dbname?params
	cfg := &Config{
		Driver: "mysql",
		Params: make(map[string]string),
	}

	// Remove mysql://
	raw = raw[8:]

	// Split on @tcp(
	atIdx := strings.Index(raw, "@tcp(")
	if atIdx == -1 {
		return nil, fmt.Errorf("invalid mysql tcp format")
	}

	userPart := raw[:atIdx]
	rest := raw[atIdx+5:] // after "@tcp("

	// Parse user:pass
	if colonIdx := strings.Index(userPart, ":"); colonIdx != -1 {
		cfg.User = userPart[:colonIdx]
		cfg.Password = userPart[colonIdx+1:]
	} else {
		cfg.User = userPart
	}

	// Parse host:port)/dbname?params
	closeIdx := strings.Index(rest, ")")
	if closeIdx == -1 {
		return nil, fmt.Errorf("invalid mysql tcp format: missing closing paren")
	}

	hostPort := rest[:closeIdx]
	afterParen := rest[closeIdx+1:]

	if colonIdx := strings.LastIndex(hostPort, ":"); colonIdx != -1 {
		cfg.Host = hostPort[:colonIdx]
		cfg.Port = hostPort[colonIdx+1:]
	} else {
		cfg.Host = hostPort
		cfg.Port = "3306"
	}

	// Parse /dbname?params
	afterParen = strings.TrimPrefix(afterParen, "/")

	if qIdx := strings.Index(afterParen, "?"); qIdx != -1 {
		cfg.Database = afterParen[:qIdx]
		queryStr := afterParen[qIdx+1:]

		for _, pair := range strings.Split(queryStr, "&") {
			if eqIdx := strings.Index(pair, "="); eqIdx != -1 {
				cfg.Params[pair[:eqIdx]] = pair[eqIdx+1:]
			}
		}
	} else {
		cfg.Database = afterParen
	}

	return cfg, nil
}

func parseSQLServer(connStr string) (*Config, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("invalid sqlserver URL: %w", err)
	}

	cfg := &Config{
		Driver: "sqlserver",
		Host:   u.Hostname(),
		Port:   u.Port(),
		Params: make(map[string]string),
	}

	if cfg.Port == "" {
		cfg.Port = "1433"
	}

	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}

	// SQL Server uses ?database=dbname instead of /dbname
	for k, v := range u.Query() {
		if strings.ToLower(k) == "database" {
			cfg.Database = v[0]
		} else {
			cfg.Params[k] = v[0]
		}
	}

	// Also check the path for /dbname if no database param
	if cfg.Database == "" {
		cfg.Database = strings.TrimPrefix(u.Path, "/")
	}

	return cfg, nil
}

func (c *Config) toPostgresDSN() string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%s", c.Host, c.Port),
		Path:   c.Database,
	}
	if c.User != "" {
		if c.Password != "" {
			u.User = url.UserPassword(c.User, c.Password)
		} else {
			u.User = url.User(c.User)
		}
	}
	q := url.Values{}
	for k, v := range c.Params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Config) toMySQLDSN() string {
	// go-sql-driver format: user:pass@tcp(host:port)/dbname?params
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		c.User, c.Password, c.Host, c.Port, c.Database)

	if len(c.Params) > 0 {
		params := make([]string, 0, len(c.Params))
		for k, v := range c.Params {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		dsn += "?" + strings.Join(params, "&")
	}
	return dsn
}

func (c *Config) toSQLServerDSN() string {
	u := &url.URL{
		Scheme: "sqlserver",
		Host:   fmt.Sprintf("%s:%s", c.Host, c.Port),
	}
	if c.User != "" {
		if c.Password != "" {
			u.User = url.UserPassword(c.User, c.Password)
		} else {
			u.User = url.User(c.User)
		}
	}
	q := url.Values{}
	if c.Database != "" {
		q.Set("database", c.Database)
	}
	for k, v := range c.Params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
