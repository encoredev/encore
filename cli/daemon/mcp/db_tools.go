package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/mark3labs/mcp-go/mcp"

	"encr.dev/cli/daemon/sqldb"
	"encr.dev/pkg/fns"
)

func (m *Manager) registerDatabaseTools() {
	// Add tool for getting all databases and optionally their tables
	m.server.AddTool(mcp.NewTool("get_databases",
		mcp.WithDescription("Retrieve metadata about all SQL databases defined in the currently open Encore, including their schema, tables, and relationships. This tool helps understand the database structure and can optionally include detailed table information."),
		mcp.WithBoolean("include_tables", mcp.Description("When true, includes detailed information about each table in the database, including column names, types, and constraints. This is useful for understanding the complete database schema.")),
		mcp.WithArray("databases",
			mcp.Items(map[string]any{
				"type":        "string",
				"description": "Optional list of specific database names to retrieve information for. If not provided, returns information for all databases in the currently open Encore.",
			})),
	), m.getDatabases)

	// Add tool for querying a database
	m.server.AddTool(mcp.NewTool("query_database",
		mcp.WithDescription("Execute SQL queries against one or more databases in the currently open Encore. This tool allows running custom SQL queries to inspect or manipulate data while respecting the application's database access patterns."),
		mcp.WithArray("queries",
			mcp.Items(map[string]any{
				"type":        "object",
				"description": "Array of query objects, where each object must contain 'database' (the database name to query) and 'query' (the SQL query to execute) fields. Multiple queries can be executed in a single call.",
				"properties": map[string]any{
					"database": map[string]any{
						"type":        "string",
						"description": "The database name to query",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "The SQL query to execute",
					},
				},
				"required": []string{"database", "query"},
			})),
	), m.runQuery)
}

func (m *Manager) getDatabases(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	includeTables := false
	if includeTablesParam, ok := request.Params.Arguments["include_tables"]; ok {
		includeTables, _ = includeTablesParam.(bool)
	}

	// Parse databases parameter if provided
	var filterDBs map[string]bool
	if dbsParam, ok := request.Params.Arguments["databases"]; ok && dbsParam != nil {
		dbsArray, ok := dbsParam.([]interface{})
		if ok && len(dbsArray) > 0 {
			filterDBs = make(map[string]bool)
			for _, db := range dbsArray {
				if dbName, ok := db.(string); ok {
					filterDBs[dbName] = true
				}
			}
		}
	}

	// Build database list
	databases := make([]map[string]interface{}, 0)
	for _, db := range md.SqlDatabases {
		// Skip if we have a filter and this database isn't in it
		if filterDBs != nil && !filterDBs[db.Name] {
			continue
		}

		dbInfo := map[string]interface{}{
			"name": db.Name,
			"doc":  db.Doc,
		}

		// If we should include tables, get table information
		if includeTables {
			tables, err := m.getTablesForDatabase(ctx, db.Name)
			if err != nil {
				// Don't fail the whole request if one database fails
				dbInfo["tables_error"] = err.Error()
			} else {
				dbInfo["tables"] = tables
			}
		}

		databases = append(databases, dbInfo)
	}

	jsonData, err := json.Marshal(databases)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal database list: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (m *Manager) getTablesForDatabase(ctx context.Context, dbName string) ([]map[string]interface{}, error) {
	var tables []map[string]interface{}

	err := m.withConn(ctx, dbName, func(db *sql.DB) error {
		// Query to get tables and their columns from PostgreSQL
		query := `
			SELECT 
				t.table_name,
				ARRAY_AGG(c.column_name ORDER BY c.ordinal_position) as columns,
				ARRAY_AGG(c.data_type ORDER BY c.ordinal_position) as column_types
			FROM 
				information_schema.tables t
			JOIN 
				information_schema.columns c ON t.table_name = c.table_name AND t.table_schema = c.table_schema
			WHERE 
				t.table_schema = 'public'
			GROUP BY 
				t.table_name
			ORDER BY 
				t.table_name;
		`

		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to query tables: %w", err)
		}
		defer rows.Close()

		tables = []map[string]interface{}{}

		for rows.Next() {
			var tableName string
			var columns pq.StringArray
			var columnTypes pq.StringArray

			if err := rows.Scan(&tableName, &columns, &columnTypes); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}

			// Create structured column information
			columnInfo := make([]map[string]string, len(columns))
			for i := range columns {
				columnInfo[i] = map[string]string{
					"name": columns[i],
					"type": columnTypes[i],
				}
			}

			tables = append(tables, map[string]interface{}{
				"table_name": tableName,
				"columns":    columnInfo,
			})
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating rows: %w", err)
		}

		return nil
	})

	return tables, err
}

