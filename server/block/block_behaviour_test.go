package block_test

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world"
)

// TestBlockRegistryBeforeDefaultFinalization verifies that independent registries can be finalized
// before the default registry and keep their block lookups when it is finalized later.
func TestBlockRegistryBeforeDefaultFinalization(t *testing.T) {
	const childEnv = "DRAGONFLY_TEST_REGISTRY_CHILD"
	if os.Getenv(childEnv) != "1" {
		// Use a fresh process so other tests cannot finalize the default registry first.
		cmd := exec.Command(os.Args[0], "-test.run=^TestBlockRegistryBeforeDefaultFinalization$")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("registry subprocess failed: %v\n%s", err, output)
		}
		return
	}

	block.NextHash()
	registries := []world.BlockRegistry{world.DefaultBlockRegistry.Clone(), world.NewBlockRegistry()}
	for _, registry := range registries {
		registry.Finalize()
	}
	blocks := registries[0].Blocks()
	hashes := make([][2]uint64, len(blocks))
	for rid, b := range blocks {
		base, state := b.Hash()
		hashes[rid] = [2]uint64{base, state}
	}

	world.DefaultBlockRegistry.Finalize()
	for rid, b := range blocks {
		base, state := b.Hash()
		if got := [2]uint64{base, state}; got != hashes[rid] {
			t.Fatalf("hash of %T changed after default finalization: %v != %v", b, got, hashes[rid])
		}
		for _, registry := range registries {
			if got := registry.BlockRuntimeID(b); got != uint32(rid) {
				t.Fatalf("runtime ID of %T = %d, want %d", b, got, rid)
			}
		}
	}
}

// TestTorchBreaksWithoutSupport verifies that a torch is broken by a neighbour
// update on the tick after its supporting block is removed, using a
// synchronous World to make the tick deterministic.
func TestTorchBreaksWithoutSupport(t *testing.T) {
	w := world.Config{Synchronous: true, Entities: entity.DefaultRegistry}.New()
	defer w.Close()

	support, torch := cube.Pos{0, 0, 0}, cube.Pos{0, 1, 0}
	w.Do(func(tx *world.Tx) {
		tx.SetBlock(support, block.Stone{}, nil)
		tx.SetBlock(torch, block.Torch{Facing: cube.FaceDown}, nil)
		tx.SetBlock(support, block.Air{}, nil)
	})
	w.AdvanceTick()

	b, err := world.Call(context.Background(), w, func(tx *world.Tx) (world.Block, error) {
		return tx.Block(torch), nil
	})
	if err != nil {
		t.Fatalf("read torch block: %v", err)
	}
	if b != (block.Air{}) {
		t.Errorf("expected torch to break after removing its support, got %v", b)
	}
}
