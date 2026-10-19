//go:build !wasip1

package world

import "github.com/df-mc/goleveldb/leveldb"

// ErrColumnNotFound is the error a Provider's LoadColumn wraps when no column exists at a position. It is
// leveldb.ErrNotFound, which the mcdb provider returns.
var ErrColumnNotFound = leveldb.ErrNotFound