func (m *Manager) withConn(ctx context.Context, dbName string, fn func(db *sql.DB) error) error {
	app, err := m.getApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	clusterNS, err := m.ns.GetActive(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to get active namespace: %w", err)
	}
	md, err := app.CachedMetadata()
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	clusterID := sqldb.GetClusterID(app, sqldb.Run, clusterNS)
	cluster := m.cluster.Create(ctx, &sqldb.CreateParams{
		ClusterID: clusterID,
		Memfs:     sqldb.Run.Memfs(),
	})
	if _, err := cluster.Start(ctx, nil); err != nil {
		return err
	} else if err := cluster.Setup(ctx, app.Root(), md); err != nil {
		return err
	}

	info, err := cluster.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster info: %w", err)
	} else if info.Status != sqldb.Running {
		return errors.New("cluster not running")
	}

	admin, ok := info.Encore.First(sqldb.RoleRead)
	if !ok {
		return errors.New("unable to find superuser or admin roles")
	}

	uri := info.ConnURI(dbName, admin)

	pool, err := sql.Open("pgx", uri)
	if err != nil {
		return err
	}
	defer fns.CloseIgnore(pool)

	return fn(pool)
}

func (m *Manager) runQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queriesParam, ok := request.Params.Arguments["queries"].([]interface{})
	if !ok || len(queriesParam) == 0 {
		return nil, fmt.Errorf("missing or invalid 'queries' parameter")
	}

	results := make(map[string][]map[string]interface{})

	for _, queryObj := range queriesParam {
		queryMap, ok := queryObj.(map[string]interface{})
		if !ok {
			continue
		}

		dbName, ok := queryMap["database"].(string)
		if !ok || dbName == "" {
			continue
		}

		sqlQuery, ok := queryMap["query"].(string)
		if !ok || sqlQuery == "" {
			continue
		}

		// Execute the query for this database
		var queryResults []map[string]interface{}
		err := m.withConn(ctx, dbName, func(db *sql.DB) error {
			rows, err := db.QueryContext(ctx, sqlQuery)
			if err != nil {
				return fmt.Errorf("failed to execute query: %w", err)
			}
			defer rows.Close()

			// Serialize rows to JSON
			columns, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("failed to get columns: %w", err)
			}

			queryResults = make([]map[string]interface{}, 0)
			for rows.Next() {
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range values {
					valuePtrs[i] = &values[i]
				}

				if err := rows.Scan(valuePtrs...); err != nil {
					return fmt.Errorf("failed to scan row: %w", err)
				}

				row := make(map[string]interface{})
				for i, col := range columns {
					row[col] = values[i]
				}
				queryResults = append(queryResults, row)
			}

			if err := rows.Err(); err != nil {
				return fmt.Errorf("error iterating rows: %w", err)
			}

			return nil
		})

		// Store results for this query
		key := fmt.Sprintf("%s: %s", dbName, sqlQuery)
		if err != nil {
			results[key] = []map[string]interface{}{
				{"error": err.Error()},
			}
		} else {
			results[key] = queryResults
		}
	}

	jsonData, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
