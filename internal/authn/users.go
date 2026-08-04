package authn

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Administering accounts (M1-016) — the endpoint family v1 shipped ungated.
//
// Nothing here decides *who* may call it. api/openapi.yaml maps the endpoints to
// `user.read` and `user.manage`, and the authorization middleware refuses
// everybody else before a handler is entered (M1-013); this file is only
// reachable by somebody who already holds the permission. What lives here is
// what a change *means*: which combinations of fields are refused, what a role
// change does to the sessions that already exist, what disabling somebody stops,
// and what goes into the activity log.
//
// Two rules are worth reading before the code:
//
//   - **A role change is not a session change.** The platform role is read off
//     the account on every single request ([Service.Authenticate], and
//     [Service.AuthenticateToken] for a service token), so a demotion applies at
//     the target's next request whether they are signed in or not, and a
//     promotion applies just as fast. Nothing here revokes a session to make
//     that true, because nothing has to — and revoking would fail the ticket's
//     own criterion that it take effect "without re-login". There is no cached
//     copy of a role anywhere in this system; that is what PLAN.md §4's
//     "rotation on privilege change" is actually protecting against.
//
//   - **The installation must always have an administrator.** Every write that
//     could remove the last one carries [lastAdminGuard], which counts inside
//     the same transaction as the change and rolls it back. Counting first and
//     writing second would be a check two administrators could race through
//     together, each seeing the other.

// AccountFilter narrows a page of accounts.
//
// Status and Role arrive as the words the wire uses rather than as their typed
// forms, for the reason [NewToken.Scopes] does: the HTTP layer must not be able
// to name a role (TestNoHandlerDecidesForItself), so the words stay words until
// this file turns them into the vocabulary internal/authz owns.
type AccountFilter struct {
	Status string
	Role   string
	Search string
	Cursor string
	Limit  int
}

// NewAccount is what an administrator asked to create. The identifier, the
// timestamps and the password *hash* are the server's.
type NewAccount struct {
	Email       string
	DisplayName string

	// Password is empty for an account that signs in through the identity
	// provider. Present, it is held to the policy in internal/authn/password —
	// there is one definition of an acceptable password and this is not a second
	// one.
	Password password.Plaintext

	// Role and Status are wire spellings. Status is empty for "derive it from
	// whether there is a password", which is what an omitted field means.
	Role   string
	Status string

	MFAEnforced bool
}

// AccountEdit is a patch: a nil field is one the request did not mention and
// this call leaves alone.
//
// Email is deliberately absent. It is what a federated sign-in links an account
// by ([Service.federatedUser]), so editing it here could hand somebody else's
// single sign-on this account — and `UpdateUserRequest` has no such field, so
// there is nothing for this struct to carry.
type AccountEdit struct {
	DisplayName *string
	Role        *string
	Status      *string
	MFAEnforced *bool
}

// ListAccounts returns one page of accounts and the cursor for the next.
func (s *Service) ListAccounts(ctx context.Context, f AccountFilter) ([]identity.User, string, error) {
	filter := identity.PageFilter{Search: f.Search, Cursor: f.Cursor, Limit: f.Limit}

	if f.Status != "" {
		status, err := parseStatus("status", f.Status)
		if err != nil {
			return nil, "", err
		}
		filter.Status = status
	}
	if f.Role != "" {
		role, err := parseRole("role", f.Role)
		if err != nil {
			return nil, "", err
		}
		filter.Role = role
	}
	return s.users.Page(ctx, filter)
}

// Account returns one account, or [apierr.NotFound].
func (s *Service) Account(ctx context.Context, id string) (identity.User, error) {
	return s.users.ByID(ctx, id)
}

