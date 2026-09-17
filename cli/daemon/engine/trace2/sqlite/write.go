package sqlite

import (
	"context"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"

	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/pkg/fns"
	tracepbcli "encr.dev/proto/encore/engine/trace2"
)

// New creates a new store backed by the given db.
func New(db *sql.DB) *Store {
	return &Store{
		db: db,
	}
}

type Store struct {
	db        *sql.DB
	listeners []chan<- trace2.NewSpanEvent
}

var _ trace2.Store = (*Store)(nil)

func scanRows[T any](rows *sql.Rows) ([]T, error) {
	defer rows.Close()
	var out []T
	for rows.Next() {
		var v T
		err := rows.Scan(&v)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) CleanEvery(ctx context.Context, freq time.Duration, maxBytesPerApp int64, minTracesKept, batchSize int) {
	for {
		timer := time.NewTimer(freq)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := s.DoClean(ctx, maxBytesPerApp, minTracesKept, batchSize); err != nil {
				log.Error().Err(err).Msg("trace cleanup failed")
			}
		}
	}
}

// DoClean drops each app's oldest traces until its trace data fits within
// maxBytesPerApp, always keeping the newest minTracesKept whatever their size.
// The budget is bytes rather than traces because trace sizes vary hugely. Only
// trace_event is measured; it holds the payloads and dominates. Deleted pages
// go to SQLite's freelist, so this bounds where the file settles, it does not
// shrink it.
func (s *Store) DoClean(ctx context.Context, maxBytesPerApp int64, minTracesKept, batchSize int) error {
	log.Info().Msg("initiating trace event cleanup sweep")
	// octet_length, not length: on TEXT the latter counts characters, not bytes.
	rows, err := s.db.QueryContext(ctx,
		"SELECT app_id FROM trace_event GROUP BY app_id HAVING SUM(octet_length(event_data)) > ?",
		maxBytesPerApp)
	if err != nil {
		return errors.Wrap(err, "query app ids")
	}
	appIDs, err := scanRows[string](rows)
	if err != nil {
		return errors.Wrap(err, "scan app ids")
	}

	for _, appID := range appIDs {
		// Accumulate bytes newest-first and take the traces past the budget.
		// The rank guard keeps the newest minTracesKept whatever their size.
		rows, err := s.db.QueryContext(ctx, `
			WITH sizes AS (
				SELECT trace_id, MIN(id) AS ord, SUM(octet_length(event_data)) AS bytes
				FROM trace_event WHERE app_id = ? GROUP BY trace_id
			), running AS (
				SELECT trace_id, ord, bytes,
				       SUM(bytes) OVER (ORDER BY ord DESC) AS cum,
				       ROW_NUMBER() OVER (ORDER BY ord DESC) AS rank
				FROM sizes
			)
			SELECT trace_id FROM running
			WHERE cum > ? AND rank > ?
			ORDER BY ord ASC LIMIT ?
		`, appID, maxBytesPerApp, minTracesKept, batchSize)
		if err != nil {
			log.Error().Err(err).Msg("failed to get old trace ids")
			continue
		}
		traceIDs, err := scanRows[string](rows)
		if err != nil {
			log.Error().Err(err).Msg("failed to scan old trace ids")
			continue
		}
		if len(traceIDs) == 0 {
			continue
		}
		idArgs := strings.Join(fns.Map(traceIDs, pq.QuoteLiteral), ",")
		res, err := s.db.ExecContext(ctx, "DELETE FROM trace_event WHERE app_id = ? AND trace_id IN ("+idArgs+")", appID)
		if err != nil {
			log.Error().Err(err).Msg("failed to delete old trace events")
			continue
		}
		rowCount, err := res.RowsAffected()
		if err != nil {
			log.Error().Err(err).Msg("failed to get rows affected")
			continue
		}
		log.Info().Str("app_id", appID).Int64("deleted", rowCount).Msg("cleaned up old trace events")
		res, err = s.db.ExecContext(ctx, "DELETE FROM trace_span_index WHERE app_id = ? AND trace_id IN ("+idArgs+")", appID)
		if err != nil {
			log.Error().Err(err).Msg("failed to delete old trace spans")
			continue
		}
		rowCount, err = res.RowsAffected()
		if err != nil {
			log.Error().Err(err).Msg("failed to get rows affected")
			continue
		}
		log.Info().Str("app_id", appID).Int64("deleted", rowCount).Msg("cleaned up old trace spans")
	}

	return nil
}

