package chunk

import "github.com/df-mc/dragonfly/server/block/cube"

// SpreadBlockLight spreads the block light given off by the blocks at the sources passed through the blocks
// around them, by the same rules as the block light stage of a light area, and returns the block light level
// of every position it reaches. It reads blocks only and writes no light to any Chunk. chunkAt returns the
// Chunk at a chunk position, or nil when none is loaded there; light does not enter a Chunk that is not
// loaded. Once limit positions are lit, no further positions are added.
func SpreadBlockLight(chunkAt func(x, z int) *Chunk, sources []cube.Pos, limit int) map[cube.Pos]uint8 {
	levels := make(map[cube.Pos]uint8)
	queue := newLightQueue(initialLightQueueCapacity)
	for _, pos := range sources {
		if c, sub := spreadSubChunk(chunkAt, pos); sub != nil {
			if level := spreadHighest(sub, pos, c.br.LightBlock); level > 0 {
				queue.push(node(pos, level, BlockLight))
			}
		}
	}
	for !queue.empty() {
		n, _ := queue.pop()
		current, lit := levels[n.pos]
		if current >= n.level || !lit && len(levels) >= limit {
			continue
		}
		levels[n.pos] = n.level
		if n.level == 1 {
			// Every neighbour attenuates light by at least one, so none can be lit further.
			continue
		}
		for _, face := range cube.Faces() {
			pos := n.pos.Side(face)
			c, sub := spreadSubChunk(chunkAt, pos)
			if sub == nil {
				continue
			}
			if next, ok := attenuate(n.level, spreadHighest(sub, pos, c.br.FilteringBlock)); ok && levels[pos] < next {
				queue.push(node(pos, next, BlockLight))
			}
		}
	}
	return levels
}

// spreadSubChunk returns the Chunk and SubChunk holding pos, or a nil SubChunk when no Chunk is loaded there
// or pos lies outside its range.
func spreadSubChunk(chunkAt func(x, z int) *Chunk, pos cube.Pos) (*Chunk, *SubChunk) {
	c := chunkAt(pos[0]>>4, pos[2]>>4)
	if c == nil || pos.OutOfBounds(c.r) {
		return nil, nil
	}
	return c, c.SubChunk(int16(pos[1]))
}

// spreadHighest returns the highest value lightBlocking reports for the blocks at pos in sub.
func spreadHighest(sub *SubChunk, pos cube.Pos, lightBlocking func(rid uint32) uint8) uint8 {
	return sub.highest(uint8(pos[0]&0xf), uint8(pos[1]&0xf), uint8(pos[2]&0xf), lightBlocking)
}
