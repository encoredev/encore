package sqlite

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encr.dev/cli/daemon/engine/trace2"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

func (s *Store) List(ctx context.Context, q *trace2.Query, iter trace2.ListEntryIterator) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
		    trace_id, span_id, started_at, span_type, is_root, service_name, endpoint_name,
		    topic_name, subscription_name, message_id, is_error, duration_nanos
		FROM trace_span_index
		WHERE app_id = ? AND has_response AND is_root AND span_type != ?
		ORDER BY started_at DESC
	`, q.AppID, tracepb2.SpanSummary_AUTH /* ignore auth spans */)
	if err != nil {
		return errors.Wrap(err, "query traces")
	}

	defer rows.Close()
	for rows.Next() {
		var t tracepb2.SpanSummary
		var startedAt int64
		err := rows.Scan(
			&t.TraceId, &t.SpanId, &startedAt, &t.Type, &t.IsRoot, &t.ServiceName, &t.EndpointName,
			&t.TopicName, &t.SubscriptionName, &t.MessageId, &t.IsError, &t.DurationNanos)
		if err != nil {
			return errors.Wrap(err, "scan trace")
		}
		// TODO set trace id
		ts := time.Unix(0, startedAt)
		t.StartedAt = timestamppb.New(ts)

		if !iter(&t) {
			return nil
		}
	}

	return errors.Wrap(rows.Err(), "iterate traces")
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

	defer rows.Close()
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
