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
func New(ctx context.Context, db *sql.DB) *Store {
	s := &Store{
		db: db,
	}
	s.startCleaner(ctx, 1*time.Minute, 500, 100, 10000)
	return s
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

func (s *Store) startCleaner(ctx context.Context, freq time.Duration, triggerAt, eventsToKeep, batchSize int) {
	go func() {
		for {
			timer := time.NewTimer(freq)
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				log.Info().Msg("initiating trace event cleanup sweep")
				rows, err := s.db.QueryContext(ctx, "SELECT app_id FROM trace_event GROUP BY app_id HAVING COUNT(distinct trace_id) > ?", triggerAt)
				if err != nil {
					log.Error().Err(err).Msg("failed to get app ids")
					continue
				}
				appIDs, err := scanRows[string](rows)
				if err != nil {
					log.Error().Err(err).Msg("failed to scan app ids")
					continue
				}
				for _, appID := range appIDs {
					row := s.db.QueryRowContext(ctx, `
						WITH latest_events AS (
							SELECT trace_id, min(id) as id FROM trace_event WHERE app_id = ? GROUP BY 1 ORDER BY 2 DESC LIMIT ?
						) SELECT min(id) FROM latest_events;
					`, appID, eventsToKeep)
					var traceID int64
					err := row.Scan(&traceID)
					if err != nil {
						log.Error().Err(err).Msg("failed to get trace id")
						continue
					}
					rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT trace_id FROM trace_event WHERE app_id = ? AND id < ? ORDER BY id DESC LIMIT ?", appID, traceID, batchSize)
					if err != nil {
						log.Error().Err(err).Msg("failed to get old trace ids")
						continue
					}
					traceIDs, err := scanRows[string](rows)
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
			}
		}

	}()
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
	if req := start.GetRequest(); req != nil {
		extRequestID := req.RequestHeaders[http.CanonicalHeaderKey("X-Request-ID")]
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name, endpoint_name, external_request_id, has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				endpoint_name = excluded.endpoint_name,
				external_request_id = excluded.external_request_id
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_REQUEST, ev.EventTime.AsTime().UnixNano(),
			isRoot, req.ServiceName, req.EndpointName, extRequestID)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if auth := start.GetAuth(); auth != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name, 
				endpoint_name, has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				endpoint_name = excluded.endpoint_name
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_AUTH, ev.EventTime.AsTime().UnixNano(),
			isRoot, auth.ServiceName, auth.EndpointName)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if msg := start.GetPubsubMessage(); msg != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name,
				topic_name, subscription_name, message_id, has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				topic_name = excluded.topic_name,
				subscription_name = excluded.subscription_name,
				message_id = excluded.message_id
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_PUBSUB_MESSAGE, ev.EventTime.AsTime().UnixNano(),
			isRoot, msg.ServiceName, msg.TopicName, msg.SubscriptionName, msg.MessageId)
		if err != nil {
			return errors.Wrap(err, "insert trace span event")
		}
		return nil
	}

	if msg := start.GetTest(); msg != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO trace_span_index (
				app_id, trace_id, span_id, span_type, started_at, is_root, service_name,
				endpoint_name, user_id, src_file, src_line, has_response, test_skipped
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, false, false)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				is_root = excluded.is_root,
				service_name = excluded.service_name,
				endpoint_name = excluded.endpoint_name
		`, meta.AppID, encodeTraceID(ev.TraceId), encodeSpanID(ev.SpanId),
			tracepbcli.SpanSummary_TEST, ev.EventTime.AsTime().UnixNano(),
			isRoot, msg.ServiceName, msg.TestName, msg.Uid, msg.TestFile, msg.TestLine)
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
				app_id, trace_id, span_id, span_type, has_response, is_error, duration_nanos
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (trace_id, span_id) DO UPDATE SET
				has_response = excluded.has_response,
				is_error = excluded.is_error,
				duration_nanos = excluded.duration_nanos
		`, meta.AppID, traceID, spanID,
			tracepbcli.SpanSummary_REQUEST, true,
			end.Error != nil, end.DurationNanos)
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
