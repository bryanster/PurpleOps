package httpapi

import (
	"net/http"
	"slices"

	"github.com/bryanster/purpleops/internal/authn/challenge"
	"github.com/bryanster/purpleops/internal/authn/session"
)

// clearSpentChallenge drops the browser's pending-MFA cookie once the sign-in it
// belongs to has finished.
//
// It is a response wrapper rather than something the verification handler does,
// for the mechanical reason csrfWriter gives: the generated response types carry
// a single Set-Cookie string, and the successful verification has three cookies
// to send — the session, its CSRF companion, and this one being cleared. Two of
// the three are therefore added on the way out.
//
// It is not load-bearing. A challenge is spent in the database before a session
// is issued, so a browser that kept the cookie is holding something that
// resolves to nothing, and it expires on its own within minutes either way. What
// this buys is that the state a browser carries matches the state the server
// has, which is worth having when the thing being carried is a credential.
func clearSpentChallenge(challenges *challenge.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if challenge.FromRequest(r) == "" {
				// Nothing to clear, and no wrapper to pay for on the requests
				// that are not MFA verifications — which is almost all of them.
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(&challengeWriter{ResponseWriter: w, challenges: challenges}, r)
		})
	}
}

// challengeWriter clears the pending cookie on any response that issues a
// session. "Issues a session" is read from the outgoing Set-Cookie rather than
// told to it, so a handler added later gets this without knowing it exists.
type challengeWriter struct {
	http.ResponseWriter

	challenges *challenge.Manager
	done       bool
}

func (w *challengeWriter) WriteHeader(status int) {
	w.reconcile()
	w.ResponseWriter.WriteHeader(status)
}

func (w *challengeWriter) Write(b []byte) (int, error) {
	// A handler that writes without calling WriteHeader has still committed to
	// a 200, and the headers go out with it.
	w.reconcile()
	return w.ResponseWriter.Write(b)
}

// Flush and Unwrap keep the wrapper transparent, exactly as csrfWriter's do.
func (w *challengeWriter) Flush() {
	w.reconcile()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *challengeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *challengeWriter) reconcile() {
	if w.done {
		return
	}
	w.done = true

	for _, raw := range slices.Clone(w.Header().Values("Set-Cookie")) {
		set, err := http.ParseSetCookie(raw)
		if err != nil || set.Name != session.CookieName {
			continue
		}
		if set.Value == "" || set.MaxAge < 0 {
			// A session being *cleared* — a logout. The pending cookie is not
			// this response's business, and a login that answered mfa_required
			// on the way past must not have its brand-new cookie deleted.
			continue
		}
		http.SetCookie(w.ResponseWriter, w.challenges.ClearCookie())
		return
	}
}
