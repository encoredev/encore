package namespace

import (
	"context"
	"database/sql"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/xid"

	"encr.dev/cli/daemon/apps"
	daemonpb "encr.dev/proto/encore/daemon"
)

var (
	ErrNotFound = errors.New("namespace not found")
	ErrActive   = errors.New("namespace is active")
)

type (
	ID   string
	Name string
)

func NewManager(db *sql.DB) *Manager {
	return &Manager{db, nil}
}

// Manager manages namespaces.
type Manager struct {
	db       *sql.DB
	handlers []DeletionHandler
}

func (mgr *Manager) RegisterDeletionHandler(h DeletionHandler) {
	mgr.handlers = append(mgr.handlers, h)
}

type Namespace struct {
	ID           ID
	App          *apps.Instance
	Name         Name
	Active       bool
	CreatedAt    time.Time
	LastActiveAt *time.Time
}

func (m *Manager) Create(ctx context.Context, app *apps.Instance, name Name) (*Namespace, error) {
	now := time.Now()
	id := ID(xid.NewWithTime(now).String())

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // committed explicitly on success

	_, err = tx.ExecContext(ctx, `
		INSERT INTO namespace (id, app_id, name, active, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, app.PlatformOrLocalID(), name, false, now)
	if err != nil {
		return nil, errors.Wrap(err, "create namespace")
	}

	ns := &Namespace{
		ID:        id,
		App:       app,
		Name:      name,
		CreatedAt: now,
	}

	// If there is no active namespace, make this one active.
	{
		var activeName string
		err = tx.QueryRowContext(ctx, `
			SELECT name FROM namespace WHERE app_id = ? AND active = true
		`, app.PlatformOrLocalID()).Scan(&activeName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// No active namespace; make this one active.
				_, err = tx.ExecContext(ctx, `
					UPDATE namespace
					SET active = true, last_active_at = ?
					WHERE id = ?
				`, now, id)
			}
			if err != nil {
				return nil, errors.Wrap(err, "create namespace")
			}
		}
		ns.Active = true
		ns.LastActiveAt = &now
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "create namespace")
	}

	return ns, nil
}

func (m *Manager) List(ctx context.Context, app *apps.Instance) ([]*Namespace, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, active, created_at, last_active_at
		FROM namespace
		WHERE app_id = ?
		ORDER BY name ASC
	`, app.PlatformOrLocalID())
	if err != nil {
		return nil, errors.Wrap(err, "list namespaces")
	}
	defer rows.Close()
	var nss []*Namespace

	for rows.Next() {
		var ns Namespace
		if err := rows.Scan(&ns.ID, &ns.Name, &ns.Active, &ns.CreatedAt, &ns.LastActiveAt); err != nil {
			return nil, errors.Wrap(err, "scan namespace")
		}
		ns.App = app
		nss = append(nss, &ns)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "list namespaces")
	}

	// If we have no namespaces at all, create a default one.
	if len(nss) == 0 {
		ns, err := m.Create(ctx, app, "default")
		if err != nil {
			return nil, err
		}
		nss = []*Namespace{ns}
	}

	return nss, nil
}

func (m *Manager) GetByName(ctx context.Context, app *apps.Instance, name Name) (*Namespace, error) {
	var ns Namespace
	err := m.db.QueryRowContext(ctx, `
		SELECT id, name, active, created_at, last_active_at
		FROM namespace
		WHERE app_id = ? AND name = ?
	`, app.PlatformOrLocalID(), name).Scan(&ns.ID, &ns.Name, &ns.Active, &ns.CreatedAt, &ns.LastActiveAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Wrap(err, "get namespace")
	}
	ns.App = app
	return &ns, nil
}

func (m *Manager) GetByID(ctx context.Context, app *apps.Instance, id ID) (*Namespace, error) {
	var ns Namespace
	err := m.db.QueryRowContext(ctx, `
		SELECT id, name, active, created_at, last_active_at
		FROM namespace
		WHERE app_id = ? AND id = ?
	`, app.PlatformOrLocalID(), id).Scan(&ns.ID, &ns.Name, &ns.Active, &ns.CreatedAt, &ns.LastActiveAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Wrap(err, "get namespace")
	}
	ns.App = app
	return &ns, nil
}

