package session

import (
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestDecodeResolvedStack_InitialisesContainersWithoutNBT(t *testing.T) {
	for _, resolved := range []world.Item{block.Chest{}, block.Barrel{}, block.ShulkerBox{}} {
		name, _ := resolved.EncodeItem()
		t.Run(name, func(t *testing.T) {
			for _, data := range []map[string]any{nil, {}} {
				wire := protocol.ItemStack{Count: 1, NBTData: data}
				first, err := DecodeResolvedStack(resolved, wire, StackDecodeOptions{DecodeEmptyNBT: true})
				if err != nil {
					t.Fatal(err)
				}
				second, err := DecodeResolvedStack(resolved, wire, StackDecodeOptions{DecodeEmptyNBT: true})
				if err != nil {
					t.Fatal(err)
				}
				firstInv := first.Item().(block.Container).Inventory(nil, cube.Pos{})
				secondInv := second.Item().(block.Container).Inventory(nil, cube.Pos{})
				if firstInv == nil || secondInv == nil || firstInv.Size() != 27 || secondInv.Size() != 27 {
					t.Fatal("decoded container did not initialise its inventory")
				}
				if firstInv == secondInv {
					t.Fatal("separately decoded stacks shared their inventory")
				}
			}
		})
	}
}

func TestDecodeResolvedStack_AppliesStackNBT(t *testing.T) {
	resolved := item.Sword{Tier: item.ToolTierDiamond}
	stack, err := DecodeResolvedStack(resolved, protocol.ItemStack{
		Count: 2,
		NBTData: map[string]any{
			"Damage":     int32(12),
			"RepairCost": int32(3),
			"display": map[string]any{
				"Name": "Named sword",
				"Lore": []any{"First line", "Second line"},
			},
			"ench": []any{map[string]any{"id": int16(9), "lvl": int16(0)}},
		},
	}, StackDecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if stack.Count() != 2 || stack.Durability() != stack.MaxDurability()-12 || stack.AnvilCost() != 3 ||
		stack.CustomName() != "Named sword" || !reflect.DeepEqual(stack.Lore(), []string{"First line", "Second line"}) {
		t.Fatalf("stack metadata was not decoded: %#v", stack)
	}
	if enchants := stack.Enchantments(); len(enchants) != 1 || enchants[0].Level() != 1 {
		t.Fatalf("enchantments = %v, want one level-1 enchantment", enchants)
	}
}

type invalidStackNBTItem struct{}

// EncodeItem identifies the invalid-NBT test item.
func (invalidStackNBTItem) EncodeItem() (string, int16) { return "test:invalid_nbt", 0 }

// EncodeNBT returns no tags for the invalid-NBT test item.
func (invalidStackNBTItem) EncodeNBT() map[string]any { return nil }

// DecodeNBT deliberately violates the decoded item contract.
func (invalidStackNBTItem) DecodeNBT(map[string]any) any { return struct{}{} }

func TestDecodeResolvedStack_InvalidDecodedTypeBecomesAir(t *testing.T) {
	stack, err := DecodeResolvedStack(invalidStackNBTItem{}, protocol.ItemStack{
		Count: 1, NBTData: map[string]any{"present": uint8(1)},
	}, StackDecodeOptions{})
	if err == nil {
		t.Fatal("invalid decoded type was not reported")
	}
	if !stack.Empty() || stack.Count() != 1 {
		t.Fatalf("invalid decoded item was not represented as air: %#v", stack)
	}
}

type invalidStackNBTBlock struct {
	block.Air
	invalidStackNBTItem
}

// EncodeItem resolves the otherwise ambiguous embedded item encodings.
func (invalidStackNBTBlock) EncodeItem() (string, int16) { return "test:invalid_nbt", 0 }

type resolvedStackBlockRegistry struct {
	world.BlockRegistry
	block world.Block
}

// BlockByRuntimeID returns the fixture's resolved block item.
func (r resolvedStackBlockRegistry) BlockByRuntimeID(uint32) (world.Block, bool) {
	return r.block, true
}

func TestStackToItem_InvalidDecodedTypeStillPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("invalid decoded type did not panic")
		}
	}()
	stackToItem(resolvedStackBlockRegistry{block: invalidStackNBTBlock{}}, protocol.ItemStack{
		BlockRuntimeID: 1, Count: 1, NBTData: map[string]any{"present": uint8(1)},
	})
}

func TestDecodeResolvedStack_EmptyNBTPolicy(t *testing.T) {
	for _, test := range []struct {
		name        string
		resolved    world.Item
		decodedName string
		decodedMeta int16
	}{
		{name: "bed", resolved: block.Bed{Colour: item.ColourRed()}, decodedName: "minecraft:bed"},
		{name: "skull", resolved: block.Skull{Type: block.ZombieHead()}, decodedName: "minecraft:skeleton_skull"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, decodeEmpty := range []bool{false, true} {
				for _, data := range []map[string]any{nil, {}} {
					stack, err := DecodeResolvedStack(test.resolved, protocol.ItemStack{Count: 1, NBTData: data},
						StackDecodeOptions{DecodeEmptyNBT: decodeEmpty})
					if err != nil {
						t.Fatal(err)
					}
					wantName, wantMeta := test.resolved.EncodeItem()
					if decodeEmpty {
						wantName, wantMeta = test.decodedName, test.decodedMeta
					}
					name, meta := stack.Item().EncodeItem()
					if name != wantName || meta != wantMeta {
						t.Fatalf("decodeEmpty=%t item=%s:%d, want %s:%d", decodeEmpty, name, meta, wantName, wantMeta)
					}
				}
			}
		})
	}
}

func TestStackToItem_PreservesResolvedVariantsWithoutNBT(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	for _, name := range []string{"minecraft:bed", "minecraft:zombie_head", "minecraft:firework_rocket", "minecraft:leather_helmet"} {
		metadata := int16(0)
		if name == "minecraft:bed" {
			metadata = int16(item.ColourRed().Uint8())
		}
		resolved, ok := world.ItemByName(name, metadata)
		if !ok {
			t.Fatalf("missing registered item %s", name)
		}
		runtimeID, meta, ok := world.ItemRuntimeID(resolved)
		if !ok {
			t.Fatalf("missing runtime ID for %s", name)
		}
		t.Run(name, func(t *testing.T) {
			for _, data := range []map[string]any{nil, {}} {
				stack := stackToItem(world.DefaultBlockRegistry, protocol.ItemStack{
					ItemType: protocol.ItemType{NetworkID: runtimeID, MetadataValue: uint32(meta)},
					Count:    1, NBTData: data,
				})
				gotName, gotMeta := stack.Item().EncodeItem()
				wantName, wantMeta := resolved.EncodeItem()
				if gotName != wantName || gotMeta != wantMeta {
					t.Fatalf("decoded item = %s:%d, want %s:%d", gotName, gotMeta, wantName, wantMeta)
				}
			}
		})
	}
}
