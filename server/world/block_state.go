package world

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

var (
	//go:embed block_states.nbt
	blockStateData []byte

	//go:embed data_driven_blocks.nbt
	dataDrivenBlockData []byte
	dataDrivenBlocks    []DataDrivenBlock
)

func init() {
	states, err := decodeBlockStates(blockStateData)
	if err != nil {
		panic(err)
	}
	for _, s := range states {
		DefaultBlockRegistry.RegisterBlockState(s)
	}

	var m struct {
		Blocks []DataDrivenBlock `nbt:"blocks"`
	}
	if err := nbt.UnmarshalEncoding(dataDrivenBlockData, &m, nbt.LittleEndian); err != nil {
		panic(err)
	}
	dataDrivenBlocks = m.Blocks
}

// decodeBlockStates reads the full embedded palette and rejects a damaged record.
func decodeBlockStates(data []byte) ([]BlockState, error) {
	reader := bytes.NewReader(data)
	decoder := nbt.NewDecoder(reader)
	var states []BlockState
	for reader.Len() > 0 {
		var state BlockState
		if err := decoder.Decode(&state); err != nil {
			return nil, fmt.Errorf("decode vanilla block state %d: %w", len(states), err)
		}
		states = append(states, state)
	}
	return states, nil
}

// DataDrivenBlock holds the name of a block and the components that define it.
type DataDrivenBlock struct {
	Name       string         `nbt:"name"`
	Components map[string]any `nbt:"components"`
}

// DataDrivenBlocks returns the vanilla data driven blocks. A client needs
// these to complete its block palette.
func DataDrivenBlocks() []DataDrivenBlock {
	return dataDrivenBlocks
}

// BlockState holds a combination of a name and properties, together with a version.
type BlockState struct {
	Name       string         `nbt:"name"`
	Properties map[string]any `nbt:"states"`
	Version    int32          `nbt:"version"`
}

// unknownBlock represents a block that has not yet been implemented. It is used for registering block
// states that haven't yet been added.
type unknownBlock struct {
	BlockState
	data map[string]any
	// placementItem supplies inventory semantics without installing block behaviour.
	placementItem Item
}

// IsUnknownBlock reports whether b is a palette state without an implemented
// block type. Its native interaction behavior is not known to Dragonfly.
func IsUnknownBlock(b Block) bool {
	_, unknown := b.(unknownBlock)
	return unknown
}

// EncodeBlock ...
func (b unknownBlock) EncodeBlock() (string, map[string]any) {
	return b.Name, b.Properties
}

// Model ...
func (unknownBlock) Model() BlockModel {
	return unknownModel{}
}

// ContainerSize returns the client-visible capacity of chest blocks that are
// present in the palette but do not yet have a dedicated block implementation.
func (b unknownBlock) ContainerSize() int {
	if b.Name != "minecraft:trapped_chest" && !strings.HasSuffix(b.Name, "copper_chest") {
		return 0
	}
	_, pairedX := b.data["pairx"].(int32)
	_, pairedZ := b.data["pairz"].(int32)
	if pairedX && pairedZ {
		return 54
	}
	return 27
}

// Friction returns the vanilla friction of the block, so a block dragonfly has no type for
// still slows or slips the way players expect. block.Frictional is matched structurally.
func (b unknownBlock) Friction() float64 {
	if p, ok := unknownBlockProps[b.Name]; ok {
		return p.friction
	}
	return 0.6
}

// LightDiffusionLevel returns how much light the block filters. Without this every unknown
// block is treated as fully opaque, which darkens anything behind glass or a fence.
func (b unknownBlock) LightDiffusionLevel() uint8 {
	if p, ok := unknownBlockProps[b.Name]; ok {
		return p.dampening
	}
	return 15
}

// LightEmissionLevel returns the light the block gives off. Blocks whose emission depends on
// their state report the resting value, since the state is not modelled.
func (b unknownBlock) LightEmissionLevel() uint8 {
	if p, ok := unknownBlockProps[b.Name]; ok {
		return p.emission
	}
	return 0
}

// Hash ...
func (b unknownBlock) Hash() (uint64, uint64) {
	return 0, math.MaxUint64
}

// EncodeNBT ...
func (b unknownBlock) EncodeNBT() map[string]any {
	return maps.Clone(b.data)
}

// DecodeNBT ...
func (b unknownBlock) DecodeNBT(data map[string]any) any {
	b.data = maps.Clone(data)
	return b
}

// stateHash is a struct that may be used as a map key for block states. It contains the name of the block state
// and an encoded version of the properties.
type stateHash struct {
	name, properties string
}

// hashProperties produces a hash for the block properties held by the BlockState.
func hashProperties(properties map[string]any) string {
	if properties == nil {
		return ""
	}
	keys := make([]string, 0, len(properties))
	for k := range properties {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	// Each property is written as its name and a tagged value, so distinct states never share a key.
	// A bool and a byte are the same tag on the network and share an encoding.
	var b []byte
	for _, k := range keys {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(k)))
		b = append(b, k...)
		switch v := properties[k].(type) {
		case bool:
			if v {
				b = append(b, 1, 1)
			} else {
				b = append(b, 1, 0)
			}
		case uint8:
			b = append(b, 1, v)
		case int32:
			b = binary.LittleEndian.AppendUint32(append(b, 3), uint32(v))
		case string:
			b = binary.LittleEndian.AppendUint32(append(b, 8), uint32(len(v)))
			b = append(b, v...)
		default:
			// Any other NBT value is written as its full NBT entry, which carries its tag type.
			entry := appendNBTEntry(nil, "", v)
			b = binary.LittleEndian.AppendUint32(append(b, 0xff), uint32(len(entry)))
			b = append(b, entry...)
		}
	}
	return string(b)
}
