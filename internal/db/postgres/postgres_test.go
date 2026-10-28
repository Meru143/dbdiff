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
	} {
		_, err := connectOnce(context.Background(), url, "disable", time.Second)
		if err == nil || strings.Contains(err.Error(), "QZX") || strings.Contains(err.Error(), "QZY") || !strings.Contains(err.Error(), "password=") {
			t.Errorf("%s: expected an error naming the URL without its password, got %v", url, err)
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
		{"postgres://app@127.0.0.1:1/db?password=QZXa&target_session_attrs=primary-only", "target_session_attrs"},
		{"postgres://127.0.0.1:1/db?password=QZXa&user=app&port=abc", "invalid port"},
	} {
		_, err := connectOnce(context.Background(), tt.url, "disable", time.Second)
		if err == nil || strings.Contains(err.Error(), "QZX") || !strings.Contains(err.Error(), tt.reason) {
			t.Errorf("%s: want the reason %q without the password, got %v", tt.url, tt.reason, err)
		}
	}
}
