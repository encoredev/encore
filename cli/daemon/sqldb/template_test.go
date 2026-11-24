package sqldb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// RecreateFromTemplates copies a test database from its template only while the template
// holds the current migrations, and Recreate always migrates again.
func TestRecreateFromTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a Postgres container")
	}
	host := startTestPostgres(t)
	c := qt.New(t)
	ctx := context.Background()

	root := t.TempDir()
	relPath := "migrations"
	c.Assert(os.Mkdir(filepath.Join(root, relPath), 0o755), qt.IsNil)
	writeMigration := func(body string) {
		c.Assert(os.WriteFile(filepath.Join(root, relPath, "1_create.up.sql"), []byte(body), 0o644), qt.IsNil)
	}
	writeMigration("CREATE TABLE a (id INT);")
	md := &meta.Data{SqlDatabases: []*meta.SQLDatabase{{
		Name:             "db",
		MigrationRelPath: &relPath,
		Migrations:       []*meta.DBMigration{{Filename: "1_create.up.sql", Number: 1}},
	}}}

	app := apps.NewInstance(root, "template-test", "")
	cluster := &Cluster{
		ID:         ClusterID{NS: &namespace.Namespace{ID: "test", App: app, Name: "default"}, Type: Test},
		Memfs:      true,
		Ctx:        ctx,
		cancel:     func() {},
		driver:     fixedHostDriver{host: host},
		started:    make(chan struct{}),
		log:        zerolog.Nop(),
		dbs:        make(map[string]*DB),
		isExternal: func(string) bool { return false },
	}
	_, err := cluster.Start(ctx, nil)
	c.Assert(err, qt.IsNil)

	conn, err := pgx.Connect(ctx, fmt.Sprintf("postgresql://postgres:postgres@%s/postgres", host))
	c.Assert(err, qt.IsNil)
	defer func() { _ = conn.Close(ctx) }()
	oid := func(name string) uint32 {
		var oid uint32
		err := conn.QueryRow(ctx, "SELECT oid FROM pg_database WHERE datname = $1", name).Scan(&oid)
		c.Assert(err, qt.IsNil)
		return oid
	}

	appQuery := func(sql string, dest any) {
		appConn, err := pgx.Connect(ctx, fmt.Sprintf("postgresql://postgres:postgres@%s/db", host))
		c.Assert(err, qt.IsNil)
		defer func() { _ = appConn.Close(ctx) }()
		c.Assert(appConn.QueryRow(ctx, sql).Scan(dest), qt.IsNil)
	}

	c.Assert(cluster.RecreateFromTemplates(ctx, root, md), qt.IsNil)
	built := oid("db_template")
	var inserted int
	appQuery("INSERT INTO a VALUES (1) RETURNING id", &inserted)

	c.Assert(cluster.RecreateFromTemplates(ctx, root, md), qt.IsNil)
	c.Assert(oid("db_template"), qt.Equals, built, qt.Commentf("unchanged migrations rebuilt the template"))

	// The copy starts empty, and the app's roles can use it.
	var rows int
	appQuery("SELECT count(*) FROM a", &rows)
	c.Assert(rows, qt.Equals, 0, qt.Commentf("rows from the previous run survived the copy"))
	write, ok := cluster.Roles.Write()
	c.Assert(ok, qt.IsTrue)
	var canWrite bool
	appQuery(fmt.Sprintf("SELECT has_table_privilege(%s, 'a', 'INSERT')", quoteString(write.Username)), &canWrite)
	c.Assert(canWrite, qt.IsTrue, qt.Commentf("the write role lost its grants on the copy"))

	writeMigration("CREATE TABLE a (id BIGINT);")
	c.Assert(cluster.RecreateFromTemplates(ctx, root, md), qt.IsNil)
	edited := oid("db_template")
	c.Assert(edited, qt.Not(qt.Equals), built, qt.Commentf("an edited migration reused the template"))

	c.Assert(cluster.Recreate(ctx, root, nil, md), qt.IsNil)
	c.Assert(oid("db_template"), qt.Not(qt.Equals), edited, qt.Commentf("Recreate reused the template"))

	// The application database is a copy of the latest template.
	var typ string
	appQuery("SELECT data_type FROM information_schema.columns WHERE table_name = 'a' AND column_name = 'id'", &typ)
	c.Assert(typ, qt.Equals, "bigint")
}

// startTestPostgres starts Encore's Postgres image the way the Docker driver runs test clusters,
// and returns its host:port. The image must match docker.Image, which this package cannot import.
func startTestPostgres(t *testing.T) string {
	t.Helper()
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("Docker is not running")
	}
	out, err := exec.Command("docker", "run", "-d", "--rm", "-p", "127.0.0.1::5432",
		"-e", "POSTGRES_USER=postgres", "-e", "POSTGRES_PASSWORD=postgres", "-e", "POSTGRES_DB=postgres",
		"-e", "PGDATA=/var/lib/postgresql/data", "--mount", "type=tmpfs,destination=/var/lib/postgresql/data",
		"encoredotdev/postgres:18", "-c", "fsync=off").CombinedOutput()
	if err != nil {
		t.Fatalf("docker run: %v: %s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })

	out, err = exec.Command("docker", "port", id, "5432/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("docker port: %v: %s", err, out)
	}
	host := strings.TrimSpace(strings.Split(string(out), "\n")[0])

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := WaitForConn(ctx, fmt.Sprintf("postgresql://postgres:postgres@%s/postgres", host)); err != nil {
		t.Fatalf("wait for Postgres: %v", err)
	}
	return host
}

// fixedHostDriver is a Driver for a Postgres server that is already running.
type fixedHostDriver struct{ host string }

func (d fixedHostDriver) CreateCluster(ctx context.Context, p *CreateParams, _ zerolog.Logger) (*ClusterStatus, error) {
	return d.ClusterStatus(ctx, p.ClusterID)
}

func (d fixedHostDriver) ClusterStatus(context.Context, ClusterID) (*ClusterStatus, error) {
	return &ClusterStatus{Status: Running, Config: &ConnConfig{
		Host:         d.host,
		Superuser:    Role{Type: RoleSuperuser, Username: "postgres", Password: "postgres"},
		RootDatabase: "postgres",
	}}, nil
}

func (fixedHostDriver) CanDestroyCluster(context.Context, ClusterID) error { return ErrUnsupported }
func (fixedHostDriver) DestroyCluster(context.Context, ClusterID) error    { return ErrUnsupported }
func (fixedHostDriver) DestroyNamespaceData(context.Context, *namespace.Namespace) error {
	return ErrUnsupported
}
func (fixedHostDriver) CheckRequirements(context.Context) error { return nil }
func (fixedHostDriver) Meta() DriverMeta                        { return DriverMeta{ClusterIsolation: true} }
