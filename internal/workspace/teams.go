package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/workspace/db"
)

// Team is a named group of members; grants can target it once roles exist.
type Team struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name" example:"Core"`
	Description string       `json:"description"`
	Members     []TeamMember `json:"members"`
	// Role is the team's workspace-scope role, if any.
	Role    *RoleRef `json:"role"`
	Version int32    `json:"version"`
}

// TeamMember is a member of a team.
type TeamMember struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
}

// Teams lists the workspace's teams with their members and workspace role;
// guests can't see them.
func (s *Service) Teams(ctx context.Context, wc Context) ([]Team, error) {
	if err := s.requireSees(ctx, wc); err != nil {
		return nil, err
	}
	roles, err := s.workspaceRoles(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	q := db.New(s.pool)
	rows, err := q.ListTeams(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	members, err := q.TeamMembers(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	byTeam := map[uuid.UUID][]TeamMember{}
	for _, m := range members {
		byTeam[m.TeamID] = append(byTeam[m.TeamID], TeamMember{UserID: m.UserID, Name: m.Name, Email: m.Email})
	}
	out := make([]Team, len(rows))
	for i, r := range rows {
		ms := byTeam[r.ID]
		if ms == nil {
			ms = []TeamMember{}
		}
		out[i] = Team{ID: r.ID, Name: r.Name, Description: r.Description, Members: ms, Role: roles[r.ID], Version: r.Version}
	}
	return out, nil
}

// TeamInput is the input of CreateTeam and UpdateTeam; nil fields are left
// unchanged on update.
type TeamInput struct {
	Name        *string
	Description *string
}

func validateTeam(in TeamInput, creating bool) (name, desc string, fields apperr.Fields) {
	if in.Name != nil || creating {
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
		}
		switch {
		case name == "":
			fields.Add("name", "required")
		case len([]rune(name)) > 60:
			fields.Add("name", "too_long")
		}
	}
	if in.Description != nil {
		desc = strings.TrimSpace(*in.Description)
		if len([]rune(desc)) > 300 {
			fields.Add("description", "too_long")
		}
	}
	return name, desc, fields
}

// CreateTeam adds a team (workspace.members.manage).
func (s *Service) CreateTeam(ctx context.Context, wc Context, in TeamInput) (Team, error) {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return Team{}, err
	}
	name, desc, fields := validateTeam(in, true)
	if err := fields.Err(); err != nil {
		return Team{}, err
	}
	var t db.Team
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		t, err = db.New(tx).CreateTeam(ctx, db.CreateTeamParams{
			WorkspaceID: wc.Workspace.ID, ID: uuid.Must(uuid.NewV7()), Name: name, Description: desc, CreatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t.WorkspaceID, actorOf(wc.Principal), "team", t.ID, int64(t.Version), "TeamCreated", nil))
	})
	if err != nil {
		return Team{}, err
	}
	return Team{ID: t.ID, Name: t.Name, Description: t.Description, Members: []TeamMember{}, Version: t.Version}, nil
}

// UpdateTeam renames or redescribes a team (workspace.members.manage).
func (s *Service) UpdateTeam(ctx context.Context, wc Context, id uuid.UUID, version int32, in TeamInput) error {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return err
	}
	name, desc, fields := validateTeam(in, false)
	if err := fields.Err(); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		t, err := s.lockTeam(ctx, q, wc.Workspace.ID, id)
		if err != nil {
			return err
		}
		if version != 0 && t.Version != version {
			return apperr.Conflict("version_conflict")
		}
		p := db.UpdateTeamParams{WorkspaceID: t.WorkspaceID, ID: t.ID, Name: t.Name, Description: t.Description, UpdatedAt: s.now()}
		var changed []string
		if in.Name != nil && name != t.Name {
			p.Name = name
			changed = append(changed, "name")
		}
		if in.Description != nil && desc != t.Description {
			p.Description = desc
			changed = append(changed, "description")
		}
		if len(changed) == 0 {
			return nil
		}
		t, err = q.UpdateTeam(ctx, p)
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t.WorkspaceID, actorOf(wc.Principal), "team", t.ID, int64(t.Version),
			"TeamUpdated", map[string]any{"fields": changed}))
	})
}

// DeleteTeam removes a team; its members stay in the workspace.
func (s *Service) DeleteTeam(ctx context.Context, wc Context, id uuid.UUID) error {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		t, err := s.lockTeam(ctx, q, wc.Workspace.ID, id)
		if err != nil {
			return err
		}
		if err := q.DeleteTeam(ctx, db.DeleteTeamParams{WorkspaceID: t.WorkspaceID, ID: t.ID}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t.WorkspaceID, actorOf(wc.Principal), "team", t.ID, int64(t.Version)+1, "TeamDeleted", nil))
	})
}

// SetTeamMember adds (true) or removes a workspace member from a team
// (workspace.members.manage).
func (s *Service) SetTeamMember(ctx context.Context, wc Context, teamID, userID uuid.UUID, member bool) error {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		t, err := s.lockTeam(ctx, q, wc.Workspace.ID, teamID)
		if err != nil {
			return err
		}
		var n int64
		typ := "TeamMemberRemoved"
		if member {
			typ = "TeamMemberAdded"
			if _, err := q.Membership(ctx, db.MembershipParams{WorkspaceID: t.WorkspaceID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("member_not_found")
			} else if err != nil {
				return err
			}
			n, err = q.AddTeamMember(ctx, db.AddTeamMemberParams{WorkspaceID: t.WorkspaceID, TeamID: t.ID, UserID: userID, AddedAt: s.now()})
		} else {
			n, err = q.RemoveTeamMember(ctx, db.RemoveTeamMemberParams{WorkspaceID: t.WorkspaceID, TeamID: t.ID, UserID: userID})
		}
		if err != nil || n == 0 {
			return err // already in the wanted state: nothing to record
		}
		t, err = q.BumpTeam(ctx, db.BumpTeamParams{WorkspaceID: t.WorkspaceID, ID: t.ID, UpdatedAt: s.now()})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t.WorkspaceID, actorOf(wc.Principal), "team", t.ID, int64(t.Version),
			typ, map[string]any{"user_id": userID}))
	})
}

func (s *Service) lockTeam(ctx context.Context, q *db.Queries, workspaceID, id uuid.UUID) (db.Team, error) {
	t, err := q.LockTeam(ctx, db.LockTeamParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Team{}, apperr.NotFound("team_not_found")
	}
	return t, err
}

// checkTeams returns ids, deduplicated, if each is a team of the workspace.
func (s *Service) checkTeams(ctx context.Context, q *db.Queries, workspaceID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]bool{}
	var unique []uuid.UUID
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return []uuid.UUID{}, nil
	}
	found, err := q.ExistingTeams(ctx, db.ExistingTeamsParams{WorkspaceID: workspaceID, Ids: unique})
	if err != nil {
		return nil, err
	}
	if len(found) != len(unique) {
		return nil, apperr.Invalid(apperr.Field{Field: "team_ids", Code: "unknown"})
	}
	return unique, nil
}
