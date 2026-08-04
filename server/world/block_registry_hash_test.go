package world

import "testing"

// NetworkBlockHash is the key Finalize sorts by, so a caller reasoning about palette
// ordering gets the same answer the registry does. The values are pinned because they are
// a wire contract with the client, not an implementation detail either side may change.
func TestNetworkBlockHash_IsStable(t *testing.T) {
	for name, want := range map[string]uint64{
		"minecraft:air":   0xbd584baf00003448,
		"minecraft:stone": 0x2727739c84aa7913,
	} {
		if got := NetworkBlockHash(name); got != want {
			t.Errorf("NetworkBlockHash(%q) = %#016x, want %#016x", name, got, want)
		}
	}
}
