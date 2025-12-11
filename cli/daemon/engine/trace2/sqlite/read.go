package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/pkg/fns"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

// spanRow is a temporary type used during parent_span_id backfilling.
type spanRow struct {
	summary *tracepb2.SpanSummary
}

func (s *Store) List(ctx context.Context, q *trace2.Query, iter trace2.ListEntryIterator) error {
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}

	args := []any{
		q.AppID, tracepb2.SpanSummary_AUTH, /* ignore auth spans */
	}

	extraWhereClause := ""

	if q.MessageID != "" {
		args = append(args, q.MessageID)
		extraWhereClause += " AND message_id = $" + strconv.Itoa(len(args))
	}

	// If we're filter for tests / not tests, add the extra where clause
	if q.TestFilter != nil {
		args = append(args, tracepb2.SpanSummary_TEST)
		if *q.TestFilter {
			extraWhereClause += " AND span_type = $" + strconv.Itoa(len(args))
		} else {
			extraWhereClause += " AND span_type != $" + strconv.Itoa(len(args))
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
		    trace_id, span_id, started_at, span_type, is_root, service_name, endpoint_name,
		    topic_name, subscription_name, message_id, is_error, test_skipped, duration_nanos, 
			src_file, src_line, parent_span_id
		FROM trace_span_index
		WHERE app_id = $1 AND has_response AND is_root AND span_type != $2 `+extraWhereClause+`
		ORDER BY started_at DESC
		LIMIT `+strconv.Itoa(limit)+`
	`, args...)
	if err != nil {
		return errors.Wrap(err, "query traces")
	}

	defer fns.CloseIgnore(rows)

	// TODO: remove after X days since merging
	/*
		for the traces that were created before we merged the change to write the parent_span_id
	*/
	// First pass: collect all rows
	var spans []spanRow
	for rows.Next() {
		if len(spans) >= limit {
			break
		}

		var t tracepb2.SpanSummary
		var startedAt int64

		err := rows.Scan(
			&t.TraceId, &t.SpanId, &startedAt, &t.Type, &t.IsRoot, &t.ServiceName, &t.EndpointName,
			&t.TopicName, &t.SubscriptionName, &t.MessageId, &t.IsError, &t.TestSkipped,
			&t.DurationNanos, &t.SrcFile, &t.SrcLine, &t.ParentSpanId)
		if err != nil {
			return errors.Wrap(err, "scan trace")
		}

		ts := time.Unix(0, startedAt)
		t.StartedAt = timestamppb.New(ts)

		spans = append(spans, spanRow{
			summary: &t,
		})
	}

	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "iterate traces")
	}

	// Second pass: batch fetch missing parent_span_ids
	if err := s.backfillParentSpanIDs(ctx, q.AppID, spans); err != nil {
		log.Error().Err(err).Msg("failed to backfill parent_span_ids from WAL")
		// Continue anyway - this is best-effort for historical data
	}

	// Third pass: emit to iterator
	for _, span := range spans {
		if !iter(span.summary) {
			return nil
		}
	}

	return nil
}

// emitCompleteSpanToListeners emits the given trace/span to all listeners
// if it's a complete root span (meaning it has a response and is not an auth span).
func (s *Store) emitCompleteSpanToListeners(ctx context.Context, appID, traceID, spanID string) {
	var t tracepb2.SpanSummary
	var startedAt int64

	err := s.db.QueryRowContext(ctx, `
		SELECT
			trace_id, span_id, started_at, span_type, is_root, service_name, endpoint_name,
			topic_name, subscription_name, message_id, is_error, test_skipped, duration_nanos, 
			src_file, src_line, parent_span_id
		FROM trace_span_index
		WHERE app_id = ? AND trace_id = ? AND span_id = ? AND has_response AND is_root AND span_type != ?
		ORDER BY started_at DESC
	`, appID, traceID, spanID, tracepb2.SpanSummary_AUTH).Scan(
		&t.TraceId, &t.SpanId, &startedAt, &t.Type, &t.IsRoot, &t.ServiceName, &t.EndpointName,
		&t.TopicName, &t.SubscriptionName, &t.MessageId, &t.IsError, &t.TestSkipped,
		&t.DurationNanos, &t.SrcFile, &t.SrcLine, &t.ParentSpanId)
	if errors.Is(err, sql.ErrNoRows) {
		return
	} else if err != nil {
		log.Error().Err(err).Msg("unable to query trace span")
		return
	}

	ts := time.Unix(0, startedAt)
	t.StartedAt = timestamppb.New(ts)

	// TODO: remover after X amount of days, since we write the parent_span_id into the table
	// Backfill parent_span_id from WAL if not in index
	if t.ParentSpanId == nil {
		// Fallback to WAL for this single span
		rows, err := s.db.QueryContext(ctx, `
			SELECT event_data FROM trace_event 
			WHERE app_id = ? AND trace_id = ? AND span_id = ? LIMIT 1
		`, appID, traceID, spanID)
		if err == nil {
			defer fns.CloseIgnore(rows)
			if rows.Next() {
				var data []byte
				if rows.Scan(&data) == nil {
					var ev tracepb2.TraceEvent
					if protojson.Unmarshal(data, &ev) == nil {
						if start := ev.GetSpanStart(); start != nil && start.ParentSpanId != nil {
							encoded := encodeSpanID(*start.ParentSpanId)
							t.ParentSpanId = &encoded
						}
					}
				}
			}
		}
	}

	for _, ln := range s.listeners {
		ln <- trace2.NewSpanEvent{
			AppID:     appID,
			TestTrace: t.Type == tracepb2.SpanSummary_TEST,
			Span:      &t,
		}
	}
}

func (s *Store) Get(ctx context.Context, appID, traceID string, iter trace2.EventIterator) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT event_data
		FROM trace_event
		WHERE app_id = ? AND trace_id = ?
	`, appID, traceID)
	if err != nil {
		return errors.Wrap(err, "get trace")
	}

	defer fns.CloseIgnore(rows)
	hasRows := false
	for rows.Next() {
		hasRows = true
		var data []byte
		err := rows.Scan(&data)
		if err != nil {
			return errors.Wrap(err, "scan trace data")
		}

		var ev tracepb2.TraceEvent
		if err := protojson.Unmarshal(data, &ev); err != nil {
			return errors.Wrap(err, "unmarshal trace event")
		}
		if !iter(&ev) {
			return nil
		}
	}

	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "iterate events")
	} else if !hasRows {
		return trace2.ErrNotFound
	}
	return nil
}

