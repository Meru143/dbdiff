package dsn

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// mask replaces each secret Redact hides.
const mask = "***"

// What ends a parameter's value. In a URL's query only '&', as pgx, net/url and go-sql-driver
// read it, which take a ';' for part of the value: CheckPassword reads values so. Redact ends
// a password-like value nowhere, and hides it with all that follows it (toEnd): a driver
// reads ?password=P&Q as the password P and a parameter Q, but Q may be the rest of the
// password the user meant, and neither the driver nor Redact can tell.
const (
	inQuery = "&"
	toEnd   = ""
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
// malformed URL; that can hide more than the password, never less. If that '@' follows the
// '=' of a password parameter, as in host:1433?password=p@ss, all that follows the ':' is
// hidden with it. The last '@' is that of the whole string, unless the URL is in double
// quotes (Go's %q, as cobra and net/url quote one in an error) or backticks (pgx), which
// then bound it; quoted text is read the same way, URLs anywhere in it included. A URL is
// found wherever "://" follows a scheme, even one glued to the text before it, as in
// cobra's error for a URL glued to a flag: unknown flag: --sourcepostgres://user:***@host/db.
// A DSN without a scheme, like go-sql-driver's user:pass@tcp(host:3306)/db, is hidden the
// same way, in quotes or as the whole of s, quotes in its password included. The value of a
// password-like parameter (see secretValues) is hidden wherever it is, before any quote in
// it is read, and so is all that follows it, to the end of s or of the quotes around it
// (see toEnd): postgres://host/db?sslmode=disable&password=***.
//
// A driver that reads part of what Redact hides as the host or the database, not the
// password, quotes that part in its errors: CheckPassword refuses such URLs.
func Redact(s string) string {
	if schemeLen(s) == 0 && userinfo(s, false) > 0 && dsnAddr.MatchString(s[strings.LastIndexByte(s, '@')+1:]) {
		return redactOne(s, toEnd)
	}
	var b strings.Builder
	quoted := false
	text, key := 0, 0 // where the text not yet written, and the name before the next '=', start
	flush := func(end int) {
		b.WriteString(redactOne(s[text:end], toEnd))
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
				if n := valueLen(s[v:], toEnd); n > 0 {
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
			b.WriteString(redactOne(s[i:], toEnd))
			return b.String()
		}
		i++
	}
	if !quoted {
		return redactOne(s, toEnd)
	}
	flush(len(s))
	return b.String()
}

// RedactToSeparator hides the password in connStr, a single connection URL, like Redact, but
// ends a password parameter's value at the next '&', the way a driver reads it, keeping the
// parameters after it instead of hiding them. connectOnce re-parses the result: a parameter
// the driver rejects still reports what is wrong with it, while the password is gone.
func RedactToSeparator(connStr string) string {
	return redactOne(connStr, inQuery)
}

// redactOne hides the secrets of one connection string: the password between the first
// ':' after the scheme and the last '@' (see passwordEnd), and the value of a password-like
// parameter with all that follows it.
func redactOne(t, ends string) string {
	n := schemeLen(t)
	head, rest := t[:n], t[n:]
	if colon := userinfo(rest, n > 0); colon > 0 {
		rest = rest[:colon] + mask + rest[passwordEnd(rest, strings.LastIndexByte(rest, '@'), ends):]
	} else if start, end := portPassword(rest); n > 0 && start > 0 {
		rest = rest[:start] + mask + rest[end:]
	}
	return head + redactParams(rest, ends)
}

// passwordEnd returns where the password that ends at rest[at], the last '@', is hidden to:
// at, unless that '@' is in the value of a password-like parameter in the query, as in
// host:1433?password=p@ss, where it starts no host; ends says what ends that value (see
// valueLen). Then all of rest is hidden: whether the '@' ends a password with a '?' in it or
// is part of the value, both are secret, and what follows the value is no host to show.
func passwordEnd(rest string, at int, ends string) int {
	q := strings.IndexByte(rest, '?')
	if q < 0 || q > at {
		return at
	}
	for _, v := range secretValues(rest[q:], ends) {
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
		end = passwordEnd(rest, at, inQuery)
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
		// go-sql-driver has no password parameter: it sends an unknown one, password among
		// them, to the server as a SET, where it never logs anyone in and, for an account with
		// no password, the server's syntax error quotes its value (and with a quoted value it
		// would run SET password on the logged-in account). So refuse one outright. Scan from
		// the first '?', not after the last '@', which an '@' in a query value moves past; a
		// '?' inside a userinfo password only reaches a value whose '@' then makes the URL
		// ambiguous and refused below anyway, so this never wrongly refuses a connectable URL.
		if q := strings.IndexByte(rest, '?'); q >= 0 && len(secretValues(rest[q:], inQuery)) > 0 {
			// No URL in the message: embedding one with tcp(host)/db would trip the scheme-less
			// DSN heuristic when main redacts the error again. The user knows their own URL.
			return fmt.Errorf("a MySQL password must be in the user info of the URL, not a query parameter; a password query parameter is sent to the server as a SET and never logs in")
		}
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

// tokenSeps separate the names and values of a query or settings string: '&'
// between parameters, ';' and ',' in settings and host lists, '=' before a
// value, and the whitespace a '%'-escape can decode to. A driver or server
// quotes a token bounded by these, so what it hides is compared token by token.
const tokenSeps = "&=;, \t\r\n"

// isTokenSep reports whether r is one of tokenSeps.
func isTokenSep(r rune) bool {
	return r < 128 && strings.IndexByte(tokenSeps, byte(r)) >= 0
}

// tokens lowercases s and splits it on tokenSeps, dropping empty fields.
func tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), isTokenSep)
}

