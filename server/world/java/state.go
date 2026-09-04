// Package java converts Java block states into states from a Bedrock registry.
package java

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

//go:generate python3 generate.py
//go:embed mapping.json.gz
var mappingData []byte

// State holds Java property strings, before conversion to Bedrock property types.
type State struct {
	Name       string
	Properties map[string]string
}

// Parse reads a Java state string with optional namespace and bracketed properties.
func Parse(value string) (State, error) {
	name, raw, bracket := strings.Cut(strings.TrimSpace(value), "[")
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, " ,]\t\r\n") {
		return State{}, fmt.Errorf("invalid block name %q", name)
	}
	if !strings.Contains(name, ":") {
		name = "minecraft:" + name
	}
	state := State{Name: name, Properties: map[string]string{}}
	if !bracket {
		return state, nil
	}
	if !strings.HasSuffix(raw, "]") {
		return State{}, fmt.Errorf("unterminated properties in %q", value)
	}
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return state, nil
	}
	for _, pair := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(pair, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" || strings.ContainsAny(key+value, "[]= \t\r\n") {
			return State{}, fmt.Errorf("invalid block property %q", pair)
		}
		if _, exists := state.Properties[key]; exists {
			return State{}, fmt.Errorf("duplicate block property %q", key)
		}
		state.Properties[key] = value
	}
	return state, nil
}

// String produces the canonical ordering used by the conversion table.
func (s State) String() string {
	var result strings.Builder
	result.WriteString(s.Name)
	result.WriteByte('[')
	for i, key := range slices.Sorted(maps.Keys(s.Properties)) {
		if i != 0 {
			result.WriteByte(',')
		}
		result.WriteString(key)
		result.WriteByte('=')
		result.WriteString(s.Properties[key])
	}
	result.WriteByte(']')
	return result.String()
}

type conversionData struct {
	States   map[string]string            `json:"states"`
	Defaults map[string]map[string]string `json:"defaults"`
	Legacy   map[string]string            `json:"legacy"`
}

var loadMapping = sync.OnceValues(func() (conversionData, error) {
	reader, err := gzip.NewReader(bytes.NewReader(mappingData))
	if err != nil {
		return conversionData{}, err
	}
	defer reader.Close()
	var data conversionData
	err = json.NewDecoder(reader).Decode(&data)
	return data, err
})

// Resolve converts a Java state, supplying Java defaults only for omitted
// properties. Unknown states fail instead of falling back to a different block.
func Resolve(registry world.BlockRegistry, state State) (world.BlockState, error) {
	data, err := loadMapping()
	if err != nil {
		return world.BlockState{}, fmt.Errorf("load Java block mapping: %w", err)
	}
	// These Java names were renamed after the legacy-ID table was published.
	switch state.Name {
	case "minecraft:grass":
		state.Name = "minecraft:short_grass"
	case "minecraft:grass_path":
		state.Name = "minecraft:dirt_path"
	case "minecraft:sign":
		state.Name = "minecraft:oak_sign"
	case "minecraft:wall_sign":
		state.Name = "minecraft:oak_wall_sign"
	}
	defaults, ok := data.Defaults[state.Name]
	if !ok {
		return world.BlockState{}, fmt.Errorf("unsupported Java block %q", state.Name)
	}
	properties := maps.Clone(defaults)
	for key, value := range state.Properties {
		if _, ok := defaults[key]; !ok {
			return world.BlockState{}, fmt.Errorf("unknown Java property %q for %s", key, state.Name)
		}
		properties[key] = value
	}
	mapped, ok := data.States[(State{Name: state.Name, Properties: properties}).String()]
	if !ok {
		return world.BlockState{}, fmt.Errorf("unsupported Java block state %s", state)
	}
	bedrock, err := Parse(mapped)
	if err != nil {
		return world.BlockState{}, fmt.Errorf("invalid mapped Bedrock state: %w", err)
	}
	if bedrock.Name == "minecraft:air" && state.Name != "minecraft:air" && state.Name != "minecraft:cave_air" && state.Name != "minecraft:void_air" {
		return world.BlockState{}, fmt.Errorf("Java block %s has no Bedrock equivalent", state.Name)
	}
	values := make(map[string]any, len(bedrock.Properties))
	for key, value := range bedrock.Properties {
		switch value {
		case "true":
			values[key] = uint8(1)
		case "false":
			values[key] = uint8(0)
		default:
			if number, err := strconv.ParseInt(value, 10, 32); err == nil {
				values[key] = int32(number)
			} else {
				values[key] = value
			}
		}
	}
	const sourceVersion = 1<<24 | 26<<16 | 30<<8
	block, ok := world.ResolveBlockState(registry, world.BlockState{Name: bedrock.Name, Properties: values, Version: sourceVersion})
	if !ok {
		return world.BlockState{}, fmt.Errorf("mapped Java block %s is unavailable in the Bedrock registry", state)
	}
	name, resolved := block.EncodeBlock()
	return world.BlockState{Name: name, Properties: maps.Clone(resolved), Version: chunk.CurrentBlockVersion}, nil
}

// Legacy resolves the numeric ID and metadata used by pre-flattening Java worlds.
func Legacy(registry world.BlockRegistry, id uint16, metadata uint8) (world.BlockState, error) {
	data, err := loadMapping()
	if err != nil {
		return world.BlockState{}, err
	}
	encoded, ok := data.Legacy[fmt.Sprintf("%d:%d", id, metadata)]
	if !ok {
		return world.BlockState{}, fmt.Errorf("unsupported legacy Java block %d:%d", id, metadata)
	}
	state, err := Parse(encoded)
	if err != nil {
		return world.BlockState{}, err
	}
	return Resolve(registry, state)
}
