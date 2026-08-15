package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/attack"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const (
	setupPath         = BasePath + "/setup"
	setupCompletePath = BasePath + "/setup/complete"
	attackReleasePath = BasePath + "/content/attack/releases"
)

// The two endpoints the first-run wizard is built on, over HTTP.

func TestSetupStartsIncompleteAndCompletesOnce(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	state := decodeJSON[gen.SetupState](t, server.get(setupPath, admin))
	if state.Completed {
		t.Fatal("a fresh installation reported setup as already complete; the wizard would never appear")
	}
	if state.CompletedAt != nil {
		t.Errorf("completedAt = %v on an installation nobody has configured", state.CompletedAt)
	}

	rec := server.post(setupCompletePath, "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body)
	}
	done := decodeJSON[gen.SetupState](t, rec)
	if !done.Completed || done.CompletedAt == nil {
		t.Fatalf("complete returned %+v, want a completed state with a timestamp", done)
	}
	if got, err := done.CompletedBy.Get(); err != nil || got != server.userID(t) {
		t.Errorf("completedBy = %v (err %v), want the administrator who finished it", got, err)
	}

	// Retried — a client that lost the response sends it again, and the
	// installation's own history should not move because of that.
	again := decodeJSON[gen.SetupState](t, server.post(setupCompletePath, "", admin))
	if !again.CompletedAt.Equal(*done.CompletedAt) {
		t.Errorf("completedAt moved from %v to %v on a retry", done.CompletedAt, again.CompletedAt)
	}

	// And it is what the next request reads, which is what stops the wizard
	// reappearing for the next administrator to sign in.
	after := decodeJSON[gen.SetupState](t, server.get(setupPath, admin))
	if !after.Completed {
		t.Error("setup read back as incomplete after being completed")
	}
}

// The wizard is an administrator's screen because only an administrator can do
// what it asks. A member gets the same refusal here as on every other
// settings endpoint rather than a wizard they cannot finish.
func TestSetupIsRefusedToAMember(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	member := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member@example.test"
		u.PlatformRole = authz.PlatformRoleMember
	})
	session := server.signInAs(t, member.Email)

	if rec := server.get(setupPath, session); rec.Code != http.StatusForbidden {
		t.Errorf("GET /setup as a member = %d, want 403\n%s", rec.Code, rec.Body)
	}
	if rec := server.post(setupCompletePath, "", session); rec.Code != http.StatusForbidden {
		t.Errorf("POST /setup/complete as a member = %d, want 403\n%s", rec.Code, rec.Body)
	}
}

func TestSetupNeedsASession(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	if rec := server.get(setupPath); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /setup unauthenticated = %d, want 401\n%s", rec.Code, rec.Body)
	}
}

// The version picker: upstream's releases, merged with what is installed.
func TestAttackReleasesMergeUpstreamWithInstalled(t *testing.T) {
	t.Parallel()

	adapter := attack.New()
	adapter.IndexBytes = mustReadAttackFixture(t, "index.json")
	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAttack: adapter,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	if rec := server.send(http.MethodPost, contentSourcePath(storecontent.SourceIDAttack)+"/enable", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	syncAttack(t, server, admin, adapter, "15.1", "enterprise-mini-15.1.json")

	list := decodeJSON[gen.ContentAttackReleaseList](t, server.get(attackReleasePath, admin))
	if !list.Reachable {
		t.Fatalf("reachable = false with an index in hand: %v", list.Unreachable)
	}
	if !list.SourceEnabled {
		t.Error("sourceEnabled = false on an enabled source")
	}
	if len(list.Items) != 2 {
		t.Fatalf("items = %+v, want the two Enterprise releases", list.Items)
	}
	if list.Items[0].Version != "15.1" || !list.Items[0].Latest || !list.Items[0].Installed {
		t.Errorf("items[0] = %+v, want 15.1 marked latest and installed", list.Items[0])
	}
	if list.Items[0].Status == nil || *list.Items[0].Status != "ready" {
		t.Errorf("items[0].status = %v, want ready", list.Items[0].Status)
	}
	if list.Items[1].Installed || list.Items[1].Latest {
		t.Errorf("items[1] = %+v, want 14.1 offered but neither installed nor latest", list.Items[1])
	}
}

// An installation with no route to the index still gets an answer it can build
// a screen from: 200, reachable false, and a reason.
func TestAttackReleasesReportAnUnreachableUpstreamAsAnAnswer(t *testing.T) {
	t.Parallel()

	// No IndexBytes and a URL policy that refuses the fetch: the production
	// posture on a machine with nothing to talk to.
	adapter := attack.New()
	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAttack: adapter,
		}
	})
	server.seedUser(t)
	admin := server.signIn(t)

	rec := server.get(attackReleasePath, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("releases with no upstream = %d, want 200\n%s", rec.Code, rec.Body)
	}
	var list gen.ContentAttackReleaseList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Reachable {
		t.Error("reachable = true with nothing to reach")
	}
	if reason, err := list.Unreachable.Get(); err != nil || reason == "" {
		t.Errorf("unreachable = %q (err %v), want the transport failure carried through", reason, err)
	}
	if list.Items == nil {
		t.Error("items is null; a client ranging over it should not have to check")
	}
}
