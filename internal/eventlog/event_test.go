package eventlog

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

func sample() Event {
	return Event{
		ID:               uuid.MustParse("01929b2a-0000-7000-8000-000000000001"),
		WorkspaceID:      uuid.NullUUID{UUID: uuid.MustParse("01929b2a-0000-7000-8000-0000000000aa"), Valid: true},
		AggregateType:    "issue",
		AggregateID:      uuid.MustParse("01929b2a-0000-7000-8000-0000000000bb"),
		AggregateVersion: 3,
		Type:             "IssueStatusUpdated",
		OccurredAt:       time.Date(2026, 10, 27, 9, 12, 0, 0, time.UTC),
		Payload:          json.RawMessage(`{"from":"a","to":"b"}`),
	}
}

func TestValidate(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatalf("valid event: %v", err)
	}
	bad := sample()
	bad.AggregateType = "Issue"
	bad.Type = "issue.status_updated"
	bad.AggregateVersion = 0
	bad.Payload = json.RawMessage(`{`)
	err := bad.Validate()
	for _, want := range []string{"lower snake case", "PascalCase", "version 0", "not valid JSON"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v: missing %q", err, want)
		}
	}
}

func TestPublicType(t *testing.T) {
	cases := []struct{ agg, typ, want string }{
		{"issue", "IssueStatusUpdated", "issue.status_updated"},
		{"issue", "IssueCreated", "issue.created"},
		{"iteration", "IterationLoaded", "iteration.loaded"},
		{"status_category", "StatusCategoryRenamed", "status_category.renamed"},
		{"issue", "CommentAdded", "issue.comment_added"},
	}
	for _, c := range cases {
		e := Event{AggregateType: c.agg, Type: c.typ}
		if got := e.PublicType(); got != c.want {
			t.Errorf("%s/%s = %q, want %q", c.agg, c.typ, got, c.want)
		}
	}
}

func TestSubject(t *testing.T) {
	s := DefaultStream(time.Hour)
	e := sample()
	want := "horizon.01929b2a-0000-7000-8000-0000000000aa.issue.01929b2a-0000-7000-8000-0000000000bb.IssueStatusUpdated"
	if got := s.Subject(e); got != want {
		t.Errorf("subject = %s", got)
	}
	e.WorkspaceID = uuid.NullUUID{}
	if got := s.Subject(e); !strings.HasPrefix(got, "horizon.instance.issue.") {
		t.Errorf("instance subject = %s", got)
	}
	if got := s.AggregateSubjects(sample().WorkspaceID, "issue", e.AggregateID); got != "horizon.01929b2a-0000-7000-8000-0000000000aa.issue.01929b2a-0000-7000-8000-0000000000bb.*" {
		t.Errorf("aggregate subjects = %s", got)
	}
}

func TestEncodeDecode(t *testing.T) {
	s := DefaultStream(time.Hour)
	e := sample()
	e.ActorID = uuid.NullUUID{UUID: uuid.MustParse("01929b2a-0000-7000-8000-0000000000cc"), Valid: true}
	msg, err := s.encode(e)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Header.Get(jetstream.MsgIDHeader) != e.ID.String() {
		t.Errorf("Nats-Msg-Id = %q", msg.Header.Get(jetstream.MsgIDHeader))
	}
	got, err := Decode(msg.Data)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID || got.WorkspaceID != e.WorkspaceID || got.ActorID != e.ActorID ||
		got.AggregateVersion != 3 || !got.OccurredAt.Equal(e.OccurredAt) || string(got.Payload) != string(e.Payload) {
		t.Errorf("round trip = %+v", got)
	}
	if !strings.Contains(string(msg.Data), `"aggregate_type":"issue"`) {
		t.Errorf("body = %s", msg.Data)
	}
}
