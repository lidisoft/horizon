package planning_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/planning"
)

func TestReleases(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pc := f.pc("HZN")
	a, b := f.create("HZN", "A", "5"), f.create("HZN", "B", "3")

	_, err := f.svc.CreateRelease(ctx, pc, planning.ReleaseInput{Name: new(""), StartDate: new("2026-12-01"), ReleaseDate: new("2026-11-01")})
	wantField(t, err, "name", "required")
	wantField(t, err, "release_date", "before_start")
	v1, err := f.svc.CreateRelease(ctx, pc, planning.ReleaseInput{Name: new("v0.1"), ReleaseDate: new("2026-12-18")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.CreateRelease(ctx, pc, planning.ReleaseInput{Name: new("V0.1")})
	wantField(t, err, "name", "taken")
	v2, err := f.svc.CreateRelease(ctx, pc, planning.ReleaseInput{Name: new("v0.2")})
	if err != nil {
		t.Fatal(err)
	}

	// Targeting issues; progress follows them.
	for _, i := range []string{a.Key, b.Key} {
		if _, err := f.issues.SetRelease(ctx, f.issue(i), 0, uuid.NullUUID{UUID: v1.ID, Valid: true}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = f.issues.SetRelease(ctx, f.issue(a.Key), 0, uuid.NullUUID{UUID: uuid.New(), Valid: true})
	wantField(t, err, "release_id", "unknown")
	f.finish(a.Key)
	v1, err = f.svc.Release(ctx, pc, v1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Progress.Issues != 2 || v1.Progress.Points != 8 || v1.Progress.ByOutcome["completed"].Points != 5 {
		t.Errorf("progress = %+v", v1.Progress)
	}
	list, err := f.svc.Releases(ctx, pc, false)
	if err != nil || len(list.Items) != 2 || list.Items[0].ID != v1.ID || !list.CanManage {
		t.Fatalf("list = %+v, %v", list, err)
	}

	// Renaming, releasing (release date stays), archiving.
	if v2, err = f.svc.UpdateRelease(ctx, pc, v2.ID, v2.Version, planning.ReleaseInput{Description: new("Planning")}); err != nil || v2.Description != "Planning" {
		t.Fatalf("update: %+v, %v", v2, err)
	}
	v1, err = f.svc.ShipRelease(ctx, pc, v1.ID, 0)
	if err != nil || v1.State != planning.ReleaseReleased || *v1.ReleaseDate != "2026-12-18" || v1.ReleasedAt == nil {
		t.Fatalf("released: %+v, %v", v1, err)
	}
	if v2, err = f.svc.ShipRelease(ctx, pc, v2.ID, 0); err != nil || *v2.ReleaseDate != "2026-11-04" {
		t.Fatalf("released without a date: %+v, %v", v2, err)
	}
	if v2, err = f.svc.ArchiveRelease(ctx, pc, v2.ID, 0); err != nil || v2.State != planning.ReleaseArchived {
		t.Fatalf("archived: %+v, %v", v2, err)
	}
	_, err = f.issues.SetRelease(ctx, f.issue(b.Key), 0, uuid.NullUUID{UUID: v2.ID, Valid: true})
	wantField(t, err, "release_id", "archived")
	_, err = f.svc.UpdateRelease(ctx, pc, v2.ID, 0, planning.ReleaseInput{Name: new("x")})
	wantCode(t, err, "release_archived")
	if list, _ := f.svc.Releases(ctx, pc, false); len(list.Items) != 1 {
		t.Errorf("archived listed")
	}

	// Deleting clears its issues.
	if err := f.svc.DeleteRelease(ctx, pc, v1.ID); err != nil {
		t.Fatal(err)
	}
	if f.issue(b.Key).Issue.ReleaseID.Valid {
		t.Errorf("release kept")
	}
	if got := types(f.history("release", v1.ID)); !slices.Equal(got, []string{"ReleaseDeleted"}) {
		t.Errorf("history = %v", got)
	}
	ih := f.history("issue", b.ID)
	if last := ih[len(ih)-1]; last.Type != "IssueReleaseChanged" || payload(last)["by"] != "release_deleted" {
		t.Errorf("issue recorded %s %v", last.Type, payload(last))
	}
}

func TestMilestones(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wc := f.wc(f.owner)
	f.project("OPS")
	a, ops := f.create("HZN", "A", "5"), f.create("OPS", "Elsewhere", "")

	_, err := f.svc.CreateMilestone(ctx, wc, planning.MilestoneInput{Name: new("Beta")})
	wantField(t, err, "date", "required")
	_, err = f.svc.CreateMilestone(ctx, wc, planning.MilestoneInput{Project: "NOPE", Name: new("x"), Date: new("2026-12-01")})
	wantField(t, err, "project", "unknown")
	beta, err := f.svc.CreateMilestone(ctx, wc, planning.MilestoneInput{Project: "HZN", Name: new("Beta"), Date: new("2026-12-01")})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := f.svc.CreateMilestone(ctx, wc, planning.MilestoneInput{Name: new("Launch"), Date: new("2027-01-15")})
	if err != nil {
		t.Fatal(err)
	}
	if beta.ProjectID.UUID != f.hzn.ID || launch.ProjectID.Valid || !launch.CanManage {
		t.Errorf("created %+v, %+v", beta, launch)
	}

	// A project milestone takes its project's issues; a workspace one any.
	_, err = f.svc.LinkMilestone(ctx, wc, beta.ID, planning.LinkInput{Add: []string{ops.Key}})
	wantField(t, err, "add", "unknown")
	if beta, err = f.svc.LinkMilestone(ctx, wc, beta.ID, planning.LinkInput{Add: []string{a.Key}}); err != nil || beta.Progress.Points != 5 {
		t.Fatalf("linked: %+v, %v", beta.Progress, err)
	}
	if launch, err = f.svc.LinkMilestone(ctx, wc, launch.ID, planning.LinkInput{Add: []string{a.Key, ops.Key}}); err != nil || launch.Progress.Issues != 2 {
		t.Fatalf("linked: %+v, %v", launch.Progress, err)
	}
	keys, err := f.svc.MilestoneIssues(ctx, wc, launch.ID)
	if err != nil || !slices.Equal(keys, []string{"HZN-1", "OPS-1"}) {
		t.Errorf("issues = %v, %v", keys, err)
	}
	if launch, err = f.svc.LinkMilestone(ctx, wc, launch.ID, planning.LinkInput{Remove: []string{ops.Key}}); err != nil || launch.Progress.Issues != 1 {
		t.Fatalf("unlinked: %+v, %v", launch.Progress, err)
	}

	list, err := f.svc.Milestones(ctx, wc, "")
	if err != nil || len(list.Items) != 2 || list.Items[0].ID != beta.ID || !list.CanCreate {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if list, _ := f.svc.Milestones(ctx, wc, "OPS"); len(list.Items) != 1 || list.Items[0].ID != launch.ID {
		t.Errorf("OPS list = %+v", list.Items)
	}

	// Moving the issue out of HZN drops it from HZN's milestone, not from
	// the workspace's.
	if _, err := f.issues.Move(ctx, f.issue(a.Key), issueMove("OPS")); err != nil {
		t.Fatal(err)
	}
	if beta, _ = f.svc.Milestone(ctx, wc, beta.ID); beta.Progress.Issues != 0 {
		t.Errorf("beta kept the moved issue")
	}
	if launch, _ = f.svc.Milestone(ctx, wc, launch.ID); launch.Progress.Issues != 1 {
		t.Errorf("launch lost the moved issue")
	}

	// A contributor sees milestones, doesn't manage them.
	member := f.principal("member@example.com")
	if err := f.ws.AddMember(ctx, f.acme.ID, member.User.ID); err != nil {
		t.Fatal(err)
	}
	mwc := f.wc(member)
	if list, err := f.svc.Milestones(ctx, mwc, ""); err != nil || len(list.Items) != 2 || list.CanCreate || list.Items[0].CanManage {
		t.Errorf("member list = %+v, %v", list, err)
	}
	_, err = f.svc.UpdateMilestone(ctx, mwc, beta.ID, 0, planning.MilestoneInput{Name: new("Mine")})
	wantCode(t, err, "forbidden")

	if beta, err = f.svc.UpdateMilestone(ctx, wc, beta.ID, beta.Version, planning.MilestoneInput{Date: new("2026-12-08")}); err != nil || beta.Date != "2026-12-08" {
		t.Fatalf("moved: %+v, %v", beta, err)
	}
	if err := f.svc.DeleteMilestone(ctx, wc, beta.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Milestone(ctx, wc, beta.ID); !apperr.Is(err, "milestone_not_found") {
		t.Errorf("after delete: %v", err)
	}
}
