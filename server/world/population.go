package world

import (
	"cmp"
	"slices"
)

// TickingChunks returns a sorted snapshot of loaded columns within the world's
// simulation distance of a loader, or all resident columns in synchronous
// worlds. It never loads or generates terrain.
func (tx *Tx) TickingChunks() []ChunkPos {
	w := tx.World()
	_, loaders := w.allViewers()
	centres := make([]ChunkPos, 0, len(loaders))
	for _, l := range loaders {
		l.mu.RLock()
		centres = append(centres, l.pos)
		l.mu.RUnlock()
	}
	radius := int32(w.tickRange())
	result := make([]ChunkPos, 0)
	for pos := range w.chunks {
		if w.conf.Synchronous || (ticker{}).anyWithinDistance(pos, centres, radius) {
			result = append(result, pos)
		}
	}
	slices.SortFunc(result, func(a, b ChunkPos) int {
		if a[0] != b[0] {
			return cmp.Compare(a[0], b[0])
		}
		return cmp.Compare(a[1], b[1])
	})
	return result
}

// TickRange returns the simulation distance in columns.
func (w *World) TickRange() int { return w.tickRange() }

// ChunkLastTick returns the last tick when a resident column was in simulation
// range while a TickHandler was installed. It never loads a column. The second
// result is false for absent columns or columns that have not yet been stamped.
func (tx *Tx) ChunkLastTick(pos ChunkPos) (int64, bool) {
	c, ok := tx.World().chunks[pos]
	if !ok {
		return 0, false
	}
	return c.lastTick, c.lastTick > 0
}
