package world

import (
	"encoding/binary"
	"fmt"
	"maps"
	"sort"

	"github.com/segmentio/fasthash/fnv1a"
)

// NetworkHashSaltProperty is the hidden int32 state a registry with hashed network IDs adds to a custom block whose
// network block hashes collide with another state's (BasicBlockRegistry.UseHashedNetworkIDs). No block may declare
// a state of this name.
const NetworkHashSaltProperty = "dragonfly:network_hash_salt"

// maxNetworkHashSalt is the largest salt tried for one block. Each salt rehashes all of the block's states, and one
// attempt fails only if one of them meets another state's hash, so even a block of 65536 states beside a million
// others succeeds within a few salts.
const maxNetworkHashSalt = 1 << 16

// indexNetworkHashesLocked builds networkhashToRids from ridsToNetworkhash, after salting colliding custom blocks if
// the registry uses hashed network IDs. br.mu must be held.
func (br *BasicBlockRegistry) indexNetworkHashesLocked() error {
	br.networkHashSalts = nil
	if br.hashedNetworkIDs {
		if err := br.saltNetworkHashesLocked(); err != nil {
			return err
		}
	}
	br.mapNetworkHashesLocked()
	return nil
}

// mapNetworkHashesLocked builds networkhashToRids from ridsToNetworkhash. A shared hash maps to the lowest runtime ID.
// br.mu must be held.
func (br *BasicBlockRegistry) mapNetworkHashesLocked() {
	br.networkhashToRids = make(map[uint32]uint32, len(br.ridsToNetworkhash))
	for rid, h := range br.ridsToNetworkhash {
		if _, ok := br.networkhashToRids[h]; !ok {
			br.networkhashToRids[h] = uint32(rid)
		}
	}
}

// restoreNetworkHashSaltsLocked gives the states of each block in salts the network block hashes that salt produced
// when the registry was recorded, then rebuilds networkhashToRids. br.mu must be held.
func (br *BasicBlockRegistry) restoreNetworkHashSaltsLocked(salts map[string]int32) {
	if len(salts) == 0 {
		return
	}
	br.networkHashSalts = maps.Clone(salts)
	var scratch []byte
	salted := make(map[string]any)
	for rid, b := range br.blocks {
		name, properties := b.EncodeBlock()
		salt, ok := salts[name]
		if !ok {
			continue
		}
		clear(salted)
		maps.Copy(salted, properties)
		salted[NetworkHashSaltProperty] = salt
		br.ridsToNetworkhash[rid], scratch = networkBlockHash(name, salted, scratch)
	}
	br.mapNetworkHashesLocked()
}

// saltNetworkHashesLocked makes every hash in ridsToNetworkhash unique. Walking runtime IDs in order, a state whose
// hash an earlier state already has marks its own block for salting if that block is custom, else the earlier
// state's block if that one is, and fails if neither is. Each marked block, in the order marked, then gets the
// smallest NetworkHashSaltProperty value from 1 up under which its states' hashes are unique among all hashes, and its
// states take those hashes. The result depends only on the registered states and their order. br.mu must be held.
func (br *BasicBlockRegistry) saltNetworkHashesLocked() error {
	held := make(map[uint32]int, len(br.ridsToNetworkhash))
	owner := make(map[uint32]uint32, len(br.ridsToNetworkhash))
	var marked []string
	for rid, h := range br.ridsToNetworkhash {
		held[h]++
		other, ok := owner[h]
		if !ok {
			owner[h] = uint32(rid)
			continue
		}
		name, ok := br.saltableBlockLocked(uint32(rid), other)
		if !ok {
			name, properties := br.blocks[rid].EncodeBlock()
			otherName, otherProperties := br.blocks[other].EncodeBlock()
			return fmt.Errorf("network block hash %#08x of (%s %+v) is already (%s %+v)'s, and neither is a custom block whose hashes can be salted", h, name, properties, otherName, otherProperties)
		}
		if _, ok := br.networkHashSalts[name]; !ok {
			if br.networkHashSalts == nil {
				br.networkHashSalts = make(map[string]int32)
			}
			br.networkHashSalts[name] = 0
			marked = append(marked, name)
		}
	}
	if len(marked) == 0 {
		return nil
	}

	states := make(map[string][]uint32, len(marked))
	for rid, b := range br.blocks {
		name, _ := b.EncodeBlock()
		if _, ok := br.networkHashSalts[name]; ok {
			states[name] = append(states[name], uint32(rid))
		}
	}
	var scratch []byte
	salted := make(map[string]any)
	for _, name := range marked {
		rids := states[name]
		for _, rid := range rids {
			held[br.ridsToNetworkhash[rid]]--
		}
		hashes := make([]uint32, 0, len(rids))
		for salt := int32(1); len(hashes) < len(rids); salt++ {
			if salt > maxNetworkHashSalt {
				return fmt.Errorf("custom block %s: no %s value up to %d gives its states unique network block hashes", name, NetworkHashSaltProperty, maxNetworkHashSalt)
			}
			for _, h := range hashes {
				held[h]--
			}
			hashes = hashes[:0]
			for _, rid := range rids {
				_, properties := br.blocks[rid].EncodeBlock()
				clear(salted)
				maps.Copy(salted, properties)
				salted[NetworkHashSaltProperty] = salt
				var h uint32
				h, scratch = networkBlockHash(name, salted, scratch)
				if held[h] > 0 {
					break
				}
				held[h]++
				hashes = append(hashes, h)
			}
			br.networkHashSalts[name] = salt
		}
		for i, rid := range rids {
			br.ridsToNetworkhash[rid] = hashes[i]
		}
	}
	return nil
}

