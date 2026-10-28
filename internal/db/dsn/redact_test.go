package dsn

import (
	"net/url"
	"strings"
	"testing"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRedact(t *testing.T) {
	tests := []struct{ name, in, want string }{
		// URL userinfo, each engine's form
		{"postgres", "postgres://user:s3cret@db.example.com:5432/app?sslmode=disable",
			"postgres://user:***@db.example.com:5432/app?sslmode=disable"},
		{"postgresql scheme", "postgresql://user:s3cret@localhost/app", "postgresql://user:***@localhost/app"},
		{"sqlserver", "sqlserver://sa:s3cret@localhost:1433?database=app&encrypt=disable",
			"sqlserver://sa:***@localhost:1433?database=app&encrypt=disable"},
		{"mysql tcp DSN", "mysql://root:s3cret@tcp(localhost:3306)/app?parseTime=true",
			"mysql://root:***@tcp(localhost:3306)/app?parseTime=true"},
		{"mysql+tcp scheme", "mysql+tcp://root:s3cret@tcp(localhost:3306)/app", "mysql+tcp://root:***@tcp(localhost:3306)/app"},
		{"upper-case scheme", "POSTGRES://USER:S3CRET@HOST/APP", "POSTGRES://USER:***@HOST/APP"},

		// passwords with the characters that end the parts of a URL
		{"password with @ : /", "postgres://user:p@ss:w/rd@host/app", "postgres://user:***@host/app"},
		{"mysql password with @ : /", "mysql://root:p@ss:w/rd@tcp(host:3306)/app", "mysql://root:***@tcp(host:3306)/app"},
		{"percent-escaped password", "postgres://user:p%40ss%3Aw%2Frd%25@host/app", "postgres://user:***@host/app"},
		{"password with ? and #", "sqlserver://sa:a?b#c@host?database=app", "sqlserver://sa:***@host?database=app"},
		{"password with spaces and quotes", "mysql://root:a \"b` c'd@tcp(host)/app", "mysql://root:***@tcp(host)/app"},
		{"user with @, as on Azure", "postgres://admin@server:s3cret@host/app", "postgres://admin@server:***@host/app"},
		{"empty user", "postgres://:s3cret@host/app", "postgres://:***@host/app"},

		// without a scheme: go-sql-driver's DSN, and semicolon-separated settings
		{"DSN without a scheme", "root:s3cret@tcp(localhost:3306)/app", "root:***@tcp(localhost:3306)/app"},
		{"DSN with a space in the password", "root:my s3cret@tcp(host)/app", "root:***@tcp(host)/app"},
		{"DSN with quote pairs in the password", "root:a\"s3\"c`r`et@tcp(host)/app", "root:***@tcp(host)/app"},
		{"settings", "server=host;user id=sa;password=s3cret;database=app", "server=host;user id=sa;password=***;database=app"},
		{"settings with a quoted password", `server=host;pwd="s3c;ret";database=app`, "server=host;pwd=***;database=app"},
		{"settings with quotes in a password", `server=host;password=a"s3"c` + "`r`" + `et;database=app`, "server=host;password=***;database=app"},
		{"settings in text, password quoted", `failed with server=host;pwd="s3cret";database=app here`,
			`failed with server=host;pwd=***;database=app here`},
		{"settings in text, quotes in a braced password", `failed with server=host;pwd={a"s3cret"c};database=app`,
			`failed with server=host;pwd=***;database=app`},
		{"URL inside quoted prose", `error: "connecting to postgres://user:s3cret@host/db failed"`,
			`error: "connecting to postgres://user:***@host/db failed"`},

		// A URL without the '@' and host after its password: what stands in the place of a
		// port is hidden, unless it is a number.
		{"password without @host", "postgres://app:s3cret", "postgres://app:***"},
		{"password without @host, then a path", "postgres://app:s3cret/db?sslmode=disable", "postgres://app:***/db?sslmode=disable"},
		{"sqlserver password without @host", "sqlserver://sa:s3cret?database=app", "sqlserver://sa:***?database=app"},
		{"mysql password without @address", "mysql://root:s3cret", "mysql://root:***"},
		{"IPv6 host and port", "postgres://[::1]:5432/app", "postgres://[::1]:5432/app"},
		{"hosts and ports", "postgres://h1:5432,h2:5433/app", "postgres://h1:5432,h2:5433/app"},
		{"password after a later host", "postgres://h1:5432,app:s3cret/app", "postgres://h1:5432,app:***/app"},
		{"empty port", "postgres://host:/app", "postgres://host:/app"},

		// nothing to hide
		{"no password", "postgres://user@host/app", "postgres://user@host/app"},
		{"empty password", "postgres://user:@host/app", "postgres://user:@host/app"},
		{"no userinfo", "postgres://host:5432/app", "postgres://host:5432/app"},
		{"mysql without userinfo", "mysql://tcp(host:3306)/app", "mysql://tcp(host:3306)/app"},
		{"no connection string", "failed to connect: connection refused", "failed to connect: connection refused"},

		// password-like parameters
		{"password parameter", "postgres://user@host/app?sslmode=disable&password=s3cret",
			"postgres://user@host/app?sslmode=disable&password=***"},
		{"pwd parameter, any case", "sqlserver://host?database=app&PWD=s3cret&encrypt=disable",
			"sqlserver://host?database=app&PWD=***&encrypt=disable"},
		{"sslpassword", "postgres://user:s3cret@host/app?sslpassword=k3y", "postgres://user:***@host/app?sslpassword=***"},
		// In a URL's query, only '&' ends a value, as pgx, net/url and go-sql-driver read it, so
		// settings in a value are hidden from the password on.
		{"settings in a value", "sqlserver://host?database=app&odbc=user id=sa;Password=s3cret;app name=x",
			"sqlserver://host?database=app&odbc=user id=sa;Password=***"},
		{"';' in a password parameter", "postgres://app@host/db?password=s3cret;s3cret&sslmode=disable",
			"postgres://app@host/db?password=***&sslmode=disable"},
		// a password parameter whose value holds a URL runs to the end
		{"URL in a password setting", "password=postgres://u:first;s3cret@host/db", "password=***"},
		{"URL in a password setting, in prose", "failed: pwd=postgres://u:first;s3cret@host/db", "failed:***@host/db"},
		{"spaces around =", "sqlserver://host?user id=sa&password = s3cret", "sqlserver://host?user id=sa&password = ***"},
		{"value with spaces", "sqlserver://host?password=my s3cret&database=app", "sqlserver://host?password=***&database=app"},
		{"value in braces", "sqlserver://host?odbc=uid=sa;pwd={a&b}}c}&database=app",
			"sqlserver://host?odbc=uid=sa;pwd=***&database=app"},
		{"value in quotes", "sqlserver://host?odbc=uid=sa;pwd='a&b''c'&database=app",
			"sqlserver://host?odbc=uid=sa;pwd=***&database=app"},
		{"text after the closing brace", "sqlserver://host?odbc=pwd={a&b}s3cret&database=app",
			"sqlserver://host?odbc=pwd=***&database=app"},
		{"percent-encoded name", "postgres://app@host/db?pass%77ord=s3cret&sslmode=disable",
			"postgres://app@host/db?pass%77ord=***&sslmode=disable"},
		// A value runs to ';' or '&', so here one hides the rest: more than the password, not less.
		{"keyword=value settings", "cannot parse `host=db sslpassword=k3y password=s3cret`", "cannot parse `host=db sslpassword=***`"},
		{"userinfo and parameter", "sqlserver://sa:s3cret@host?database=app&password=s3cret",
			"sqlserver://sa:***@host?database=app&password=***"},
		// With a ':' before it, an '@' in a parameter's value would end a password that starts
		// at the ':': the value is hidden whole with it.
		{"parameter holding an @, after a port", "sqlserver://host:1433?database=app&password=s3cret@s3cret",
			"sqlserver://host:***"},
		{"parameter holding an @, then others", "postgres://host:5432/app?password=s3cret@s3cret&sslmode=disable",
			"postgres://host:***&sslmode=disable"},
		{"parameter holding an @, without a port", "sqlserver://host?database=app&password=s3cret@s3cret",
			"sqlserver://host?database=app&password=***"},

		// text holding connection strings, as errors and logs do
		{"after text", "unsupported scheme in postgress://user:s3cret@host/app", "unsupported scheme in postgress://user:***@host/app"},
		{"cobra quotes an argument", `unknown command "postgres://user:s3cret@host/app" for "dbdiff"`,
			`unknown command "postgres://user:***@host/app" for "dbdiff"`},
		{"%q-escaped quote in the password", `invalid argument "mysql://root:a\"b@tcp(host)/app" for "--timeout" flag`,
			`invalid argument "mysql://root:***@tcp(host)/app" for "--timeout" flag`},
		{"pgx's backticks", "cannot parse `POSTGRES://user:xxxxxx@s3cret@host/app`: failed to parse as keyword/value",
			"cannot parse `POSTGRES://user:***@host/app`: failed to parse as keyword/value"},
		{"quoted DSN without a scheme", `unknown command "root:s3cret@tcp(host:3306)/app" for "dbdiff"`,
			`unknown command "root:***@tcp(host:3306)/app" for "dbdiff"`},
		// pflag quotes a URL glued to a flag whole, after the flag's dashes or letters
		{"URL glued to a shorthand flag", "unknown shorthand flag: 'p' in -postgres://user:s3cret@host/app",
			"unknown shorthand flag: 'p' in -postgres://user:***@host/app"},
		{"URL glued to a long flag", "unknown flag: --sourcepostgres://user:s3cret@host/app",
			"unknown flag: --sourcepostgres://user:***@host/app"},
		{"URL after two dashes", "unknown flag: --mysql://root:s3cret@tcp(host:3306)/app",
			"unknown flag: --mysql://root:***@tcp(host:3306)/app"},
		{"URL after three dashes", "bad flag syntax: ---sqlserver://sa:s3cret@host?database=app",
			"bad flag syntax: ---sqlserver://sa:***@host?database=app"},
		{"URL after a digit", "unknown shorthand flag: '1' in -1postgres://user:s3cret@host/app",
			"unknown shorthand flag: '1' in -1postgres://user:***@host/app"},
		{"quoted prose", `reason: "user root: denied to root@host"`, `reason: "user root: denied to root@host"`},
		{"driver error prose", "Error 1045 (28000): Access denied for user 'root'@'172.17.0.1' (using password: YES)",
			"Error 1045 (28000): Access denied for user 'root'@'172.17.0.1' (using password: YES)"},
	}
	for _, tt := range tests {
		got := Redact(tt.in)
		if got != tt.want {
			t.Errorf("%s:\nRedact(%q)\n got %q\nwant %q", tt.name, tt.in, got, tt.want)
		}
		if again := Redact(got); again != got {
			t.Errorf("%s: redacting again changed %q to %q", tt.name, got, again)
		}
		if strings.Contains(got, "s3cret") || strings.Contains(got, "p@ss") || strings.Contains(got, "k3y") {
			t.Errorf("%s: secret left in %q", tt.name, got)
		}
	}
}

// Any password is hidden whole, whatever it holds: in a URL's userinfo, in a DSN without a
// scheme, in a percent-encoded query parameter, and in braces in settings (go test -fuzz
// FuzzRedactPassword explores beyond these seeds).
func FuzzRedactPassword(f *testing.F) {
	for _, p := range []string{"s3cret", "p@ss:w/rd", "a\"b`c d", "x://y@z", "pwd=a;b", "%zz%40", "{a;b}}", "a\"b\"c", "x`y`z", "a?pwd=b"} {
		f.Add("app", p)
	}
	f.Fuzz(func(t *testing.T, user, pass string) {
		// "//" after "user:" would make the DSN without a scheme a URL with the scheme user.
		if pass == "" || strings.HasPrefix(pass, "//") ||
			strings.Trim(user, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._@-") != "" {
			t.Skip()
		}
		cases := map[string]string{
			"server=host;user id=" + user + ";password={" + strings.ReplaceAll(pass, "}", "}}") + "};database=db": "server=host;user id=" + user + ";password=***;database=db",
			"postgres://" + user + "@host/db?pass%77ord=" + url.QueryEscape(pass) + "&sslmode=disable":            "postgres://" + user + "@host/db?pass%77ord=***&sslmode=disable",
		}
		for _, form := range []string{"postgres://%s@host/db", "mysql://%s@tcp(host:3306)/db", "sqlserver://%s@host?database=db", "%s@tcp(host:3306)/db",
			"unknown flag: --sourcepostgres://%s@host/db", "unknown shorthand flag: 'p' in -postgres://%s@host/db"} {
			cases[strings.Replace(form, "%s", user+":"+pass, 1)] = strings.Replace(form, "%s", user+":***", 1)
		}
		for in, want := range cases {
			// After a '?' in the password, a password parameter's value can hold the '@' that
			// ends it: then all that follows the ':' is hidden (see passwordEnd).
			got := Redact(in)
			hidesAll := strings.Contains(pass, "?") && strings.HasSuffix(got, ":"+mask) && strings.HasPrefix(want, got)
			if got != want && !hidesAll {
				t.Errorf("password %q: Redact(%q) = %q, want %q", pass, in, got, want)
			}
		}
	})
}

// CheckPassword lets through each URL whose driver reads as the password all that Redact
// hides, and refuses, without the password, those whose driver would read part of it as the
// host or the database.
func TestCheckPassword(t *testing.T) {
	for _, tt := range []struct {
		url string
		ok  bool
	}{
		{"postgres://app:s3cret@host:5432/db?sslmode=disable", true},
		{"postgresql://app:s3cret?#:x@host/db", true}, // pgx's password runs to the first '@'
		{"postgres://app:p%40ss%2Fw@host/db", true},
		{"postgres://app@host/db?application_name=a@b", true}, // no password in the userinfo
		{"postgres://host/db?password=s3cret", true},
		{"sqlserver://sa:p@ss:w@host?database=db", true},     // net/url's runs to the host's last '@'
		{"mysql://root:p@ss/w:rd?#@tcp(host:3306)/db", true}, // go-sql-driver's, to the last '@' before the last '/'
		{"MYSQL+TCP://root:s3cret@tcp(host:3306)/db?parseTime=true", true},
		{"mariadb://app:a/s3cret@host/db", true},            // no driver: NewDriver refuses the scheme
		{"postgres://host/db?password=s3cret@s3cret", true}, // pgx: no user name before the '/'
		{"sqlserver://host?database=db&password=s3cret@s3cret", true},

		{"postgres://app:/s3cret@host:5432/db", false}, // pgx: host app, database s3cret@host:5432/db
		{"postgres://app:4321/s3cret@host/db", false},  // pgx: port 4321
		{"postgres://app:p@s3cret@host/db", false},     // pgx: password p, host s3cret@host
		{"postgres://app:s3cret@host/db?x=a@b", false}, // the password Redact hides runs to a@
		{"sqlserver://sa:4321/s3cret@host?database=db", false},
		{"sqlserver://sa:a?s3cret@host?database=db", false},
		{"mysql://root:s3cret/x@tcp(host:3306)", false}, // go-sql-driver: database x@tcp(host:3306)
		{"mysql://root:s3cret@tcp(host:3306)", false},
		{"mysql+tcp://root:s3cret@tcp(host:3306)/db?x=a@b", false},
		// an '@' in a password parameter's value, with a ':' before it, or, for pgx, before any '/'
		{"postgres://host:5432/db?password=s3cret@s3cret", false},
		{"postgres://host?sslmode=disable&password=s3cret@s3cret", false}, // pgx: user host?sslmode=disable&password=s3cret
		{"sqlserver://host:1433?database=db&password=s3cret@s3cret", false},
		{"mysql://root@tcp(host:3306)/db?password=s3cret@s3cret&x=/", false},
	} {
		err := CheckPassword(tt.url)
		if tt.ok != (err == nil) || err != nil && (strings.Contains(err.Error(), "s3cret") || !strings.Contains(err.Error(), "***")) {
			t.Errorf("CheckPassword(%q) = %v, want ok=%v and no password", tt.url, err, tt.ok)
		}
		// main prints errors through Redact, which must leave this one as it is.
		if err != nil && Redact("Error: failed to initialize driver: "+err.Error()) != "Error: failed to initialize driver: "+err.Error() {
			t.Errorf("CheckPassword(%q): Redact changes its error to %q", tt.url, Redact(err.Error()))
		}
	}
}

// A URL CheckPassword lets through is one whose driver, parsing it the way dbdiff's driver
// does, reads as the password all that Redact hides, or fails (go test -fuzz
// FuzzCheckPassword explores beyond these seeds).
func FuzzCheckPassword(f *testing.F) {
	for _, s := range [][3]string{{"app", "s3cret", "host:5432/db?sslmode=disable"}, {"app", "s3cret?#:x", "host/db"},
		{"app", "p%40ss%2Fw", "host/db"}, {"sa", "p@ss:w", "host?database=db"}, {"root", "p@ss/w:rd?#", "tcp(host:3306)/db"},
		{"app", "4321/s3cret", "host/db"}, {"app", "s3cret", "host/db?x=a@b"}, {"a@b", "s3cret", "host"}} {
		f.Add(s[0], s[1], s[2])
	}
	f.Fuzz(func(t *testing.T, user, pass, tail string) {
		for _, scheme := range []string{"postgres://", "sqlserver://", "mysql://"} {
			u := scheme + user + ":" + pass + "@" + tail
			rest := u[len(scheme):]
			if CheckPassword(u) != nil || userinfo(rest, true) == 0 {
				continue
			}
			at := strings.LastIndexByte(rest, '@')
			hidden := rest[strings.IndexByte(rest[:at], ':')+1 : passwordEnd(rest, at)]
			var got, want string
			switch scheme {
			case "postgres://":
				// pgx drops the spaces around a password, then looks for one elsewhere.
				var err error
				if want, err = url.PathUnescape(strings.Trim(hidden, " ")); err != nil || want == "" {
					continue
				}
				cfg, err := pgconn.ParseConfig(u)
				if err != nil {
					continue
				}
				got = cfg.Password
			case "sqlserver://":
				p, err := url.Parse(u)
				if err != nil {
					continue
				}
				if p.User != nil {
					got, _ = p.User.Password()
				}
				want, _ = url.PathUnescape(hidden)
			case "mysql://":
				cfg, err := gomysql.ParseDSN(rest)
				if err != nil {
					continue
				}
				got, want = cfg.Passwd, hidden
			}
			if got != want {
				t.Errorf("%q: the driver reads the password %q, Redact hides %q", u, got, hidden)
			}
		}
	})
}

// Redact takes time linear in its input: a long run of letters, or of quotes that never
// close, made it scan the rest of the input again at each byte.
func TestRedactTakesLinearTime(t *testing.T) {
	for _, s := range []string{strings.Repeat("a", 300000), strings.Repeat(`"\`, 300000), strings.Repeat("`a", 150000)} {
		in := s + " postgres://app:s3cret@host/db"
		start := time.Now()
		got := Redact(in)
		// ponytail: a wall-clock bound, wide enough for -race on a slow runner; the quadratic
		// scans took about 30 s for the first two, without -race.
		if d := time.Since(start); d > 10*time.Second || strings.Contains(got, "s3cret") {
			t.Errorf("Redact of %d bytes took %v, and left the password: %v", len(in), d, strings.Contains(got, "s3cret"))
		}
	}
}
