package session

import (
	"bytes"

	"github.com/cespare/xxhash/v2"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	// subChunkRequests enables Bedrock's on-demand sub-chunk request system.
	// Keep it enabled so edits such as //cut and //set air invalidate and update
	// the client view immediately instead of waiting for full chunk reloads.
	subChunkRequests = true

	// clientChunkBlobCache controls Bedrock's client-side blob cache for chunks
	// and sub-chunks. Keep it enabled so cached chunk/sub-chunk updates are sent
	// through the normal Bedrock cache-miss/status path.
	clientChunkBlobCache = true

	// maxPendingBlobs is the maximum number of client-cache blobs that may be pending for a session. Raised high for large/fast chunk sends.
	maxPendingBlobs = 1 << 20
)

func (s *Session) chunkBlobCacheEnabled() bool {
	return clientChunkBlobCache && s.conn.ClientCacheEnabled()
}

// ViewChunk ...
func (s *Session) ViewChunk(pos world.ChunkPos, dim world.Dimension, blockEntities map[cube.Pos]world.Block, c *chunk.Chunk) {
	if !s.chunkBlobCacheEnabled() {
		s.sendNetworkChunk(pos, dim, c, blockEntities)
		return
	}
	s.sendBlobHashes(pos, dim, c, blockEntities)
}

// ViewBulkChunk queues an authoritative full chunk refresh after a bulk edit.
// The queue is deduplicated and drained by the session tick so a huge WorldEdit
// operation does not synchronously flood the packet channel from the world
// transaction. Normal chunk loading still uses blob-cache/sub-chunk requests.
func (s *Session) ViewBulkChunk(pos world.ChunkPos) {
	s.bulkChunkMu.Lock()
	if _, ok := s.bulkChunkQueued[pos]; !ok {
		s.bulkChunkQueued[pos] = struct{}{}
		s.bulkChunkQueue = append(s.bulkChunkQueue, pos)
	}
	s.bulkChunkMu.Unlock()
}

func (s *Session) sendBulkChunkRefreshes(tx *world.Tx) {
	const maxBulkChunkRefreshesPerTick = 8
	for sent := 0; sent < maxBulkChunkRefreshesPerTick; {
		pos, ok := s.nextBulkChunkRefresh()
		if !ok {
			return
		}
		col, ok := s.chunkLoader.Chunk(pos)
		if !ok {
			// The player moved away before the refresh was sent.
			continue
		}
		s.sendFullNetworkChunk(pos, tx.World().Dimension(), col.Chunk, col.BlockEntities)
		sent++
	}
}

func (s *Session) nextBulkChunkRefresh() (world.ChunkPos, bool) {
	s.bulkChunkMu.Lock()
	defer s.bulkChunkMu.Unlock()
	if len(s.bulkChunkQueue) == 0 {
		return world.ChunkPos{}, false
	}

	// Pick the queued chunk using a centre-out pinwheel priority. Large bulk
	// edits are usually queued in scan-line order, which makes one edge update
	// first. Draining each outward ring with the horizontal/vertical arms first
	// and then the clockwise hook at the tip of each arm (so each arm reads
	// like an upside-down, mirrored 'L') makes the player's view correct itself
	// from the middle in a predictable, block-friendly pattern.
	centreX2, centreZ2 := bulkChunkQueueCentre2(s.bulkChunkQueue)
	bestIndex := 0
	bestRing, bestTier, bestDistance := bulkChunkRefreshPriority(s.bulkChunkQueue[0], centreX2, centreZ2)
	for i := 1; i < len(s.bulkChunkQueue); i++ {
		ring, tier, distance := bulkChunkRefreshPriority(s.bulkChunkQueue[i], centreX2, centreZ2)
		if ring < bestRing || ring == bestRing && (tier < bestTier || tier == bestTier && distance < bestDistance) {
			bestIndex, bestRing, bestTier, bestDistance = i, ring, tier, distance
		}
	}

	pos := s.bulkChunkQueue[bestIndex]
	delete(s.bulkChunkQueued, pos)
	copy(s.bulkChunkQueue[bestIndex:], s.bulkChunkQueue[bestIndex+1:])
	s.bulkChunkQueue[len(s.bulkChunkQueue)-1] = world.ChunkPos{}
	s.bulkChunkQueue = s.bulkChunkQueue[:len(s.bulkChunkQueue)-1]
	if len(s.bulkChunkQueue) == 0 && cap(s.bulkChunkQueue) > 4096 {
		s.bulkChunkQueue = nil
	}
	return pos, true
}

