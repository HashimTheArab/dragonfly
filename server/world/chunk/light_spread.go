package chunk

import "github.com/df-mc/dragonfly/server/block/cube"

// SpreadBlockLight spreads the block light given off by the blocks at the sources passed through the blocks
// around them, by the same emission, filtering and attenuation rules a light area applies, and returns the
// block light level of every position it reaches. It treats the blocks as one area, as a single light area
// filled over all of them does, rather than lighting each chunk on its own and then spreading across chunk
// borders as a World does. It writes no light anywhere, so the blocks may come from a world that is not
// stored in chunks the caller owns.
//
// blocksAt returns the runtime IDs, in br, of the blocks at a position on its first and second layer, and how
// many of those layers hold a block: 1 when only the first does, and 0 when the blocks at the position are not
// known, which light does not enter. Once limit positions are lit, no further positions are added.
func SpreadBlockLight(br BlockRegistry, blocksAt func(pos cube.Pos) (first, second uint32, layers int), sources []cube.Pos, limit int) map[cube.Pos]uint8 {
	highest := func(pos cube.Pos, lightBlocking func(rid uint32) uint8) (uint8, bool) {
		first, second, layers := blocksAt(pos)
		if layers == 0 {
			return 0, false
		}
		return highestLayer(lightBlocking, first, second, layers), true
	}
	levels := make(map[cube.Pos]uint8)
	queue := newLightQueue(initialLightQueueCapacity)
	for _, pos := range sources {
		if level, ok := highest(pos, br.LightBlock); ok && level > 0 {
			queue.push(node(pos, level, BlockLight))
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
			filtering, ok := highest(pos, br.FilteringBlock)
			if !ok {
				continue
			}
			if next, ok := attenuate(n.level, filtering); ok && levels[pos] < next {
				queue.push(node(pos, next, BlockLight))
			}
		}
	}
	return levels
}
