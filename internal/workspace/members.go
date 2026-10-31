package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/workspace/db"
)

// Member statuses. Pending people are invitations, not members.
const (
	statusActive    = "active"
	statusSuspended = "suspended"
)

// Member is one person of a workspace.
type Member struct {
	UserID      uuid.UUID  `json:"user_id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Status      string     `json:"status" enums:"active,suspended"`
	JoinedAt    time.Time  `json:"joined_at"`
	SuspendedAt *time.Time `json:"suspended_at,omitempty"`
	// Deactivated is set when the account itself is deactivated instance-wide.
	Deactivated bool       `json:"deactivated"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
	Teams       []TeamRef  `json:"teams"`
	Version     int32      `json:"version"`
}

// TeamRef names a team.
type TeamRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// join adds userID as an active member, inside tx. via says how: creation,
// invitation, admin.
func (s *Service) join(ctx context.Context, tx pgx.Tx, workspaceID, userID uuid.UUID, actor uuid.NullUUID, via string) (db.WorkspaceMember, error) {
	m, err := db.New(tx).AddMember(ctx, db.AddMemberParams{
		WorkspaceID: workspaceID, ID: uuid.Must(uuid.NewV7()), UserID: userID, JoinedAt: s.now(),
	})
	if isUniqueViolation(err) {
		return db.WorkspaceMember{}, apperr.Conflict("already_member")
	}
	if err != nil {
		return db.WorkspaceMember{}, err
	}
	return m, eventlog.Append(ctx, tx, s.event(workspaceID, actor, "workspace_member", m.ID, int64(m.Version),
		"WorkspaceMemberJoined", map[string]any{"user_id": userID, "via": via}))
}

// AddMember makes an existing account a member directly, without an
// invitation (admin CLI).
func (s *Service) AddMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := s.join(ctx, tx, workspaceID, userID, uuid.NullUUID{}, "admin")
		return err
	})
}

// Members lists the workspace's members with their teams. Every member may
// see the list (guests won't, once roles exist).
func (s *Service) Members(ctx context.Context, wc Context) ([]Member, error) {
	q := db.New(s.pool)
	rows, err := q.ListMembers(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	teams, err := q.MemberTeams(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	seen, err := q.MembersLastSeen(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	lastSeen := map[uuid.UUID]*time.Time{}
	for _, r := range seen {
		lastSeen[r.UserID] = &r.LastSeenAt
	}
	byUser := map[uuid.UUID][]TeamRef{}
	for _, t := range teams {
		byUser[t.UserID] = append(byUser[t.UserID], TeamRef{ID: t.TeamID, Name: t.TeamName})
	}
	out := make([]Member, len(rows))
	for i, r := range rows {
		ts := byUser[r.UserID]
		if ts == nil {
			ts = []TeamRef{}
		}
		out[i] = Member{
			UserID: r.UserID, Name: r.Name, Email: r.Email, Status: r.Status, JoinedAt: r.JoinedAt,
			SuspendedAt: r.SuspendedAt, Deactivated: r.DeactivatedAt != nil, LastSeenAt: lastSeen[r.UserID],
			Teams: ts, Version: r.Version,
		}
	}
	return out, nil
}

// SetSuspended suspends a member (true) or reactivates them
// (workspace.members.manage). Suspension removes access to this workspace
// only; grants, teams and history stay. Nobody can suspend themselves.
func (s *Service) SetSuspended(ctx context.Context, wc Context, userID uuid.UUID, suspended bool) error {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return err
	}
	if userID == wc.Principal.User.ID {
		return apperr.Conflict("cannot_change_self")
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		m, err := q.LockMembership(ctx, db.LockMembershipParams{WorkspaceID: wc.Workspace.ID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("member_not_found")
		}
		if err != nil {
			return err
		}
		if (m.Status == statusSuspended) == suspended {
			return nil
		}
		status, typ := statusActive, "WorkspaceMemberReactivated"
		var at *time.Time
		if suspended {
			now := s.now()
			status, typ, at = statusSuspended, "WorkspaceMemberSuspended", &now
		}
		m, err = q.SetMemberStatus(ctx, db.SetMemberStatusParams{WorkspaceID: m.WorkspaceID, UserID: userID, Status: status, SuspendedAt: at})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(m.WorkspaceID, actorOf(wc.Principal), "workspace_member", m.ID, int64(m.Version),
			typ, map[string]any{"user_id": userID}))
	})
}

// RemoveMember takes someone out of the workspace and its teams
// (workspace.members.manage). Their issues and history stay.
func (s *Service) RemoveMember(ctx context.Context, wc Context, userID uuid.UUID) error {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return err
	}
	if userID == wc.Principal.User.ID {
		return apperr.Conflict("cannot_change_self")
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		m, err := q.LockMembership(ctx, db.LockMembershipParams{WorkspaceID: wc.Workspace.ID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("member_not_found")
		}
		if err != nil {
			return err
		}
		if err := q.RemoveMember(ctx, db.RemoveMemberParams{WorkspaceID: m.WorkspaceID, UserID: userID}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(m.WorkspaceID, actorOf(wc.Principal), "workspace_member", m.ID, int64(m.Version)+1,
			"WorkspaceMemberRemoved", map[string]any{"user_id": userID}))
	})
}
