//go:build wasip1

package world

import "errors"

// ErrColumnNotFound is the error a Provider's LoadColumn wraps when no column exists at a position.
// goleveldb has no file storage on wasip1, so this build does not depend on it.
var ErrColumnNotFound = errors.New("leveldb: not found")
