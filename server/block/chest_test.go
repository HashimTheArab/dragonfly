package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

type chestOpeningSource map[cube.Pos]world.Block

func (s chestOpeningSource) Block(pos cube.Pos) world.Block {
	if b, ok := s[pos]; ok {
		return b
	}
	return Air{}
}

type chestActivationUser struct {
	item.User
	opened bool
}

func (u *chestActivationUser) OpenBlockContainer(cube.Pos, *world.Tx) { u.opened = true }

func TestChest_ActivatePreservesBlockedUseConsumption(t *testing.T) {
	for _, test := range []struct {
		name                                                string
		paired, ownBlocked, partnerBlocked, consumes, opens bool
	}{
		{name: "single clear", consumes: true, opens: true},
		{name: "single blocked", ownBlocked: true, consumes: true},
		{name: "pair clear", paired: true, consumes: true, opens: true},
		{name: "pair own blocked", paired: true, ownBlocked: true, consumes: true},
		{name: "pair partner blocked", paired: true, partnerBlocked: true},
		{name: "pair both blocked", paired: true, ownBlocked: true, partnerBlocked: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			defer w.Close()
			u := &chestActivationUser{}
			runWorld(w, func(tx *world.Tx) {
				pos := cube.Pos{0, 64, 0}
				c := Chest{paired: test.paired, pairX: 1}
				if test.ownBlocked {
					tx.SetBlock(pos.Side(cube.FaceUp), Stone{}, nil)
				}
				if test.partnerBlocked {
					tx.SetBlock(cube.Pos{1, 65, 0}, Stone{}, nil)
				}
				if got := c.Activate(pos, cube.FaceNorth, tx, u, &item.UseContext{}); got != test.consumes || u.opened != test.opens {
					t.Fatalf("consumes %v, opens %v; want %v, %v", got, u.opened, test.consumes, test.opens)
				}
			})
		})
	}
}

func TestChest_CanOpenAt(t *testing.T) {
	pos, pair := cube.Pos{0, 64, 0}, cube.Pos{1, 64, 0}
	for _, test := range []struct {
		name         string
		paired       bool
		own, partner world.Block
		want         bool
	}{
		{name: "air lid", own: Air{}, want: true},
		{name: "glass lid", own: Glass{}, want: true},
		{name: "stone lid", own: Stone{}},
		{name: "unpaired ignores neighbour", own: Air{}, partner: Stone{}, want: true},
		{name: "both paired lids clear", paired: true, own: Air{}, partner: Air{}, want: true},
		{name: "paired own lid blocked", paired: true, own: Stone{}, partner: Air{}},
		{name: "paired partner lid blocked", paired: true, own: Air{}, partner: Stone{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := Chest{paired: test.paired, pairX: pair[0], pairZ: pair[2]}
			source := chestOpeningSource{pos.Side(cube.FaceUp): test.own, pair.Side(cube.FaceUp): test.partner}
			if got := c.CanOpenAt(pos, source); got != test.want {
				t.Fatalf("can open = %v, want %v", got, test.want)
			}
		})
	}
}
