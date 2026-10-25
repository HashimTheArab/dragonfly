package session

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// maxPacketsPerTransaction is the maximum number of queued client packets handled before yielding the world owner.
// Any packets left in the queue are handled in a later transaction, allowing queued work from other sessions to run.
const maxPacketsPerTransaction = 64

// readPackets is the sole caller of read. It closes packets when read returns an error. If done is closed while
// packets is full, readPackets returns without trying to enqueue the packet it just read.
func readPackets(read func() (packet.Packet, error), done <-chan struct{}, packets chan<- packet.Packet) {
	defer close(packets)
	for {
		pk, err := read()
		if err != nil {
			return
		}
		select {
		case packets <- pk:
		case <-done:
			return
		}
	}
}

// handlePendingPackets handles first, then polls packets without waiting for more. It stops after
// maxPacketsPerTransaction packets or when handle returns false or an error. A false result from handle leaves the
// next queued packet for a new owner transaction. sourceClosed reports that packets was closed and fully drained.
func handlePendingPackets(first packet.Packet, packets <-chan packet.Packet, handle func(packet.Packet) (bool, error)) (sourceClosed bool, err error) {
	pk := first
	for i := 0; ; i++ {
		sameOwner, err := handle(pk)
		if err != nil || !sameOwner {
			return false, err
		}
		if i == maxPacketsPerTransaction-1 {
			return false, nil
		}
		select {
		case next, ok := <-packets:
			if !ok {
				return true, nil
			}
			pk = next
		default:
			return false, nil
		}
	}
}
