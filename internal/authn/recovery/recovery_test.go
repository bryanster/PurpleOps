package recovery_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn/recovery"
)

// What a recovery code is, tested where it is decided. Everything here is about
// the two properties M1-007 asks for and one it does not: that a code is
// unguessable, that a person can copy it onto paper and type it back, and that
// it does not end up in a log on the way past.

// testKey is thirty-two bytes so [recovery.NewHasher] accepts it. Fixed rather
// than random: nothing here depends on it being unguessable, and a random one
// would make a failure impossible to reproduce.
const testKey = "test-encryption-key-also-not-real"

func hasher(t *testing.T) *recovery.Hasher {
	t.Helper()

	h, err := recovery.NewHasher([]byte(testKey))
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	return h
}

// --- What a code is ---------------------------------------------------------

// TestAGeneratedSetIsTenDistinctCodesOverTheAlphabet is the acceptance
// criterion about entropy and transcribability, as far as a shape can carry it.
func TestAGeneratedSetIsTenDistinctCodesOverTheAlphabet(t *testing.T) {
	t.Parallel()

	codes, err := recovery.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(codes) != recovery.SetSize {
		t.Fatalf("%d codes, want %d", len(codes), recovery.SetSize)
	}

	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		raw := code.Reveal()
		if len(raw) != recovery.Length {
			t.Errorf("a code has %d characters, want %d", len(raw), recovery.Length)
		}
		// The ambiguous four are what M1-007 asks to be absent. They are the
		// ones a person transcribes wrongly, and every one of them is also a
		// character Parse folds away — so their presence here would mean two
		// different codes could be typed identically.
		if strings.ContainsAny(raw, "ILOU") {
			t.Errorf("a code contains one of I, L, O or U, which look like other characters")
		}
		if seen[raw] {
			t.Error("Generate produced the same code twice in one set")
		}
		seen[raw] = true
	}
}

// TestTheGeneratorUsesItsWholeAlphabet is the entropy claim, checked the only
// way a test can check it: a generator that had collapsed onto a subset — a
// broken mask, a truncated alphabet — would show up as characters that never
// appear.
//
// Two hundred codes is four thousand characters over thirty-two symbols, so a
// symbol appearing zero times has probability about 32·(31/32)^4000, which is
// far below any threshold worth worrying about. It is not a randomness test and
// does not pretend to be; it is a test that the mapping is onto.
func TestTheGeneratorUsesItsWholeAlphabet(t *testing.T) {
	t.Parallel()

	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

	counts := map[rune]int{}
	for range 20 {
		codes, err := recovery.Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		for _, code := range codes {
			for _, r := range code.Reveal() {
				counts[r]++
			}
		}
	}

	for _, r := range alphabet {
		if counts[r] == 0 {
			t.Errorf("the character %q never appeared in 4000 draws; the generator is not using its whole alphabet", r)
		}
	}
	if len(counts) != len(alphabet) {
		t.Errorf("%d distinct characters appeared, want %d", len(counts), len(alphabet))
	}
}

// TestACodeCarriesAtLeastEightyBits is the acceptance criterion stated as
// arithmetic rather than as a comment, so that shortening the code or narrowing
// the alphabet fails here instead of quietly.
func TestACodeCarriesAtLeastEightyBits(t *testing.T) {
	t.Parallel()

	const alphabetSize = 32
	bits := float64(recovery.Length) * math.Log2(alphabetSize)
	if bits < 80 {
		t.Errorf("a code carries %.0f bits, want at least 80", bits)
	}
}

// TestACodeIsPrintedInGroupsAndReadsBackTheSame is the round trip that makes
// the grouping presentation rather than data.
func TestACodeIsPrintedInGroupsAndReadsBackTheSame(t *testing.T) {
	t.Parallel()

	codes, err := recovery.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	for _, code := range codes {
		printed := code.Printed()
		if strings.Count(printed, "-") != recovery.Length/4-1 {
			t.Errorf("Printed() = %q, want groups of four separated by hyphens", printed)
		}

		back, err := recovery.Parse(printed)
		if err != nil {
			t.Fatalf("Parse(%q): %v", printed, err)
		}
		if back != code {
			t.Errorf("Parse(Printed()) is a different code")
		}
	}
}

