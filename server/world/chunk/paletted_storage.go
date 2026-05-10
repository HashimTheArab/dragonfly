package chunk

import (
	"bytes"
	"unsafe"
)

const (
	// uint32ByteSize is the amount of bytes in a uint32.
	uint32ByteSize = 4
	// uint32BitSize is the amount of bits in a uint32.
	uint32BitSize = uint32ByteSize * 8
)

// PalettedStorage is a storage of 4096 blocks encoded in a variable amount of uint32s, storages may have values
// with a bit size per block of 0, 1, 2, 3, 4, 5, 6, 8 or 16 bits.
// 3 of these formats have additional padding in every uint32 and an additional uint32 at the end, to cater
// for the blocks that don't fit. This padding is present when the storage has a block size of 3, 5 or 6
// bytes.
// Methods on PalettedStorage must not be called simultaneously from multiple goroutines.
type PalettedStorage struct {
	// bitsPerIndex is the amount of bits required to store one block. The number increases as the block
	// storage holds more unique block states.
	bitsPerIndex uint16
	// filledBitsPerIndex returns the amount of blocks that are actually filled per uint32.
	filledBitsPerIndex uint16
	// indexMask is the equivalent of 1 << bitsPerIndex - 1.
	indexMask uint32

	// indicesStart holds an unsafe.Pointer to the first byte in the indices slice below.
	indicesStart unsafe.Pointer

	// Palette holds all block runtime IDs that the indices in the indices slice point to. These runtime IDs
	// point to block states.
	palette *Palette

	// indices contains all indices in the PalettedStorage. This slice has a variable size, but may not be changed
	// unless the whole PalettedStorage is resized, including the Palette.
	indices []uint32
}

// PaletteIndexCache caches palette indexes for runtime IDs in one storage.
// Palette indexes are storage-local, so a cache must not be used with multiple
// paletted storages or shared across concurrent storage writers.
type PaletteIndexCache struct {
	lastValue uint32
	lastIndex uint16
	lastSet   bool
	small     [4]paletteIndexCacheEntry
	smallLen  uint8
	overflow  map[uint32]uint16
}

type paletteIndexCacheEntry struct {
	value uint32
	index uint16
}

// newPalettedStorage creates a new block storage using the uint32 slice as the indices and the palette passed.
// The bits per block are calculated using the length of the uint32 slice.
func newPalettedStorage(indices []uint32, palette *Palette) *PalettedStorage {
	var (
		bitsPerIndex       = uint16(len(indices) / uint32BitSize / uint32ByteSize)
		indexMask          = (uint32(1) << bitsPerIndex) - 1
		indicesStart       = (unsafe.Pointer)(unsafe.SliceData(indices))
		filledBitsPerIndex uint16
	)
	if bitsPerIndex != 0 {
		filledBitsPerIndex = uint32BitSize / bitsPerIndex * bitsPerIndex
	}
	return &PalettedStorage{filledBitsPerIndex: filledBitsPerIndex, indexMask: indexMask, indicesStart: indicesStart, bitsPerIndex: bitsPerIndex, indices: indices, palette: palette}
}

// emptyStorage creates a PalettedStorage filled completely with a value v.
func emptyStorage(v uint32) *PalettedStorage {
	return newPalettedStorage([]uint32{}, newPalette(0, []uint32{v}))
}

// Palette returns the Palette of the PalettedStorage.
func (storage *PalettedStorage) Palette() *Palette {
	return storage.palette
}

// At returns the value of the PalettedStorage at a given x, y and z.
func (storage *PalettedStorage) At(x, y, z byte) uint32 {
	return storage.palette.Value(storage.paletteIndex(x&15, y&15, z&15))
}

// Set sets a value at a specific x, y and z. The Palette and PalettedStorage are expanded
// automatically to make space for the value, should that be needed.
func (storage *PalettedStorage) Set(x, y, z byte, v uint32) {
	index := storage.palette.Index(v)
	if index == -1 {
		// The runtime ID was not yet available in the palette. We add it, then check if the block storage
		// needs to be resized for the palette pointers to fit.
		index = storage.addNew(v)
	}
	storage.setPaletteIndex(x&15, y&15, z&15, uint16(index))
}

// SetCached sets a value at a specific x, y and z, reusing palette indexes
// from cache when possible.
func (storage *PalettedStorage) SetCached(x, y, z byte, v uint32, cache *PaletteIndexCache) {
	if cache == nil {
		storage.Set(x, y, z, v)
		return
	}
	storage.setPaletteIndex(x&15, y&15, z&15, cache.index(storage, v))
}

