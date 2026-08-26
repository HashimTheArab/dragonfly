package chunk

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// SubChunkHeightMaps is a chunk's surface height map prepared once to describe
// any number of sub-chunk entries. It must be rebuilt after the Chunk is edited.
type SubChunkHeightMaps struct {
	c               *Chunk
	heights         HeightMap
	lowest, highest int16
}

// NewSubChunkHeightMaps computes the reusable surface summary for c.
func NewSubChunkHeightMaps(c *Chunk) SubChunkHeightMaps {
	heights := c.HeightMap()
	lowest := c.SubIndex(heights[0])
	highest := lowest
	for _, y := range heights[1:] {
		at := c.SubIndex(y)
		lowest, highest = min(lowest, at), max(highest, at)
	}
	return SubChunkHeightMaps{c: c, heights: heights, lowest: lowest, highest: highest}
}

// At describes where the surface sits within the sub-chunk at index. The
// returned heights are nil unless the type is protocol.HeightMapDataHasData.
func (m SubChunkHeightMaps) At(index int16) (byte, []int8) {
	switch {
	case index < m.lowest:
		return protocol.HeightMapDataTooHigh, nil
	case index > m.highest:
		return protocol.HeightMapDataTooLow, nil
	}
	heightMap := make([]int8, 256)
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			y := m.heights.At(x, z)
			at, i := m.c.SubIndex(y), (uint16(z)<<4)|uint16(x)
			switch {
			case at > index:
				heightMap[i] = 16
			case at < index:
				heightMap[i] = -1
			default:
				heightMap[i] = int8(y - m.c.SubY(at))
			}
		}
	}
	return protocol.HeightMapDataHasData, heightMap
}

// RequestModeLevelChunk returns the SubChunkCount and SubChunkLimit a LevelChunk carries to
// announce a column without its terrain, which is what prompts the client to request it a
// sub-chunk at a time. Protocol 2168 constrains SubChunkCount to 0..64, so the limit alone
// marks request mode and the older protocol.SubChunkRequestMode* sentinels are rejected.
func RequestModeLevelChunk(c *Chunk) (subChunkCount uint32, subChunkLimit protocol.Optional[int32]) {
	return 0, protocol.Option(int32(c.HighestFilledSubChunk()))
}
