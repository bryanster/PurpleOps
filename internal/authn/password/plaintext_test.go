package password_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/bryanster/purpleops/internal/authn/password"
)

// theSecret is distinctive enough that finding it in any output is unambiguous.
const theSecret = "hunter2-swordfish-correct-horse"

// loginRequest stands in for the request body M1-003 will decode. The point is
// that it is an ordinary struct with an ordinary mix of fields: nothing about
// logging or printing one has to be done carefully.
type loginRequest struct {
	Email    string
	Password password.Plaintext
}

func TestAPlaintextRedactsItselfEverywhereItCouldBePrinted(t *testing.T) {
	t.Parallel()

	secret := password.Plaintext(theSecret)
	body := loginRequest{Email: "alice@example.com", Password: secret}

	// Every way a value reaches a log or a response in this codebase.
	rendered := map[string]string{
		"%s":                fmt.Sprintf("%s", secret),
		"%q":                fmt.Sprintf("%q", secret),
		"%v":                fmt.Sprintf("%v", secret),
		"%+v":               fmt.Sprintf("%+v", secret),
		"%#v":               fmt.Sprintf("%#v", secret),
		"%x":                fmt.Sprintf("%x", secret),
		"String()":          secret.String(),
		"print":             fmt.Sprint(secret),
		"wrapped in error":  fmt.Errorf("login failed for %v", secret).Error(),
		"struct %v":         fmt.Sprintf("%v", body),
		"struct %+v":        fmt.Sprintf("%+v", body),
		"struct %#v":        fmt.Sprintf("%#v", body),
		"pointer to struct": fmt.Sprintf("%+v", &body),
	}
	for name, got := range rendered {
		if strings.Contains(got, theSecret) {
			t.Errorf("%s printed the password: %q", name, got)
		}
		if !strings.Contains(got, "[redacted]") {
			t.Errorf("%s = %q, want it to say the value was redacted", name, got)
		}
	}

	// JSON, for a value on its way back to a client or into a file.
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() = %v, want nil", err)
	}
	if strings.Contains(string(encoded), theSecret) {
		t.Errorf("JSON contains the password: %s", encoded)
	}
}

func TestAPlaintextIsRedactedInALogLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	secret := password.Plaintext(theSecret)
	log.Info("login attempt",
		slog.Any("password", secret),
		slog.Any("request", loginRequest{Email: "alice@example.com", Password: secret}),
		slog.String("interpolated", fmt.Sprintf("password=%v", secret)))

	if strings.Contains(buf.String(), theSecret) {
		t.Errorf("the log line contains the password: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "[redacted]") {
		t.Errorf("the log line does not say the value was redacted: %s", buf.String())
	}
}

func TestAPlaintextDecodesFromARequestBody(t *testing.T) {
	t.Parallel()

	// Redaction is one-way: a password still has to arrive from a client, or
	// nobody could log in.
	var body loginRequest
	if err := json.Unmarshal([]byte(`{"Email":"alice@example.com","Password":"`+theSecret+`"}`), &body); err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	if body.Password.Reveal() != theSecret {
		t.Errorf("decoded password is not what was sent")
	}

	// And a non-string is a decoding error, not a password of "42".
	if err := json.Unmarshal([]byte(`{"Password":42}`), &body); err == nil {
		t.Error("Unmarshal() of a number into a password = nil, want an error")
	}
}

func TestRevealIsTheOnlyWayToTheCharacters(t *testing.T) {
	t.Parallel()

	secret := password.Plaintext(theSecret)
	if secret.Reveal() != theSecret {
		t.Errorf("Reveal() = %q, want the password itself", secret.Reveal())
	}
	// A conversion is the other way, and is deliberate enough to see in review;
	// what matters is that nothing implicit produces it.
	if string(secret) != theSecret {
		t.Error("converting to string did not produce the password")
	}
}