// CreateAccount stores a new account and returns it as stored.
//
// The status follows the password when the caller did not name one: an account
// with a password is `active` and ready, an account without one is `invited` and
// is claimed by the first single sign-on that resolves to its address (see
// [Service.federatedUser]). Naming `invited` *and* a password is refused rather
// than quietly honoured — the password would be one nobody could ever use,
// because an invited account has no local sign-in.
func (s *Service) CreateAccount(ctx context.Context, actor Subject, in NewAccount) (identity.User, error) {
	role, err := parseRole("platformRole", in.Role)
	if err != nil {
		return identity.User{}, err
	}

	displayName, err := requireDisplayName(in.DisplayName)
	if err != nil {
		return identity.User{}, err
	}

	status := identity.StatusActive
	if in.Password == "" {
		status = identity.StatusInvited
	}
	if in.Status != "" {
		asked, err := parseStatus("status", in.Status)
		if err != nil {
			return identity.User{}, err
		}
		switch {
		case asked == identity.StatusDisabled:
			return identity.User{}, apierr.Validation(apierr.Field("status",
				"an account cannot be created disabled; create it and then disable it if that is really what you mean"))
		case asked == identity.StatusInvited && in.Password != "":
			return identity.User{}, apierr.Validation(apierr.Field("status",
				"an invited account has no local sign-in, so the password would be one nobody could use; "+
					"omit the password, or create the account active"))
		}
		status = asked
	}

	var hash string
	if in.Password != "" {
		if err := password.Validate("password", in.Password); err != nil {
			return identity.User{}, err
		}
		if hash, err = password.Hash(in.Password); err != nil {
			return identity.User{}, fmt.Errorf("authn: hash the password of a new account: %w", err)
		}
	}

	created, err := s.users.Create(ctx, identity.NewUser{
		Email:        strings.TrimSpace(in.Email),
		DisplayName:  displayName,
		PasswordHash: hash,
		PlatformRole: role,
		Status:       status,
		MFAEnforced:  in.MFAEnforced,
	}, s.recordInTx(actor.UserID, events.VerbUserCreated, map[string]any{
		"email":          strings.TrimSpace(in.Email),
		"display_name":   displayName,
		"platform_role":  string(role),
		"status":         string(status),
		"mfa_enforced":   in.MFAEnforced,
		"local_password": in.Password != "",
	}))
	if err != nil {
		return identity.User{}, err
	}

	s.log.InfoContext(ctx, "account created",
		slog.String("user_id", created.ID),
		slog.String("actor_id", actor.UserID),
		slog.String("status", string(created.Status)))
	return created, nil
}

// UpdateAccount applies a patch to one account and returns it as stored.
func (s *Service) UpdateAccount(ctx context.Context, actor Subject, id string, edit AccountEdit) (identity.User, error) {
	before, err := s.users.ByID(ctx, id)
	if err != nil {
		return identity.User{}, err
	}

	after := before
	if edit.DisplayName != nil {
		if after.DisplayName, err = requireDisplayName(*edit.DisplayName); err != nil {
			return identity.User{}, err
		}
	}
	if edit.Role != nil {
		if after.PlatformRole, err = parseRole("platformRole", *edit.Role); err != nil {
			return identity.User{}, err
		}
	}
	if edit.Status != nil {
		if after.Status, err = parseStatus("status", *edit.Status); err != nil {
			return identity.User{}, err
		}
	}
	if edit.MFAEnforced != nil {
		after.MFAEnforced = *edit.MFAEnforced
	}
	return s.saveAccount(ctx, actor, before, after)
}

// SetAccountStatus is what disable, enable and the soft delete all are: one
// field, written the same way an ordinary patch writes it, so there is one path
// through the guard and one shape of activity row.
func (s *Service) SetAccountStatus(ctx context.Context, actor Subject, id string, status identity.Status) (identity.User, error) {
	before, err := s.users.ByID(ctx, id)
	if err != nil {
		return identity.User{}, err
	}
	after := before
	after.Status = status
	return s.saveAccount(ctx, actor, before, after)
}

// RenameSelf changes the caller's own display name, and can change nothing else
// — `UpdateSelfRequest` has one field, so there is nothing else to pass.
//
// It goes through [Service.saveAccount] like every other edit, which is what
// puts a self-service rename in the activity log beside an administrative one.
func (s *Service) RenameSelf(ctx context.Context, actor Subject, displayName string) (identity.User, error) {
	before, err := s.users.ByID(ctx, actor.UserID)
	if err != nil {
		return identity.User{}, err
	}
	after := before
	if after.DisplayName, err = requireDisplayName(displayName); err != nil {
		return identity.User{}, err
	}
	return s.saveAccount(ctx, actor, before, after)
}

