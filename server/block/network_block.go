package block

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// UnresolvedNetworkBlock preserves a server state whose local behaviour could not be decoded.
// Consumers must not use its placeholder model for automated targeting or movement prediction.
// The original definition can still be forwarded to a client that understands it.
type UnresolvedNetworkBlock interface {
	world.Block
	BehaviourError() error
}

type unresolvedNetworkBlock struct {
	networkBlock
	cause error
}

type unresolvedNetworkModel struct{ networkBlockModel }

// FaceSolid refuses local attachments when the supporting shape is unknown.
func (unresolvedNetworkModel) FaceSolid(cube.Pos, cube.Face, world.BlockSource) bool { return false }

// Model returns an obstruction placeholder, not a usable prediction or selection shape.
func (b unresolvedNetworkBlock) Model() world.BlockModel { return unresolvedNetworkModel{b.model} }

// BehaviourError explains why this state has no reliable local behaviour model.
func (b unresolvedNetworkBlock) BehaviourError() error { return b.cause }

// unresolvedBlock keeps IDs usable without claiming mining or attachment support.
func unresolvedBlock(entry protocol.BlockEntry, state map[string]any, cause error) world.Block {
	return unresolvedNetworkBlock{networkBlock: networkBlock{
		state:    world.BlockState{Name: entry.Name, Properties: state},
		friction: 0.6, dampening: 15,
		model: networkBlockModel{collision: []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}},
	}, cause: cause}
}

// networkBlock is an immutable, session-owned implementation of a server-defined state.
// The server remains responsible for drops, scripts, and other authoritative behaviour.
type networkBlock struct {
	state               world.BlockState
	model               networkBlockModel
	friction            float64
	emission, dampening uint8
	tags                []string
}

// EncodeBlock returns a copy so a consumer cannot mutate a shared session definition.
func (b networkBlock) EncodeBlock() (string, map[string]any) {
	return b.state.Name, maps.Clone(b.state.Properties)
}

// Hash selects the registry's state lookup instead of a process-global Go block hash.
func (networkBlock) Hash() (uint64, uint64) { return 0, math.MaxUint64 }

// Model returns the resolved collision and selection geometry.
func (b networkBlock) Model() world.BlockModel { return b.model }

// Tags returns the state-local block tags advertised by the server.
func (b networkBlock) Tags() []string { return slices.Clone(b.tags) }

// Friction returns the drag multiplier used by Dragonfly's movement simulation.
func (b networkBlock) Friction() float64 { return b.friction }

// LightEmissionLevel returns the light emitted by this state.
func (b networkBlock) LightEmissionLevel() uint8 { return b.emission }

// LightDiffusionLevel returns the light blocked by this state.
func (b networkBlock) LightDiffusionLevel() uint8 { return b.dampening }

type mineableNetworkBlock struct {
	networkBlock
	hardness float64
	rules    []networkMiningRule
}

// BreakInfo returns the hardness decoded from the network component.
func (b mineableNetworkBlock) BreakInfo() BreakInfo {
	return b.breakInfoForItem(item.Stack{})
}

type networkBlockModel struct{ collision, selection []cube.BBox }

// BBox returns a copy of this state's collision boxes.
func (m networkBlockModel) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	return slices.Clone(m.collision)
}

// SelectionBBox returns a copy of the boxes the client uses to target this block.
func (m networkBlockModel) SelectionBBox(cube.Pos, world.BlockSource) []cube.BBox {
	return slices.Clone(m.selection)
}

// FaceSolid reports a full supporting face only when a collision box covers it.
func (m networkBlockModel) FaceSolid(_ cube.Pos, face cube.Face, _ world.BlockSource) bool {
	// Vec3 uses X, Y, Z order; cube.Axis does not.
	axis := 0
	for i, component := range face.Axis().Vec3() {
		if component != 0 {
			axis = i
			break
		}
	}
	for _, box := range m.collision {
		lo, hi := box.Min(), box.Max()
		a, b := (axis+1)%3, (axis+2)%3
		if lo[a] > 0 || hi[a] < 1 || lo[b] > 0 || hi[b] < 1 {
			continue
		}
		if face == cube.FaceDown || face == cube.FaceNorth || face == cube.FaceWest {
			if lo[axis] <= 0 && hi[axis] > 0 {
				return true
			}
		} else if hi[axis] >= 1 && lo[axis] < 1 {
			return true
		}
	}
	return false
}

