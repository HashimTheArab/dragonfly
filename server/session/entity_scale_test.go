package session

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// scaledMetadataType keeps a scaled world box distinct from its packet dimensions.
type scaledMetadataType struct {
	networkEncodedOffsetType
	scale float64
}

// BBox supplies the actual world bounds used by collision queries.
func (t scaledMetadataType) BBox(world.Entity) cube.BBox {
	return cube.Box(-t.scale/2, 0, -t.scale/2, t.scale/2, 2*t.scale, t.scale/2)
}

// Open exposes the same scale through the existing entity metadata interface.
func (t scaledMetadataType) Open(_ *world.Tx, h *world.EntityHandle, d *world.EntityData) world.Entity {
	return scaledMetadataEntity{ridingEntity{h: h, d: d}, t.scale}
}

// scaledMetadataEntity adds visual scale to the replication fixture.
type scaledMetadataEntity struct {
	ridingEntity
	scale float64
}

// Scale returns the modifier that a client applies to received dimensions.
func (e scaledMetadataEntity) Scale() float64 { return e.scale }

func TestEntityMetadataAppliesScaleToCollisionOnlyOnce(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	for _, scale := range []float64{0.5, 1, 2} {
		typ := scaledMetadataType{scale: scale}
		h := world.NewEntity(typ, ridingConfig{})
		w.Do(func(tx *world.Tx) {
			e := tx.AddEntity(h)
			metadata := ridingSession().entityMetadata(e)
			width := metadata[protocol.EntityDataKeyWidth].(float32)
			height := metadata[protocol.EntityDataKeyHeight].(float32)
			visualScale := metadata[protocol.EntityDataKeyScale].(float32)
			box := typ.BBox(e)
			if float64(width*visualScale) != box.Width() || float64(height*visualScale) != box.Height() {
				t.Errorf("scale %v replicated %v x %v at %v for world bounds %v x %v", scale, width, height, visualScale, box.Width(), box.Height())
			}
			tx.RemoveEntity(e)
			_ = h.Close()
		})
	}
}