// RevokeAccountSessions ends every live session an account holds and reports
// how many, without touching the account itself.
//
// Service tokens are untouched on purpose: they are not sessions, nobody is
// holding one in a browser they have lost, and the endpoint that stops those is
// the one that disables the account.
func (s *Service) RevokeAccountSessions(ctx context.Context, actor Subject, id string) (int64, error) {
	user, err := s.users.ByID(ctx, id)
	if err != nil {
		return 0, err
	}

	revoked, err := s.sessions.RevokeAll(ctx, user.ID)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "revoked every session of an account",
		slog.String("user_id", user.ID),
		slog.String("actor_id", actor.UserID),
		slog.Int64("sessions_revoked", revoked))
	s.recordAlone(ctx, events.Entry{
		ActorID:    actor.UserID,
		Verb:       events.VerbUserSessionsRevoked,
		ObjectType: events.ObjectUser,
		ObjectID:   user.ID,
		Delta:      events.Delta(map[string]any{"revoked": revoked}),
	})
	return revoked, nil
}

// saveAccount writes an edited account, with the guard and the activity rows
// that go with whatever actually changed.
//
// Everything an edit implies is decided here rather than at the four call sites,
// which is the reason they are four thin functions: a rule added to one of them
// would be a rule the other three did not have.
func (s *Service) saveAccount(ctx context.Context, actor Subject, before, after identity.User) (identity.User, error) {
	if after == before {
		// Nothing changed. Returning early rather than writing keeps a
		// no-op patch out of the activity log, where a row saying nothing
		// happened is worse than no row.
		return before, nil
	}

	hooks := make([]identity.After, 0, 4)
	if wouldRemoveAnAdmin(before, after) {
		// First, so that a change which is about to be rolled back does not
		// leave an activity row behind claiming it happened.
		hooks = append(hooks, lastAdminGuard)
	}
	hooks = append(hooks, s.changeRecords(actor, before, after)...)

	updated, err := s.users.Update(ctx, after, hooks...)
	if err != nil {
		return identity.User{}, err
	}

	if updated.Status != identity.StatusActive && before.Status == identity.StatusActive {
		// The account has just stopped being usable, so the sessions it holds
		// are ended now rather than left to expire.
		//
		// A separate transaction, and it has to be: writes are serialized
		// (PLAN.md §1), so a revocation inside the one above would be a nested
		// write waiting for a lock it is itself holding. It is also not a
		// correctness hole if it fails — [Service.Authenticate] refuses a
		// session whose owner is not active on every request, so the access is
		// already gone; what the revocation buys is that the rows say so, and
		// that re-enabling the account does not silently hand back a browser tab
		// somebody left open.
		revoked, err := s.sessions.RevokeAll(ctx, updated.ID)
		if err != nil {
			return identity.User{}, err
		}
		s.log.InfoContext(ctx, "account disabled",
			slog.String("user_id", updated.ID),
			slog.String("actor_id", actor.UserID),
			slog.Int64("sessions_revoked", revoked))
	}
	return updated, nil
}

// changeRecords is one activity row per kind of change, so that an incident
// review can filter for "who was made an administrator" without reading the
// delta of every edit.
//
// A patch that changes a role and disables an account in one request writes two
// rows in one transaction, which is the honest description of it: two things
// happened, and somebody filtering for either should find it.
func (s *Service) changeRecords(actor Subject, before, after identity.User) []identity.After {
	var hooks []identity.After

	if before.PlatformRole != after.PlatformRole {
		hooks = append(hooks, s.recordInTx(actor.UserID, events.VerbUserRoleChanged, map[string]any{
			"platform_role": change(string(before.PlatformRole), string(after.PlatformRole)),
		}))
	}

	// Everything that is not the role or the status. Collected into one row
	// because they are the ordinary edits and nobody filters for them
	// individually.
	other := map[string]any{}
	if before.DisplayName != after.DisplayName {
		other["display_name"] = change(before.DisplayName, after.DisplayName)
	}
	if before.MFAEnforced != after.MFAEnforced {
		other["mfa_enforced"] = change(before.MFAEnforced, after.MFAEnforced)
	}

	if before.Status != after.Status {
		delta := map[string]any{"status": change(string(before.Status), string(after.Status))}
		switch after.Status {
		case identity.StatusDisabled:
			hooks = append(hooks, s.recordInTx(actor.UserID, events.VerbUserDisabled, delta))
		case identity.StatusActive:
			hooks = append(hooks, s.recordInTx(actor.UserID, events.VerbUserEnabled, delta))
		default:
			// Neither on nor off — an account moved back to `invited`. There is
			// no verb for it and inventing one for a state nothing else reaches
			// would be vocabulary nobody filters by, so it travels as an
			// ordinary edit.
			other["status"] = delta["status"]
		}
	}

	if len(other) > 0 {
		hooks = append(hooks, s.recordInTx(actor.UserID, events.VerbUserUpdated, other))
	}
	return hooks
}

