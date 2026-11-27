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
	a := spreadArea{br: br, blocksAt: blocksAt, sections: make(map[cube.Pos]*spreadSection)}
	queue := newLightQueue(initialLightQueueCapacity)
	lit := 0
	raise := func(pos cube.Pos, level uint8) {
		s, i := a.section(pos)
		if current := s.level[i]; current >= level || current == 0 && lit >= limit {
			return
		} else if current == 0 {
			lit++
		}
		s.level[i] = level
		if level > 1 {
			// Every neighbour attenuates light by at least one, so a level of 1 lights none.
			queue.push(node(pos, level, BlockLight))
		}
	}
	for _, pos := range sources {
		if first, second, layers := blocksAt(pos); layers > 0 {
			raise(pos, highestLayer(br.LightBlock, first, second, layers))
		}
	}
	for !queue.empty() {
		n, _ := queue.pop()
		if s, i := a.section(n.pos); s.level[i] != n.level {
			continue
		}
		for _, face := range cube.Faces() {
			pos := n.pos.Side(face)
			s, i := a.section(pos)
			if s.level[i] >= n.level-1 {
				continue
			}
			if filtering, ok := a.filtering(s, i, pos); ok {
				if next, ok := attenuate(n.level, filtering); ok {
					raise(pos, next)
				}
			}
		}
	}
	levels := make(map[cube.Pos]uint8, lit)
	for key, s := range a.sections {
		for i, level := range s.level {
			if level > 0 {
				levels[cube.Pos{key[0]<<3 | i>>6, key[1]<<3 | i&7, key[2]<<3 | i>>3&7}] = level
			}
		}
	}
	return levels
}

// unknownBlocks marks a position in spreadSection.filter whose blocks are not known.
const unknownBlocks = 0xff

// spreadSection holds the block light level of every position in one 8x8x8 section, and the filtering
// of the blocks there plus one: 0 until they are read, or unknownBlocks if they are not known.
type spreadSection struct {
	level, filter [512]uint8
}

// spreadArea holds the sections SpreadBlockLight has reached, so each position's blocks are read once.
type spreadArea struct {
	br       BlockRegistry
	blocksAt func(pos cube.Pos) (first, second uint32, layers int)
	sections map[cube.Pos]*spreadSection
	lastKey  cube.Pos
	last     *spreadSection
}

// section returns the section holding pos, adding it if it is new, and the index of pos in it.
func (a *spreadArea) section(pos cube.Pos) (*spreadSection, int) {
	key := cube.Pos{pos[0] >> 3, pos[1] >> 3, pos[2] >> 3}
	if a.last == nil || key != a.lastKey {
		s, ok := a.sections[key]
		if !ok {
			s = new(spreadSection)
			a.sections[key] = s
		}
		a.last, a.lastKey = s, key
	}
	return a.last, (pos[0]&7)<<6 | (pos[2]&7)<<3 | pos[1]&7
}

// filtering returns how much the blocks at pos, index i of s, filter light, reading them the first time,
// or false if they are not known.
func (a *spreadArea) filtering(s *spreadSection, i int, pos cube.Pos) (uint8, bool) {
	if f := s.filter[i]; f != 0 {
		return f - 1, f != unknownBlocks
	}
	first, second, layers := a.blocksAt(pos)
	if layers == 0 {
		s.filter[i] = unknownBlocks
		return 0, false
	}
	f := highestLayer(a.br.FilteringBlock, first, second, layers)
	s.filter[i] = f + 1
	return f, true
}
