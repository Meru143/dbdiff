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
