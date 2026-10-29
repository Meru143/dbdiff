package postgres

import (
	"context"
	"strings"
	"testing"
	"time"
)

// pgx hides the password in the URL it quotes when it can't parse one, but can quote part
// of the password in its reason; connectOnce's error holds neither.
func TestConnectParseErrorHidesPassword(t *testing.T) {
	for _, url := range []string{
		"postgres://app:QZXa:@/%QZYb@localhost:5432/db", // pgx: invalid percent-encoded token: "%QZYb"
		"postgres://app:QZXa/QZYb@localhost:5432/db",    // pgx: invalid port
		"POSTGRES://app:QZXa@QZYb@localhost:5432/db",    // pgx reads keyword=value, and masks only up to the first '@'
	} {
		_, err := connectOnce(context.Background(), url, "disable", time.Second)
		if err == nil || strings.Contains(err.Error(), "QZX") || strings.Contains(err.Error(), "QZY") {
			t.Errorf("%s: expected an error without the password, got %v", url, err)
		}
	}
	_, err := connectOnce(context.Background(), "postgres://app:QZXa@localhost:5432/db?sslmode=bogus", "disable", time.Second)
	if err == nil || strings.Contains(err.Error(), "QZX") || !strings.Contains(err.Error(), "sslmode") {
		t.Errorf("an sslmode error should keep its reason, not the password, got %v", err)
	}
}

// pgx ends a password parameter's value at the next '&', and when what follows fails to
// parse, its reason quotes it, though it may be the rest of the password. The URL
// connectOnce parses again hides all that follows the '=', so its error holds none of it.
func TestConnectParseErrorHidesPasswordParameter(t *testing.T) {
	for _, url := range []string{
		"postgres://app@localhost:5432/db?sslmode=disable&password=QZXa&QZYb", // missing key/value separator "=" in URI query parameter: "QZYb"
		"postgres://app@localhost/db?password=QZXa&QZYb",
		"postgres://app@localhost:5432/db?password=QZXa&QZYb=QZYc=QZYd", // extra key/value separator "=" in URI query parameter: "QZYb"
		"postgres://app:QZXa@localhost:5432/db?sslmode=disable&sslpassword=QZXb&QZYb",
		"postgres://app@localhost:5x/db?password=QZXa&QZYb", // invalid port: the URL parsed again fails too
		// A 1-2 character piece after a raw '&' must not show either (regress/leaks finding 1).
		"postgres://app@localhost:5432/db?sslmode=disable&password=QZXa&Zq", // missing key/value separator "=" ... "Zq"
		"postgres://app@localhost/db?password=QZXa&9a",                      // "9a"
		"postgres://app@localhost:5432/db?password=QZXa&ab=QZYc=QZYd",       // extra key/value separator "=" ... "ab"
		"postgres://app:QZXa@localhost:5432/db?sslmode=disable&sslpassword=QZXb&Qz",
	} {
		_, err := connectOnce(context.Background(), url, "disable", time.Second)
		if err == nil || strings.Contains(err.Error(), "QZX") || strings.Contains(err.Error(), "QZY") ||
			strings.Contains(err.Error(), "Zq") || strings.Contains(err.Error(), "9a") ||
			strings.Contains(err.Error(), `"ab"`) || strings.Contains(err.Error(), "Qz") ||
			!strings.Contains(err.Error(), "password=") {
			t.Errorf("%s: expected an error naming the URL without its password, got %v", url, err)
		}
	}
}

// A parameter after the password whose value pgx echoes (unknown
// target_session_attrs value, a bad sslrootcert path) gives the generic hint, not the value,
// since the value may be the rest of the password (regress finding 2). connect_timeout and
// pool_max_conns, which also quote their value, do the same.
func TestConnectParseErrorHintsOnParamValue(t *testing.T) {
	for _, url := range []string{
		"postgres://app@127.0.0.1:1/db?password=Zq9simple&target_session_attrs=QZtS",
		"postgres://app@127.0.0.1:1/db?password=Zq9simple&channel_binding=QZcB",
		"postgres://app@127.0.0.1:1/db?password=Zq9simple&sslrootcert=QZrC",
		"postgres://app@127.0.0.1:1/db?password=Zq9simple&connect_timeout=QZen",
		"postgres://app@127.0.0.1:1/db?password=Zq9simple&pool_max_conns=QZmy",
	} {
		_, err := connectOnce(context.Background(), url, "disable", time.Second)
		if err == nil || strings.Contains(err.Error(), "QZ") || !strings.Contains(err.Error(), "password=***") {
			t.Errorf("%s: want the generic hint without the value, got %v", url, err)
		}
	}
}

// A parameter after the password that pgx rejects keeps its reason in the error, with the
// password hidden. Ending the redaction at the password hid the faulty parameter too and left
// only a generic hint; connectOnce re-parses a copy that keeps the parameters (a raw '&' tail,
// which pgx can't tell from one, still gets the hint — see the test above).
func TestConnectParseErrorKeepsParamReason(t *testing.T) {
	for _, tt := range []struct{ url, reason string }{
		{"postgres://app@127.0.0.1:1/db?password=QZXa&sslmode=disabled", "sslmode is invalid"},
		{"postgres://127.0.0.1:1/db?password=QZXa&user=app&port=abc", "invalid port"},
	} {
		_, err := connectOnce(context.Background(), tt.url, "disable", time.Second)
		if err == nil || strings.Contains(err.Error(), "QZX") || !strings.Contains(err.Error(), tt.reason) {
			t.Errorf("%s: want the reason %q without the password, got %v", tt.url, tt.reason, err)
		}
	}
}
