package stdlibdriver

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestConnectCallsBeforeConnectOnACopy(t *testing.T) {
	cfg, err := pgx.ParseConfig("host=127.0.0.1 port=1 user=user dbname=db sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	stop := errors.New("stop before dialing")
	var got *pgx.ConnConfig
	connStr := RegisterConnConfig(cfg, func(ctx context.Context, c *pgx.ConnConfig) error {
		got = c
		c.Password = "token"
		return stop
	})
	t.Cleanup(func() { UnregisterConnConfig(connStr) })

	conn, err := pgxDriver.OpenConnector(connStr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Connect(context.Background()); !errors.Is(err, stop) {
		t.Fatalf("got error %v, want the one BeforeConnect returned", err)
	}
	if got == nil || got == cfg {
		t.Fatalf("BeforeConnect got %p, want a copy of the registered config %p", got, cfg)
	}
	if cfg.Password != "" {
		t.Errorf("the registered config's password changed to %q", cfg.Password)
	}
}