// hiddenRegion returns the tokens of what Redact hides after the '=' of the
// first password parameter in connStr's query: each raw piece between
// tokenSeps, and what it decodes to both ways (url.PathUnescape, as pgx and
// net/url read it, and url.QueryUnescape, as go-sql-driver and go-mssqldb do,
// where '+' is a space), all split into tokens. Each piece is decoded on its
// own and before it is lowercased, as the drivers decode each parameter's raw
// bytes: a bad escape in one (a password of 50%off) leaves the others decoded,
// and a raw byte still joins the bytes an escape beside it decodes to (%C3 and
// a raw 0xA9 are é). ok is false when the query holds no password parameter.
func hiddenRegion(connStr string) (region map[string]struct{}, ok bool) {
	n := schemeLen(connStr)
	q := strings.IndexByte(connStr[n:], '?')
	if n == 0 || q < 0 {
		return nil, false
	}
	secrets := secretValues(connStr[n+q:], toEnd)
	if len(secrets) == 0 {
		return nil, false
	}
	region = map[string]struct{}{}
	for _, p := range strings.FieldsFunc(connStr[n+q+secrets[0][0]:], isTokenSep) {
		forms := []string{p}
		if d, err := url.PathUnescape(p); err == nil {
			forms = append(forms, d)
		}
		if d, err := url.QueryUnescape(p); err == nil {
			forms = append(forms, d)
		}
		for _, f := range forms {
			for _, t := range tokens(f) {
				region[t] = struct{}{}
			}
		}
	}
	return region, true
}

// HasPasswordParam reports whether connStr's query holds a password parameter, whose value a
// driver ends at the next '&' while Redact hides all that follows its '='.
func HasPasswordParam(connStr string) bool {
	_, ok := hiddenRegion(connStr)
	return ok
}

// HideParams returns err, the error for connecting with connStr, unless it quotes a token of
// what Redact hides after the '=' of the first password parameter in connStr's query. A
// driver reads password=pass&word=x as the password pass and a parameter word, and can quote
// that parameter, or the server it sends it to, when it refuses it (unrecognized
// configuration parameter "word"), but word may be the rest of the password. The server or
// driver may pad a name with the spaces, tabs or control bytes a '%'-escape decoded to
// (parameter "word "), hold a setting name inside an options value, or turn a '+' into a
// space, so each token of each quoted string in the error is tested for equality, at any
// length, against a token of the region (raw and decoded both ways), not matched parameter by
// parameter. go-mssqldb names a duplicated key without quotes (key word provided more than
// once), so the tokens between "key " and " provided more than once" are tested too. Matching
// whole tokens, not substrings, keeps an ordinary error that merely contains the user or
// database name: password authentication failed, and database "db" does not exist, pass
// through. Then HideParams returns an error that names connStr redacted.
func HideParams(connStr string, err error) error {
	if err == nil {
		return err
	}
	region, ok := hiddenRegion(connStr)
	if !ok {
		return err
	}
	inRegion := func(s string) bool {
		for _, t := range tokens(s) {
			if _, ok := region[t]; ok {
				return true
			}
		}
		return false
	}
	hidden := false
	for _, tok := range quotedTokens(err.Error()) {
		hidden = hidden || inRegion(tok)
	}
	if _, after, ok := strings.Cut(err.Error(), "key "); ok {
		if name, _, ok := strings.Cut(after, " provided more than once"); ok {
			hidden = hidden || inRegion(name)
		}
	}
	// A quote inside a token breaks the pairing above (parameter "a"b" yields a), so a token
	// that holds one is searched for whole.
	msg := strings.ToLower(err.Error())
	for t := range region {
		hidden = hidden || strings.ContainsAny(t, `"'`) && hasWord(msg, t)
	}
	if hidden {
		return fmt.Errorf("connecting with %q failed with an error that quotes part of what follows the '=' of its password parameter, which may be part of the password, so the error is left out: put the password after the user name, or write an '&' in it as %%26", Redact(connStr))
	}
	return err
}

// hasWord reports whether the token t occurs whole in the lowercase message s: bounded on each
// side by an end, whitespace or a separator, or the punctuation messages wrap a value in
// (quotes, parentheses, a trailing colon). t may hold that punctuation itself (parameter
// "a"b"), so it is searched for as it is, not split up.
func hasWord(s, t string) bool {
	if t == "" {
		return false
	}
	edge := func(i int) bool {
		return i < 0 || i >= len(s) || strings.IndexByte(tokenSeps+":()[]{}\"'`", s[i]) >= 0
	}
	for from := 0; ; {
		i := strings.Index(s[from:], t)
		if i < 0 {
			return false
		}
		i += from
		if edge(i-1) && edge(i+len(t)) {
			return true
		}
		from = i + 1
	}
}

// quotedTokens returns the text inside each "..." and '...' pair in s, left to right within
// each quote kind. An unpaired final quote starts no token: the loop stops before the last
// field, which Split puts whatever follows the final quote into.
func quotedTokens(s string) []string {
	var toks []string
	for _, q := range []string{`"`, "'"} {
		parts := strings.Split(s, q)
		for i := 1; i < len(parts)-1; i += 2 {
			toks = append(toks, parts[i])
		}
	}
	return toks
}

// redactParams hides the value of each password-like parameter in t (see secretValues),
// each ending at a byte of ends: with toEnd the first value runs to the end of t, so all that
// follows a password parameter is hidden; with inQuery each value ends at the next '&'.
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
// ends, in a URL's query (?password=x&...) or in settings (user id=sa;password=x), a value
// ending at a byte of ends (see valueLen): one whose name, read percent-decoded and in any
// case, ends in password, passwd or pwd (password, sslpassword, Pwd, pass%77ord).
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
