package sqldb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

// WaitForConn waits for a successful connection to uri to be established.
func WaitForConn(ctx context.Context, uri string) error {
	var err error
	for i := 0; i < 40; i++ {
		var conn *pgx.Conn
		conn, err = pgx.Connect(ctx, uri)
		if err == nil {
			err = conn.Ping(ctx)
			_ = conn.Close(ctx)
			if err == nil {
				return nil
			}
		} else if ctx.Err() != nil {
			// We'll never succeed once the context has been canceled.
			// Give up straight away.
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("database did not come up: %v", err)
}

// IsUsed reports whether the application uses SQL databases at all.
func IsUsed(md *meta.Data) bool {
	for _, svc := range md.Svcs {
		if len(svc.Migrations) > 0 {
			return true
		}
	}
	return false
}
