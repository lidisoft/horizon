package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/workspace/db"
)

// Invitations (docs/security.md#accounts): valid InvitationLifetime, single
// use; accepting one proves the email address, so it can create the account.
const (
	InvitationLifetime = 7 * 24 * time.Hour
	// MaxInvitesPerRequest bounds one batch of invitations.
	MaxInvitesPerRequest = 50
)

// Invitation is the public view of a pending invitation.
type Invitation struct {
	ID          uuid.UUID   `json:"id"`
	Email       string      `json:"email"`
	InviterName string      `json:"inviter_name"`
	TeamIDs     []uuid.UUID `json:"team_ids"`
	// RoleID is the workspace role given on acceptance.
	RoleID    uuid.NullUUID `json:"role_id" swaggertype:"string"`
	SentAt    time.Time     `json:"sent_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	Expired   bool          `json:"expired"`
}

func (s *Service) invitationFrom(i db.Invitation, inviter string) Invitation {
	return Invitation{
		ID: i.ID, Email: i.Email, InviterName: inviter, TeamIDs: nonNil(i.TeamIds), RoleID: i.RoleID,
		SentAt: i.SentAt, ExpiresAt: i.ExpiresAt, Expired: !i.ExpiresAt.After(s.now()),
	}
}

func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

func newInvitationToken() (string, []byte) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, hashInvitationToken(token)
}

func hashInvitationToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Invite is the input of InvitePeople.
type Invite struct {
	Emails  []string
	TeamIDs []uuid.UUID
	// RoleID is the workspace role to give; unset gives Contributor. Any
	// other role needs workspace.roles.manage, so inviting can't escalate.
	RoleID uuid.NullUUID
}

// pendingMail is an invitation email to send once the transaction commits.
type pendingMail struct {
	email, link string
}

// InvitePeople invites each email (workspace.members.manage). An email that
// already has an open invitation gets it renewed and sent again; members
// can't be invited. Emails go out after the commit.
func (s *Service) InvitePeople(ctx context.Context, wc Context, in Invite) ([]Invitation, error) {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return nil, err
	}
	if s.mail == nil || s.baseURL == nil {
		return nil, errors.New("workspace: invitations need SMTP and BASE_URL")
	}
	var fields apperr.Fields
	emails := map[string]bool{}
	var list []string
	for i, raw := range in.Emails {
		e := strings.TrimSpace(raw)
		a, err := netmail.ParseAddress(e)
		if err != nil || a.Address != e || len(e) > 320 {
			fields.Add(fmt.Sprintf("emails[%d]", i), "invalid")
			continue
		}
		if key := strings.ToLower(e); !emails[key] {
			emails[key] = true
			list = append(list, e)
		}
	}
	switch {
	case len(in.Emails) == 0:
		fields.Add("emails", "required")
	case len(in.Emails) > MaxInvitesPerRequest:
		fields.Add("emails", "too_many")
	}
	if err := fields.Err(); err != nil {
		return nil, err
	}

	var out []Invitation
	var mails []pendingMail
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		teams, err := s.checkTeams(ctx, q, wc.Workspace.ID, in.TeamIDs)
		if err != nil {
			return err
		}
		role, err := s.inviteRole(ctx, tx, wc, in.RoleID)
		if err != nil {
			return err
		}
		members, err := q.ListMembers(ctx, wc.Workspace.ID)
		if err != nil {
			return err
		}
		memberEmails := map[string]bool{}
		for _, m := range members {
			memberEmails[strings.ToLower(m.Email)] = true
		}
		var fields apperr.Fields
		for i, e := range list {
			if memberEmails[strings.ToLower(e)] {
				fields.Add(fmt.Sprintf("emails[%d]", i), "already_member")
			}
		}
		if err := fields.Err(); err != nil {
			return err
		}

		now := s.now()
		for _, e := range list {
			token, hash := newInvitationToken()
			inv, err := q.OpenInvitationByEmail(ctx, db.OpenInvitationByEmailParams{WorkspaceID: wc.Workspace.ID, Email: e})
			typ := "InvitationResent"
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				typ = "InvitationSent"
				inv, err = q.CreateInvitation(ctx, db.CreateInvitationParams{
					WorkspaceID: wc.Workspace.ID, ID: uuid.Must(uuid.NewV7()), Email: e, TokenHash: hash,
					InvitedBy: actorOf(wc.Principal), TeamIds: teams, RoleID: role, CreatedAt: now, ExpiresAt: now.Add(InvitationLifetime),
				})
			case err == nil:
				inv, err = q.RenewInvitation(ctx, db.RenewInvitationParams{
					WorkspaceID: inv.WorkspaceID, ID: inv.ID, TokenHash: hash, InvitedBy: actorOf(wc.Principal),
					TeamIds: teams, RoleID: role, SentAt: now, ExpiresAt: now.Add(InvitationLifetime),
				})
			}
			if err != nil {
				return err
			}
			ev := s.event(inv.WorkspaceID, actorOf(wc.Principal), "invitation", inv.ID, int64(inv.Version), typ,
				map[string]any{"team_ids": nonNil(teams), "role_id": role})
			if err := eventlog.Append(ctx, tx, ev); err != nil {
				return err
			}
			out = append(out, s.invitationFrom(inv, wc.Principal.User.Name))
			mails = append(mails, pendingMail{email: e, link: s.invitationLink(token)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.sendInvitations(wc, mails)
	return out, nil
}

// Resend renews an open invitation: new link, new 7 days.
func (s *Service) Resend(ctx context.Context, wc Context, id uuid.UUID) (Invitation, error) {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return Invitation{}, err
	}
	var out Invitation
	var m pendingMail
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		inv, err := s.lockOpen(ctx, q, wc.Workspace.ID, id)
		if err != nil {
			return err
		}
		token, hash := newInvitationToken()
		now := s.now()
		inv, err = q.RenewInvitation(ctx, db.RenewInvitationParams{
			WorkspaceID: inv.WorkspaceID, ID: inv.ID, TokenHash: hash, InvitedBy: actorOf(wc.Principal),
			TeamIds: inv.TeamIds, RoleID: inv.RoleID, SentAt: now, ExpiresAt: now.Add(InvitationLifetime),
		})
		if err != nil {
			return err
		}
		out = s.invitationFrom(inv, wc.Principal.User.Name)
		m = pendingMail{email: inv.Email, link: s.invitationLink(token)}
		return eventlog.Append(ctx, tx, s.event(inv.WorkspaceID, actorOf(wc.Principal), "invitation", inv.ID, int64(inv.Version),
			"InvitationResent", map[string]any{"team_ids": nonNil(inv.TeamIds)}))
	})
	if err != nil {
		return Invitation{}, err
	}
	s.sendInvitations(wc, []pendingMail{m})
	return out, nil
}

// Revoke cancels an open invitation; its link stops working.
func (s *Service) Revoke(ctx context.Context, wc Context, id uuid.UUID) error {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		inv, err := s.lockOpen(ctx, q, wc.Workspace.ID, id)
		if err != nil {
			return err
		}
		now := s.now()
		inv, err = q.RevokeInvitation(ctx, db.RevokeInvitationParams{WorkspaceID: inv.WorkspaceID, ID: inv.ID, RevokedAt: &now})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(inv.WorkspaceID, actorOf(wc.Principal), "invitation", inv.ID, int64(inv.Version),
			"InvitationRevoked", nil))
	})
}

// Invitations lists open invitations, expired ones included
// (workspace.members.manage).
func (s *Service) Invitations(ctx context.Context, wc Context) ([]Invitation, error) {
	if err := s.Require(ctx, wc, access.WorkspaceMembersManage); err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ListOpenInvitations(ctx, wc.Workspace.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Invitation, len(rows))
	for i, r := range rows {
		out[i] = s.invitationFrom(db.Invitation{
			ID: r.ID, Email: r.Email, TeamIds: r.TeamIds, RoleID: r.RoleID, SentAt: r.SentAt, ExpiresAt: r.ExpiresAt,
		}, r.InviterName)
	}
	return out, nil
}

func (s *Service) lockOpen(ctx context.Context, q *db.Queries, workspaceID, id uuid.UUID) (db.Invitation, error) {
	inv, err := q.LockInvitation(ctx, db.LockInvitationParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (inv.AcceptedAt != nil || inv.RevokedAt != nil)) {
		return db.Invitation{}, apperr.NotFound("invitation_not_found")
	}
	return inv, err
}

// --- Accepting ----------------------------------------------------------------

// InvitationPreview is what the acceptance page shows before accepting.
type InvitationPreview struct {
	Workspace   Workspace `json:"workspace"`
	Email       string    `json:"email"`
	InviterName string    `json:"inviter_name"`
	ExpiresAt   time.Time `json:"expires_at"`
	// AccountExists tells the page to ask for sign-in rather than a new
	// password.
	AccountExists bool `json:"account_exists"`
}

// open loads a usable invitation by token: not accepted, revoked or expired.
func (s *Service) open(ctx context.Context, q *db.Queries, token string) (db.InvitationByTokenHashRow, error) {
	row, err := q.InvitationByTokenHash(ctx, hashInvitationToken(strings.TrimSpace(token)))
	if errors.Is(err, pgx.ErrNoRows) {
		return row, apperr.NotFound("invitation_invalid")
	}
	if err != nil {
		return row, err
	}
	inv := row.Invitation
	if inv.AcceptedAt != nil || inv.RevokedAt != nil || !inv.ExpiresAt.After(s.now()) {
		return row, apperr.NotFound("invitation_invalid")
	}
	return row, nil
}

// Preview describes an invitation from its token.
func (s *Service) Preview(ctx context.Context, token string) (InvitationPreview, error) {
	row, err := s.open(ctx, db.New(s.pool), token)
	if err != nil {
		return InvitationPreview{}, err
	}
	exists := true
	if _, err := s.identity.UserByEmail(ctx, row.Invitation.Email); apperr.Is(err, "user_not_found") {
		exists = false
	} else if err != nil {
		return InvitationPreview{}, err
	}
	return InvitationPreview{
		Workspace: workspaceFrom(row.Workspace), Email: row.Invitation.Email, InviterName: row.InviterName,
		ExpiresAt: row.Invitation.ExpiresAt, AccountExists: exists,
	}, nil
}

// Acceptance is the input of Accept.
type Acceptance struct {
	Token string
	// Signed-in caller, if any; their email must be the invited one.
	Principal *identity.Principal
	// Name and Password create the account when there is none.
	Name, Password string
	Client         identity.Client
}

// Accepted is the result of Accept. SignedIn is set when the account was
// created, to sign the browser in.
type Accepted struct {
	Workspace Workspace
	SignedIn  *identity.SignedIn
}

// Accept joins the workspace with an invitation: as the signed-in user, or
// by creating the account. Someone with an account must sign in first
// (sign_in_required).
func (s *Service) Accept(ctx context.Context, in Acceptance) (Accepted, error) {
	var out Accepted
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		row, err := s.open(ctx, q, in.Token)
		if err != nil {
			return err
		}
		inv := row.Invitation
		out.Workspace = workspaceFrom(row.Workspace)

		var userID uuid.UUID
		switch _, err := s.identity.UserByEmail(ctx, inv.Email); {
		case in.Principal != nil:
			if !strings.EqualFold(in.Principal.User.Email, inv.Email) {
				return apperr.Forbidden("email_mismatch")
			}
			userID = in.Principal.User.ID
		case err == nil:
			return apperr.Unauthorized("sign_in_required")
		case apperr.Is(err, "user_not_found"):
			if in.Password == "" {
				return apperr.Invalid(apperr.Field{Field: "password", Code: "required"})
			}
			u, err := s.identity.CreateUserInTx(ctx, tx, inv.InvitedBy, identity.NewUser{Email: inv.Email, Name: in.Name, Password: in.Password})
			if err != nil {
				return err
			}
			userID = u.ID
			signed, err := s.identity.OpenSessionInTx(ctx, tx, u.ID, in.Client)
			if err != nil {
				return err
			}
			out.SignedIn = &signed
		default:
			return err
		}

		if _, err := q.Membership(ctx, db.MembershipParams{WorkspaceID: inv.WorkspaceID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
			if _, err := s.join(ctx, tx, inv.WorkspaceID, userID, inv.InvitedBy, "invitation"); err != nil {
				return err
			}
			role := inv.RoleID.UUID
			if !inv.RoleID.Valid { // the chosen role was deleted since
				if role, err = s.access.BuiltinRole(ctx, tx, inv.WorkspaceID, access.DefaultRoleForInvites); err != nil {
					return err
				}
			}
			_, err := s.access.SetGrantInTx(ctx, tx, inv.WorkspaceID, inv.InvitedBy, access.GrantInput{
				RoleID: role, UserID: uuid.NullUUID{UUID: userID, Valid: true},
			})
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		now := s.now()
		for _, team := range inv.TeamIds {
			// Teams deleted since the invitation are skipped by the join.
			if _, err := q.AddTeamMemberIfTeam(ctx, db.AddTeamMemberIfTeamParams{WorkspaceID: inv.WorkspaceID, TeamID: team, UserID: userID, AddedAt: now}); err != nil {
				return err
			}
		}
		inv, err = q.AcceptInvitation(ctx, db.AcceptInvitationParams{
			WorkspaceID: inv.WorkspaceID, ID: inv.ID, AcceptedAt: &now, AcceptedBy: uuid.NullUUID{UUID: userID, Valid: true},
		})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(inv.WorkspaceID, uuid.NullUUID{UUID: userID, Valid: true}, "invitation", inv.ID,
			int64(inv.Version), "InvitationAccepted", map[string]any{"user_id": userID}))
	})
	return out, err
}

// --- Email --------------------------------------------------------------------

func (s *Service) invitationLink(token string) string {
	u := s.baseURL.JoinPath("invitation")
	u.Fragment = "token=" + token // the fragment never reaches server logs
	return u.String()
}

// sendInvitations mails after the commit, in the background; failures are
// logged, and the invitation can be resent.
func (s *Service) sendInvitations(wc Context, mails []pendingMail) {
	inviter, workspace := wc.Principal.User.Name, wc.Workspace.Name
	for _, m := range mails {
		msg := mail.Message{
			To:      &netmail.Address{Address: m.email},
			Subject: fmt.Sprintf("%s invited you to %s on Horizon", inviterOr(inviter), workspace),
			Body: fmt.Sprintf(`Hello,

%s invited you to join the %s workspace on Horizon.

To accept, open this link within 7 days:

%s

If you weren't expecting this, you can ignore this email.
`, inviterOr(inviter), workspace, m.link),
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := s.mail.Send(ctx, msg); err != nil {
				s.log.Error().Err(err).Str("workspace_id", wc.Workspace.ID.String()).Msg("sending invitation")
			}
		}()
	}
}

func inviterOr(name string) string {
	if name == "" {
		return "An administrator"
	}
	return name
}

// inviteRole resolves the role an invitation gives: Contributor by default;
// another role needs workspace.roles.manage.
func (s *Service) inviteRole(ctx context.Context, tx pgx.Tx, wc Context, requested uuid.NullUUID) (uuid.NullUUID, error) {
	def, err := s.access.BuiltinRole(ctx, tx, wc.Workspace.ID, access.DefaultRoleForInvites)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	if !requested.Valid || requested.UUID == def {
		return uuid.NullUUID{UUID: def, Valid: true}, nil
	}
	if err := s.Require(ctx, wc, access.WorkspaceRolesManage); err != nil {
		return uuid.NullUUID{}, err
	}
	roles, err := s.access.Roles(ctx, wc.Workspace.ID)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	for _, r := range roles {
		if r.ID == requested.UUID {
			return requested, nil
		}
	}
	return uuid.NullUUID{}, apperr.Invalid(apperr.Field{Field: "role_id", Code: "unknown"})
}
