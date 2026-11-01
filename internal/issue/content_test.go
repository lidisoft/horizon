package issue_test

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/issue"
)

// fakeStorage keeps objects in memory; "uploading" is put.
type fakeStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeStorage() *fakeStorage { return &fakeStorage{objects: map[string][]byte{}} }

func (s *fakeStorage) put(key string, b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = b
}

func (s *fakeStorage) PresignPut(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key + "?X-Amz-Signature=put", nil
}

func (s *fakeStorage) PresignGet(_ context.Context, key string, params url.Values, _ time.Duration) (string, error) {
	return "https://storage.test/" + key + "?" + params.Encode(), nil
}

func (s *fakeStorage) Stat(_ context.Context, key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objects[key]
	if !ok {
		return 0, issue.ErrNotStored
	}
	return int64(len(b)), nil
}

func (s *fakeStorage) ReadHead(_ context.Context, key string, n int64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.objects[key]
	if int64(len(b)) > n {
		b = b[:n]
	}
	return b, nil
}

func objectKeyOf(u issue.Upload) string {
	p, _ := url.Parse(u.URL)
	return strings.TrimPrefix(p.Path, "/")
}

func TestComments(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	ann := f.member("ann@example.com")

	c1, err := f.svc.AddComment(ctx, f.enter(ann, a.Key), "  First!  ")
	if err != nil || c1.Body != "First!" || c1.AuthorName != "ann" {
		t.Fatalf("comment = %+v, %v", c1, err)
	}
	_, err = f.svc.AddComment(ctx, f.enter(ann, a.Key), " ")
	wantField(t, err, "body", "required")
	c2, _ := f.svc.AddComment(ctx, f.enter(f.owner, a.Key), "Second")
	c3, _ := f.svc.AddComment(ctx, f.enter(f.owner, a.Key), "Third")

	page, err := f.svc.Comments(ctx, f.enter(ann, a.Key), "", 2)
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != c1.ID || page.Next == "" {
		t.Fatalf("page 1 = %+v, %v", page, err)
	}
	page, _ = f.svc.Comments(ctx, f.enter(ann, a.Key), page.Next, 2)
	if len(page.Items) != 1 || page.Items[0].ID != c3.ID || page.Next != "" {
		t.Errorf("page 2 = %+v", page)
	}

	// Only the author edits; moderators delete.
	_, err = f.svc.EditComment(ctx, f.enter(ann, a.Key), c2.ID, 0, "mine now")
	wantCode(t, err, "not_author")
	edited, err := f.svc.EditComment(ctx, f.enter(ann, a.Key), c1.ID, c1.Version, "First, edited")
	if err != nil || edited.EditedAt == nil || edited.Version != 2 {
		t.Fatalf("edited = %+v, %v", edited, err)
	}
	_, err = f.svc.EditComment(ctx, f.enter(ann, a.Key), c1.ID, c1.Version, "stale")
	wantCode(t, err, "version_conflict")
	wantCode(t, f.svc.DeleteComment(ctx, f.enter(ann, a.Key), c2.ID), "forbidden")
	if err := f.svc.DeleteComment(ctx, f.enter(f.owner, a.Key), c1.ID); err != nil {
		t.Errorf("moderator delete: %v", err)
	}
	if err := f.svc.DeleteComment(ctx, f.enter(f.owner, a.Key), c2.ID); err != nil {
		t.Errorf("own delete: %v", err)
	}
	// Comments of another issue aren't found through this one.
	b := f.create(f.owner, "Task", "B")
	wantCode(t, f.svc.DeleteComment(ctx, f.enter(f.owner, b.Key), c3.ID), "comment_not_found")

	if _, err := f.svc.SetArchived(ctx, f.enter(f.owner, a.Key), 0, true); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AddComment(ctx, f.enter(ann, a.Key), "late")
	wantCode(t, err, "issue_archived")

	want := "issue.created issue.commented issue.commented issue.commented issue.comment_edited issue.comment_deleted issue.comment_deleted issue.archived"
	if h := f.history(a.ID); strings.Join(h, " ") != want {
		t.Errorf("history = %v", h)
	}
}