// backfillParentSpanIDs fetches parent_span_id from trace_event (WAL) for spans
// that don't have it populated in the trace_span_index table.
func (s *Store) backfillParentSpanIDs(ctx context.Context, appID string, spans []spanRow) error {
	// Collect spans needing backfill
	type key struct{ traceID, spanID string }
	needsBackfill := make(map[key]*tracepb2.SpanSummary)

	for _, span := range spans {
		if span.summary.ParentSpanId == nil {
			k := key{span.summary.TraceId, span.summary.SpanId}
			needsBackfill[k] = span.summary
		}
	}

	if len(needsBackfill) == 0 {
		return nil
	}

	// Build batch query
	var args []any
	args = append(args, appID)

	var placeholders []string
	for k := range needsBackfill {
		args = append(args, k.traceID, k.spanID)
		placeholders = append(placeholders, "(?, ?)")
	}

	query := fmt.Sprintf(`
		SELECT trace_id, span_id, event_data
		FROM trace_event
		WHERE app_id = ? AND (trace_id, span_id) IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "batch query trace events")
	}
	defer fns.CloseIgnore(rows)

	// Process results
	for rows.Next() {
		var traceID, spanID string
		var data []byte
		if err := rows.Scan(&traceID, &spanID, &data); err != nil {
			return errors.Wrap(err, "scan trace event")
		}

		var ev tracepb2.TraceEvent
		if err := protojson.Unmarshal(data, &ev); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal trace event")
			continue
		}

		if start := ev.GetSpanStart(); start != nil && start.ParentSpanId != nil {
			k := key{traceID, spanID}
			if summary, ok := needsBackfill[k]; ok {
				encoded := encodeSpanID(*start.ParentSpanId)
				summary.ParentSpanId = &encoded
			}
		}
	}

	return errors.Wrap(rows.Err(), "iterate trace events")
}
