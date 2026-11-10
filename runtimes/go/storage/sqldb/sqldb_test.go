package sqldb

import (
	"os"
	"strings"
	"testing"
	"time"
	_ "unsafe" // for go:linkname

	"github.com/jackc/pgx/v5"

	"encore.dev/appruntime/exported/config"
)

func TestDBConf(t *testing.T) {
	// Unset all the "PG*" variables that may affect the test.
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(key, "PG") {
			_ = os.Unsetenv(key)
		}
	}

	tests := []struct {
		Srv      *config.SQLServer
		DB       *config.SQLDatabase
		Host     string
		Port     uint16
		MaxConns uint32
	}{
		{
			Srv: &config.SQLServer{
				Host: "/cloudsql/foo",
			},
			DB: &config.SQLDatabase{
				EncoreName:     "ignore",
				DatabaseName:   "dbname",
				User:           "user",
				Password:       "password",
				MaxConnections: 10,
			},
			Host:     "/cloudsql/foo",
			Port:     5432,
			MaxConns: 10,
		},
		{
			Srv: &config.SQLServer{
				Host: "test:123",
			},
			DB: &config.SQLDatabase{
				EncoreName:     "ignore",
				DatabaseName:   "dbname",
				User:           "user",
				Password:       "password",
				MaxConnections: 0,
			},
			Host:     "test",
			Port:     123,
			MaxConns: 30,
		},
		{
			Srv: &config.SQLServer{
				Host: "hostname",
			},
			DB: &config.SQLDatabase{
				EncoreName:     "ignore",
				DatabaseName:   "dbname",
				User:           "user",
				Password:       "password",
				MaxConnections: 100,
			},
			Host:     "hostname",
			Port:     5432,
			MaxConns: 100,
		},
	}

	for i, test := range tests {
		cfg, err := dbConf(test.Srv, test.DB, "")
		if err != nil {
			t.Fatalf("test %d: unexpected error: %v", i, err)
		}

		if cfg.ConnConfig.Host != test.Host {
			t.Fatalf("test %d: got host %s, want %q", i, cfg.ConnConfig.Host, test.Host)
		} else if cfg.ConnConfig.Port != test.Port {
			t.Fatalf("test %d: got port %d, want %d", i, cfg.ConnConfig.Port, test.Port)
		} else if cfg.ConnConfig.Database != test.DB.DatabaseName {
			t.Fatalf("test %d: got db %s, want %q", i, cfg.ConnConfig.Database, test.DB.DatabaseName)
		} else if cfg.ConnConfig.User != test.DB.User {
			t.Fatalf("test %d: got user %s, want %q", i, cfg.ConnConfig.User, test.DB.User)
		} else if cfg.ConnConfig.Password != test.DB.Password {
			t.Fatalf("test %d: got password %s, want %q", i, cfg.ConnConfig.Password, test.DB.Password)
		} else if cfg.MaxConnLifetime != time.Hour {
			t.Fatalf("test %d: got max conn lifetime %v, want %v", i, cfg.MaxConnLifetime, time.Hour)
		} else if cfg.MaxConnLifetimeJitter != 10*time.Minute {
			t.Fatalf("test %d: got max conn lifetime jitter %v, want %v", i, cfg.MaxConnLifetimeJitter, 10*time.Minute)
		}
	}
}

// Apps open connections of their own from the pool config's ConnString, so it
// has to carry the password, quoted so that any password survives the trip.
func TestDBConfConnStringCarriesPassword(t *testing.T) {
	unsetPGEnv(t)

	passwords := []string{
		"password",
		`with space`,
		`it's`,
		`back\slash`,
		`'quoted'`,
		`a=b dbname=other`,
		`!#$^&*()-_=+[]{}|;:,.<>?/` + "`~",
	}
	for _, pw := range passwords {
		cfg, err := dbConf(&config.SQLServer{Host: "test:123"}, &config.SQLDatabase{
			DatabaseName: "dbname",
			User:         "user",
			Password:     pw,
		}, "")
		if err != nil {
			t.Fatalf("password %q: unexpected error: %v", pw, err)
		}
		if cfg.ConnConfig.Password != pw {
			t.Errorf("password %q: config has password %q", pw, cfg.ConnConfig.Password)
		}

		reparsed, err := pgx.ParseConfig(cfg.ConnString())
		if err != nil {
			t.Fatalf("password %q: parse ConnString: %v", pw, err)
		}
		if reparsed.Password != pw {
			t.Errorf("password %q: ConnString has password %q", pw, reparsed.Password)
		}
		if reparsed.Database != "dbname" {
			t.Errorf("password %q: ConnString has database %q", pw, reparsed.Database)
		}
	}
}

// An empty password is left out rather than written as "password= dbname=x",
// which would be parsed as the password "dbname=x".
func TestDBConfEmptyPassword(t *testing.T) {
	unsetPGEnv(t)

	cfg, err := dbConf(&config.SQLServer{Host: "test:123"}, &config.SQLDatabase{
		DatabaseName: "dbname",
		User:         "user",
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConnConfig.Password != "" {
		t.Errorf("got password %q, want none", cfg.ConnConfig.Password)
	}
	if cfg.ConnConfig.Database != "dbname" {
		t.Errorf("got database %q, want %q", cfg.ConnConfig.Database, "dbname")
	}
}

// unsetPGEnv unsets the "PG*" variables that may affect a test for its duration.
func unsetPGEnv(t *testing.T) {
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(key, "PG") {
			t.Setenv(key, "")
			_ = os.Unsetenv(key)
		}
	}
}