func bulkChunkQueueCentre2(queue []world.ChunkPos) (x2, z2 int64) {
	minX, maxX := queue[0][0], queue[0][0]
	minZ, maxZ := queue[0][1], queue[0][1]
	for _, pos := range queue[1:] {
		minX = min(minX, pos[0])
		maxX = max(maxX, pos[0])
		minZ = min(minZ, pos[1])
		maxZ = max(maxZ, pos[1])
	}
	return int64(minX) + int64(maxX), int64(minZ) + int64(maxZ)
}

func bulkChunkRefreshPriority(pos world.ChunkPos, centreX2, centreZ2 int64) (ring, tier, distance int64) {
	dx := int64(pos[0])*2 - centreX2
	dz := int64(pos[1])*2 - centreZ2
	absDx, absDz := absInt64(dx), absInt64(dz)
	ring = max(absDx, absDz)
	return ring, bulkChunkRefreshTier(dx, dz, ring), absDx*absDx + absDz*absDz
}

// bulkChunkRefreshTier classifies a queued chunk relative to the centre using
// a clockwise pinwheel:
//
//	0 -- a straight axis arm (north/south/east/west of centre).
//	1 -- on the clockwise hook line at the tip of an arm (east arm bends
//	     south, south arm bends west, west arm bends north, north arm bends
//	     east), giving each arm an upside-down, mirrored 'L' shape.
//	2 -- elsewhere on the ring.
func bulkChunkRefreshTier(dx, dz, ring int64) int64 {
	if dx == 0 || dz == 0 {
		return 0
	}
	if (dx == ring && dz > 0) ||
		(dz == ring && dx < 0) ||
		(dx == -ring && dz < 0) ||
		(dz == -ring && dx > 0) {
		return 1
	}
	return 2
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func (s *Session) clearBulkChunkRefreshes() {
	s.bulkChunkMu.Lock()
	clear(s.bulkChunkQueued)
	s.bulkChunkQueue = nil
	s.bulkChunkMu.Unlock()
}

// ViewSubChunks ...
func (s *Session) ViewSubChunks(centre world.SubChunkPos, offsets []protocol.SubChunkOffset, tx *world.Tx) {
	r := tx.Range()

	entries := make([]protocol.SubChunkEntry, 0, len(offsets))
	transaction := make(map[uint64]struct{})
	for _, offset := range offsets {
		ind := int16(centre.Y()) + int16(offset[1]) - int16(r[0])>>4
		if ind < 0 || ind > int16(r.Height()>>4) {
			entries = append(entries, protocol.SubChunkEntry{Result: protocol.SubChunkResultIndexOutOfBounds, Offset: offset})
			continue
		}
		col, ok := s.chunkLoader.Chunk(world.ChunkPos{
			centre.X() + int32(offset[0]),
			centre.Z() + int32(offset[2]),
		})
		if !ok {
			entries = append(entries, protocol.SubChunkEntry{Result: protocol.SubChunkResultChunkNotFound, Offset: offset})
			continue
		}
		entries = append(entries, s.subChunkEntry(offset, ind, col, transaction))
	}
	if s.chunkBlobCacheEnabled() && len(transaction) > 0 {
		s.blobMu.Lock()
		s.openChunkTransactions = append(s.openChunkTransactions, transaction)
		s.blobMu.Unlock()
	}
	dim, _ := world.DimensionID(tx.World().Dimension())
	s.writePacket(&packet.SubChunk{
		Dimension:       int32(dim),
		Position:        protocol.SubChunkPos(centre),
		CacheEnabled:    s.chunkBlobCacheEnabled(),
		SubChunkEntries: entries,
	})
}

func (s *Session) subChunkEntry(offset protocol.SubChunkOffset, ind int16, col *world.Column, transaction map[uint64]struct{}) protocol.SubChunkEntry {
	chunkMap := col.HeightMap()
	subMapType, subMap := byte(protocol.HeightMapDataHasData), make([]int8, 256)
	higher, lower := true, true
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			y, i := chunkMap.At(x, z), (uint16(z)<<4)|uint16(x)
			otherInd := col.SubIndex(y)
			switch {
			case otherInd > ind:
				subMap[i], lower = 16, false
			case otherInd < ind:
				subMap[i], higher = -1, false
			default:
				subMap[i], lower, higher = int8(y-col.SubY(otherInd)), false, false
			}
		}
	}
	if higher {
		subMapType, subMap = protocol.HeightMapDataTooHigh, nil
	} else if lower {
		subMapType, subMap = protocol.HeightMapDataTooLow, nil
	}

	sub := col.Sub()[ind]
	if sub.Empty() {
		return protocol.SubChunkEntry{
			Result:              protocol.SubChunkResultSuccessAllAir,
			HeightMapType:       subMapType,
			HeightMapData:       subMap,
			RenderHeightMapType: subMapType,
			RenderHeightMapData: subMap,
			Offset:              offset,
		}
	}

	serialisedSubChunk := chunk.EncodeSubChunk(col.Chunk, chunk.NetworkEncoding, int(ind))

	blockEntityBuf := bytes.NewBuffer(nil)
	enc := nbt.NewEncoderWithEncoding(blockEntityBuf, nbt.NetworkLittleEndian)
	for pos, b := range col.BlockEntities {
		if n, ok := b.(world.NBTer); ok && col.SubIndex(int16(pos.Y())) == ind {
			d := n.EncodeNBT()
			d["x"], d["y"], d["z"] = int32(pos[0]), int32(pos[1]), int32(pos[2])
			_ = enc.Encode(d)
		}
	}

	entry := protocol.SubChunkEntry{
		Result:              protocol.SubChunkResultSuccess,
		RawPayload:          append(serialisedSubChunk, blockEntityBuf.Bytes()...),
		HeightMapType:       subMapType,
		HeightMapData:       subMap,
		RenderHeightMapType: subMapType,
		RenderHeightMapData: subMap,
		Offset:              offset,
	}
	if s.chunkBlobCacheEnabled() {
		if hash := xxhash.Sum64(serialisedSubChunk); s.trackBlob(hash, serialisedSubChunk) {
			transaction[hash] = struct{}{}

			entry.BlobHash = hash
			entry.RawPayload = blockEntityBuf.Bytes()
		}
	}
	return entry
}

