package session

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

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
