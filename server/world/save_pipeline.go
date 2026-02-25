package world

import "github.com/df-mc/dragonfly/server/world/chunk"

const saveWorkers = 1

type saveRequest struct {
	pos ChunkPos
	col *chunk.Column
	ack chan struct{}
}

// saveLoop runs background persistence workers that handle provider writes for queued save requests.
func (w *World) saveLoop() {
	for range saveWorkers {
		go func() {
			defer w.running.Done()

			for {
				select {
				case req := <-w.saveRequests:
					w.storeColumn(req.pos, req.col)
					if req.ack != nil {
						close(req.ack)
					}
				case <-w.closing:
					// Drain any already queued requests before exiting.
					for {
						select {
						case req := <-w.saveRequests:
							w.storeColumn(req.pos, req.col)
							if req.ack != nil {
								close(req.ack)
							}
						default:
							return
						}
					}
				}
			}
		}()
	}
}

// submitAsyncColumnSave queues a chunk save request. If the queue is saturated, it falls back to synchronous storage
// to preserve ordering guarantees.
func (w *World) submitAsyncColumnSave(pos ChunkPos, col *chunk.Column) {
	req := saveRequest{pos: pos, col: col}
	select {
	case w.saveRequests <- req:
		return
	default:
		// Queue is saturated: Block until there is room to preserve write ordering across sync/async save paths.
		w.saveRequests <- req
	}
}

// submitSyncColumnSave queues a save request and blocks until persistence completed.
func (w *World) submitSyncColumnSave(pos ChunkPos, col *chunk.Column) {
	ack := make(chan struct{})
	req := saveRequest{pos: pos, col: col, ack: ack}
	select {
	case w.saveRequests <- req:
		<-ack
	case <-w.closing:
		// Fall back to direct write if called after shutdown has started.
		w.storeColumn(pos, col)
	}
}

func (w *World) storeColumn(pos ChunkPos, col *chunk.Column) {
	if err := w.conf.Provider.StoreColumn(pos, w.conf.Dim, col); err != nil {
		w.conf.Log.Error("save chunk: "+err.Error(), "X", pos[0], "Z", pos[1])
	}
}
