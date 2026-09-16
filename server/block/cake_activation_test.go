package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type cakeActivationTestUser struct {
	item.User
	held item.Stack
}

// HeldItems supplies the item used on the cake.
func (u *cakeActivationTestUser) HeldItems() (item.Stack, item.Stack) { return u.held, item.Stack{} }

// Position supplies a location for eating sounds.
func (*cakeActivationTestUser) Position() mgl64.Vec3 { return mgl64.Vec3{0, 64, 0} }

// Saturate makes this actor eligible for the cake's eating action.
func (*cakeActivationTestUser) Saturate(int, float64) {}

func TestCakeActivationSeparatesEatingFromIgnition(t *testing.T) {
	for _, held := range []item.Stack{
		item.NewStack(item.FlintAndSteel{}, 1),
		item.NewStack(item.FireCharge{}, 1),
		item.NewStack(item.Sword{Tier: item.ToolTierIron}, 1).WithEnchantments(item.NewEnchantment(enchantment.FireAspect, 1)),
	} {
		for _, candle := range []bool{false, true} {
			name, _ := held.Item().EncodeItem()
			t.Run(name+"/"+map[bool]string{false: "plain", true: "candle"}[candle], func(t *testing.T) {
				pos := cube.Pos{0, 64, 0}
				cake := Cake{Candle: candle}
				src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: cake})
				view := &placementView{src: src, known: true}
				complete := world.RunBlockTransaction(view, func(tx *world.Tx) {
					cake.Activate(pos, cube.FaceNorth, tx, &cakeActivationTestUser{held: held}, &item.UseContext{})
				})
				if !complete {
					t.Fatal("activation unexpectedly attempted non-block effects")
				}
				got := view.Block(pos).(Cake)
				if !candle && got.Bites != 1 {
					t.Fatalf("plain cake bites=%d, want1", got.Bites)
				}
				if candle && got.Bites != 0 {
					t.Fatalf("ignition consumed cake: %+v", got)
				}
			})
		}
	}
}
