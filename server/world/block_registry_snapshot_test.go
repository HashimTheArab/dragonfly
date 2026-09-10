package world

import (
	"bytes"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"testing"
)

// TestBlockRegistrySnapshotRoundTrip preserves session runtime IDs and typed state properties.
func TestBlockRegistrySnapshotRoundTrip(t *testing.T) {
	r := NewBlockRegistry()
	r.RegisterBlockState(BlockState{Name: "server:snapshot_test", Properties: map[string]any{"level": int32(3), "active": uint8(1), "kind": "blue"}})
	r.Finalize()
	data, err := MarshalBlockRegistry(r)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := UnmarshalBlockRegistry(data)
	if err != nil {
		t.Fatal(err)
	}
	for rid := range r.BlockCount() {
		name, props, _ := r.RuntimeIDToState(uint32(rid))
		got, ok := restored.StateToRuntimeID(name, props)
		if !ok || got != uint32(rid) {
			t.Fatalf("runtime ID %d restored as %d (%v)", rid, got, ok)
		}
	}
}

// TestBlockRegistrySnapshotRejectsDuplicates rejects ambiguous runtime-ID layouts.
func TestBlockRegistrySnapshotRejectsDuplicates(t *testing.T) {
	r := NewBlockRegistry()
	r.Finalize()
	data, err := MarshalBlockRegistry(r)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		States []BlockState `nbt:"states"`
	}
	if err := nbt.NewDecoder(bytes.NewReader(data)).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.States[0] = snapshot.States[1]
	var out bytes.Buffer
	if err := nbt.NewEncoder(&out).Encode(snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := UnmarshalBlockRegistry(out.Bytes()); err == nil {
		t.Fatal("duplicate block state accepted")
	}
}

// TestBlockRegistrySnapshotRejectsMalformedData checks that corrupt captures return an error.
func TestBlockRegistrySnapshotRejectsMalformedData(t *testing.T) {
	if _, err := UnmarshalBlockRegistry([]byte{1, 2, 3}); err == nil {
		t.Fatal("malformed snapshot accepted")
	}
}

// TestBlockRegistrySnapshotAppendedCustomPalette preserves the finalized-registry custom append API's runtime IDs.
func TestBlockRegistrySnapshotAppendedCustomPalette(t *testing.T) {
	registry, err := NewCustomBlockRegistry([]protocol.BlockEntry{{Name: "server:appended_snapshot"}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalBlockRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := UnmarshalBlockRegistry(data)
	if err != nil {
		t.Fatal(err)
	}
	for rid := range registry.BlockCount() {
		name, props, _ := registry.RuntimeIDToState(uint32(rid))
		actual, ok := restored.StateToRuntimeID(name, props)
		if !ok || actual != uint32(rid) {
			t.Fatalf("runtime ID %d restored as %d (%v)", rid, actual, ok)
		}
	}
}

// TestBlockRegistrySnapshotRejectsChangedPropertyName checks exact snapshot state identity.
func TestBlockRegistrySnapshotRejectsChangedPropertyName(t *testing.T) {
	r := NewBlockRegistry()
	r.Finalize()
	data, err := MarshalBlockRegistry(r)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		States []BlockState `nbt:"states"`
	}
	if err := nbt.NewDecoder(bytes.NewReader(data)).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	changed := false
	for i := range snapshot.States {
		if len(snapshot.States[i].Properties) != 1 {
			continue
		}
		for key, value := range snapshot.States[i].Properties {
			delete(snapshot.States[i].Properties, key)
			snapshot.States[i].Properties["review_invalid_property"] = value
			t.Logf("changed property %s of %s at rid%d", key, snapshot.States[i].Name, i)
			changed = true
		}
		break
	}
	if !changed {
		t.Fatal("missing test state")
	}
	var out bytes.Buffer
	if err := nbt.NewEncoder(&out).Encode(snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := UnmarshalBlockRegistry(out.Bytes()); err == nil {
		t.Fatal("restored different property names without rejecting snapshot")
	}
}
