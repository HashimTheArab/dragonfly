package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
)

func TestMayActivate_HeldItemAndStance(t *testing.T) {
	stone := item.NewStack(Stone{}, 1)
	flaming := item.NewStack(item.Sword{Tier: item.ToolTierDiamond}, 1).WithEnchantments(item.NewEnchantment(enchantment.FireAspect, 1))
	fullCampfire := Campfire{}
	for i := range fullCampfire.Items {
		fullCampfire.Items[i].Item = item.NewStack(item.Beef{}, 1)
	}
	for _, test := range []struct {
		name           string
		block          world.Block
		held           item.Stack
		sneaking, want bool
	}{
		{name: "ordinary block", block: Stone{}, held: stone},
		{name: "chest", block: Chest{}, held: stone, want: true},
		{name: "sneaking with block", block: Chest{}, held: stone, sneaking: true},
		{name: "sneaking empty handed", block: Chest{}, sneaking: true, want: true},
		{name: "TNT declines block", block: TNT{}, held: stone},
		{name: "TNT fire aspect", block: TNT{}, held: flaming, want: true},
		{name: "TNT ignition item falls through", block: TNT{}, held: item.NewStack(item.FlintAndSteel{}, 1)},
		{name: "candle declines block", block: Candle{}, held: stone},
		{name: "candle fire aspect", block: Candle{}, held: flaming, want: true},
		{name: "composter conservative activation", block: Composter{}, held: stone, want: true},
		{name: "composter ready", block: Composter{Level: 8}, held: stone, want: true},
		{name: "empty jukebox conservative activation", block: Jukebox{}, held: stone, want: true},
		{name: "jukebox accepts disc", block: Jukebox{}, held: item.NewStack(item.MusicDisc{DiscType: sound.Disc13()}, 1), want: true},
		{name: "empty lectern conservative activation", block: Lectern{}, held: stone, want: true},
		{name: "campfire declines block", block: Campfire{}, held: stone},
		{name: "campfire cooks food", block: Campfire{}, held: item.NewStack(item.Beef{}, 1), want: true},
		{name: "full campfire declines food", block: fullCampfire, held: item.NewStack(item.Beef{}, 1)},
		{name: "campfire shovel", block: Campfire{}, held: item.NewStack(item.Shovel{Tier: item.ToolTierIron}, 1), want: true},
		{name: "extinguished campfire declines shovel", block: Campfire{Extinguished: true}, held: item.NewStack(item.Shovel{Tier: item.ToolTierIron}, 1)},
		{name: "redstone ore falls through", block: RedstoneOre{}, held: stone},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := MayActivate(test.block, test.held, test.sneaking); got != test.want {
				t.Fatalf("MayActivate = %t, want %t", got, test.want)
			}
		})
	}
}

type activationTestUser struct {
	item.User
	held item.Stack
}

// HeldItems supplies only the inventory fact used by conditional activations.
func (u activationTestUser) HeldItems() (item.Stack, item.Stack) { return u.held, item.Stack{} }

func TestConditionalActivation_DeclinesPlacementItem(t *testing.T) {
	for _, b := range []world.Block{TNT{}, Candle{}, Campfire{}, RedstoneOre{}} {
		name, _ := b.EncodeBlock()
		t.Run(name, func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			defer w.Close()
			runWorld(w, func(tx *world.Tx) {
				pos := cube.Pos{0, 64, 0}
				tx.SetBlock(pos.Side(cube.FaceDown), Stone{}, nil)
				tx.SetBlock(pos, b, nil)
				u := activationTestUser{held: item.NewStack(Stone{}, 1)}
				if MayActivate(b, u.held, false) || b.(Activatable).Activate(pos, cube.FaceUp, tx, u, &item.UseContext{}) {
					t.Fatal("placement item consumed by activation")
				}
				if ore, ok := b.(RedstoneOre); ok {
					ore.Lit = true
					if tx.Block(pos) != ore {
						t.Fatal("fall-through lost redstone ore's lighting side effect")
					}
				}
			})
		})
	}
}
