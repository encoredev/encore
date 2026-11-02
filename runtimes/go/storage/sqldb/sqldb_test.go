package sqldb

import (
	"os"
	"strings"
	"testing"
	"time"
	_ "unsafe" // for go:linkname

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

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
		}
	}
}

func TestPoolLifetimeFromStaticConfig(t *testing.T) {
	runtime := &config.Runtime{
		SQLServers: []*config.SQLServer{{Host: "localhost:1"}},
		SQLDatabases: []*config.SQLDatabase{
			{EncoreName: "tuned", DatabaseName: "tuned", User: "u", Password: "p"},
			{EncoreName: "jitter-only", DatabaseName: "jitter-only", User: "u", Password: "p"},
			{EncoreName: "plain", DatabaseName: "plain", User: "u", Password: "p"},
		},
	}
	static := &config.Static{
		SQLDatabases: map[string]*config.StaticSQLDatabase{
			"tuned":       {MaxConnLifetime: 30 * time.Minute, MaxConnLifetimeJitter: 10 * time.Minute},
			"jitter-only": {MaxConnLifetimeJitter: 5 * time.Minute},
		},
	}
	mgr := NewManager(static, runtime, nil, nil, zerolog.Nop())

	// A test clone, as made by NewTestDatabase, must pick up the original's settings.
	tuned := mgr.GetDB("tuned")
	clone := &Database{name: "tuned_clone", origName: "tuned", mgr: mgr, hooks: tuned.hooks}

	tests := []struct {
		DB           *Database
		WantLifetime time.Duration
		WantJitter   time.Duration
	}{
		{DB: tuned, WantLifetime: 30 * time.Minute, WantJitter: 10 * time.Minute},
		{DB: clone, WantLifetime: 30 * time.Minute, WantJitter: 10 * time.Minute},
		{DB: mgr.GetDB("jitter-only"), WantLifetime: time.Hour, WantJitter: 5 * time.Minute},
		{DB: mgr.GetDB("plain"), WantLifetime: time.Hour, WantJitter: 0},
	}
	for _, test := range tests {
		cfg := Driver[*pgxpool.Pool](test.DB).Config()
		t.Cleanup(test.DB.shutdown)
		if cfg.MaxConnLifetime != test.WantLifetime {
			t.Errorf("%s: got MaxConnLifetime %s, want %s", test.DB.name, cfg.MaxConnLifetime, test.WantLifetime)
		}
		if cfg.MaxConnLifetimeJitter != test.WantJitter {
			t.Errorf("%s: got MaxConnLifetimeJitter %s, want %s", test.DB.name, cfg.MaxConnLifetimeJitter, test.WantJitter)
		}
	}
}
