package customentry

import (
	"math"
	"slices"
	"testing"

	"github.com/df-mc/dragonfly/server/block/customblock"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// namedBlock is a custom block without states, named at will.
type namedBlock struct{ name string }

func (b namedBlock) EncodeBlock() (string, map[string]any) { return b.name, map[string]any{} }
func (namedBlock) Hash() (uint64, uint64)                  { return 0, math.MaxUint64 }
func (namedBlock) Model() world.BlockModel                 { return nil }
func (namedBlock) Properties() customblock.Properties      { return customblock.Properties{Cube: true} }

// A client that builds its palette from the entries of a server with hashed network IDs hashes a
// salted block's states as the server does. The names collide unsalted (see the world package).
func TestSaltedEntriesHashAsTheServerDoes(t *testing.T) {
	names := []string{"test:collide_1029599", "test:collide_1632382"}
	server := world.NewBlockRegistry().(*world.BasicBlockRegistry)
	server.UseHashedNetworkIDs()
	for _, name := range names {
		server.RegisterBlock(namedBlock{name})
	}
	server.Finalize()

	var entries []protocol.BlockEntry
	salted := 0
	for i, name := range names {
		entry := Block(namedBlock{name}, 10000+int32(i))
		if salt, ok := server.NetworkHashSalt(name); ok {
			DeclareNetworkHashSalt(&entry, salt)
			salted++
		}
		entries = append(entries, entry)
	}
	if salted != 1 {
		t.Fatalf("server salted %d blocks, want 1", salted)
	}
	client := world.NewBlockRegistry().(*world.BasicBlockRegistry)
	client.UseHashedNetworkIDs()
	client.Finalize()
	if err := world.AddCustomBlocks(client, entries); err != nil {
		t.Fatalf("client AddCustomBlocks() error = %v", err)
	}
	for _, name := range names {
		rid, _ := server.StateToRuntimeID(name, map[string]any{})
		hash, _ := server.RuntimeIDToHash(rid)
		clientRID, ok := client.HashToRuntimeID(hash)
		if !ok {
			t.Fatalf("client has no state with %s's hash %#08x", name, hash)
		}
		if got, _, _ := client.RuntimeIDToState(clientRID); got != name {
			t.Fatalf("client resolves %s's hash %#08x to %s", name, hash, got)
		}
	}
}

func propertyNames(entry protocol.BlockEntry) []string {
	var names []string
	for _, p := range entry.Properties["properties"].([]map[string]any) {
		names = append(names, p["name"].(string))
	}
	return names
}

// The salt is declared as a one-value property among the block's own, in name order.
func TestDeclareNetworkHashSaltKeepsPropertiesSorted(t *testing.T) {
	entry := protocol.BlockEntry{Name: "test:block", Properties: map[string]any{"properties": []map[string]any{
		{"name": "a:first", "enum": []any{int32(0), int32(1)}},
		{"name": "z:last", "enum": []any{false, true}},
	}}}
	DeclareNetworkHashSalt(&entry, 7)

	if got, want := propertyNames(entry), []string{"a:first", world.NetworkHashSaltProperty, "z:last"}; !slices.Equal(got, want) {
		t.Fatalf("properties = %v, want %v", got, want)
	}
	salt := entry.Properties["properties"].([]map[string]any)[1]
	if values := salt["enum"].([]any); len(values) != 1 || values[0] != int32(7) {
		t.Fatalf("salt values = %v, want [7]", values)
	}
}

func TestDeclareNetworkHashSaltOnBlockWithoutProperties(t *testing.T) {
	entry := protocol.BlockEntry{Name: "test:plain", Properties: map[string]any{}}
	DeclareNetworkHashSalt(&entry, 1)
	if got, want := propertyNames(entry), []string{world.NetworkHashSaltProperty}; !slices.Equal(got, want) {
		t.Fatalf("properties = %v, want %v", got, want)
	}
}
