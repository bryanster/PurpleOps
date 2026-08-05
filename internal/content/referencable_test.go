package content_test

import (
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

func TestAssertReferencable(t *testing.T) {
	t.Parallel()

	enabled := storecontent.Source{ID: storecontent.SourceIDAttack, Name: "ATT&CK", Enabled: true}
	if err := content.AssertReferencable(enabled); err != nil {
		t.Fatalf("enabled source refused: %v", err)
	}

	disabled := storecontent.Source{ID: storecontent.SourceIDAttack, Name: "ATT&CK Enterprise", Enabled: false}
	err := content.AssertReferencable(disabled)
	if err == nil {
		t.Fatal("disabled source accepted")
	}
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
}
