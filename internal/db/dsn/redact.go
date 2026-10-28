package dsn

import (
	"net/url"
	"regexp"
	"strings"
)

// mask replaces each secret Redact hides.
const mask = "***"

// dsnAddr matches what follows the '@' of a go-sql-driver DSN: a network and its address,
// both optional, then the '/' before the database: tcp(host:3306)/db, unix(/tmp/sock)/db, /db.
var dsnAddr = regexp.MustCompile(`^[A-Za-z0-9]*(\([^)]*\))?/`)

// settings matches the start of semicolon-separated settings: server=host;user id=sa;...
var settings = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 _.%-]*=`)

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
// whole of s, quotes in its password included. Password-like parameters are hidden
// wherever they are (see redactParams); in settings like server=host;pwd=x, before
// anything else, so that their values' quotes are read as such.
func Redact(s string) string {
	if schemeLen(s) == 0 {
		if userinfo(s, false) > 0 && dsnAddr.MatchString(s[strings.LastIndexByte(s, '@')+1:]) {
			return redactOne(s)
		}
		if settings.MatchString(s) {
			s = redactParams(s)
		}
	}
	var b strings.Builder
	quoted, text := false, 0 // text: where the text not yet written starts
	for i := 0; i < len(s); {
		c := s[i]
		if c == '"' || c == '`' {
			if end := closingQuote(s, i); end > 0 {
				before := s[text:i]
				p := redactOne(before)
				b.WriteString(p)
				if secretValueNext(before) { // pwd="x": the quotes hold a password
					if !strings.HasSuffix(p, mask) { // else they end a value already hidden
						b.WriteString(mask)
					}
					i += valueLen(s[i:])
				} else {
					b.WriteByte(c)
					b.WriteString(redactOne(s[i+1 : end]))
					b.WriteByte(c)
					i = end + 1
				}
				text, quoted = i, true
				continue
			}
		} else if (i == 0 || !isSchemeChar(s[i-1])) && schemeLen(s[i:]) > 0 {
			b.WriteString(redactOne(s[text:i]))
			b.WriteString(redactOne(s[i:]))
			return b.String()
		}
		i++
	}
	if !quoted {
		return redactOne(s)
	}
	b.WriteString(redactOne(s[text:]))
	return b.String()
}

// redactOne hides the secrets of one connection string: the password between the first
// ':' after the scheme and the last '@', and the value of each password-like parameter.
func redactOne(t string) string {
	n := schemeLen(t)
	head, rest := t[:n], t[n:]
	if colon := userinfo(rest, n > 0); colon > 0 {
		at := strings.LastIndexByte(rest, '@')
		rest = rest[:colon] + mask + rest[at:]
	}
	return head + redactParams(rest)
}

// userinfo returns the index after the ':' that starts a password in rest, the part of a
// connection string after its scheme, or 0 if rest has none: the password runs from the
// first ':' to the last '@'. Without a scheme, rest may be a user:pass@host DSN, unless the
// part before the ':' holds a space, '=', ';' or '&', which no user name in one does: then
// rest is prose, like a driver's error, or settings.
func userinfo(rest string, scheme bool) int {
	at := strings.LastIndexByte(rest, '@')
	if at <= 0 {
		return 0
	}
	colon := strings.IndexByte(rest[:at], ':')
	if colon < 0 || colon+1 == at || !scheme && strings.ContainsAny(rest[:colon], " \t\r\n=;&") {
		return 0
	}
	return colon + 1
}

// redactParams hides the value of each password-like parameter in t, in a URL's query
// (?password=x&...) or in semicolon-separated settings (user id=sa;password=x): one whose
// name, read percent-decoded and in any case, ends in password, passwd or pwd (password,
// sslpassword, Pwd, pass%77ord).
func redactParams(t string) string {
	var b strings.Builder
	key := 0 // where the name before the next '=' starts
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case '?', '&', ';':
			key = i + 1
		case '=':
			if !secretKey(t[key:i]) {
				key = i + 1
				continue
			}
			v := i + 1 + len(t[i+1:]) - len(strings.TrimLeft(t[i+1:], " \t"))
			b.WriteString(t[:v])
			n := valueLen(t[v:])
			if n > 0 {
				b.WriteString(mask)
			}
			t, i, key = t[v+n:], -1, 0
		}
	}
	b.WriteString(t)
	return b.String()
}

// secretKey reports whether a parameter name is password-like.
func secretKey(name string) bool {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(name, "password") || strings.HasSuffix(name, "passwd") || strings.HasSuffix(name, "pwd")
}

// secretValueNext reports whether t ends with a password-like parameter's name and '=',
// so that what follows t is its value.
func secretValueNext(t string) bool {
	t = strings.TrimRight(t, " \t")
	if !strings.HasSuffix(t, "=") {
		return false
	}
	t = t[:len(t)-1]
	return secretKey(t[strings.LastIndexAny(t, "?&;=")+1:])
}

// valueLen is the length of the parameter value v starts with: up to the next '&' or ';'
// outside braces or quotes (password={a;b}), where a doubled closing brace or quote stands
// for itself.
func valueLen(v string) int {
	start := 0
	if v != "" && strings.IndexByte("{\"'`", v[0]) >= 0 {
		closer := v[0]
		if closer == '{' {
			closer = '}'
		}
		start = len(v) // never closed: all of v
		for i := 1; i < len(v); i++ {
			if v[i] == closer {
				if i+1 < len(v) && v[i+1] == closer {
					i++
					continue
				}
				start = i + 1
				break
			}
		}
	}
	if i := strings.IndexAny(v[start:], "&;"); i >= 0 {
		return start + i
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
