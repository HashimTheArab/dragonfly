package world

import (
	"errors"
	"iter"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/goleveldb/leveldb"
)

// blockSearchArea represents a horizontal square searched around a position.
type blockSearchArea struct {
	min, max cube.Pos
}

// blockSearchIndex stores chunk-level candidates for block searches.
type blockSearchIndex struct {
	indexed map[ChunkPos]map[uint32]struct{}
	chunks  map[uint32]map[ChunkPos]struct{}
}

// newBlockSearchIndex returns a new block search index.
func newBlockSearchIndex() *blockSearchIndex {
	return &blockSearchIndex{
		indexed: make(map[ChunkPos]map[uint32]struct{}),
		chunks:  make(map[uint32]map[ChunkPos]struct{}),
	}
}

// newBlockSearchArea returns the horizontal square searched around pos.
func newBlockSearchArea(pos cube.Pos, radius int) blockSearchArea {
	return blockSearchArea{
		min: cube.Pos{pos.X() - radius, 0, pos.Z() - radius},
		max: cube.Pos{pos.X() + radius, 0, pos.Z() + radius},
	}
}

// valid reports whether the search area has positive X and Z bounds.
func (area blockSearchArea) valid() bool {
	return area.min.X() < area.max.X() && area.min.Z() < area.max.Z()
}

// contains reports whether pos is inside the search area's horizontal bounds.
func (area blockSearchArea) contains(pos cube.Pos) bool {
	return pos.X() >= area.min.X() && pos.X() < area.max.X() && pos.Z() >= area.min.Z() && pos.Z() < area.max.Z()
}

// forEachChunk calls f for each chunk touched by the search area.
func (area blockSearchArea) forEachChunk(f func(ChunkPos)) {
	minChunk := chunkPosFromBlockPos(area.min)
	maxChunk := chunkPosFromBlockPos(cube.Pos{area.max.X() - 1, 0, area.max.Z() - 1})
	for chunkX := minChunk.X(); chunkX <= maxChunk.X(); chunkX++ {
		for chunkZ := minChunk.Z(); chunkZ <= maxChunk.Z(); chunkZ++ {
			f(ChunkPos{chunkX, chunkZ})
		}
	}
}

// blocksWithin returns positions of matching block states in loaded or saved chunks.
func (w *World) blocksWithin(pos cube.Pos, radius int, blocks ...Block) iter.Seq[cube.Pos] {
	return func(yield func(cube.Pos) bool) {
		area := newBlockSearchArea(pos, radius)
		if !area.valid() || len(blocks) == 0 {
			return
		}

		targets := make(map[uint32]struct{}, len(blocks))
		for _, b := range blocks {
			targets[w.conf.Blocks.BlockRuntimeID(b)] = struct{}{}
		}

		stop := false
		area.forEachChunk(func(pos ChunkPos) {
			if stop {
				return
			}
			if col, ok := w.chunks[pos]; ok {
				w.blockSearch.indexColumn(pos, col.Chunk)
				if !w.blockSearch.mayContainAny(pos, targets) {
					return
				}
				stop = !yieldMatchingBlocks(yield, area, pos, col.Chunk, targets)
				return
			}

			if !w.blockSearch.indexedChunk(pos) {
				col, err := w.conf.Provider.LoadColumn(pos, w.conf.Dim)
				switch {
				case err == nil:
					w.blockSearch.indexColumn(pos, col.Chunk)
					if w.blockSearch.mayContainAny(pos, targets) {
						stop = !yieldMatchingBlocks(yield, area, pos, col.Chunk, targets)
					}
				case errors.Is(err, leveldb.ErrNotFound):
					w.blockSearch.markMissing(pos)
				default:
					w.conf.Log.Error("blocks within: "+err.Error(), "X", pos[0], "Z", pos[1])
				}
				return
			}

			if !w.blockSearch.mayContainAny(pos, targets) {
				return
			}
			col, err := w.conf.Provider.LoadColumn(pos, w.conf.Dim)
			switch {
			case err == nil:
				stop = !yieldMatchingBlocks(yield, area, pos, col.Chunk, targets)
			case errors.Is(err, leveldb.ErrNotFound):
				w.blockSearch.markMissing(pos)
			default:
				w.conf.Log.Error("blocks within: "+err.Error(), "X", pos[0], "Z", pos[1])
			}
		})
	}
}

