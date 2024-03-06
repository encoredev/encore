package sqlite

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/pkg/fns"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

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
		    topic_name, subscription_name, message_id, is_error, test_skipped, duration_nanos, src_file, src_line
		FROM trace_span_index
		WHERE app_id = $1 AND has_response AND is_root AND span_type != $2 `+extraWhereClause+`
		ORDER BY started_at DESC
		LIMIT `+strconv.Itoa(limit)+`
	`, args...)
	if err != nil {
		return errors.Wrap(err, "query traces")
	}

	defer fns.CloseIgnore(rows)
	n := 0
	for rows.Next() {
		if n >= limit {
			break
		}
		n++

		var t tracepb2.SpanSummary
		var startedAt int64
		err := rows.Scan(
			&t.TraceId, &t.SpanId, &startedAt, &t.Type, &t.IsRoot, &t.ServiceName, &t.EndpointName,
			&t.TopicName, &t.SubscriptionName, &t.MessageId, &t.IsError, &t.TestSkipped, &t.DurationNanos, &t.SrcFile, &t.SrcLine)
		if err != nil {
			return errors.Wrap(err, "scan trace")
		}
		ts := time.Unix(0, startedAt)
		t.StartedAt = timestamppb.New(ts)

		if !iter(&t) {
			return nil
		}
	}

	return errors.Wrap(rows.Err(), "iterate traces")
}

// emitCompleteSpanToListeners emits the given trace/span to all listeners
// if it's a complete root span (meaning it has a response and is not an auth span).
func (s *Store) emitCompleteSpanToListeners(ctx context.Context, appID, traceID, spanID string) {
	var t tracepb2.SpanSummary
	var startedAt int64
	err := s.db.QueryRowContext(ctx, `
		SELECT
			trace_id, span_id, started_at, span_type, is_root, service_name, endpoint_name,
			topic_name, subscription_name, message_id, is_error, test_skipped, duration_nanos, src_file, src_line
		FROM trace_span_index
		WHERE app_id = ? AND trace_id = ? AND span_id = ? AND has_response AND is_root AND span_type != ?
		ORDER BY started_at DESC
	`, appID, traceID, spanID, tracepb2.SpanSummary_AUTH).Scan(
		&t.TraceId, &t.SpanId, &startedAt, &t.Type, &t.IsRoot, &t.ServiceName, &t.EndpointName,
		&t.TopicName, &t.SubscriptionName, &t.MessageId, &t.IsError, &t.TestSkipped, &t.DurationNanos, &t.SrcFile, &t.SrcLine)
	if errors.Is(err, sql.ErrNoRows) {
		return
	} else if err != nil {
		log.Error().Err(err).Msg("unable to query trace span")
		return
	}

	ts := time.Unix(0, startedAt)
	t.StartedAt = timestamppb.New(ts)
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
	for rows.Next() {
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

	return errors.Wrap(rows.Err(), "iterate events")
}
