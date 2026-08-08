package password

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// redacted is what a [Plaintext] renders as, whoever asks. It is deliberately
// the same for an empty password as for a long one: a length is a fact about a
// secret, and a log line is not the place to publish it.
const redacted = "[redacted]"

// Plaintext is a password as its owner typed it, on its way to [Hash] or
// [Verify] and nowhere else.
//
// It is a distinct type so that the compiler, not a reviewer's memory, decides
// where a password may go. Every way a value ordinarily reaches a log or a
// response is overridden to produce [redacted]: fmt verbs (including %q and
// %#v, and including when the value is a field of a struct being printed),
// log/slog attributes, and JSON encoding. Reading the characters back requires
// [Plaintext.Reveal], which is greppable.
//
// The zero value is the empty password, which [Validate] rejects.
type Plaintext string

// Reveal returns the password itself. Call it at the point of use — hashing,
// verifying, or handing the value to a library that needs a string — and do
// not store what it returns.
func (p Plaintext) Reveal() string { return string(p) }

// String implements fmt.Stringer for callers that reach for it directly.
func (Plaintext) String() string { return redacted }

// GoString implements fmt.GoStringer, so %#v is redacted too.
func (Plaintext) GoString() string { return redacted }

// Format implements fmt.Formatter, which is what makes the redaction total:
// fmt consults it for every verb, so %s, %q, %v, %x and anything else all
// produce [redacted] rather than only the verbs Stringer covers.
func (Plaintext) Format(f fmt.State, verb rune) {
	switch verb {
	case 'q':
		fmt.Fprintf(f, "%q", redacted)
	default:
		fmt.Fprint(f, redacted)
	}
}

// LogValue implements slog.LogValuer, so slog.Any("password", p) and a struct
// logged with slog alike record [redacted].
func (Plaintext) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON implements json.Marshaler. A password is only ever an input, so
// serializing one means it is on its way back out to a client or into a file,
// and the value is not what should arrive.
func (Plaintext) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

// UnmarshalJSON implements json.Unmarshaler, so a request body can be decoded
// straight into a field of this type. It is the only way a Plaintext is
// populated from the outside.
func (p *Plaintext) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		// The error comes from the decoder and describes the JSON, not the
		// value: json.Unmarshal into a string reports the type it found.
		return err
	}
	*p = Plaintext(s)
	return nil
}

// MarshalText implements encoding.TextMarshaler, which covers the encoders
// that prefer it to MarshalJSON — and, with it, url.Values and anything else
// that formats through the text interfaces.
func (Plaintext) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// UnmarshalText implements encoding.TextUnmarshaler, the counterpart to
// [Plaintext.UnmarshalJSON] for form and query decoding.
func (p *Plaintext) UnmarshalText(data []byte) error {
	*p = Plaintext(data)
	return nil
}