// saltableBlockLocked returns the name of the custom block of the state at rid, else of the state at other, that two
// states sharing a network block hash can be told apart by salting. br.mu must be held.
func (br *BasicBlockRegistry) saltableBlockLocked(rid, other uint32) (string, bool) {
	for _, r := range [2]uint32{rid, other} {
		name, _ := br.blocks[r].EncodeBlock()
		if _, ok := br.customBlocks[name]; ok {
			return name, true
		}
	}
	return "", false
}

// networkBlockHash produces the canonical "network block hash" for a (name, properties) block state.
// This hash is used for mapping network palette entries back to runtime IDs.
//
// The scratch slice is used to reduce allocations. The returned slice should be passed back in on subsequent calls to
// reuse the same backing array.
func networkBlockHash(name string, properties map[string]any, scratch []byte) (uint32, []byte) {
	if name == "minecraft:unknown" {
		return 0xfffffffe, scratch // -2
	}

	keys := make([]string, 0, len(properties))
	for k := range properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	data := scratch[:0]
	writeString := func(str string) {
		data = binary.LittleEndian.AppendUint16(data, uint16(len(str)))
		data = append(data, []byte(str)...)
	}

	data = append(data, 10) // compound
	data = append(data, 0)
	data = append(data, 0)

	data = append(data, 8) // string
	writeString("name")
	writeString(name)

	data = append(data, 10) // compound
	writeString("states")
	for _, k := range keys {
		v := properties[k]
		switch v := v.(type) {
		case string:
			data = append(data, 8) // string
			writeString(k)
			writeString(v)

		case uint8:
			data = append(data, 1) // tagByte
			writeString(k)
			data = append(data, byte(v))
		case int8:
			data = append(data, 1) // tagByte
			writeString(k)
			data = append(data, byte(v))
		case bool:
			b := 0
			if v {
				b = 1
			}
			data = append(data, 1) // tagByte
			writeString(k)
			data = append(data, byte(b))

		case uint16:
			data = append(data, 2) // tagInt16
			writeString(k)
			data = binary.LittleEndian.AppendUint16(data, uint16(v))
		case int16:
			data = append(data, 2) // tagInt16
			writeString(k)
			data = binary.LittleEndian.AppendUint16(data, uint16(v))

		case uint32:
			data = append(data, 3) // tagInt32
			writeString(k)
			data = binary.LittleEndian.AppendUint32(data, uint32(v))
		case int32:
			data = append(data, 3) // tagInt32
			writeString(k)
			data = binary.LittleEndian.AppendUint32(data, uint32(v))
		default:
			panic(fmt.Sprintf("unhandled nbt type: %T", v))
		}
	}
	data = append(data, 0) // end
	data = append(data, 0) // end

	return fnv1a.HashBytes32(data), data
}