func (s *Store) Listen(ch chan<- trace2.NewSpanEvent) {
	s.listeners = append(s.listeners, ch)
}

func (s *Store) Clear(ctx context.Context, appID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM trace_event WHERE app_id = ?", appID)
	if err != nil {
		return errors.Wrap(err, "failed to clear trace events")
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM trace_span_index WHERE app_id = ?", appID)
	return errors.Wrap(err, "failed to clear trace spans")
}

func (s *Store) WriteEvents(ctx context.Context, meta *trace2.Meta, events []*tracepbcli.TraceEvent) error {
	for _, ev := range events {
		if err := s.insertEvent(ctx, meta, ev); err != nil {
			log.Error().Err(err).Msg("unable to insert trace span event")
			continue
		}
	}

	return nil
}

func (s *Store) insertEvent(ctx context.Context, meta *trace2.Meta, ev *tracepbcli.TraceEvent) error {
	data, err := protojson.Marshal(ev)
	if err != nil {
		return errors.Wrap(err, "marshal trace event")
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO trace_event (
			app_id, trace_id, span_id, event_data)
		VALUES (?, ?, ?, ?)
	`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId), data)
	if err != nil {
		return errors.Wrap(err, "insert trace span event")
	}

	if start := ev.GetSpanStart(); start != nil {
		if err := s.updateSpanStartIndex(ctx, meta, ev, start); err != nil {
			return errors.Wrap(err, "update span start index")
		}
	} else if end := ev.GetSpanEnd(); end != nil {
		if err := s.updateSpanEndIndex(ctx, meta, ev, end); err != nil {
			return errors.Wrap(err, "update span end index")
		}
	}

	return nil
}

func (s *Store) updateSpanStartIndex(ctx context.Context, meta *trace2.Meta, ev *tracepbcli.TraceEvent, start *tracepbcli.SpanStart) error {
	isRoot := start.ParentSpanId == nil

	// parentTraceID is the trace that started this span, if any. It's recorded
	// regardless of span type, so compute it once for all the branches below.
	var parentTraceID *string
	if pt := start.GetParentTraceId(); pt != nil {
		encoded := encodeTraceID(pt)
		parentTraceID = &encoded
	}

	if req := start.GetRequest(); req != nil {
		extRequestID := req.RequestHeaders[http.CanonicalHeaderKey("X-Request-ID")]
		var parentSpanID *string
		if start.ParentSpanId != nil {
			encodedParentSpanID := encodeSpanID(*start.ParentSpanId)
			parentSpanID = &encodedParentSpanID
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name, endpoint_name, external_request_id, parent_span_id, parent_trace_id, caller_event_id,
				has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				endpoint_name = excluded.endpoint_name,
				external_request_id = excluded.external_request_id,
				parent_span_id = excluded.parent_span_id,
				parent_trace_id = excluded.parent_trace_id,
				caller_event_id = excluded.caller_event_id
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_REQUEST, ev.EventTime.AsTime().UnixNano(),
			isRoot, req.ServiceName, req.EndpointName, extRequestID, parentSpanID, parentTraceID, start.CallerEventId)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if auth := start.GetAuth(); auth != nil {
		var parentSpanID *string
		if start.ParentSpanId != nil {
			encodedParentSpanID := encodeSpanID(*start.ParentSpanId)
			parentSpanID = &encodedParentSpanID
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name, endpoint_name, parent_span_id, parent_trace_id, caller_event_id,
				has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				endpoint_name = excluded.endpoint_name,
				parent_span_id = excluded.parent_span_id,
				parent_trace_id = excluded.parent_trace_id,
				caller_event_id = excluded.caller_event_id
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_AUTH, ev.EventTime.AsTime().UnixNano(),
			isRoot, auth.ServiceName, auth.EndpointName, parentSpanID, parentTraceID, start.CallerEventId)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if msg := start.GetPubsubMessage(); msg != nil {
		var parentSpanID *string
		if start.ParentSpanId != nil {
			encodedParentSpanID := encodeSpanID(*start.ParentSpanId)
			parentSpanID = &encodedParentSpanID
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name,
				topic_name, subscription_name, message_id, parent_span_id, parent_trace_id, caller_event_id,
				has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				topic_name = excluded.topic_name,
				subscription_name = excluded.subscription_name,
				message_id = excluded.message_id,
				parent_span_id = excluded.parent_span_id,
				parent_trace_id = excluded.parent_trace_id,
				caller_event_id = excluded.caller_event_id
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_PUBSUB_MESSAGE, ev.EventTime.AsTime().UnixNano(),
			isRoot, msg.ServiceName, msg.TopicName, msg.SubscriptionName, msg.MessageId, parentSpanID, parentTraceID, start.CallerEventId)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if msg := start.GetTest(); msg != nil {
		var parentSpanID *string
		if start.ParentSpanId != nil {
			encodedParentSpanID := encodeSpanID(*start.ParentSpanId)
			parentSpanID = &encodedParentSpanID
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name, endpoint_name, user_id, src_file, src_line, parent_span_id, parent_trace_id, caller_event_id,
				has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				endpoint_name = excluded.endpoint_name,
				parent_span_id = excluded.parent_span_id,
				parent_trace_id = excluded.parent_trace_id,
				caller_event_id = excluded.caller_event_id
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_TEST, ev.EventTime.AsTime().UnixNano(),
			isRoot, msg.ServiceName, msg.TestName, msg.Uid, msg.TestFile, msg.TestLine, parentSpanID, parentTraceID, start.CallerEventId)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	return nil
}

func (s *Store) updateSpanEndIndex(ctx context.Context, meta *trace2.Meta, ev *tracepbcli.TraceEvent, end *tracepbcli.SpanEnd) (err error) {
	traceID := encodeTraceID(ev.TraceId)
	spanID := encodeSpanID(ev.SpanId)

	defer func() {
		if err == nil {
			// If the span is complete, emit it to listeners.
			s.emitCompleteSpanToListeners(ctx, meta.AppID, traceID, spanID)
		}
	}()

	if req := end.GetRequest(); req != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, has_response, is_error, duration_nanos, caller_event_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				has_response = excluded.has_response,
				is_error = excluded.is_error,
				duration_nanos = excluded.duration_nanos,
				caller_event_id = excluded.caller_event_id
		`, meta.AppID, traceID, spanID,
			tracepbcli.SpanSummary_REQUEST, true,
			end.Error != nil, end.DurationNanos, req.CallerEventId,
		)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if auth := end.GetAuth(); auth != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, has_response, is_error, duration_nanos, user_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				has_response = excluded.has_response,
				is_error = excluded.is_error,
				duration_nanos = excluded.duration_nanos,
				user_id = excluded.user_id
		`, meta.AppID, traceID, spanID,
			tracepbcli.SpanSummary_AUTH, true,
			end.Error != nil, end.DurationNanos, auth.Uid)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if msg := end.GetPubsubMessage(); msg != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, has_response, is_error, duration_nanos
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				has_response = excluded.has_response,
				is_error = excluded.is_error,
				duration_nanos = excluded.duration_nanos
		`, meta.AppID, traceID, spanID,
			tracepbcli.SpanSummary_PUBSUB_MESSAGE, true,
			end.Error != nil, end.DurationNanos)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if msg := end.GetTest(); msg != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, has_response, is_error, test_skipped, duration_nanos
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				has_response = excluded.has_response,
				is_error = excluded.is_error,
				test_skipped = excluded.test_skipped,
				duration_nanos = excluded.duration_nanos
		`, meta.AppID, traceID, spanID,
			tracepbcli.SpanSummary_TEST, true,
			msg.Failed, msg.Skipped, end.DurationNanos)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	return nil
}

var (
	binBE = binary.BigEndian
	binLE = binary.LittleEndian
)

// encodeTraceID encodes the trace id as a human-readable string.
func encodeTraceID(id *tracepbcli.TraceID) string {
	var b [16]byte
	binLE.PutUint64(b[0:8], id.Low)
	binLE.PutUint64(b[8:16], id.High)
	return base32hex.EncodeToString(b[:])
}

// encodeSpanID encodes the span id as a human-readable string.
func encodeSpanID(id uint64) string {
	var b [8]byte
	binLE.PutUint64(b[:], id)
	return base32hex.EncodeToString(b[:])
}

var (
	// base32hex is a lowercase base32 hex encoding without padding
	// that preserves lexicographic sort order.
	base32hex = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)
)
