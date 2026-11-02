package sqldb

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
)

type fakeConn struct{ driver.Conn }

type fakeConnector struct{}

func (fakeConnector) Connect(context.Context) (driver.Conn, error) { return fakeConn{}, nil }
func (fakeConnector) Driver() driver.Driver                        { return nil }

func TestWrappedConnExpiry(t *testing.T) {
	past, future := time.Now().Add(-time.Second), time.Now().Add(time.Hour)
	tests := []struct {
		Name      string
		Conn      wrappedConn
		WantValid bool
	}{
		{Name: "no_lifetime", Conn: wrappedConn{parent: fakeConn{}}, WantValid: true},
		{Name: "not_expired", Conn: wrappedConn{parent: fakeConn{}, expiresAt: future}, WantValid: true},
		{Name: "expired", Conn: wrappedConn{parent: fakeConn{}, expiresAt: past}, WantValid: false},
		// sql.Open("encore", ...) wraps the connection twice; the outer one must pass the answer on.
		{Name: "expired_inner", Conn: wrappedConn{parent: wrappedConn{parent: fakeConn{}, expiresAt: past}}, WantValid: false},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if got := test.Conn.IsValid(); got != test.WantValid {
				t.Errorf("IsValid() = %v, want %v", got, test.WantValid)
			}
			err := test.Conn.ResetSession(context.Background())
			if test.WantValid && err != nil {
				t.Errorf("ResetSession() = %v, want nil", err)
			} else if !test.WantValid && !errors.Is(err, driver.ErrBadConn) {
				t.Errorf("ResetSession() = %v, want driver.ErrBadConn", err)
			}
		})
	}
}

func TestWrappedConnectorSetsExpiry(t *testing.T) {
	c := wrappedConnector{
		parent:            fakeConnector{},
		driverRef:         &wrappedDriver{},
		maxLifetime:       20 * time.Second,
		maxLifetimeJitter: 10 * time.Second,
	}
	for i := 0; i < 100; i++ {
		before := time.Now()
		conn, err := c.Connect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		expiresAt := conn.(wrappedConn).expiresAt
		if expiresAt.Before(before.Add(20*time.Second)) || expiresAt.After(time.Now().Add(30*time.Second)) {
			t.Fatalf("expiresAt is %s after connecting, want between 20s and 30s", expiresAt.Sub(before))
		}
	}

	c.maxLifetime = 0
	conn, _ := c.Connect(context.Background())
	if !conn.(wrappedConn).expiresAt.IsZero() {
		t.Errorf("with no lifetime, expiresAt = %v, want zero", conn.(wrappedConn).expiresAt)
	}
}

func TestRegisterStdlibDriverLifetime(t *testing.T) {
	runtime := &config.Runtime{
		SQLServers: []*config.SQLServer{{Host: "localhost:1"}},
		SQLDatabases: []*config.SQLDatabase{
			{EncoreName: "adapter-tuned", DatabaseName: "adapter-tuned", User: "u", Password: "p"},
			{EncoreName: "adapter-plain", DatabaseName: "adapter-plain", User: "u", Password: "p"},
		},
	}
	static := &config.Static{
		SQLDatabases: map[string]*config.StaticSQLDatabase{
			"adapter-tuned": {MaxConnLifetime: 20 * time.Second, MaxConnLifetimeJitter: 10 * time.Second},
		},
	}
	mgr := NewManager(static, runtime, nil, nil, zerolog.Nop())

	tests := []struct {
		DB                       string
		WantLifetime, WantJitter time.Duration
	}{
		{DB: "adapter-tuned", WantLifetime: 20 * time.Second, WantJitter: 10 * time.Second},
		{DB: "adapter-plain"},
	}
	for _, test := range tests {
		db := mgr.GetDB(test.DB)
		t.Cleanup(db.shutdown)
		c, err := adapterDriver{}.OpenConnector(RegisterStdlibDriver(db))
		if err != nil {
			t.Fatal(err)
		}
		wc := c.(wrappedConnector)
		if wc.maxLifetime != test.WantLifetime || wc.maxLifetimeJitter != test.WantJitter {
			t.Errorf("%s: got lifetime %s and jitter %s, want %s and %s",
				test.DB, wc.maxLifetime, wc.maxLifetimeJitter, test.WantLifetime, test.WantJitter)
		}
	}
}