// dimensionID returns the dimension ID of the world that the session is in.
func (s *Session) dimensionID(dim world.Dimension) int32 {
	d, _ := world.DimensionID(dim)
	return int32(d)
}

// sendBlobHashes sends chunk blob hashes of the data of the chunk and stores the data in a map of blobs. Only
// data that the client doesn't yet have will be sent over the network.
func (s *Session) sendBlobHashes(pos world.ChunkPos, dim world.Dimension, c *chunk.Chunk, blockEntities map[cube.Pos]world.Block) {
	if subChunkRequests {
		biomes := chunk.EncodeBiomes(c, chunk.NetworkEncoding)
		if hash := xxhash.Sum64(biomes); s.trackBlob(hash, biomes) {
			s.writePacket(&packet.LevelChunk{
				Dimension:       s.dimensionID(dim),
				SubChunkCount:   protocol.SubChunkRequestModeLimited,
				Position:        protocol.ChunkPos(pos),
				HighestSubChunk: c.HighestFilledSubChunk(),
				BlobHashes:      []uint64{hash},
				RawPayload:      []byte{0},
				CacheEnabled:    true,
			})
			return
		}
	}

	var (
		data   = chunk.Encode(c, chunk.NetworkEncoding)
		count  = uint32(len(data.SubChunks))
		blobs  = append(data.SubChunks, data.Biomes)
		hashes = make([]uint64, len(blobs))
		m      = make(map[uint64]struct{}, len(blobs))
	)
	for i, blob := range blobs {
		h := xxhash.Sum64(blob)
		hashes[i], m[h] = h, struct{}{}
	}

	s.blobMu.Lock()
	s.openChunkTransactions = append(s.openChunkTransactions, m)
	if l := len(s.blobs); l > maxPendingBlobs {
		s.blobMu.Unlock()
		s.conf.Log.Error("too many blobs pending", "n", l)
		return
	}
	for i := range hashes {
		s.blobs[hashes[i]] = blobs[i]
	}
	s.blobMu.Unlock()

	// Length of 1 byte for the border block count.
	raw := bytes.NewBuffer(make([]byte, 1, 32))
	enc := nbt.NewEncoderWithEncoding(raw, nbt.NetworkLittleEndian)
	for bp, b := range blockEntities {
		if n, ok := b.(world.NBTer); ok {
			d := n.EncodeNBT()
			d["x"], d["y"], d["z"] = int32(bp[0]), int32(bp[1]), int32(bp[2])
			_ = enc.Encode(d)
		}
	}

	s.writePacket(&packet.LevelChunk{
		Dimension:     s.dimensionID(dim),
		Position:      protocol.ChunkPos{pos.X(), pos.Z()},
		SubChunkCount: count,
		CacheEnabled:  true,
		BlobHashes:    hashes,
		RawPayload:    raw.Bytes(),
	})
}