// decodeNetworkBlock resolves the components for one state before the registry is published.
func decodeNetworkBlock(entry protocol.BlockEntry, state map[string]any) (world.Block, error) {
	components, err := networkBlockComponents(entry, state)
	if err != nil {
		return nil, err
	}
	full := []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	b := networkBlock{state: world.BlockState{Name: entry.Name, Properties: state}, friction: 0.6, dampening: 15}
	if raw := entry.Properties["blockTags"]; raw != nil {
		values, ok := customBlockEnumValues(raw)
		if !ok {
			return nil, fmt.Errorf("blockTags must be a list")
		}
		for _, value := range values {
			tag, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("block tag must be a string")
			}
			b.tags = append(b.tags, tag)
		}
	}
	for name := range components {
		if tag, ok := strings.CutPrefix(name, "tag:"); ok {
			b.tags = append(b.tags, tag)
		}
	}
	slices.Sort(b.tags)
	b.tags = slices.Compact(b.tags)

	b.model.collision, err = networkBlockBoxes(components["minecraft:collision_box"], full)
	if err != nil {
		return nil, fmt.Errorf("collision_box: %w", err)
	}
	b.model.selection, err = networkBlockBoxes(components["minecraft:selection_box"], full)
	if err != nil {
		return nil, fmt.Errorf("selection_box: %w", err)
	}
	if raw, ok := components["minecraft:friction"]; ok {
		component, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("friction requires a network value compound, not a pack JSON scalar")
		}
		b.friction, err = networkComponentNumber(component, "value")
		if err != nil || b.friction < 0 || b.friction > 1 {
			return nil, fmt.Errorf("invalid friction %v", raw)
		}
	}
	for name, dst := range map[string]*uint8{"minecraft:light_emission": &b.emission, "minecraft:light_dampening": &b.dampening} {
		if raw, ok := components[name]; ok {
			key := "emission"
			if name == "minecraft:light_dampening" {
				key = "lightLevel"
			}
			n, e := networkComponentNumber(raw, key)
			if e != nil || n < 0 || n > 15 || math.Trunc(n) != n {
				return nil, fmt.Errorf("invalid %s", name)
			}
			*dst = uint8(n)
		}
	}
	if raw, ok := components["minecraft:transformation"]; ok {
		b.model, err = transformNetworkBlockModel(b.model, raw)
		if err != nil {
			return nil, err
		}
	}
	raw, ok := components["minecraft:destructible_by_mining"]
	if !ok {
		return b, nil
	} // The network representation omits this component for unbreakable blocks.
	if enabled, err := customBlockTraitEnabled(raw); err == nil {
		if !enabled {
			return b, nil
		}
		return mineableNetworkBlock{networkBlock: b}, nil
	}
	hardness, err := networkComponentNumber(raw, "value")
	if err != nil {
		return nil, fmt.Errorf("destructible_by_mining: %w", err)
	}
	if m, ok := raw.(map[string]any); ok {
		rules, err := decodeNetworkMiningRules(m["item_specific_speeds"])
		if err != nil {
			return nil, err
		}
		if len(rules) != 0 {
			return mineableNetworkBlock{networkBlock: b, hardness: hardness, rules: rules}, nil
		}
	}
	if hardness < 0 {
		return b, nil
	}
	return mineableNetworkBlock{networkBlock: b, hardness: hardness}, nil
}

// networkComponentNumber reads the wire scalar or its named compound field.
func networkComponentNumber(raw any, keys ...string) (float64, error) {
	if m, ok := raw.(map[string]any); ok {
		for _, key := range keys {
			if v, exists := m[key]; exists {
				return networkComponentNumber(v)
			}
		}
		return 0, fmt.Errorf("missing %v", keys)
	}
	var n float64
	switch v := raw.(type) {
	case float32:
		n = float64(v)
	case float64:
		n = v
	case int32:
		n = float64(v)
	case int64:
		n = float64(v)
	case uint8:
		n = float64(v)
	default:
		return 0, fmt.Errorf("expected number, got %T", raw)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("non-finite number")
	}
	return n, nil
}

// networkBlockComponents overlays matching permutations in their transmitted order.
func networkBlockComponents(entry protocol.BlockEntry, state map[string]any) (map[string]any, error) {
	components := map[string]any{}
	if raw := entry.Properties["components"]; raw != nil {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("components must be a compound")
		}
		maps.Copy(components, m)
	}
	permutations, err := customBlockMaps(entry.Properties["permutations"], "permutations")
	if err != nil {
		return nil, err
	}
	for _, p := range permutations {
		condition, ok := p["condition"].(string)
		if !ok {
			return nil, fmt.Errorf("permutation condition must be a string")
		}
		applies, err := evaluateBlockCondition(condition, state)
		if err != nil {
			return nil, err
		}
		if !applies {
			continue
		}
		m, ok := p["components"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("permutation components must be a compound")
		}
		maps.Copy(components, m)
	}
	return components, nil
}