// --- Reading one back -------------------------------------------------------

// TestParseForgivesEverythingExceptTheCode is the criterion behind the
// unambiguous alphabet: it is not enough that the printed characters cannot be
// confused, the ones somebody writes *instead* have to land on the right value.
func TestParseForgivesEverythingExceptTheCode(t *testing.T) {
	t.Parallel()

	// One canonical code, written down eight ways. The first is what the server
	// printed; the rest are what comes back.
	const canonical = "3K9M2PTVXA47QRJH58WY"

	cases := map[string]string{
		"as printed":                "3K9M-2PTV-XA47-QRJH-58WY",
		"canonical":                 canonical,
		"lower case":                "3k9m-2ptv-xa47-qrjh-58wy",
		"spaces":                    "3K9M 2PTV XA47 QRJH 58WY",
		"surrounding space":         "  3K9M-2PTV-XA47-QRJH-58WY\t",
		"grouped differently":       "3K9M2-PTVXA-47QRJ-H58WY",
		"no separators, mixed case": "3k9M2pTVxa47QrjH58wY",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := recovery.Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", input, err)
			}
			if got.Reveal() != canonical {
				t.Errorf("Parse(%q).Reveal() = %q, want %q", input, got.Reveal(), canonical)
			}
		})
	}
}

// TestParseFoldsTheOmittedCharactersOntoTheirLookAlikes states the fold on its
// own, with inputs that differ from the canonical form only by the substitution.
func TestParseFoldsTheOmittedCharactersOntoTheirLookAlikes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ typed, want string }{
		"O for 0": {"OOOO-1111-2222-3333-4444", "00001111222233334444"},
		"o for 0": {"oooo-1111-2222-3333-4444", "00001111222233334444"},
		"I for 1": {"0000-IIII-2222-3333-4444", "00001111222233334444"},
		"L for 1": {"0000-LLLL-2222-3333-4444", "00001111222233334444"},
		"l for 1": {"0000-llll-2222-3333-4444", "00001111222233334444"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := recovery.Parse(tc.typed)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.typed, err)
			}
			if got.Reveal() != tc.want {
				t.Errorf("Parse(%q).Reveal() = %q, want %q", tc.typed, got.Reveal(), tc.want)
			}
		})
	}
}

// TestParseRefusesWhatIsNotACode. Length is the line: everything shorter or
// longer is refused, because there is nothing to compare a partial code
// against.
func TestParseRefusesWhatIsNotACode(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"empty":            "",
		"too short":        "3K9M-2PTV-XA47-QRJH-58W",
		"too long":         "3K9M-2PTV-XA47-QRJH-58WYZ",
		"only separators":  "----",
		"a TOTP code":      "492817",
		"punctuation":      "3K9M-2PTV-XA47-QRJH-58W!",
		"non-ASCII":        "3K9M-2PTV-XA47-QRJH-58Wé",
		"a whole sentence": "the codes are in the drawer",
		"a UUID":           "0192f3c4-5d6e-7f80-9123-456789abcdef",
		// Twenty characters, every one of them foldable or in the alphabet, and
		// then one more. The length check is the last thing standing.
		"one character too many": "DEADBEEFDEADBEEFDEADB",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := recovery.Parse(input); !errors.Is(err, recovery.ErrMalformed) {
				t.Errorf("Parse(%q) error = %v, want ErrMalformed", input, err)
			}
		})
	}
}

// --- Hashing ----------------------------------------------------------------

// TestHashingIsDeterministicAndPerCode. Deterministic is what makes a set of
// ten comparable in one pass; per-code is what stops one code standing in for
// another.
func TestHashingIsDeterministicAndPerCode(t *testing.T) {
	t.Parallel()

	h := hasher(t)
	codes, err := recovery.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		first, second := h.Hash(code), h.Hash(code)
		if first != second {
			t.Fatal("the same code hashed to two different values")
		}
		if first == code.Reveal() {
			t.Fatal("the hash is the code")
		}
		if seen[first] {
			t.Error("two codes share a hash")
		}
		seen[first] = true
	}
}

