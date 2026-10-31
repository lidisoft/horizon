// Package eventlog appends domain events in the caller's transaction and
// relays them to JetStream (ADR-0004, docs/architecture.md#domain-events-and-the-outbox).
package eventlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/eventlog/db"
)

// Event is one domain event. Type is the event storm's name in PascalCase
// (IssueStatusUpdated); AggregateType is lower snake case (issue).
type Event struct {
	ID               uuid.UUID
	WorkspaceID      uuid.NullUUID // invalid for instance-level events
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	Type             string
	ActorID          uuid.NullUUID // invalid for system actions
	OccurredAt       time.Time
	// Payload carries ids and changed values, never the content of other
	// aggregates (ADR-0012): it must survive their deletion.
	Payload json.RawMessage
}

var (
	aggregateTypeRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	eventTypeRe     = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
)

// Validate checks the fields the database and subject scheme rely on.
func (e Event) Validate() error {
	var errs []error
	if !aggregateTypeRe.MatchString(e.AggregateType) {
		errs = append(errs, fmt.Errorf("aggregate type %q: want lower snake case", e.AggregateType))
	}
	if !eventTypeRe.MatchString(e.Type) {
		errs = append(errs, fmt.Errorf("event type %q: want PascalCase", e.Type))
	}
	if e.AggregateID == uuid.Nil {
		errs = append(errs, errors.New("aggregate id is required"))
	}
	if e.AggregateVersion < 1 {
		errs = append(errs, fmt.Errorf("aggregate version %d: want ≥ 1", e.AggregateVersion))
	}
	if len(e.Payload) > 0 && !json.Valid(e.Payload) {
		errs = append(errs, errors.New("payload is not valid JSON"))
	}
	return errors.Join(errs...)
}

// PublicType is the name used by the API, WebSockets and webhooks
// (docs/api.md#domain-event-format): IssueStatusUpdated on an issue becomes
// issue.status_updated.
func (e Event) PublicType() string {
	name := e.Type
	prefix := pascal(e.AggregateType)
	if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
		name = name[len(prefix):]
	}
	return e.AggregateType + "." + snake(name)
}

func pascal(s string) string {
	var b strings.Builder
	for _, part := range strings.Split(s, "_") {
		if part != "" {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}

func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Payload marshals v for Event.Payload, panicking on values that cannot be
// encoded (a programming error, like a channel in the payload).
func Payload(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("eventlog: payload: %v", err))
	}
	return b
}

// Append writes events with the caller's transaction (or connection), so they
// commit or roll back with the state change they describe. Missing ids and
// timestamps are filled in.
func Append(ctx context.Context, tx db.DBTX, events ...Event) error {
	if len(events) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]db.AppendEventsParams, len(events))
	for i := range events {
		e := &events[i]
		if e.ID == uuid.Nil {
			e.ID = uuid.Must(uuid.NewV7())
		}
		if e.OccurredAt.IsZero() {
			e.OccurredAt = now
		}
		if len(e.Payload) == 0 {
			e.Payload = json.RawMessage(`{}`)
		}
		if err := e.Validate(); err != nil {
			return fmt.Errorf("eventlog: %s %s: %w", e.AggregateType, e.Type, err)
		}
		rows[i] = db.AppendEventsParams{
			ID:               e.ID,
			WorkspaceID:      e.WorkspaceID,
			AggregateType:    e.AggregateType,
			AggregateID:      e.AggregateID,
			AggregateVersion: e.AggregateVersion,
			Type:             e.Type,
			ActorID:          e.ActorID,
			OccurredAt:       e.OccurredAt,
			Payload:          e.Payload,
		}
	}
	n, err := db.New(tx).AppendEvents(ctx, rows)
	if err != nil {
		return fmt.Errorf("eventlog: append: %w", err)
	}
	if int(n) != len(rows) {
		return fmt.Errorf("eventlog: appended %d of %d events", n, len(rows))
	}
	return nil
}

// History returns the stored events of one aggregate, oldest first.
func History(ctx context.Context, conn db.DBTX, aggregateType string, aggregateID uuid.UUID) ([]Event, error) {
	rows, err := db.New(conn).AggregateEvents(ctx, db.AggregateEventsParams{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
	})
	if err != nil {
		return nil, fmt.Errorf("eventlog: history: %w", err)
	}
	events := make([]Event, len(rows))
	for i, r := range rows {
		events[i] = Event{
			ID:               r.ID,
			WorkspaceID:      r.WorkspaceID,
			AggregateType:    r.AggregateType,
			AggregateID:      r.AggregateID,
			AggregateVersion: r.AggregateVersion,
			Type:             r.Type,
			ActorID:          r.ActorID,
			OccurredAt:       r.OccurredAt,
			Payload:          r.Payload,
		}
	}
	return events, nil
}
