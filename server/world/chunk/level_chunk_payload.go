package chunk

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// EncodeLevelChunkPayload builds the RawPayload for a LevelChunk packet from
// network-serialised chunk data and trailing block entity compounds.
func EncodeLevelChunkPayload(data SerialisedData, blockEntities []BlockEntity) ([]byte, error) {
	return EncodeLevelChunkPayloadWithBlockNBTs(data, blockEntityNBTs(blockEntities))
}

// EncodeLevelChunkPayloadFromMap builds the RawPayload for a LevelChunk packet
// from network-serialised chunk data and raw block entity NBT indexed by
// absolute block position.
func EncodeLevelChunkPayloadFromMap(data SerialisedData, blockEntities map[cube.Pos]map[string]any) ([]byte, error) {
	entries := make([]BlockEntity, 0, len(blockEntities))
	for pos, blockNBT := range blockEntities {
		entries = append(entries, BlockEntity{Pos: pos, Data: blockNBT})
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i].Pos, entries[j].Pos
		if a[1] != b[1] {
			return a[1] < b[1]
		}
		if a[2] != b[2] {
			return a[2] < b[2]
		}
		return a[0] < b[0]
	})
	return EncodeLevelChunkPayload(data, entries)
}

// EncodeLevelChunkPayloadWithBlockNBTs builds the RawPayload for a LevelChunk
// packet from network-serialised chunk data and block entity NBT compounds that
// already include their id/x/y/z fields.
func EncodeLevelChunkPayloadWithBlockNBTs(data SerialisedData, blockNBTs []map[string]any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	for _, sub := range data.SubChunks {
		_, _ = buf.Write(sub)
	}
	_, _ = buf.Write(data.Biomes)

	// Length of 1 byte for the border block count.
	buf.WriteByte(0)

	staging := bytes.NewBuffer(nil)
	for i, blockNBT := range blockNBTs {
		if blockNBT == nil {
			continue
		}
		staging.Reset()
		if err := nbt.NewEncoderWithEncoding(staging, nbt.NetworkLittleEndian).Encode(blockNBT); err != nil {
			return nil, fmt.Errorf("encode block entity %d at %v/%v/%v: %w", i, blockNBT["x"], blockNBT["y"], blockNBT["z"], err)
		}
		_, _ = buf.Write(staging.Bytes())
	}
	return append([]byte(nil), buf.Bytes()...), nil
}

// RequestModeLevelChunk returns the SubChunkCount and SubChunkLimit a LevelChunk carries to
// announce c without its terrain, which is what prompts the client to request the column a
// sub-chunk at a time. Protocol 2168 constrains SubChunkCount to 0..64, so the limit alone
// marks request mode and the older protocol.SubChunkRequestMode* sentinels are rejected.
func RequestModeLevelChunk(c *Chunk) (subChunkCount uint32, subChunkLimit protocol.Optional[int32]) {
	return 0, protocol.Option(int32(c.HighestFilledSubChunk()))
}

// RequestModeLevelChunkPayload returns the RawPayload for an uncached request-mode
// announcement: the biomes, then the border block count servers leave empty.
func RequestModeLevelChunkPayload(c *Chunk) []byte {
	return append(EncodeBiomes(c, NetworkEncoding), 0)
}

// blockEntityNBTs copies each entity's data with its position injected, which is how the
// client keys a block entity to the block it belongs to. Entities with no data are dropped.
func blockEntityNBTs(blockEntities []BlockEntity) []map[string]any {
	blockNBTs := make([]map[string]any, 0, len(blockEntities))
	for _, blockEntity := range blockEntities {
		if blockEntity.Data == nil {
			continue
		}
		blockNBT := make(map[string]any, len(blockEntity.Data)+3)
		for k, v := range blockEntity.Data {
			blockNBT[k] = v
		}
		blockNBT["x"] = int32(blockEntity.Pos[0])
		blockNBT["y"] = int32(blockEntity.Pos[1])
		blockNBT["z"] = int32(blockEntity.Pos[2])
		blockNBTs = append(blockNBTs, blockNBT)
	}
	return blockNBTs
}

// EncodeBlockEntities encodes entities as the network NBT that trails a sub-chunk payload,
// each carrying its own position. A SubChunk entry only draws its block actors when their
// compounds follow the sub-chunk data this way.
func EncodeBlockEntities(blockEntities []BlockEntity) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := nbt.NewEncoderWithEncoding(buf, nbt.NetworkLittleEndian)
	for i, blockNBT := range blockEntityNBTs(blockEntities) {
		if err := enc.Encode(blockNBT); err != nil {
			return nil, fmt.Errorf("encode block entity %d at %v/%v/%v: %w", i, blockNBT["x"], blockNBT["y"], blockNBT["z"], err)
		}
	}
	return buf.Bytes(), nil
}