// TestADifferentKeyProducesADifferentHash is what "keyed" means, and the reason
// a copy of the database alone is not enough to forge a code.
func TestADifferentKeyProducesADifferentHash(t *testing.T) {
	t.Parallel()

	other, err := recovery.NewHasher([]byte("a completely different encryption key"))
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}

	code, err := recovery.Parse("3K9M-2PTV-XA47-QRJH-58WY")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if hasher(t).Hash(code) == other.Hash(code) {
		t.Error("two keys produced the same hash; the hash is not keyed")
	}
}

// TestHashAllPreservesOrder: the codes handed to a person and the hashes stored
// for them are matched up by position, so a reordering would hand out ten codes
// that check against the wrong rows.
func TestHashAllPreservesOrder(t *testing.T) {
	t.Parallel()

	h := hasher(t)
	codes, err := recovery.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	hashes := h.HashAll(codes)
	if len(hashes) != len(codes) {
		t.Fatalf("HashAll returned %d hashes for %d codes", len(hashes), len(codes))
	}
	for i, code := range codes {
		if hashes[i] != h.Hash(code) {
			t.Fatalf("hash %d does not belong to code %d", i, i)
		}
	}
}

// TestANewHasherRefusesShortKeyMaterial. HKDF will happily stretch four bytes
// into a well-formed key, and the result is a well-formed key nobody has to
// guess very hard.
func TestANewHasherRefusesShortKeyMaterial(t *testing.T) {
	t.Parallel()

	if _, err := recovery.NewHasher([]byte("short")); err == nil {
		t.Error("NewHasher accepted five bytes of key material")
	}
}

// --- Not leaking one --------------------------------------------------------

// TestACodeRedactsItselfEverywhere is the criterion that codes are shown once:
// once is a property of the response, and this is what stops the second showing
// being a log line. Every ordinary way of rendering a value is covered, because
// the one that is not covered is the one that will be used.
func TestACodeRedactsItselfEverywhere(t *testing.T) {
	t.Parallel()

	code, err := recovery.Parse("3K9M-2PTV-XA47-QRJH-58WY")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	const secret = "3K9M2PTVXA47QRJH58WY"

	rendered := map[string]string{
		"%s":            fmt.Sprintf("%s", code),
		"%v":            fmt.Sprintf("%v", code),
		"%q":            fmt.Sprintf("%q", code),
		"%#v":           fmt.Sprintf("%#v", code),
		"%d":            fmt.Sprintf("%d", code),
		"String()":      code.String(),
		"inside struct": fmt.Sprintf("%v", struct{ Code recovery.Code }{code}),
	}
	for verb, got := range rendered {
		if strings.Contains(got, secret) {
			t.Errorf("%s rendered the code: %s", verb, got)
		}
		if !strings.Contains(got, "redacted") {
			t.Errorf("%s = %s, want the placeholder", verb, got)
		}
	}

	// The encoders, which is how a code would reach a response body it was not
	// meant to be in. The handler that legitimately sends one spells out
	// Printed() instead, which is the point.
	encoded, err := json.Marshal(struct {
		Code recovery.Code `json:"code"`
	}{code})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Errorf("JSON carried the code: %s", encoded)
	}

	// And slog, which is the one that matters most: it is what the service
	// writes to when it reports that a code was used.
	var logged strings.Builder
	slog.New(slog.NewTextHandler(&logged, nil)).Info("used", slog.Any("code", code))
	if strings.Contains(logged.String(), secret) {
		t.Errorf("slog carried the code: %s", logged.String())
	}
}

// TestRevealAndPrintedAreTheOnlyWayOut states the other half: the redaction is
// total *except* through the two methods a caller has to type, so showing a code
// stays a deliberate act.
func TestRevealAndPrintedAreTheOnlyWayOut(t *testing.T) {
	t.Parallel()

	code, err := recovery.Parse("3K9M-2PTV-XA47-QRJH-58WY")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := code.Reveal(); got != "3K9M2PTVXA47QRJH58WY" {
		t.Errorf("Reveal() = %q", got)
	}
	if got := code.Printed(); got != "3K9M-2PTV-XA47-QRJH-58WY" {
		t.Errorf("Printed() = %q", got)
	}
}
