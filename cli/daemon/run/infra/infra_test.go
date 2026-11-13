package infra

import (
	"net"
	"strconv"
	"testing"

	"encore.dev/appruntime/exported/config"
	"encr.dev/cli/daemon/sqldb"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// The database proxy listens on 127.0.0.1 (listenTCPRetry's default in the
// daemon). Both runtime configs must advertise that address. "localhost"
// resolves to ::1 first on Windows, and the refused IPv6 attempt adds about
// two seconds to every new SQL connection.
func TestLocalSQLProxyHostIsIPv4Loopback(t *testing.T) {
	const port = 9500
	rm := &ResourceManager{
		dbProxyPort: port,
		servers: map[Type]Resource{
			SQLDB: &sqldb.Cluster{Password: "test-password"},
		},
	}
	md := &meta.Data{
		SqlDatabases: []*meta.SQLDatabase{{Name: "mydb"}},
	}

	cfg := &config.Runtime{}
	if err := rm.UpdateConfig(cfg, md, port); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if len(cfg.SQLServers) != 1 {
		t.Fatalf("UpdateConfig SQLServers = %d, want 1", len(cfg.SQLServers))
	}
	assertIPv4LoopbackHost(t, "UpdateConfig", cfg.SQLServers[0].Host, port)
	if got := cfg.SQLDatabases[0].Password; got != "test-password" {
		t.Fatalf("UpdateConfig password = %q, want the cluster password", got)
	}

	srv, err := rm.SQLServerConfig()
	if err != nil {
		t.Fatalf("SQLServerConfig: %v", err)
	}
	assertIPv4LoopbackHost(t, "SQLServerConfig", srv.Host, port)
}

func assertIPv4LoopbackHost(t *testing.T, name, hostport string, port int) {
	t.Helper()
	host, gotPort, err := net.SplitHostPort(hostport)
	if err != nil {
		t.Fatalf("%s host %q: %v", name, hostport, err)
	}
	if host != "127.0.0.1" {
		t.Errorf("%s host = %q, want 127.0.0.1", name, host)
	}
	if gotPort != strconv.Itoa(port) {
		t.Errorf("%s port = %s, want %d", name, gotPort, port)
	}
}
