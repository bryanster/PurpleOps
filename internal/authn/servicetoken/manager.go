package servicetoken

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Store is the part of the identity store this package needs.
// [*identity.ServiceTokens] satisfies it.
//
// It is declared here, in the package that consumes it, so that this package's
// dependency is the five methods it calls rather than everything a database
// happens to offer — and so a test can substitute one that fails on demand.
type Store interface {
	Create(ctx context.Context, in identity.NewServiceToken, after ...identity.After) (identity.ServiceToken, error)
	ByPrefix(ctx context.Context, prefix string) (identity.ServiceToken, error)
	ListByOwner(ctx context.Context, ownerUserID string) ([]identity.ServiceToken, error)
	Revoke(ctx context.Context, id, ownerUserID, revokedBy string, at time.Time,
		after ...identity.After) (identity.ServiceToken, error)
	SetLastUsedAt(ctx context.Context, id string, at time.Time) error
}

// ErrNoToken reports that a request carries no usable service token: no header,
// a value that is not a token, a prefix nobody holds, a secret that does not
// match, or a token that has expired or been revoked.
//
// The five are one error on purpose. Nothing above this package acts
// differently on the difference — the answer is 401 in every case, and telling a
// caller *which* of the five it was is telling them whether a prefix they
// guessed exists — and the specific reason travels in the wrapped text for the
// log.
var ErrNoToken = errors.New("servicetoken: no usable service token")

// MaxLifetime is the longest a token may be given, counted from creation.
//
// A year, which is M1-011's suggestion. The number is arguable and the
// requirement is not: a credential with no expiry is a credential nobody ever
// revokes, because nothing ever reminds them to. An expiry that arrives is a
// scheduled reminder that somebody still needs this.
const MaxLifetime = 365 * 24 * time.Hour

// touchInterval is how stale last_used_at is allowed to get before a use is
// written back.
//
// The write is debounced *and* moved off the request, which is the pair
// M1-011 asks for. Debounced, because recording every request would put a write
// behind every request an integration makes, and writes are serialized
// (PLAN.md §1) — one lock, held by a nightly export's ten thousand reads, for a
// column whose only consumer is a person asking "is this still in use?". Off
// the request, because even one write per minute is a serialized write a caller
// would otherwise wait on for no benefit to that caller.
//
// A minute of slack costs the answer to that question a minute of precision.
const touchInterval = time.Minute

// nameLimit is how long a token's name may be. It is a label for whoever is
// deciding which row to revoke, so it is bounded by what fits on a screen
// rather than by what a column can hold.
const nameLimit = 100

// Options configures a [Manager]. Build one from the process configuration with
// [OptionsFrom]; construct it by hand in a test that needs a clock.
type Options struct {
	// Hasher turns the secret half of a token into the value stored for it.
	// Required: a Manager without one could not tell a presented token from any
	// other string.
	Hasher *Hasher

	// MaxLifetime is the longest expiry a caller may ask for. Zero means
	// [MaxLifetime].
	MaxLifetime time.Duration

	// Now reads the clock. Nil means time.Now; a test supplies its own so that
	// an expiry can be reached without waiting for one.
	Now func() time.Time

	// Background runs a job that must not delay the request that scheduled it —
	// today, only the last_used_at write. Nil means one goroutine per job,
	// which is the production behaviour and is bounded by the debounce above:
	// at most one in flight per token per [touchInterval].
	//
	// It is injectable so that a test asserting on last_used_at can run the job
	// inline and not race it. That is the whole of its purpose; there is no
	// second implementation in the binary.
	Background func(job func())

	// Log receives what a response cannot carry. Nil means slog.Default().
	Log *slog.Logger

	// Activity records token.created / token.revoked / token.first_used
	// (M1-015). Nil skips the durable row.
	Activity *events.Log
}

// OptionsFrom derives the token policy from the process configuration.
//
// The hashing key is the deployment's encryption key and deliberately not its
// session secret — see [Hasher], which explains why rotating the lever that
// signs everybody out must not also break every integration.
func OptionsFrom(cfg config.Config) (Options, error) {
	hasher, err := NewHasher(cfg.Encryption.Key.Reveal())
	if err != nil {
		return Options{}, err
	}
	return Options{Hasher: hasher}, nil
}

