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
	held      item.Stack
	food      int
	mode      world.GameMode
	saturated bool
}

// HeldItems supplies the item used on the cake.
func (u *cakeActivationTestUser) HeldItems() (item.Stack, item.Stack) { return u.held, item.Stack{} }

// Position supplies a location for eating sounds.
func (*cakeActivationTestUser) Position() mgl64.Vec3 { return mgl64.Vec3{0, 64, 0} }

// Saturate records whether eating reached the hunger update.
func (u *cakeActivationTestUser) Saturate(int, float64) { u.saturated = true }

// Food supplies the actor's current hunger level.
func (u *cakeActivationTestUser) Food() int { return u.food }

// GameMode supplies the actor's interaction and creative permissions.
func (u *cakeActivationTestUser) GameMode() world.GameMode { return u.mode }

// TestCakeActivationSeparatesEatingFromIgnition keeps item ignition separate from plain cake eating.
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
					cake.Activate(pos, cube.FaceNorth, tx, &cakeActivationTestUser{held: held, food: 18, mode: world.GameModeSurvival}, &item.UseContext{})
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

// TestCakeEatingEligibility checks hunger, game mode and peaceful difficulty before a bite.
func TestCakeEatingEligibility(t *testing.T) {
	for _, tt := range []struct {
		name       string
		food       int
		mode       world.GameMode
		difficulty world.Difficulty
		want       bool
	}{
		{"full survival", 20, world.GameModeSurvival, world.DifficultyNormal, false},
		{"hungry survival", 18, world.GameModeSurvival, world.DifficultyNormal, true},
		{"full adventure", 20, world.GameModeAdventure, world.DifficultyNormal, false},
		{"hungry adventure", 18, world.GameModeAdventure, world.DifficultyNormal, true},
		{"full creative", 20, world.GameModeCreative, world.DifficultyNormal, true},
		{"hungry spectator", 18, world.GameModeSpectator, world.DifficultyNormal, false},
		{"full peaceful survival", 20, world.GameModeSurvival, world.DifficultyPeaceful, true},
		{"full peaceful adventure", 20, world.GameModeAdventure, world.DifficultyPeaceful, true},
		{"unknown game mode", 18, nil, world.DifficultyNormal, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			defer w.Close()
			w.SetDifficulty(tt.difficulty)
			u := &cakeActivationTestUser{food: tt.food, mode: tt.mode}
			runWorld(w, func(tx *world.Tx) {
				pos := cube.Pos{0, 64, 0}
				tx.SetBlock(pos.Side(cube.FaceDown), Stone{}, nil)
				tx.SetBlock(pos, Cake{}, nil)
				if got := (Cake{}).Activate(pos, cube.FaceNorth, tx, u, &item.UseContext{}); got != tt.want {
					t.Errorf("activation=%v, want %v", got, tt.want)
				}
				wantBites := 0
				if tt.want {
					wantBites = 1
				}
				if got := tx.Block(pos).(Cake).Bites; got != wantBites || u.saturated != tt.want {
					t.Errorf("bites=%d saturated=%v, want bites=%d saturated=%v", got, u.saturated, wantBites, tt.want)
				}
			})
		})
	}
}

// TestFullPlayerCakeCandleInteractions prevents a bite or candle drop without blocking attachment and extinguishing.
func TestFullPlayerCakeCandleInteractions(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	w.SetDifficulty(world.DifficultyNormal)
	runWorld(w, func(tx *world.Tx) {
		pos := cube.Pos{0, 64, 0}
		tx.SetBlock(pos.Side(cube.FaceDown), Stone{}, nil)
		u := &cakeActivationTestUser{food: 20, mode: world.GameModeSurvival}
		cake := Cake{Candle: true, CandleLit: true}
		tx.SetBlock(pos, cake, nil)
		if cake.Activate(pos, cube.FaceNorth, tx, u, &item.UseContext{}) || tx.Block(pos) != cake || u.saturated {
			t.Fatal("full player ate or removed candle")
		}
		if !cake.Activate(pos, cube.FaceUp, tx, u, &item.UseContext{}) || tx.Block(pos).(Cake).CandleLit {
			t.Fatal("full player could not extinguish candle")
		}
		u.held = item.NewStack(Candle{}, 1)
		tx.SetBlock(pos, Cake{}, nil)
		if !(Cake{}).Activate(pos, cube.FaceUp, tx, u, &item.UseContext{}) || !tx.Block(pos).(Cake).Candle {
			t.Fatal("full player could not attach candle")
		}
	})
}

// TestCakeEatingDeclinesUnknownActorState checks that Saturate alone does not imply known hunger.
func TestCakeEatingDeclinesUnknownActorState(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: Cake{}})
	view := &placementView{src: src, known: true}
	u := &unknownCakeActor{}
	complete := world.RunBlockTransaction(view, func(tx *world.Tx) {
		if (Cake{}).Activate(pos, cube.FaceNorth, tx, u, &item.UseContext{}) {
			t.Fatal("unknown hunger accepted a bite")
		}
	})
	if !complete || view.Block(pos) != (Cake{}) || u.saturated {
		t.Fatal("unknown actor changed cake or hunger")
	}
}

type unknownCakeActor struct {
	item.User
	saturated bool
}

// HeldItems supplies an empty hand without supplying hunger or game mode.
func (*unknownCakeActor) HeldItems() (item.Stack, item.Stack) { return item.Stack{}, item.Stack{} }

// Position supplies the location required if a faulty activation tries to eat.
func (*unknownCakeActor) Position() mgl64.Vec3 { return mgl64.Vec3{0, 64, 0} }

// Saturate records an unexpected hunger update for an actor with unknown state.
func (u *unknownCakeActor) Saturate(int, float64) { u.saturated = true }

// TestCakeEatingWaitsForWorldRules rejects full-hunger prediction when difficulty is unavailable.
func TestCakeEatingWaitsForWorldRules(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	cake := Cake{Candle: true}
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: cake})
	view := &placementView{src: src, known: true}
	u := &cakeActivationTestUser{food: 20, mode: world.GameModeSurvival}
	complete := world.RunBlockTransaction(view, func(tx *world.Tx) {
		cake.Activate(pos, cube.FaceNorth, tx, u, &item.UseContext{})
	})
	if complete || view.Block(pos) != cake || u.saturated {
		t.Fatal("unknown difficulty changed cake or hunger")
	}
}
