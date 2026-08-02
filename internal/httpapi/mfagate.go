package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// The enforcement half of M1-008. internal/authn decides *whether* a session is
// confined to enrolling; this decides what "confined" reaches, and it is a
// middleware for the reason the CSRF check is one: v1's equivalent was a
// redirect that individual handlers could and did skip, and a rule that lives in
// one place cannot be skipped by an endpoint that forgot about it.

// enrolmentOnlyRoutes is everything a session confined to enrolment may reach,
// keyed by "METHOD path" and valued with the reason it is reachable. Everything
// else is refused, including endpoints that do not exist yet — which is the
// direction the default has to point.
//
// There are no prefixes and no patterns. An entry is one route with an argument
// next to it, and TestAnEnrolmentOnlySessionReachesOnlyTheseRoutes walks the
// router and fails when a route is neither refused nor listed here — so an
// endpoint added in M2 cannot join this list without somebody typing why.
var enrolmentOnlyRoutes = map[string]string{
	// The two that lead out of the state. Enrolling and then confirming is the
	// whole of what this session is for, and confirming rotates it into an
	// ordinary one.
	"POST " + BasePath + mfaPathPrefix + "/totp/enroll":  "enrolling is the one thing this session exists to do",
	"POST " + BasePath + mfaPathPrefix + "/totp/confirm": "the other half of enrolling, and what ends the confinement",

	// The interface has to know why it is showing an enrolment screen, and the
	// answer is on the profile: mfa.required true, mfa.enrolled false. Refusing
	// this would leave a client with a session, a 403 and no way to explain it
	// to the person holding the keyboard.
	"GET " + BasePath + "/auth/me": "the profile is how a client knows to show the enrolment screen",

	// Signing out is not a privilege. Refusing it would mean somebody who does
	// not want to enrol on this machine — a shared one, a borrowed one — has no
	// way to end their session except by waiting for it to expire, and the
	// ticket's "no way to navigate past it" is about the application, not about
	// the door out of it.
	"POST " + BasePath + "/auth/logout": "ending a session grants nothing, and trapping somebody in one is not enforcement",

	// The public credential routes. A caller in this state may sign in as
	// somebody else, and the sign-in endpoints do not consult the session cookie
	// anyway — gating them would turn "sign in as an administrator to fix this"
	// into a 403 that depends on what was left in the cookie jar. That is the
	// same argument csrfExemptRoutes makes about the same three routes.
	"POST " + BasePath + "/auth/login":                      "signing in ignores the session cookie; a stale one must not block it",
	"POST " + BasePath + mfaPathPrefix + "/totp/verify":     "the second half of somebody else's sign-in, on a pending cookie rather than this session",
	"POST " + BasePath + mfaPathPrefix + "/recovery/verify": "the same, with a printed code (M1-007)",

	// Public and stateless. A monitor is not a caller with a session at all, and
	// answering these differently because somebody's browser is mid-enrolment
	// would be a health check that reports on the wrong thing.
	"GET " + BasePath + "/healthz": "public, and unauthenticated callers reach it regardless",
	"GET " + BasePath + "/version": "public, and unauthenticated callers reach it regardless",
}

// requireMFAEnrolment refuses a confined session everywhere except
// [enrolmentOnlyRoutes]. It is the last step of the chain described in
// internal/httpapi/server.go, immediately in front of the handlers.
//
// The state it acts on is decided in [authn.Service.Authenticate] and read off
// the subject, not re-derived here — see [authn.Subject.MFAEnrolmentRequired].
// This middleware knows one thing: which routes survive it.
//
// Anonymous requests pass straight through. Nothing is being enforced against
// somebody who has not signed in, and the endpoints they can reach are decided
// by authorization (M1-013), which is a different question and will sit next to
// this one.
func requireMFAEnrolment(responder *apierr.Responder, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, _ := authn.SubjectFrom(r.Context())
			if !subject.MFAEnrolmentRequired {
				next.ServeHTTP(w, r)
				return
			}

			route := r.Method + " " + routePath(r)
			if _, allowed := enrolmentOnlyRoutes[route]; allowed {
				next.ServeHTTP(w, r)
				return
			}

			// Info rather than warn. In a deployment that has just turned the
			// policy on this happens once per signed-in user per client poll,
			// and it is the system working — but it is worth having, because
			// "the application went blank for everybody" is a support ticket
			// whose answer is this line.
			log.InfoContext(r.Context(), "refused a request from a session confined to MFA enrolment",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("user_id", subject.UserID),
				slog.String("session_id", subject.SessionID))

			responder.Write(w, r, apierr.MFAEnrolmentRequired(route))
		})
	}
}
