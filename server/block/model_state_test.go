package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// airSource is a block source of air everywhere.
type airSource struct{}

func (airSource) Block(cube.Pos) world.Block { return Air{} }

// Every registered block state has a model as it is, before any block-entity
// data is decoded into it.
func TestRegisteredStatesHaveModels(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	for _, b := range world.Blocks() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					name, _ := b.EncodeBlock()
					t.Errorf("%s (%T): Model().BBox panicked: %v", name, b, r)
				}
			}()
			_ = b.Model().BBox(cube.Pos{}, airSource{})
		}()
	}
}