// Fill sets every block in the half-open cuboid [x0,x1), [y0,y1), [z0,z1)
// to v. Coordinates must be within a single sub-chunk. Fill resolves v's
// palette index once, making it suitable for uniform bulk writes.
func (storage *PalettedStorage) Fill(x0, x1, y0, y1, z0, z1 byte, v uint32) {
	if x0 >= x1 || y0 >= y1 || z0 >= z1 {
		return
	}
	if x0 == 0 && x1 == 16 && y0 == 0 && y1 == 16 && z0 == 0 && z1 == 16 {
		*storage = *emptyStorage(v)
		return
	}

	index := storage.palette.Index(v)
	if index == -1 {
		index = storage.addNew(v)
	}
	i := uint16(index)
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			for z := z0; z < z1; z++ {
				storage.setPaletteIndex(x, y, z, i)
			}
		}
	}
}

// Equal checks if two PalettedStorages are equal value wise. False is returned
// if either of the storages are nil.
func (storage *PalettedStorage) Equal(other *PalettedStorage) bool {
	if storage == nil || other == nil {
		return false
	}
	if len(storage.indices) == 0 || len(other.indices) == 0 || storage.palette.values[0] == 0 || other.palette.values[0] == 0 {
		return false
	}
	indicesA := unsafe.Slice((*byte)(unsafe.Pointer(&storage.indices[0])), len(storage.indices)*4)
	indicesB := unsafe.Slice((*byte)(unsafe.Pointer(&other.indices[0])), len(other.indices)*4)
	if !bytes.Equal(indicesA, indicesB) {
		return false
	}
	paletteA := unsafe.Slice((*byte)(unsafe.Pointer(&storage.palette.values[0])), len(storage.palette.values)*4)
	paletteB := unsafe.Slice((*byte)(unsafe.Pointer(&other.palette.values[0])), len(other.palette.values)*4)
	return bytes.Equal(paletteA, paletteB)
}

// addNew adds a new value to the PalettedStorage's Palette and returns its index. If needed, the storage is resized.
func (storage *PalettedStorage) addNew(v uint32) int16 {
	index, resize := storage.palette.Add(v)
	if resize {
		storage.resize(storage.palette.size)
	}
	return index
}

func (cache *PaletteIndexCache) index(storage *PalettedStorage, v uint32) uint16 {
	if cache.lastSet && cache.lastValue == v {
		return cache.lastIndex
	}
	for i := uint8(0); i < cache.smallLen; i++ {
		entry := cache.small[i]
		if entry.value == v {
			cache.lastValue, cache.lastIndex, cache.lastSet = v, entry.index, true
			return entry.index
		}
	}
	if cache.overflow != nil {
		if index, ok := cache.overflow[v]; ok {
			cache.lastValue, cache.lastIndex, cache.lastSet = v, index, true
			return index
		}
	}

	index := storage.palette.Index(v)
	if index == -1 {
		index = storage.addNew(v)
	}
	i := uint16(index)
	if !cache.lastSet {
		cache.small[0] = paletteIndexCacheEntry{value: v, index: i}
		cache.smallLen = 1
		cache.lastValue, cache.lastIndex, cache.lastSet = v, i, true
		return i
	}
	if cache.smallLen < uint8(len(cache.small)) {
		cache.small[cache.smallLen] = paletteIndexCacheEntry{value: v, index: i}
		cache.smallLen++
		cache.lastValue, cache.lastIndex, cache.lastSet = v, i, true
		return i
	}
	if cache.overflow == nil {
		cache.overflow = make(map[uint32]uint16, 8)
		for j := uint8(0); j < cache.smallLen; j++ {
			entry := cache.small[j]
			cache.overflow[entry.value] = entry.index
		}
		cache.overflow[cache.lastValue] = cache.lastIndex
	}
	cache.overflow[v] = i
	cache.lastValue, cache.lastIndex, cache.lastSet = v, i, true
	return i
}

// paletteIndex looks up the Palette index at a given x, y and z value in the PalettedStorage. This palette
// index is not the value at this offset, but merely an index in the Palette pointing to a value.
func (storage *PalettedStorage) paletteIndex(x, y, z byte) uint16 {
	if storage.bitsPerIndex == 0 {
		// Unfortunately our default logic cannot deal with 0 bits per index, meaning we'll have to special case
		// this. This comes with a little performance hit, but it seems to be the only way to go. An alternative would
		// be not to have 0 bits per block storages in memory, but that would cause a strongly increased memory usage
		// by biomes.
		return 0
	}
	offset := ((uint16(x) << 8) | (uint16(z) << 4) | uint16(y)) * storage.bitsPerIndex
	uint32Offset, bitOffset := offset/storage.filledBitsPerIndex, offset%storage.filledBitsPerIndex

	w := *(*uint32)(unsafe.Pointer(uintptr(storage.indicesStart) + uintptr(uint32Offset<<2)))
	return uint16((w >> bitOffset) & storage.indexMask)
}

