package identity_test

import (
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

func TestIdentityRoundTrips(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	created, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID:   user.ID,
		Provider: identity.ProviderOIDC,
		Subject:  "8a7c1e2f-oidc-subject",
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}
	if created.ID == "" {
		t.Error("Create() returned an identity with no identifier")
	}
	if created.UserID != user.ID || created.Provider != identity.ProviderOIDC {
		t.Errorf("Create() returned %+v, want it attached to %q as OIDC", created, user.ID)
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreatedAt is the zero time")
	}

	found, err := r.identities.BySubject(t.Context(), identity.ProviderOIDC, "8a7c1e2f-oidc-subject")
	if err != nil {
		t.Fatalf("BySubject() = %v, want the identity", err)
	}
	if found != created {
		t.Errorf("BySubject() = %+v, want %+v", found, created)
	}
}

// TestOneUserHoldsSeveralLoginMethods is the reason this table exists: somebody
// who signs in with a password today and through their identity provider
// tomorrow is one account, not two.
func TestOneUserHoldsSeveralLoginMethods(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	for _, in := range []identity.NewIdentity{
		{UserID: user.ID, Provider: identity.ProviderLocal, Subject: "alice@example.com"},
		{UserID: user.ID, Provider: identity.ProviderOIDC, Subject: "oidc-sub"},
		{UserID: user.ID, Provider: identity.ProviderSAML, Subject: "saml-nameid"},
	} {
		if _, err := r.identities.Create(t.Context(), in); err != nil {
			t.Fatalf("Create(%s) = %v, want nil", in.Provider, err)
		}
	}

	found, err := r.identities.ListByUser(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("ListByUser() = %v, want nil", err)
	}
	if len(found) != 3 {
		t.Fatalf("ListByUser() returned %d identities, want 3", len(found))
	}
	// Oldest first: UUIDv7 identifiers sort by creation, so this is the order
	// they were attached in.
	want := []identity.Provider{identity.ProviderLocal, identity.ProviderOIDC, identity.ProviderSAML}
	for i, provider := range want {
		if found[i].Provider != provider {
			t.Errorf("identity %d is %q, want %q", i, found[i].Provider, provider)
		}
	}
}

func TestBySubjectReportsNotFound(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	if _, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID: user.ID, Provider: identity.ProviderOIDC, Subject: "known",
	}); err != nil {
		t.Fatal(err)
	}

	// The right subject at the wrong provider is not a match: a subject only
	// means anything alongside who was asked.
	if _, err := r.identities.BySubject(t.Context(), identity.ProviderSAML, "known"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("BySubject(saml, known) = %v, want not found", err)
	}
	if _, err := r.identities.BySubject(t.Context(), identity.ProviderOIDC, "unknown"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("BySubject(oidc, unknown) = %v, want not found", err)
	}
}

func TestListByUserIsEmptyForSomebodyWithNoIdentities(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	found, err := r.identities.ListByUser(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("ListByUser() = %v, want nil", err)
	}
	if len(found) != 0 {
		t.Errorf("ListByUser() = %v, want nothing", found)
	}
}

func TestDeleteDetachesALoginMethod(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	created, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID: user.ID, Provider: identity.ProviderOIDC, Subject: "going-away",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := r.identities.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("Delete() = %v, want nil", err)
	}
	if _, err := r.identities.BySubject(t.Context(), identity.ProviderOIDC, "going-away"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("the identity is still there: %v", err)
	}

	// Deleting it twice reports that it is gone rather than pretending to work,
	// because the second caller's view of the world was wrong.
	if err := r.identities.Delete(t.Context(), created.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("second Delete() = %v, want not found", err)
	}

	// The subject is free again — reattaching it after an account was cleaned
	// up is a supported thing to do.
	if _, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID: user.ID, Provider: identity.ProviderOIDC, Subject: "going-away",
	}); err != nil {
		t.Errorf("reattaching a released subject = %v, want nil", err)
	}
}

func TestAnIdentityMustBelongToARealUser(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	_, err := r.identities.Create(t.Context(), identity.NewIdentity{
		UserID: "no-such-user", Provider: identity.ProviderLocal, Subject: "ghost@example.com",
	})
	if err == nil {
		t.Fatal("an identity was attached to a user who does not exist")
	}
}
