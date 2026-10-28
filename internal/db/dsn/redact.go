package dsn

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// mask replaces each secret Redact hides.
const mask = "***"

// What ends a parameter's value: in a URL's query only '&', as pgx, net/url and go-sql-driver
// read it, which take a ';' for part of the value; in settings (server=host;password=x) a ';'
// too.
const (
	inQuery    = "&"
	inSettings = "&;"
)

// dsnAddr matches what follows the '@' of a go-sql-driver DSN: a network with its address
// in parentheses, or nothing, then the '/' before the database: tcp(host:3306)/db,
// unix(/tmp/sock)/db, /db.
var dsnAddr = regexp.MustCompile(`^([A-Za-z0-9]+\([^)]*\))?/`)

// Redact hides the secrets in s, a connection string or text that quotes them, and keeps
// the rest, user and host included, since they tell a reviewer which database is meant:
// postgres://user:***@host/db, mysql://user:***@tcp(host:3306)/db,
// sqlserver://host?database=db&password=***.
//
// A URL's password runs from the first ':' after the scheme to the last '@', so one that
// holds '@', ':', '/', spaces or %-escapes is hidden whole, and so is the password of a
// malformed URL; that can hide more than the password, never less. If that '@' is in the
// value of a password parameter, as in host:1433?password=p@ss, the rest of the value is
// hidden with it. The last '@' is that of the whole string, unless the URL is in double
// quotes (Go's %q, as cobra and net/url quote one in an error) or backticks (pgx), which
// then bound it; quoted text is read the same way, URLs anywhere in it included. A URL is
// found wherever "://" follows a scheme, even one glued to the text before it, as in
// cobra's error for a URL glued to a flag: unknown flag: --sourcepostgres://user:***@host/db.
// A DSN without a scheme, like go-sql-driver's user:pass@tcp(host:3306)/db, is hidden the
// same way, in quotes or as the whole of s, quotes in its password included. The value of a
// password-like parameter (see secretValues) is hidden whole wherever it is, before any
// quote in it is read.
//
// A driver that reads part of what Redact hides as the host or the database, not the
// password, quotes that part in its errors: CheckPassword refuses such URLs.
func Redact(s string) string {
	if schemeLen(s) == 0 && userinfo(s, false) > 0 && dsnAddr.MatchString(s[strings.LastIndexByte(s, '@')+1:]) {
		return redactOne(s)
	}
	var b strings.Builder
	quoted := false
	text, key := 0, 0 // where the text not yet written, and the name before the next '=', start
	flush := func(end int) {
		b.WriteString(redactOne(s[text:end]))
		text = end
	}
	starts := schemeStarts(s)
	unclosed := "" // quotes that no later quote of their kind closes: none closes a later one either
	for i := 0; i < len(s); {
		for len(starts) > 0 && starts[0] < i {
			starts = starts[1:]
		}
		c := s[i]
		switch {
		case c == '?' || c == '&' || c == ';':
			key = i + 1
		case c == '=':
			if secretKey(s[key:i]) {
				v := i + 1 + len(s[i+1:]) - len(strings.TrimLeft(s[i+1:], " \t"))
				flush(v)
				if n := valueLen(s[v:], inSettings); n > 0 {
					b.WriteString(mask)
					text = v + n
				}
				i, key = text, text
				continue
			}
			key = i + 1
		case (c == '"' || c == '`') && strings.IndexByte(unclosed, c) < 0:
			end := closingQuote(s, i)
			if end < 0 {
				unclosed += string(c)
				break
			}
			flush(i)
			b.WriteByte(c)
			b.WriteString(Redact(s[i+1 : end]))
			b.WriteByte(c)
			i, text, key, quoted = end+1, end+1, end+1, true
			continue
		case len(starts) > 0 && starts[0] == i:
			flush(i)
			b.WriteString(redactOne(s[i:]))
			return b.String()
		}
		i++
	}
	if !quoted {
		return redactOne(s)
	}
	flush(len(s))
	return b.String()
}

// redactOne hides the secrets of one connection string: the password between the first
// ':' after the scheme and the last '@' (see passwordEnd), and the value of each
// password-like parameter, read as a URL's query if t has a scheme.
func redactOne(t string) string {
	n := schemeLen(t)
	head, rest := t[:n], t[n:]
	if colon := userinfo(rest, n > 0); colon > 0 {
		rest = rest[:colon] + mask + rest[passwordEnd(rest, strings.LastIndexByte(rest, '@')):]
	} else if start, end := portPassword(rest); n > 0 && start > 0 {
		rest = rest[:start] + mask + rest[end:]
	}
	if n > 0 {
		return head + redactParams(rest, inQuery)
	}
	return redactParams(rest, inSettings)
}

// passwordEnd returns where the password that ends at rest[at], the last '@', is hidden to:
// at, unless that '@' is in the value of a password-like parameter in the query, as in
// host:1433?password=p@ss, where it starts no host. Then all of rest is hidden: whether the
// '@' ends a password with a '?' in it or is part of the value, both are secret, and what
// follows the value is no host to show.
func passwordEnd(rest string, at int) int {
	q := strings.IndexByte(rest, '?')
	if q < 0 || q > at {
		return at
	}
	for _, v := range secretValues(rest[q:], inQuery) {
		if q+v[0] <= at && at < q+v[1] {
			return len(rest)
		}
	}
	return at
}

