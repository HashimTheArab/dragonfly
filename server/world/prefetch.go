package world

import (
	"errors"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/goleveldb/leveldb"
)

const prefetchWorkers = 4

// prefetchLoop runs worker goroutines that load columns from the provider and precompute per-chunk lighting outside
// of the world transaction goroutine. The results are then installed using a short transaction.
func (w *World) prefetchLoop() {
	for range prefetchWorkers {
		go func() {
			defer w.running.Done()
			for {
				select {
				case <-w.closing:
					return
				case pos, ok := <-w.prefetchRequests:
					if !ok {
						return
					}
					w.prefetchOne(pos)
				}
			}
		}()
	}
}

// prefetchOne performs provider IO and per-chunk light filling, then schedules installation in the world. It must
// not mutate world state directly.
func (w *World) prefetchOne(pos ChunkPos) {
	// Do the expensive IO/CPU work off the world transaction goroutine.
	c, err := w.conf.Provider.LoadColumn(pos, w.conf.Dim)
	if errors.Is(err, leveldb.ErrNotFound) {
		// The provider doesn't have a chunk saved at this position, so we generate a new one.
		// Generation is not expected to be common for servers using pre-made maps, but we still handle it.
		w.genMu.Lock()
		col := &chunk.Column{Chunk: chunk.New(airRID, w.Range())}
		w.conf.Generator.GenerateChunk(pos, col.Chunk)
		w.genMu.Unlock()

		c, err = col, nil
	} else if err != nil {
		// Failed reading from provider. We'll still create an empty chunk so that the world can keep running.
		c = &chunk.Column{Chunk: chunk.New(airRID, w.Range())}
	}

	// Fill light for this chunk only. Cross-chunk spreading is done when installing the chunk (transaction-owned).
	chunk.LightArea([]*chunk.Chunk{c.Chunk}, int(pos[0]), int(pos[1])).Fill()

	select {
	case <-w.closing:
		// World is closing: don't enqueue a transaction that might never run.
		w.prefetchMu.Lock()
		delete(w.prefetchInFlight, pos)
		w.prefetchMu.Unlock()
		return
	default:
		_ = w.Exec(func(tx *Tx) {
			w.installPrefetched(pos, c, err)
		})
	}
}

func (w *World) installPrefetched(pos ChunkPos, c *chunk.Column, loadErr error) {
	// Mark no longer in-flight, regardless of whether we end up installing.
	w.prefetchMu.Lock()
	delete(w.prefetchInFlight, pos)
	w.prefetchMu.Unlock()

	if _, ok := w.chunks[pos]; ok {
		// Already loaded by some other path.
		return
	}

	col := w.columnFrom(c, pos)
	w.chunks[pos] = col
	for _, e := range col.Entities {
		w.entities[e] = pos
		e.w = w
	}

	// Spreading light requires neighbouring chunks and must happen with transaction-owned access to chunks.
	w.calculateLight(pos)

	if loadErr != nil && !errors.Is(loadErr, leveldb.ErrNotFound) {
		w.conf.Log.Error("load chunk: "+loadErr.Error(), "X", pos[0], "Z", pos[1])
	}
}

// requestPrefetch schedules an asynchronous load of the chunk at pos if it isn't already loaded or in-flight.
// This method is intended to be called from within a world transaction.
func (w *World) requestPrefetch(pos ChunkPos) {
	if _, ok := w.chunks[pos]; ok {
		return
	}

	w.prefetchMu.Lock()
	if _, ok := w.prefetchInFlight[pos]; ok {
		w.prefetchMu.Unlock()
		return
	}
	w.prefetchInFlight[pos] = struct{}{}
	w.prefetchMu.Unlock()

	select {
	case w.prefetchRequests <- pos:
	case <-w.closing:
	}
}
