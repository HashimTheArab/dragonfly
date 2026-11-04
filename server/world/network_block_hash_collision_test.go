package world

import (
	"math"
	"strings"
	"testing"

	"github.com/df-mc/dragonfly/server/block/customblock"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// Two block names whose network block hashes collide, found by searching "test:collide_<n>".
const collideA, collideB = "test:collide_1029599", "test:collide_1632382"

// namedBlock is a custom block without states, named at will.
type namedBlock struct{ name string }

func (b namedBlock) EncodeBlock() (string, map[string]any) { return b.name, map[string]any{} }
func (namedBlock) Hash() (uint64, uint64)                  { return 0, math.MaxUint64 }
func (namedBlock) Model() BlockModel                       { return nil }
func (namedBlock) Properties() customblock.Properties      { return customblock.Properties{Cube: true} }

// saltStateBlock declares the state the registry reserves for salts.
type saltStateBlock struct{ namedBlock }

func (saltStateBlock) States() map[string][]any {
	return map[string][]any{NetworkHashSaltProperty: {int32(0)}}
}
func (saltStateBlock) Permutations() []customblock.Permutation { return nil }

func requireCollision(t *testing.T) uint32 {
	t.Helper()
	a, _ := networkBlockHash(collideA, map[string]any{}, nil)
	b, _ := networkBlockHash(collideB, map[string]any{}, nil)
	if a != b {
		t.Fatalf("fixture names no longer collide: %#08x and %#08x", a, b)
	}
	return a
}

// registryWith returns a finalized registry holding a custom block of each name, registered in
// the order given, using hashed network IDs if hashed is set.
func registryWith(t *testing.T, hashed bool, names ...string) *BasicBlockRegistry {
	t.Helper()
	registry := NewBlockRegistry().(*BasicBlockRegistry)
	if hashed {
		registry.UseHashedNetworkIDs()
	}
	for _, name := range names {
		registry.RegisterBlock(namedBlock{name})
	}
	registry.Finalize()
	return registry
}

func runtimeIDOf(t *testing.T, registry *BasicBlockRegistry, name string) uint32 {
	t.Helper()
	rid, ok := registry.StateToRuntimeID(name, map[string]any{})
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	return rid
}

// Without hashed network IDs, states may share a hash: Finalize does not panic, nothing is salted,
// and the shared hash resolves to the lower runtime ID.
func TestFinalizeKeepsNetworkHashCollisionsWithoutHashedIDs(t *testing.T) {
	hash := requireCollision(t)
	registry := registryWith(t, false, collideA, collideB)

	a, b := runtimeIDOf(t, registry, collideA), runtimeIDOf(t, registry, collideB)
	for _, rid := range []uint32{a, b} {
		if got, _ := registry.RuntimeIDToHash(rid); got != hash {
			t.Fatalf("RuntimeIDToHash(%d) = %#08x, want the shared %#08x", rid, got, hash)
		}
	}
	if got, _ := registry.HashToRuntimeID(hash); got != min(a, b) {
		t.Fatalf("HashToRuntimeID(%#08x) = %d, want the lower runtime ID %d", hash, got, min(a, b))
	}
	for _, name := range []string{collideA, collideB} {
		if salt, ok := registry.NetworkHashSalt(name); ok {
			t.Fatalf("%s salted with %d without hashed network IDs", name, salt)
		}
	}
}

// With hashed network IDs, Finalize salts one of two colliding custom blocks, whatever order they
// were registered in, and every state then has its own hash.
func TestFinalizeSaltsCollidingCustomBlockWithHashedIDs(t *testing.T) {
	requireCollision(t)
	registry := registryWith(t, true, collideA, collideB)
	reversed := registryWith(t, true, collideB, collideA)

	seen := make(map[uint32]uint32, registry.BlockCount())
	for rid := range uint32(registry.BlockCount()) {
		hash, _ := registry.RuntimeIDToHash(rid)
		if other, ok := seen[hash]; ok {
			t.Fatalf("runtime IDs %d and %d share network hash %#08x", other, rid, hash)
		}
		seen[hash] = rid
		if got, _ := registry.HashToRuntimeID(hash); got != rid {
			t.Fatalf("HashToRuntimeID(%#08x) = %d, want %d", hash, got, rid)
		}
	}

	var salted []string
	for _, name := range []string{collideA, collideB} {
		salt, ok := registry.NetworkHashSalt(name)
		if !ok {
			continue
		}
		salted = append(salted, name)
		want, _ := networkBlockHash(name, map[string]any{NetworkHashSaltProperty: salt}, nil)
		if got, _ := registry.RuntimeIDToHash(runtimeIDOf(t, registry, name)); got != want {
			t.Fatalf("%s hashes as %#08x, want its salted hash %#08x", name, got, want)
		}
		if otherSalt, _ := reversed.NetworkHashSalt(name); otherSalt != salt {
			t.Fatalf("%s salted with %d, or %d when registered in reverse", name, salt, otherSalt)
		}
	}
	if len(salted) != 1 {
		t.Fatalf("salted %v, want exactly one of the colliding blocks", salted)
	}
	for rid := range uint32(registry.BlockCount()) {
		a, _ := registry.RuntimeIDToHash(rid)
		b, _ := reversed.RuntimeIDToHash(rid)
		if a != b {
			t.Fatalf("runtime ID %d hashes as %#08x, or %#08x when registered in reverse", rid, a, b)
		}
	}
}

// With hashed network IDs, a collision between two states that are not custom blocks cannot be
// salted, and Finalize names both.
func TestFinalizeRefusesUnsaltableCollisionWithHashedIDs(t *testing.T) {
	requireCollision(t)
	registry := NewBlockRegistry().(*BasicBlockRegistry)
	registry.UseHashedNetworkIDs()
	registry.RegisterBlockState(BlockState{Name: collideA, Properties: map[string]any{}})
	registry.RegisterBlockState(BlockState{Name: collideB, Properties: map[string]any{}})
	defer func() {
		err, _ := recover().(error)
		if err == nil || !strings.Contains(err.Error(), collideA) || !strings.Contains(err.Error(), collideB) {
			t.Fatalf("Finalize panicked with %v, want an error naming %s and %s", err, collideA, collideB)
		}
	}()
	registry.Finalize()
}

func TestRegisterBlockRejectsNetworkHashSaltState(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RegisterBlock accepted a block declaring the reserved salt state")
		}
	}()
	NewBlockRegistry().RegisterBlock(saltStateBlock{namedBlock{"test:salted"}})
}

// A finalized registry takes appended states that share a hash, unless runtime IDs go to clients as
// network block hashes, where the client could not tell them apart.
func TestAddCustomBlocksRefusesCollisionOnlyWithHashedIDs(t *testing.T) {
	requireCollision(t)
	entries := []protocol.BlockEntry{{Name: collideA}, {Name: collideB}}
	if _, err := NewCustomBlockRegistry(entries); err != nil {
		t.Fatalf("NewCustomBlockRegistry() error = %v, want the collision accepted", err)
	}

	hashed := NewBlockRegistry().(*BasicBlockRegistry)
	hashed.UseHashedNetworkIDs()
	hashed.Finalize()
	err := AddCustomBlocks(hashed, entries)
	if err == nil || !strings.Contains(err.Error(), collideA) || !strings.Contains(err.Error(), collideB) {
		t.Fatalf("AddCustomBlocks() error = %v, want one naming %s and %s", err, collideA, collideB)
	}
}
