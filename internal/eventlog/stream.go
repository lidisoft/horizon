package eventlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// InstanceToken stands in for the workspace in subjects of instance-level
// events (users, workspaces themselves).
const InstanceToken = "instance"

// Stream describes where events are published. Production uses
// DefaultStream; tests use their own name and prefix so they don't share
// messages.
type Stream struct {
	Name      string        // JetStream stream name
	Prefix    string        // first subject token
	Retention time.Duration // MaxAge; PostgreSQL is the long-term record
}

// DefaultStream returns the production stream with the given retention
// (HORIZON_EVENTS_RETENTION).
func DefaultStream(retention time.Duration) Stream {
	return Stream{Name: "HORIZON_EVENTS", Prefix: "horizon", Retention: retention}
}

// Subject is <prefix>.<workspace_id>.<aggregate_type>.<aggregate_id>.<event_type>.
// The aggregate id makes scrubbing one aggregate a single subject-filtered
// purge (ADR-0012).
func (s Stream) Subject(e Event) string {
	ws := InstanceToken
	if e.WorkspaceID.Valid {
		ws = e.WorkspaceID.UUID.String()
	}
	return fmt.Sprintf("%s.%s.%s.%s.%s", s.Prefix, ws, e.AggregateType, e.AggregateID, e.Type)
}

// AggregateSubjects matches every event of one aggregate, for purges.
func (s Stream) AggregateSubjects(workspaceID uuid.NullUUID, aggregateType string, aggregateID uuid.UUID) string {
	ws := InstanceToken
	if workspaceID.Valid {
		ws = workspaceID.UUID.String()
	}
	return fmt.Sprintf("%s.%s.%s.%s.*", s.Prefix, ws, aggregateType, aggregateID)
}

// Ensure creates or updates the stream (docs/operations.md: streams are
// created by Horizon at startup).
func (s Stream) Ensure(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	cfg := jetstream.StreamConfig{
		Name:        s.Name,
		Description: "Horizon domain events, relayed from the PostgreSQL outbox",
		Subjects:    []string{s.Prefix + ".*.*.*.*"},
		Retention:   jetstream.LimitsPolicy,
		Storage:     jetstream.FileStorage,
		MaxAge:      s.Retention,
		Duplicates:  2 * time.Minute,
	}
	st, err := js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("eventlog: stream %s: %w", s.Name, err)
	}
	return st, nil
}

// Message is the JSON body published for each event. It is the stored row,
// ids only; consumers enrich it (workspace slug, issue key, actor name) at
// the edge (docs/api.md#domain-event-format).
type Message struct {
	ID               uuid.UUID       `json:"id"`
	WorkspaceID      *uuid.UUID      `json:"workspace_id,omitempty"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      uuid.UUID       `json:"aggregate_id"`
	AggregateVersion int64           `json:"aggregate_version"`
	Type             string          `json:"type"`
	ActorID          *uuid.UUID      `json:"actor_id,omitempty"`
	OccurredAt       time.Time       `json:"occurred_at"`
	Payload          json.RawMessage `json:"payload"`
}

func toMessage(e Event) Message {
	m := Message{
		ID:               e.ID,
		AggregateType:    e.AggregateType,
		AggregateID:      e.AggregateID,
		AggregateVersion: e.AggregateVersion,
		Type:             e.Type,
		OccurredAt:       e.OccurredAt.UTC(),
		Payload:          e.Payload,
	}
	if e.WorkspaceID.Valid {
		id := e.WorkspaceID.UUID
		m.WorkspaceID = &id
	}
	if e.ActorID.Valid {
		id := e.ActorID.UUID
		m.ActorID = &id
	}
	return m
}

// Event converts a decoded message back to an Event.
func (m Message) Event() Event {
	e := Event{
		ID:               m.ID,
		AggregateType:    m.AggregateType,
		AggregateID:      m.AggregateID,
		AggregateVersion: m.AggregateVersion,
		Type:             m.Type,
		OccurredAt:       m.OccurredAt,
		Payload:          m.Payload,
	}
	if m.WorkspaceID != nil {
		e.WorkspaceID = uuid.NullUUID{UUID: *m.WorkspaceID, Valid: true}
	}
	if m.ActorID != nil {
		e.ActorID = uuid.NullUUID{UUID: *m.ActorID, Valid: true}
	}
	return e
}

// encode builds the NATS message. Nats-Msg-Id is the event id, so JetStream
// drops duplicates within its window and consumers can dedupe after it.
func (s Stream) encode(e Event) (*nats.Msg, error) {
	body, err := json.Marshal(toMessage(e))
	if err != nil {
		return nil, err
	}
	msg := nats.NewMsg(s.Subject(e))
	msg.Header.Set(jetstream.MsgIDHeader, e.ID.String())
	msg.Data = body
	return msg, nil
}

// Decode parses a relayed message.
func Decode(data []byte) (Event, error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return Event{}, fmt.Errorf("eventlog: decode: %w", err)
	}
	if m.ID == uuid.Nil {
		return Event{}, errors.New("eventlog: decode: missing id")
	}
	return m.Event(), nil
}

// Reset empties the stream and deletes its consumers (admin reconcile after
// a restore): the relay publishes from the outbox's position again, and the
// consumers, recreated when workers start, take only what comes next. It
// returns how many messages it dropped and the consumers it deleted.
func (s Stream) Reset(ctx context.Context, js jetstream.JetStream) (uint64, []string, error) {
	st, err := s.Ensure(ctx, js)
	if err != nil {
		return 0, nil, err
	}
	info, err := st.Info(ctx)
	if err != nil {
		return 0, nil, err
	}
	var names []string
	for name := range st.ConsumerNames(ctx).Name() {
		names = append(names, name)
	}
	for _, name := range names {
		if err := st.DeleteConsumer(ctx, name); err != nil {
			return 0, nil, fmt.Errorf("eventlog: delete consumer %s: %w", name, err)
		}
	}
	if err := st.Purge(ctx); err != nil {
		return 0, nil, fmt.Errorf("eventlog: purge %s: %w", s.Name, err)
	}
	return info.State.Msgs, names, nil
}
