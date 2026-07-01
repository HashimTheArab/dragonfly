package session

import (
	"fmt"
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestRethrowPanicErrorPanicsWithOriginalValue(t *testing.T) {
	const value = "boom"
	defer func() {
		if v := recover(); v != value {
			t.Fatalf("expected panic value %q, got %v", value, v)
		}
	}()

	rethrowPanicError(fmt.Errorf("packet: %w", &world.PanicError{Value: value}))
}

func TestRethrowPanicErrorIgnoresOrdinaryError(t *testing.T) {
	rethrowPanicError(fmt.Errorf("ordinary error"))
}