// setPaletteIndex sets the palette index at a given x, y and z to paletteIndex. This index should point
// to a value in the PalettedStorage's Palette.
func (storage *PalettedStorage) setPaletteIndex(x, y, z byte, i uint16) {
	if storage.bitsPerIndex == 0 {
		return
	}
	offset := ((uint16(x) << 8) | (uint16(z) << 4) | uint16(y)) * storage.bitsPerIndex
	uint32Offset, bitOffset := offset/storage.filledBitsPerIndex, offset%storage.filledBitsPerIndex

	ptr := (*uint32)(unsafe.Pointer(uintptr(storage.indicesStart) + uintptr(uint32Offset<<2)))
	*ptr = (*ptr &^ (storage.indexMask << bitOffset)) | (uint32(i) << bitOffset)
}

// resize changes the size of a PalettedStorage to newPaletteSize. A new PalettedStorage is constructed,
// and all values available in the current storage are set in their appropriate locations in the
// new storage.
func (storage *PalettedStorage) resize(newPaletteSize paletteSize) {
	if newPaletteSize == paletteSize(storage.bitsPerIndex) {
		return // Don't resize if the size is already equal.
	}
	// Construct a new storage and set all values in there manually. We can't easily do this in a better
	// way, because all values will be at a different index with a different length.
	newStorage := newPalettedStorage(make([]uint32, newPaletteSize.uint32s()), storage.palette)
	for x := byte(0); x < 16; x++ {
		for y := byte(0); y < 16; y++ {
			for z := byte(0); z < 16; z++ {
				newStorage.setPaletteIndex(x, y, z, storage.paletteIndex(x, y, z))
			}
		}
	}
	// Set the new storage.
	*storage = *newStorage
}

// compact clears unused indexes in the palette by scanning for usages in the PalettedStorage. This is a
// relatively heavy task which should only happen right before the sub chunk holding this PalettedStorage is
// saved to disk. compact also shrinks the palette size if possible.
func (storage *PalettedStorage) compact() {
	if storage.palette == nil || storage.palette.Len() == 0 {
		return
	}
	if storage.palette.Len() == 1 {
		// A single unique value can always be represented using 0 bits per index. This avoids scanning the
		// entire storage and drops any backing indices slice.
		storage.bitsPerIndex = 0
		storage.filledBitsPerIndex = 0
		storage.indexMask = 0
		storage.indicesStart = nil
		storage.indices = nil
		storage.palette.size = 0
		return
	}

	usedIndices := make([]bool, storage.palette.Len())
	for x := byte(0); x < 16; x++ {
		for y := byte(0); y < 16; y++ {
			for z := byte(0); z < 16; z++ {
				usedIndices[storage.paletteIndex(x, y, z)] = true
			}
		}
	}

	usedCount := 0
	allUsed := true
	for _, used := range usedIndices {
		if used {
			usedCount++
		} else {
			allUsed = false
		}
	}

	// If every palette entry is used and the palette size cannot shrink, nothing changes.
	// This avoids allocating a new indices slice and palette values slice for already-optimal storages.
	size := paletteSizeFor(usedCount)
	if allUsed && size == storage.palette.size {
		return
	}

	newRuntimeIDs := make([]uint32, 0, usedCount)
	conversion := make([]uint16, len(usedIndices))
	for index, used := range usedIndices {
		if used {
			conversion[index] = uint16(len(newRuntimeIDs))
			newRuntimeIDs = append(newRuntimeIDs, storage.palette.values[index])
		}
	}
	// Construct a new storage and set all values in there manually. We can't easily do this in a better
	// way, because all values will be at a different index with a different length.
	newStorage := newPalettedStorage(make([]uint32, size.uint32s()), newPalette(size, newRuntimeIDs))

	for x := byte(0); x < 16; x++ {
		for y := byte(0); y < 16; y++ {
			for z := byte(0); z < 16; z++ {
				// Replace all usages of the old palette indexes with the new indexes using the map we
				// produced earlier.
				newStorage.setPaletteIndex(x, y, z, conversion[storage.paletteIndex(x, y, z)])
			}
		}
	}
	*storage = *newStorage
}
