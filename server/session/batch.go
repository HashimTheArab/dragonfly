package session

import (
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ClientBatchStats describes the work performed for one preserved client
// network batch.
type ClientBatchStats struct {
	PacketCount            int
	OutboundQueueDepth     int
	ProcessingDuration     time.Duration
	ImmediateFlushDuration time.Duration
	Latency                time.Duration
}

// ClientBatchFunc is called after a preserved client batch is processed and
// flushed. It should be fast and non-blocking.
type ClientBatchFunc func(ClientBatchStats)

type packetReader interface {
	ReadPacket() (packet.Packet, error)
}

type packetBatchReader interface {
	ReadBatch() ([]packet.Packet, error)
}

func readPacketBatch(reader packetReader, enabled bool) ([]packet.Packet, bool, error) {
	if enabled {
		if reader, ok := reader.(packetBatchReader); ok {
			packets, err := reader.ReadBatch()
			return packets, true, err
		}
	}
	pk, err := reader.ReadPacket()
	if err != nil {
		return nil, false, err
	}
	return []packet.Packet{pk}, false, nil
}