// recordInTx returns a hook that appends one activity row on the caller's
// transaction, against whatever entity the repository has just written.
//
// Nil when this Service has no activity log, which is the state a test that does
// not care about the feed leaves it in — [identity.runAfter] skips nil hooks.
func (s *Service) recordInTx(actorID string, verb events.Verb, delta map[string]any) identity.After {
	if s.activity == nil {
		return nil
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return s.activity.Record(ctx, tx, events.Entry{
			ActorID:    actorID,
			Verb:       verb,
			ObjectType: events.ObjectUser,
			ObjectID:   identity.AfterEntityID(ctx),
			Delta:      events.Delta(delta),
		})
	}
}

// change renders one field's before and after for an activity delta.
func change(from, to any) map[string]any {
	return map[string]any{"from": from, "to": to}
}

// wouldRemoveAnAdmin reports whether an edit takes an account out of the set of
// administrators who can sign in — by demoting it, by disabling it, or by
// un-claiming it. It is what decides whether the write needs [lastAdminGuard],
// and it is deliberately generous: guarding a change that turns out to be
// harmless costs one query inside a transaction that was happening anyway.
func wouldRemoveAnAdmin(before, after identity.User) bool {
	was := before.PlatformRole == authz.PlatformRoleAdmin && before.Status == identity.StatusActive
	is := after.PlatformRole == authz.PlatformRoleAdmin && after.Status == identity.StatusActive
	return was && !is
}

// lastAdminGuard refuses a change that has just left the installation with no
// administrator who can sign in.
//
// It runs *after* the write, inside its transaction, and reads the answer off
// the database rather than working it out from the row being edited. That order
// is the point: "would this be the last one?" asked beforehand is a question two
// administrators demoting each other at the same moment can both answer "no" to.
// Asked afterwards, one of them commits and the other rolls back.
func lastAdminGuard(ctx context.Context, tx *sql.Tx) error {
	remaining, err := identity.CountActiveAdmins(ctx, tx)
	if err != nil {
		return err
	}
	if remaining == 0 {
		return apierr.Conflict(
			"this is the last administrator who can sign in; give somebody else the admin role first")
	}
	return nil
}

// parseRole resolves a wire spelling to the platform role internal/authz owns,
// naming the request field in the failure so a form can put the message beside
// the input.
//
// The request validator has already held the value to the enum in
// api/openapi.yaml, so this is unreachable from a request that got this far. It
// is here for the callers that do not come through one — blctl, and a future
// import — because an unrecognised role is not a weaker role, it is one that
// holds nothing, and storing one would be an account nobody could explain.
func parseRole(field, name string) (authz.PlatformRole, error) {
	role := authz.PlatformRole(name)
	if !role.Valid() {
		return "", apierr.Validation(apierr.Field(field, "is not a platform role this server defines"))
	}
	return role, nil
}

// parseStatus does the same for an account's status, which identity owns.
func parseStatus(field, name string) (identity.Status, error) {
	status := identity.Status(name)
	switch status {
	case identity.StatusInvited, identity.StatusActive, identity.StatusDisabled:
		return status, nil
	default:
		return "", apierr.Validation(apierr.Field(field, "is not an account status this server defines"))
	}
}

// requireDisplayName trims a display name and refuses one that is nothing but
// space. The schema's minLength cannot: "   " is three characters.
func requireDisplayName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", apierr.Validation(apierr.Field("displayName", "cannot be blank"))
	}
	return trimmed, nil
}
