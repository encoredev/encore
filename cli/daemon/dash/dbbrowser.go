package dash

import (
	"context"

	"encr.dev/cli/daemon/sqldb"
	"encr.dev/pkg/fns"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// QueryRequest represents the request body for the /query endpoint
type QueryRequest struct {
	Query     string `json:"query"`
	Params    []any  `json:"params"`
	ArrayMode bool   `json:"arrayMode"`
	DbID      string `json:"dbId"`
	AppID     string `json:"appId"`
}

// TransactionRequest represents the request body for the /transaction endpoint
type TransactionRequest struct {
	Queries []struct {
		SQL    string `json:"sql"`
		Params []any  `json:"params"`
	} `json:"queries"`
	DbID  string `json:"dbId"`
	AppID string `json:"appId"`
}

func (h *handler) Query(ctx context.Context, req QueryRequest) ([]any, error) {

	pgConn, err := h.browserConn(ctx, req.AppID, req.DbID)
	if err != nil {
		return nil, err
	}

	defer fns.CloseIgnoreCtx(ctx, pgConn.Close)

	rows, err := pgConn.Query(context.Background(), req.Query, req.Params...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []any{}
	if req.ArrayMode {
		// Return results as arrays
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				return nil, err
			}
			results = append(results, values)
		}
	} else {
		// Return results as objects
		fieldDescriptions := rows.FieldDescriptions()
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				return nil, err
			}

			row := make(map[string]any)
			for i, value := range values {
				row[fieldDescriptions[i].Name] = value
			}
			results = append(results, row)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// handleTransaction handles the /transaction endpoint
func (h *handler) Transaction(ctx context.Context, req TransactionRequest) ([]any, error) {
	// Start a transaction
	conn, err := h.browserConn(ctx, req.AppID, req.DbID)
	if err != nil {
		return nil, err
	}
	defer fns.CloseIgnoreCtx(ctx, conn.Close)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	results := []any{}
	for _, query := range req.Queries {
		rows, err := tx.Query(context.Background(), query.SQL, query.Params...)
		if err != nil {
			return nil, err
		}

		var queryResults []map[string]any
		fieldDescriptions := rows.FieldDescriptions()
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				rows.Close()
				return nil, err
			}

			row := make(map[string]any)
			for i, value := range values {
				row[fieldDescriptions[i].Name] = value
			}
			queryResults = append(queryResults, row)
		}
		rows.Close()

		if err := rows.Err(); err != nil {
			return nil, err
		}

		results = append(results, queryResults)
	}

	// Commit the transaction
	if err := tx.Commit(context.Background()); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *handler) browserConn(ctx context.Context, appID string, dbID string) (*pgx.Conn, error) {
	// Find the latest app by platform ID or local ID.
	app, err := s.apps.FindLatestByPlatformOrLocalID(appID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find latest app")
	}

	namespace, err := s.GetNamespace(ctx, appID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get namespace")
	}

	clusterType := sqldb.Run
	cluster := s.run.ClusterMgr.Create(ctx, &sqldb.CreateParams{
		ClusterID: sqldb.GetClusterID(app, clusterType, namespace),
		Memfs:     false,
	})
	appMeta, err := s.GetMeta(appID)
	if err != nil {
		return nil, err
	}

	if _, err = cluster.Start(ctx, nil); err != nil {
		return nil, errors.Wrap(err, "failed to start database cluster")
	}
	db, ok := cluster.GetDB(dbID)
	if !ok {
		if err := cluster.Setup(ctx, app.Root(), appMeta); err != nil {
			return nil, errors.Wrap(err, "failed to setup database cluster")
		}
		db, ok = cluster.GetDB(dbID)
		if !ok {
			return nil, errors.Newf("failed to get database %s", dbID)
		}
	}

	info, err := db.Cluster.Info(ctx)
	if err != nil {
		return nil, err
	}
	uri := info.ConnURI(db.EncoreName, info.Config.Superuser)
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		return nil, err
	}
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "char",
		OID:   18,
		Codec: pgtype.TextCodec{},
	})
	return conn, nil
}