// markIndexed marks a chunk as indexed.
func (index *blockSearchIndex) markIndexed(pos ChunkPos) {
	if _, ok := index.indexed[pos]; !ok {
		index.indexed[pos] = make(map[uint32]struct{})
	}
}

// markMissing marks a chunk as indexed with no matching block states.
func (index *blockSearchIndex) markMissing(pos ChunkPos) {
	index.clearChunk(pos)
	index.markIndexed(pos)
}

// indexedChunk reports whether a chunk has been indexed already.
func (index *blockSearchIndex) indexedChunk(pos ChunkPos) bool {
	_, ok := index.indexed[pos]
	return ok
}

// clearChunk removes all indexed state for a chunk.
func (index *blockSearchIndex) clearChunk(pos ChunkPos) {
	for rid := range index.indexed[pos] {
		chunks := index.chunks[rid]
		delete(chunks, pos)
		if len(chunks) == 0 {
			delete(index.chunks, rid)
		}
	}
	delete(index.indexed, pos)
}

// indexRuntimeID marks a chunk as a candidate for a runtime ID.
func (index *blockSearchIndex) indexRuntimeID(pos ChunkPos, rid uint32) {
	rids, ok := index.indexed[pos]
	if !ok {
		return
	}
	rids[rid] = struct{}{}

	chunks, ok := index.chunks[rid]
	if !ok {
		chunks = make(map[ChunkPos]struct{})
		index.chunks[rid] = chunks
	}
	chunks[pos] = struct{}{}
}

// indexColumn indexes the primary block palettes in a chunk.
func (index *blockSearchIndex) indexColumn(pos ChunkPos, col *chunk.Chunk) {
	if index.indexedChunk(pos) {
		return
	}
	index.reindexColumn(pos, col)
}

// reindexColumn rebuilds the indexed runtime IDs for a chunk.
func (index *blockSearchIndex) reindexColumn(pos ChunkPos, col *chunk.Chunk) {
	index.clearChunk(pos)
	index.markIndexed(pos)
	for _, sub := range col.Sub() {
		if sub.Empty() {
			continue
		}
		layers := sub.Layers()
		if len(layers) == 0 {
			continue
		}
		palette := layers[0].Palette()
		for i := 0; i < palette.Len(); i++ {
			index.indexRuntimeID(pos, palette.Value(uint16(i)))
		}
	}
}

// mayContainAny reports whether a chunk may contain any target runtime ID.
func (index *blockSearchIndex) mayContainAny(pos ChunkPos, targets map[uint32]struct{}) bool {
	for rid := range targets {
		if chunks, ok := index.chunks[rid]; ok {
			if _, ok := chunks[pos]; ok {
				return true
			}
		}
	}
	return false
}

// yieldMatchingBlocks yields matching block positions from a single chunk.
func yieldMatchingBlocks(yield func(cube.Pos) bool, area blockSearchArea, chunkPos ChunkPos, col *chunk.Chunk, targets map[uint32]struct{}) bool {
	for i, sub := range col.Sub() {
		if sub.Empty() {
			continue
		}
		layers := sub.Layers()
		if len(layers) == 0 || !storageContainsAnyBlock(layers[0], targets) {
			continue
		}

		baseX, baseY, baseZ := int(chunkPos.X())<<4, int(col.SubY(int16(i))), int(chunkPos.Z())<<4
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				for z := 0; z < 16; z++ {
					if _, ok := targets[layers[0].At(byte(x), byte(y), byte(z))]; !ok {
						continue
					}
					pos := cube.Pos{baseX + x, baseY + y, baseZ + z}
					if area.contains(pos) && !yield(pos) {
						return false
					}
				}
			}
		}
	}
	return true
}

// storageContainsAnyBlock reports whether a paletted storage may contain one of the target runtime IDs.
func storageContainsAnyBlock(storage *chunk.PalettedStorage, targets map[uint32]struct{}) bool {
	palette := storage.Palette()
	for i := 0; i < palette.Len(); i++ {
		if _, ok := targets[palette.Value(uint16(i))]; ok {
			return true
		}
	}
	return false
}
