package dsn

import (
	"regexp"
	"strings"
)

// mask replaces each secret Redact hides.
const mask = "***"

// secretParam matches a password-like parameter up to its value, in any case: password=,
// sslpassword=, passwd= and pwd=, in a URL's query (?password=x&...) or in a value of
// semicolon-separated settings (user id=sa;password=x).
var secretParam = regexp.MustCompile(`(?i)(?:password|passwd|pwd)\s*=\s*`)

// Redact hides the secrets in s, a connection string or text that quotes them, and keeps
// the rest, user and host included, since they tell a reviewer which database is meant:
// postgres://user:***@host/db, mysql://user:***@tcp(host:3306)/db,
// sqlserver://host?database=db&password=***.
//
// A URL's password runs from the first ':' after the scheme to the last '@', so one that
// holds '@', ':', '/', spaces or %-escapes is hidden whole, and so is the password of a
// malformed URL; that can hide more than the password, never less. The last '@' is that of
// the whole string, unless the URL is in double quotes (Go's %q, as cobra and net/url quote
// one in an error) or backticks (pgx), which then bound it. A DSN without a scheme, like
// go-sql-driver's user:pass@tcp(host:3306)/db, is hidden the same way, in quotes or as the
// whole of s.
func Redact(s string) string {
	var b strings.Builder
	quoted := false
	for i := 0; i < len(s); {
		c := s[i]
		if c == '"' || c == '`' {
			if end := closingQuote(s, i); end > 0 {
				b.WriteByte(c)
				b.WriteString(redactOne(s[i+1 : end]))
				b.WriteByte(c)
				i = end + 1
				quoted = true
				continue
			}
		} else if (i == 0 || !isSchemeChar(s[i-1])) && schemeLen(s[i:]) > 0 {
			b.WriteString(redactOne(s[i:]))
			return b.String()
		}
		b.WriteByte(c)
		i++
	}
	if !quoted {
		return redactOne(s)
	}
	return b.String()
}

// redactOne hides the secrets of one connection string: the password between the first
// ':' after the scheme and the last '@', and the value of each password-like parameter.
// Without a scheme, t may be a user:pass@host DSN, unless the part before the ':' has a
// space: then t is prose, like a driver's error.
func redactOne(t string) string {
	n := schemeLen(t)
	head, rest := t[:n], t[n:]
	if at := strings.LastIndexByte(rest, '@'); at > 0 {
		if colon := strings.IndexByte(rest[:at], ':'); colon >= 0 && colon+1 < at &&
			(n > 0 || !strings.ContainsAny(rest[:colon], " \t\r\n")) {
			rest = rest[:colon+1] + mask + rest[at:]
		}
	}
	return head + redactParams(rest)
}

// redactParams hides the values of password-like parameters in t.
func redactParams(t string) string {
	var b strings.Builder
	for {
		loc := secretParam.FindStringIndex(t)
		if loc == nil {
			b.WriteString(t)
			return b.String()
		}
		b.WriteString(t[:loc[1]])
		t = t[loc[1]:]
		if n := valueLen(t); n > 0 {
			b.WriteString(mask)
			t = t[n:]
		}
	}
}

// valueLen is the length of the parameter value v starts with: up to the next '&' or ';',
// or for a value in braces or quotes (password={a;b}), up to the closing brace or quote,
// which a doubled one escapes.
func valueLen(v string) int {
	var closer byte
	if v != "" {
		switch v[0] {
		case '{':
			closer = '}'
		case '"', '\'':
			closer = v[0]
		}
	}
	if closer == 0 {
		if i := strings.IndexAny(v, "&;"); i >= 0 {
			return i
		}
		return len(v)
	}
	for i := 1; i < len(v); i++ {
		if v[i] == closer {
			if i+1 < len(v) && v[i+1] == closer {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(v)
}

// closingQuote returns the index of the quote that closes the one at s[open], or -1. In
// double quotes, a backslash escapes the next byte, as in Go's %q.
func closingQuote(s string, open int) int {
	for i := open + 1; i < len(s); i++ {
		switch {
		case s[i] == '\\' && s[open] == '"':
			i++
		case s[i] == s[open]:
			return i
		}
	}
	return -1
}

// schemeLen is the length of the URL scheme and "://" that t starts with, or 0.
func schemeLen(t string) int {
	if t == "" || !isLetter(t[0]) {
		return 0
	}
	i := 1
	for i < len(t) && isSchemeChar(t[i]) {
		i++
	}
	if strings.HasPrefix(t[i:], "://") {
		return i + 3
	}
	return 0
}

func isLetter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func isSchemeChar(c byte) bool {
	return isLetter(c) || '0' <= c && c <= '9' || c == '+' || c == '-' || c == '.'
}