func (m *Manager) Delete(ctx context.Context, app *apps.Instance, name Name) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // committed explicitly on success

	var ns Namespace
	err = tx.QueryRowContext(ctx, `
		DELETE FROM namespace
		WHERE app_id = ? AND name = ?
		RETURNING id, name, active, created_at, last_active_at
	`, app.PlatformOrLocalID(), name).Scan(&ns.ID, &ns.Name, &ns.Active, &ns.CreatedAt, &ns.LastActiveAt)
	if ns.Active {
		return ErrActive
	}

	// Check all the deletion handlers.
	for _, h := range m.handlers {
		if err := h.CanDeleteNamespace(ctx, app, &ns); err != nil {
			return errors.Newf("cannot delete namespace: %v", err)
		}
	}

	// Actually delete the namespace.
	for _, h := range m.handlers {
		if err := h.DeleteNamespace(ctx, app, &ns); err != nil {
			return errors.Newf("failed to delete namespace: %v", err)
		}
	}

	err = tx.Commit()
	return errors.Wrap(err, "delete namespace")
}

func (m *Manager) Switch(ctx context.Context, app *apps.Instance, name Name) (*Namespace, error) {
	// Resolve the namespace to switch to.
	var target *Namespace

	// If the name is "-", switch to the previous namespace.
	if name == "-" {
		nss, err := m.List(ctx, app)
		if err != nil {
			return nil, err
		}

		// Find the non-active namespace that was most recently active
		var lastActive *Namespace
		for _, ns := range nss {
			if !ns.Active && ns.LastActiveAt != nil {
				if lastActive == nil || ns.LastActiveAt.After(*lastActive.LastActiveAt) {
					lastActive = ns
				}
			}
		}

		if lastActive == nil {
			return nil, ErrNotFound
		}
		target = lastActive
	} else {
		var err error
		target, err = m.GetByName(ctx, app, name)
		if err != nil {
			return nil, err
		}
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer tx.Rollback() // committed explicitly on success

	// Mark all namespaces as inactive.
	_, err = tx.ExecContext(ctx, `
		UPDATE namespace SET active = false
		WHERE app_id = ?
	`, app.PlatformOrLocalID())
	if err != nil {
		return nil, errors.Wrap(err, "switch namespace")
	}

	// Mark the selected namespace as active.
	_, err = tx.ExecContext(ctx, `
		UPDATE namespace SET active = true, last_active_at = ?
		WHERE id = ?
	`, time.Now(), target.ID)
	if err != nil {
		return nil, errors.Wrap(err, "switch namespace")
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "switch namespace")
	}

	target.Active = true
	return target, nil
}

// GetActive returns the active namespace for the given app.
func (m *Manager) GetActive(ctx context.Context, app *apps.Instance) (*Namespace, error) {
	var ns Namespace
	err := m.db.QueryRowContext(ctx, `
		SELECT id, name, active, created_at, last_active_at
		FROM namespace
		WHERE app_id = ? AND active = true
	`, app.PlatformOrLocalID()).Scan(&ns.ID, &ns.Name, &ns.Active, &ns.CreatedAt, &ns.LastActiveAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	} else if err == nil {
		return &ns, nil
	}

	// No active namespace.

	// Do we have any namespaces at all?
	nss, err := m.List(ctx, app)
	if err != nil {
		return nil, err
	} else if len(nss) > 0 {
		return m.Switch(ctx, app, nss[0].Name)
	} else {
		// No namespaces. Create a new one.
		return m.Create(ctx, app, "default")
	}
}

func (ns *Namespace) ToProto() *daemonpb.Namespace {
	res := &daemonpb.Namespace{
		Id:        string(ns.ID),
		Name:      string(ns.Name),
		Active:    ns.Active,
		CreatedAt: ns.CreatedAt.String(),
	}
	if ns.LastActiveAt != nil {
		s := ns.LastActiveAt.String()
		res.LastActiveAt = &s
	}
	return res
}

// DeletionHandler is the interface for components that want to listen for
// and handle namespace deletion events.
type DeletionHandler interface {
	// CanDeleteNamespace is called to determine whether the namespace can be deleted
	// by the component. To signal the namespace cannot be deleted, return a non-nil error.
	CanDeleteNamespace(ctx context.Context, app *apps.Instance, ns *Namespace) error

	// DeleteNamespace is called when a namespace is deleted.
	// Due to the non-atomic nature of many components, failure to handle
	// the deletion cannot be fully rolled back.
	DeleteNamespace(ctx context.Context, app *apps.Instance, ns *Namespace) error
}