// portPassword returns where a password may stand in place of a port in rest, the part of
// a URL after its scheme, when no '@' ends a user name and password in its host part:
// after the ':' of a host whose "port" is no number, as in postgres://app:s3cret, which
// lacks its "@host", up to the '/', '?' or '#' that ends the host part, as only those end
// one in a URL (a password may hold a ';' or an '&'). It returns 0, 0 if there is none. A
// port that is a number stays, after each host of a list (h1:5432,h2:5433), and a ':' in
// brackets ([::1]) or parentheses (tcp(host:3306)) is no port's.
func portPassword(rest string) (start, end int) {
	depth, bad := 0, -1
	end = len(rest)
	for i := 0; i < end; i++ {
		switch c := rest[i]; {
		case c == '[' || c == '(':
			depth++
		case (c == ']' || c == ')') && depth > 0:
			depth--
		case depth > 0:
		case c == '/' || c == '?' || c == '#':
			end = i
		case c == '@':
			return 0, 0
		case c == ':' && bad < 0:
			j := i + 1
			for j < len(rest) && '0' <= rest[j] && rest[j] <= '9' {
				j++
			}
			if j < len(rest) && strings.IndexByte(",/?#", rest[j]) < 0 {
				bad = i + 1
			}
		}
	}
	if bad < 0 {
		return 0, 0
	}
	return bad, end
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

// CheckPassword returns an error, naming connStr redacted, if the driver for connStr would
// read part of what Redact hides in it as something other than a password, like the user
// name, the host or the database, which the driver's errors quote: failed to connect to
// `user=app database=s3cret@host/db`. Redact ends a URL's password at the last '@' (see
// passwordEnd). pgx (postgres://, postgresql://) ends the user name and password at the
// first '@' before any '/', even one after a '?'; net/url, which the SQL Server driver
// parses with, ends the host at the first '/', '?' or '#'; and go-sql-driver (mysql://,
// mysql+tcp://) reads the database after the last '/'.
func CheckPassword(connStr string) error {
	n := schemeLen(connStr)
	rest := connStr[n:]
	colon := userinfo(rest, true)
	at := strings.LastIndexByte(rest, '@')
	end := at
	if colon > 0 {
		end = passwordEnd(rest, at)
	}
	var ambiguous bool
	var fix string
	switch strings.ToLower(connStr[:n]) {
	case "postgres://", "postgresql://":
		fix = "percent-encode '@' and '/' in its user name and password, and any '@' after its host"
		if colon > 0 {
			ambiguous = strings.ContainsAny(rest[:end], "@/")
		} else if first := strings.IndexAny(rest, "@/"); first >= 0 && rest[first] == '@' {
			// No password ends at an '@', but pgx still ends a user name at the first one,
			// even in a password parameter's value: it would read the start of the value
			// as part of the user name, and the rest as the host.
			secrets := secretValues(rest, inQuery)
			ambiguous = len(secrets) > 0 && secrets[0][0] < first
		}
	case "sqlserver://":
		fix = "percent-encode '/', '?' and '#' in its user name and password, and any '@' after its host"
		ambiguous = colon > 0 && strings.ContainsAny(rest[:end], "/?#")
	case "mysql://", "mysql+tcp://":
		fix = "end it with /database after the address, and percent-encode any '@' after that '/'"
		ambiguous = colon > 0 && (end != at || !strings.Contains(rest[at:], "/"))
	}
	if !ambiguous {
		return nil
	}
	// No ':' after the URL: Redact, which main prints errors through, would take one before
	// the '@' in fix for a password's start.
	return fmt.Errorf("can't tell where the password ends in %q; %s", Redact(connStr), fix)
}

// redactParams hides the value of each password-like parameter in t (see secretValues).
func redactParams(t, ends string) string {
	var b strings.Builder
	written := 0
	for _, v := range secretValues(t, ends) {
		if v[1] > v[0] {
			b.WriteString(t[written:v[0]])
			b.WriteString(mask)
			written = v[1]
		}
	}
	b.WriteString(t[written:])
	return b.String()
}

// secretValues returns where the value of each password-like parameter in t starts and
// ends, in a URL's query (?password=x&...) or in semicolon-separated settings (user
// id=sa;password=x), whichever ends says t holds: one whose name, read percent-decoded and
// in any case, ends in password, passwd or pwd (password, sslpassword, Pwd, pass%77ord).
func secretValues(t, ends string) [][2]int {
	var values [][2]int
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
			end := v + valueLen(t[v:], ends)
			values = append(values, [2]int{v, end})
			i, key = end-1, end
		}
	}
	return values
}

// secretKey reports whether a parameter name is password-like.
func secretKey(name string) bool {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(name, "password") || strings.HasSuffix(name, "passwd") || strings.HasSuffix(name, "pwd")
}

// valueLen is the length of the parameter value v starts with: up to the next byte of ends
// outside braces or quotes (password={a;b}), where a doubled closing brace or quote stands
// for itself. A value that holds a URL, outside braces or quotes, runs to the end of v:
// what ends it is the URL's own, like the '@' after its password.
func valueLen(v, ends string) int {
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
	if i := strings.IndexAny(v[start:], ends); i >= 0 && (start > 0 || !strings.Contains(v[:i], "://")) {
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

// schemeStarts returns where the scheme of each URL in s starts, in order: at the first
// letter of the scheme characters before each "://", which can be glued to the text before
// them, as in --sourcepostgres://. Each byte is looked at a bounded number of times.
func schemeStarts(s string) []int {
	var starts []int
	for at := strings.Index(s, "://"); at >= 0; {
		start := at
		for start > 0 && isSchemeChar(s[start-1]) {
			start--
		}
		for start < at && !isLetter(s[start]) {
			start++
		}
		if start < at {
			starts = append(starts, start)
		}
		next := strings.Index(s[at+3:], "://")
		if next < 0 {
			break
		}
		at += 3 + next
	}
	return starts
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