// sendNetworkChunk sends a network encoded chunk to the client.
func (s *Session) sendNetworkChunk(pos world.ChunkPos, dim world.Dimension, c *chunk.Chunk, blockEntities map[cube.Pos]world.Block) {
	if subChunkRequests {
		s.writePacket(&packet.LevelChunk{
			Dimension:       s.dimensionID(dim),
			SubChunkCount:   protocol.SubChunkRequestModeLimited,
			Position:        protocol.ChunkPos(pos),
			HighestSubChunk: c.HighestFilledSubChunk(),
			RawPayload:      append(chunk.EncodeBiomes(c, chunk.NetworkEncoding), 0),
		})
		return
	}

	s.sendFullNetworkChunk(pos, dim, c, blockEntities)
}

// sendFullNetworkChunk sends a complete chunk payload. It bypasses the
// blob-cache/sub-chunk request shortcuts and is used only for queued bulk-edit
// refreshes where the client must be corrected without requiring movement.
func (s *Session) sendFullNetworkChunk(pos world.ChunkPos, dim world.Dimension, c *chunk.Chunk, blockEntities map[cube.Pos]world.Block) {
	data := chunk.Encode(c, chunk.NetworkEncoding)
	chunkBuf := bytes.NewBuffer(nil)
	for _, s := range data.SubChunks {
		_, _ = chunkBuf.Write(s)
	}
	_, _ = chunkBuf.Write(data.Biomes)

	// Length of 1 byte for the border block count.
	chunkBuf.WriteByte(0)

	enc := nbt.NewEncoderWithEncoding(chunkBuf, nbt.NetworkLittleEndian)
	for bp, b := range blockEntities {
		if n, ok := b.(world.NBTer); ok {
			d := n.EncodeNBT()
			d["x"], d["y"], d["z"] = int32(bp[0]), int32(bp[1]), int32(bp[2])
			_ = enc.Encode(d)
		}
	}

	s.writePacket(&packet.LevelChunk{
		Dimension:     s.dimensionID(dim),
		Position:      protocol.ChunkPos{pos.X(), pos.Z()},
		SubChunkCount: uint32(len(data.SubChunks)),
		RawPayload:    append([]byte(nil), chunkBuf.Bytes()...),
	})
}

// trackBlob attempts to track the given blob. If the player has too many pending blobs, it returns false and closes the
// connection.
func (s *Session) trackBlob(hash uint64, blob []byte) bool {
	s.blobMu.Lock()
	if l := len(s.blobs); l > maxPendingBlobs {
		s.blobMu.Unlock()
		s.conf.Log.Error("too many blobs pending", "n", l)
		return false
	}
	s.blobs[hash] = blob
	s.blobMu.Unlock()
	return true
}