func TestAttachments(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := newFakeStorage()
	a := f.create(f.owner, "Task", "A")

	_, err := f.svc.StartUpload(ctx, f.enter(f.owner, a.Key), "x.png", "image/png", 10)
	wantCode(t, err, "storage_unavailable")
	f.svc.SetStorage(store)

	_, err = f.svc.StartUpload(ctx, f.enter(f.owner, a.Key), "../", "", issue.MaxAttachmentSize+1)
	wantField(t, err, "filename", "required")
	wantField(t, err, "size", "too_large")

	// A "PNG" that is really HTML is sniffed and downloads.
	html := []byte("<!DOCTYPE html><script>alert(1)</script>")
	up, err := f.svc.StartUpload(ctx, f.enter(f.owner, a.Key), `C:\Users\me\evil.png`, "image/png", int64(len(html)))
	if err != nil {
		t.Fatal(err)
	}
	if up.Attachment.Filename != "evil.png" || up.Method != "PUT" || !strings.Contains(up.URL, "ws/"+f.acme.ID.String()+"/att/"+up.Attachment.ID.String()) {
		t.Errorf("upload = %+v", up)
	}
	_, err = f.svc.CompleteUpload(ctx, f.enter(f.owner, a.Key), up.Attachment.ID)
	wantCode(t, err, "upload_missing")
	store.put(objectKeyOf(up), append(html, '!'))
	_, err = f.svc.CompleteUpload(ctx, f.enter(f.owner, a.Key), up.Attachment.ID)
	wantField(t, err, "size", "mismatch")
	store.put(objectKeyOf(up), html)
	got, err := f.svc.CompleteUpload(ctx, f.enter(f.owner, a.Key), up.Attachment.ID)
	if err != nil || !strings.HasPrefix(got.ContentType, "text/html") || got.Inline {
		t.Fatalf("completed = %+v, %v", got, err)
	}
	dl, err := f.svc.DownloadURL(ctx, f.enter(f.owner, a.Key), got.ID)
	if err != nil || !strings.Contains(dl, "response-content-disposition=attachment%3B+filename%3Devil.png") {
		t.Errorf("download = %s, %v", dl, err)
	}

	// A real PNG opens inline.
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	up2, _ := f.svc.StartUpload(ctx, f.enter(f.owner, a.Key), "shot.png", "image/png", int64(len(png)))
	store.put(objectKeyOf(up2), png)
	img, err := f.svc.CompleteUpload(ctx, f.enter(f.owner, a.Key), up2.Attachment.ID)
	if err != nil || img.ContentType != "image/png" || !img.Inline {
		t.Errorf("png = %+v, %v", img, err)
	}
	if dl, _ := f.svc.DownloadURL(ctx, f.enter(f.owner, a.Key), img.ID); !strings.Contains(dl, "inline") {
		t.Errorf("png download = %s", dl)
	}

	// A pending upload is listed nowhere and expires.
	pending, _ := f.svc.StartUpload(ctx, f.enter(f.owner, a.Key), "never.txt", "text/plain", 3)
	list, _ := f.svc.Attachments(ctx, f.enter(f.owner, a.Key))
	if len(list) != 2 {
		t.Errorf("listed %d attachments", len(list))
	}
	if err := issue.ExpireUploads(ctx, f.pool, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.CompleteUpload(ctx, f.enter(f.owner, a.Key), pending.Attachment.ID)
	wantCode(t, err, "attachment_not_found")

	// Ann can't remove the owner's file; the owner can, and its object is
	// queued for purging.
	ann := f.member("ann@example.com")
	wantCode(t, f.svc.DeleteAttachment(ctx, f.enter(ann, a.Key), got.ID), "forbidden")
	if err := f.svc.DeleteAttachment(ctx, f.enter(f.owner, a.Key), got.ID); err != nil {
		t.Fatal(err)
	}
	if h := f.history(a.ID); strings.Join(h, " ") != "issue.created issue.attachment_added issue.attachment_added issue.attachment_removed" {
		t.Errorf("history = %v", h)
	}

	// Deleting the issue queues the remaining objects.
	if err := f.svc.Delete(ctx, f.enter(f.owner, a.Key), ""); err != nil {
		t.Fatal(err)
	}
	var queued []string
	rows, _ := f.pool.Query(ctx, `SELECT unnest(objects) FROM purges ORDER BY 1`)
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		queued = append(queued, k)
	}
	rows.Close()
	if len(queued) != 3 || !strings.Contains(strings.Join(queued, " "), objectKeyOf(up2)) {
		t.Errorf("queued objects = %v", queued)
	}
}

func TestActivity(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.create(f.owner, "Task", "A")
	b := f.create(f.owner, "Bug", "B")
	if _, err := f.svc.Transition(ctx, f.enter(f.owner, a.Key), 0, f.status("Todo")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Assign(ctx, f.enter(f.owner, a.Key), 0, f.owner.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AddLink(ctx, f.enter(f.owner, a.Key), issue.LinkInput{Type: "blocks", TargetKey: b.Key}); err != nil {
		t.Fatal(err)
	}

	page, err := f.svc.Activity(ctx, f.enter(f.owner, a.Key), "", 2)
	if err != nil || len(page.Items) != 2 || page.Next == "" {
		t.Fatalf("page = %+v, %v", page, err)
	}
	linked := page.Items[0]
	if linked.Type != "issue.linked" || linked.Workspace != "acme" || linked.Aggregate.Key != a.Key || linked.Actor.Name != "owner" {
		t.Errorf("linked = %+v", linked)
	}
	if ref, ok := linked.Payload["issue_id"].(*issue.Ref); !ok || ref.Key != b.Key || ref.Name != "B" {
		t.Errorf("linked issue = %#v", linked.Payload["issue_id"])
	}
	if ref, ok := page.Items[1].Payload["to"].(*issue.Ref); !ok || ref.Name != "owner" {
		t.Errorf("assignee = %#v", page.Items[1].Payload["to"])
	}
	page, _ = f.svc.Activity(ctx, f.enter(f.owner, a.Key), page.Next, 10)
	if len(page.Items) != 2 || page.Items[0].Type != "issue.status_updated" || page.Next != "" {
		t.Fatalf("page 2 = %+v", page)
	}
	if ref, ok := page.Items[0].Payload["to"].(*issue.Ref); !ok || ref.Name != "Todo" {
		t.Errorf("status = %#v", page.Items[0].Payload["to"])
	}
	_ = uuid.Nil
}
