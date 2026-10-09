package entity_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestAnimalSpawnerCollisionModelsDoNotGenerateNeighbours(t *testing.T) {
	g := &countedPasture{}
	w := world.Config{Synchronous: true, Generator: g, Entities: entity.DefaultRegistry}.New()
	defer w.Close()
	w.SetTickRange(4)
	w.SetTime(6000)
	w.StopTime()
	s := entity.NewAnimalSpawner(w.Handler(), 42)
	w.Do(func(tx *world.Tx) {
		tx.AddEntity(world.NewEntity(player.Type, player.Config{Position: mgl64.Vec3{0, 1, 0}, GameMode: world.GameModeCreative}))
		tx.Block(cube.Pos{32, 1, 0})
		for z := range 16 {
			// Panes search adjacent blocks when resolving their collision bounds.
			// The west neighbour of this column must stay unloaded during spawning.
			tx.SetBlock(cube.Pos{32, 1, z}, block.GlassPane{}, &world.SetOpts{DisableBlockUpdates: true, DisableRedstoneUpdates: true})
		}
		before := g.generated
		// Omit entity physics so any additional terrain comes from population.
		for tick := range int64(20000) {
			s.HandleTick(tx, tick)
		}
		if g.generated != before {
			t.Fatalf("population collision checks generated %d extra columns", g.generated-before)
		}
	})
}
