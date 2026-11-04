package session

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world"
)

type networkEncodedOffsetType struct{}

func (networkEncodedOffsetType) EncodeEntity() string                        { return "test:minecart" }
func (networkEncodedOffsetType) NetworkEncodeEntity() string                 { return "minecraft:minecart" }
func (networkEncodedOffsetType) BBox(world.Entity) cube.BBox                 { return cube.BBox{} }
func (networkEncodedOffsetType) DecodeNBT(map[string]any, *world.EntityData) {}
func (networkEncodedOffsetType) EncodeNBT(*world.EntityData) map[string]any  { return nil }
func (networkEncodedOffsetType) Open(*world.Tx, *world.EntityHandle, *world.EntityData) world.Entity {
	return nil
}

type explicitNetworkOffsetType struct{ networkEncodedOffsetType }

func (explicitNetworkOffsetType) NetworkOffset() float64 { return 0.7 }

func TestEntityNetworkIdentifierUsesClientFacingIdentifier(t *testing.T) {
	t.Parallel()
	if got := entityNetworkIdentifier(networkEncodedOffsetType{}); got != "minecraft:minecart" {
		t.Fatalf("entityNetworkIdentifier() = %q, want minecraft:minecart", got)
	}
}

func TestEntityTypeOffsetUsesDefaultAfterExplicitOverride(t *testing.T) {
	t.Parallel()
	if got := entityTypeOffset(networkEncodedOffsetType{}); got != 0.5 {
		t.Fatalf("network-encoded minecart offset = %v, want 0.5", got)
	}
	if got := entityTypeOffset(explicitNetworkOffsetType{}); got != 0.7 {
		t.Fatalf("explicit offset = %v, want 0.7", got)
	}
	if got := entityTypeOffset(entity.TextType); got != 0 {
		t.Fatalf("text surrogate offset = %v, want 0", got)
	}
}