// Manager issues, resolves, lists and revokes service tokens. Construct it with
// [New].
//
// What it does not do is decide what a token may reach. That is [authz.Can]'s
// job, through the two fences M1-012 built — the owner's live permissions and
// the token's scopes — and this package's contribution to it is producing an
// honest subject: the scopes as stored, the binding as stored, and nothing
// inferred.
type Manager struct {
	store    Store
	hasher   *Hasher
	activity *events.Log

	maxLifetime time.Duration
	now         func() time.Time
	background  func(job func())
	log         *slog.Logger

	// touched is the debounce: token ID → when a use was last written for it.
	// It is in-process state and a restart forgets it, which costs at most one
	// extra write per token after a deploy.
	mu      sync.Mutex
	touched map[string]time.Time
}

// New returns a Manager over store, or an error describing an option that
// cannot produce a usable one.
func New(store Store, opts Options) (*Manager, error) {
	switch {
	case store == nil:
		return nil, errors.New("servicetoken: no store; there is nowhere to keep a token")
	case opts.Hasher == nil:
		return nil, errors.New("servicetoken: no hasher; a presented token could not be checked")
	case opts.MaxLifetime < 0:
		return nil, fmt.Errorf("servicetoken: the maximum lifetime is %s, which is not a duration a token can have",
			opts.MaxLifetime)
	}

	maxLifetime := opts.MaxLifetime
	if maxLifetime == 0 {
		maxLifetime = MaxLifetime
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	background := opts.Background
	if background == nil {
		background = func(job func()) { go job() }
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		store:       store,
		hasher:      opts.Hasher,
		activity:    opts.Activity,
		maxLifetime: maxLifetime,
		now:         now,
		background:  background,
		log:         log,
		touched:     map[string]time.Time{},
	}, nil
}

// NewToken is the caller's half of creating a token: what the owner asked for.
type NewToken struct {
	// Name is the label its owner will recognise it by.
	Name string

	// OwnerUserID is whose authority the token spends, and CreatedBy is who
	// issued it. They are the same account on every path that exists today.
	OwnerUserID string
	CreatedBy   string

	// Scopes is what the caller asked for, in the wire spelling and not yet
	// checked. Strings rather than [authz.TokenScope] deliberately: a caller
	// hands over words, and turning words into scopes is a judgement with a
	// wrong answer — which makes it [Manager.checkScopes]'s, in one place, with
	// a field error to show for it.
	Scopes []string

	// EngagementID binds the token to one engagement, and is empty for a token
	// that may reach every engagement its owner can.
	EngagementID string

	// ExpiresAt is when it stops working. Required, and held to
	// [Options.MaxLifetime].
	ExpiresAt time.Time
}

// Issued is a newly created token: the row, and the value that reaches its
// owner. The token is returned exactly once, here — it is never stored and
// cannot be recovered afterwards, by this server or by anybody holding its
// database.
type Issued struct {
	// ServiceToken is the row, and Token is the credential. The pair mirrors
	// [session.Issued] deliberately: the two things that come out of minting a
	// credential are what was stored and what was handed over, and calling them
	// the same names in both packages means neither reads as the special case.
	ServiceToken identity.ServiceToken
	Token        Token
}

// Issue mints a token, stores its hash and returns both halves.
//
// Everything it refuses is refused as a field error, because every one of them
// is something the caller typed and can fix: an empty name, no scopes, a scope
// this build does not define, an expiry in the past or further out than the
// maximum.
func (m *Manager) Issue(ctx context.Context, in NewToken) (Issued, error) {
	name, err := checkName(in.Name)
	if err != nil {
		return Issued{}, err
	}
	scopes, err := m.checkScopes(in.Scopes)
	if err != nil {
		return Issued{}, err
	}
	expires, err := m.checkExpiry(in.ExpiresAt)
	if err != nil {
		return Issued{}, err
	}

	secret, parts, err := mint()
	if err != nil {
		return Issued{}, err
	}

	created, err := m.store.Create(ctx, identity.NewServiceToken{
		Name:         name,
		Prefix:       parts.prefix,
		TokenHash:    m.hasher.Hash(parts.secret),
		OwnerUserID:  in.OwnerUserID,
		CreatedBy:    in.CreatedBy,
		Scopes:       scopes,
		EngagementID: in.EngagementID,
		ExpiresAt:    expires,
	}, m.createdAfter(in, name, parts.prefix, scopes, expires))
	if err != nil {
		return Issued{}, err
	}

	m.log.InfoContext(ctx, "service token created",
		slog.String("token_id", created.ID),
		slog.String("owner_user_id", created.OwnerUserID),
		slog.String("created_by", created.CreatedBy),
		slog.String("prefix", created.Prefix),
		slog.Any("scopes", created.Scopes),
		slog.String("engagement_id", created.EngagementID),
		slog.Time("expires_at", created.ExpiresAt))

	return Issued{ServiceToken: created, Token: secret}, nil
}

func (m *Manager) createdAfter(in NewToken, name, prefix string, scopes []authz.TokenScope, expires time.Time) identity.After {
	if m.activity == nil {
		return nil
	}
	scopeNames := make([]string, len(scopes))
	for i, s := range scopes {
		scopeNames[i] = string(s)
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return m.activity.Record(ctx, tx, events.Entry{
			ActorID:    in.CreatedBy,
			Verb:       events.VerbTokenCreated,
			ObjectType: events.ObjectToken,
			ObjectID:   identity.AfterEntityID(ctx),
			Delta: events.Delta(map[string]any{
				"name":          name,
				"scopes":        scopeNames,
				"prefix":        prefix,
				"expires_at":    expires.UTC().Format(time.RFC3339),
				"owner_user_id": in.OwnerUserID,
				"engagement_id": in.EngagementID,
			}),
		})
	}
}

// Resolve returns the token a presented value stands for, and records that it
// was used.
//
// Anything that means "this is not a usable credential" is [ErrNoToken] with
// the specific reason wrapped for the log. Any other error is the database
// failing, which is not the caller's fault and must not be reported to them as
// a failure to authenticate — the same division [session.Manager.Resolve]
// keeps, and for the same reason: reporting a database hiccup as a 401 would
// tell every integration in the deployment that its credentials had stopped
// working.
//
// The lookup is by prefix and the comparison is constant-time. Neither is
// optional: a scan over stored hashes would grow with the number of tokens, and
// a byte-by-byte comparison of the hash would leak how far a guess got.
func (m *Manager) Resolve(ctx context.Context, presented Token) (identity.ServiceToken, error) {
	parts, err := parse(presented)
	if err != nil {
		// Before any query. A value that is not a token cannot be one somebody
		// holds, so there is nothing to look up.
		return identity.ServiceToken{}, fmt.Errorf("%w: %w", ErrNoToken, err)
	}

	found, err := m.store.ByPrefix(ctx, parts.prefix)
	if errors.Is(err, apierr.ErrNotFound) {
		return identity.ServiceToken{}, fmt.Errorf("%w: no token has that prefix", ErrNoToken)
	}
	if err != nil {
		return identity.ServiceToken{}, err
	}

	// The comparison is constant-time; the *lookup* preceding it is not, and
	// deliberately is not defended. What a timing difference here could reveal
	// is whether a prefix exists, and a prefix is the public half of the
	// credential — it is printed in the listing endpoint. The secret is what
	// the constant-time comparison protects.
	if !equal(m.hasher.Hash(parts.secret), found.TokenHash) {
		return identity.ServiceToken{}, fmt.Errorf(
			"%w: the secret presented for token %s does not match", ErrNoToken, found.ID)
	}

	if err := m.usable(found); err != nil {
		return identity.ServiceToken{}, err
	}

	m.recordUse(ctx, found)
	return found, nil
}

// usable reports why a token may not be used, or nil.
//
// The two reasons are kept apart in the message and joined in the error: an
// operator reading a log wants to know which it was, and a client must not be
// able to tell.
func (m *Manager) usable(t identity.ServiceToken) error {
	now := m.now()
	switch {
	case !t.RevokedAt.IsZero():
		return fmt.Errorf("%w: token %s was revoked at %s",
			ErrNoToken, t.ID, t.RevokedAt.Format(time.RFC3339))
	case !now.Before(t.ExpiresAt):
		return fmt.Errorf("%w: token %s expired at %s",
			ErrNoToken, t.ID, t.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}

// recordUse writes last_used_at back, at most once per [touchInterval] per
// token, and never on the request's own critical path.
//
// The stored timestamp is consulted first so that a fresh process does not
// write once for every token it sees; the in-process map is consulted second so
// that a burst arriving inside one interval schedules one write rather than
// one per request.
//
// A failure is logged and dropped. The request has already been authenticated
// by then — refusing it because a bookkeeping column could not be written would
// turn a nicety into an outage.
func (m *Manager) recordUse(ctx context.Context, t identity.ServiceToken) {
	now := m.now()
	if now.Sub(t.LastUsedAt) < touchInterval {
		return
	}

	m.mu.Lock()
	if written, ok := m.touched[t.ID]; ok && now.Sub(written) < touchInterval {
		m.mu.Unlock()
		return
	}
	_, seenBefore := m.touched[t.ID]
	m.touched[t.ID] = now
	m.prune(now)
	m.mu.Unlock()

	if !seenBefore && t.LastUsedAt.IsZero() {
		// The moment a token stops being a credential somebody made and starts
		// being one something is using. Both conditions are required: the column
		// is the durable answer and survives a restart, and the map is what
		// stops two requests arriving together from both reading a zero column
		// and both reporting a first use.
		m.log.InfoContext(ctx, "service token used for the first time",
			slog.String("token_id", t.ID),
			slog.String("owner_user_id", t.OwnerUserID),
			slog.String("prefix", t.Prefix))
		if m.activity != nil {
			// No sibling mutation on this path — last_used_at is written in the
			// background — so the activity row stands alone.
			if err := m.activity.RecordAlone(ctx, events.Entry{
				ActorID:    t.OwnerUserID,
				Verb:       events.VerbTokenFirstUsed,
				ObjectType: events.ObjectToken,
				ObjectID:   t.ID,
				Delta: events.Delta(map[string]any{
					"prefix":        t.Prefix,
					"owner_user_id": t.OwnerUserID,
				}),
			}); err != nil {
				m.log.WarnContext(ctx, "could not record token.first_used",
					slog.String("token_id", t.ID),
					slog.String("error", err.Error()))
			}
		}
	}

	// Detached from the request's context: this outlives the response, and a
	// cancelled request must not leave the column unwritten forever — the
	// debounce above has already claimed this interval.
	ctx = context.WithoutCancel(ctx)
	id := t.ID
	m.background(func() {
		if err := m.store.SetLastUsedAt(ctx, id, now); err != nil {
			m.log.WarnContext(ctx, "could not record the use of a service token",
				slog.String("token_id", id),
				slog.String("error", err.Error()))
		}
	})
}

// prune drops debounce entries that have aged past the interval, so the map
// tracks the tokens in use rather than every token ever seen. Called under the
// lock, from the path that just added one.
func (m *Manager) prune(now time.Time) {
	for id, written := range m.touched {
		if now.Sub(written) >= touchInterval {
			delete(m.touched, id)
		}
	}
}

// List returns every token one person holds, newest first. It never returns a
// secret, because no secret is stored: [identity.ServiceToken] carries a hash.
func (m *Manager) List(ctx context.Context, ownerUserID string) ([]identity.ServiceToken, error) {
	return m.store.ListByOwner(ctx, ownerUserID)
}

// Revoke ends one token belonging to ownerUserID, on the authority of revokedBy.
//
// A token that does not belong to ownerUserID is [apierr.NotFound],
// indistinguishable from one that does not exist — see
// [identity.ServiceTokens.Revoke], where the ownership is part of the statement
// rather than a check in front of it.
//
// The two callers differ only in the two arguments. An owner ending their own
// passes their own identifier twice; an administrator ending somebody else's
// (M1-018) passes the account named in the path and then themselves. What
// happens to the token is the same act, which is why it is one function — a
// second one would be a second definition of "revoked" to keep in step.
func (m *Manager) Revoke(ctx context.Context, id, ownerUserID, revokedBy string) (identity.ServiceToken, error) {
	revoked, err := m.store.Revoke(ctx, id, ownerUserID, revokedBy, m.now(),
		m.revokedAfter(ownerUserID, revokedBy))
	if err != nil {
		return identity.ServiceToken{}, err
	}

	m.log.InfoContext(ctx, "service token revoked",
		slog.String("token_id", revoked.ID),
		slog.String("owner_user_id", revoked.OwnerUserID),
		slog.String("revoked_by", revoked.RevokedBy),
		slog.String("prefix", revoked.Prefix),
		slog.Time("revoked_at", revoked.RevokedAt))

	return revoked, nil
}

// revokedAfter records the revocation, under the verb that says who did it to
// whose (M1-015's vocabulary, M1-018's requirement).
//
// Two verbs rather than one with a delta field somebody has to notice: an
// incident review filters for "an administrator ended somebody's credential",
// and a filter that returns every routine rotation as well is a filter nobody
// uses. It is the same distinction [events.VerbUserSessionsRevoked] draws
// against [events.VerbSessionLogout].
func (m *Manager) revokedAfter(ownerUserID, revokedBy string) identity.After {
	if m.activity == nil {
		return nil
	}
	verb := events.VerbTokenRevoked
	delta := map[string]any{}
	if revokedBy != ownerUserID {
		verb = events.VerbTokenAdminRevoked
		// Whose credential it was. The row's own object is the token, and an
		// administrator's feed entry that does not say whose access just
		// stopped is one a reader has to go and look up.
		delta["owner_user_id"] = ownerUserID
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return m.activity.Record(ctx, tx, events.Entry{
			ActorID:    revokedBy,
			Verb:       verb,
			ObjectType: events.ObjectToken,
			ObjectID:   identity.AfterEntityID(ctx),
			Delta:      events.Delta(delta),
		})
	}
}

// checkName holds a token's label to something a person can act on.
func checkName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return "", apierr.Validation(apierr.Field("name",
			"is required: it is how you will recognise this token when you come to revoke it"))
	case len(trimmed) > nameLimit:
		return "", apierr.Validation(apierr.Field("name",
			fmt.Sprintf("is %d characters; the limit is %d", len(trimmed), nameLimit)))
	}
	return trimmed, nil
}

// checkScopes refuses a token that could do nothing and one that names a scope
// this build does not define, and drops repeats.
//
// The unknown scope is refused rather than ignored. Ignoring it would grant
// nothing — [authz.Can] tests for the scope a rule requires and never for the
// absence of one — but it would hand somebody a credential that quietly does
// less than the list they typed, and they would find out from a 403 in a
// pipeline weeks later.
func (m *Manager) checkScopes(requested []string) ([]authz.TokenScope, error) {
	if len(requested) == 0 {
		return nil, apierr.Validation(apierr.Field("scopes",
			"is required: a token with no scopes could not do anything"))
	}

	scopes := make([]authz.TokenScope, 0, len(requested))
	for _, scope := range requested {
		known, ok := authz.ParseTokenScope(scope)
		if !ok {
			return nil, apierr.Validation(apierr.Field("scopes",
				fmt.Sprintf("contains %q, which is not a scope this server defines", scope)))
		}
		if !slices.Contains(scopes, known) {
			scopes = append(scopes, known)
		}
	}
	return scopes, nil
}

// checkExpiry requires one and holds it to the maximum.
func (m *Manager) checkExpiry(expires time.Time) (time.Time, error) {
	now := m.now()
	switch {
	case expires.IsZero():
		return time.Time{}, apierr.Validation(apierr.Field("expiresAt",
			"is required: a token that never expires is a token nobody remembers to revoke"))
	case !expires.After(now):
		return time.Time{}, apierr.Validation(apierr.Field("expiresAt",
			"is in the past, so the token would be born expired"))
	case expires.Sub(now) > m.maxLifetime:
		return time.Time{}, apierr.Validation(apierr.Field("expiresAt",
			fmt.Sprintf("is further away than the maximum lifetime of %s", m.maxLifetime)))
	}
	return expires, nil
}
