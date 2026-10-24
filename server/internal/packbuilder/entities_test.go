package packbuilder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// resourceEntityType supplies a custom network identity and independent resources.
type resourceEntityType struct{ resources world.EntityResources }

func (resourceEntityType) EncodeEntity() string        { return "test:saved_ship" }
func (resourceEntityType) NetworkEncodeEntity() string { return "test:ship" }
func (resourceEntityType) BBox(world.Entity) cube.BBox { return cube.BBox{} }
func (resourceEntityType) Open(*world.Tx, *world.EntityHandle, *world.EntityData) world.Entity {
	return nil
}
func (resourceEntityType) DecodeNBT(map[string]any, *world.EntityData) {}
func (resourceEntityType) EncodeNBT(*world.EntityData) map[string]any  { return nil }
func (t resourceEntityType) Resources() world.EntityResources          { return t.resources }

func TestBuildEntityPacksUseNetworkIdentity(t *testing.T) {
	client := []byte(`{"format_version":"1.10.0","minecraft:client_entity":{"description":{"identifier":"test:ship"}}}`)
	definition := []byte(`{"format_version":"1.20.0","minecraft:entity":{"description":{"identifier":"test:ship"}}}`)
	reg := world.EntityRegistryConfig{}.New([]world.EntityType{resourceEntityType{resources: world.EntityResources{ClientEntity: client, Definition: definition, Assets: map[string][]byte{"models/entity/test/ship.geo.json": []byte(`{}`)}}}})
	dir := t.TempDir()
	if count := buildEntities(dir, []world.EntityRegistry{reg}); count != 1 {
		t.Fatalf("entity count: %d", count)
	}
	data, err := os.ReadFile(filepath.Join(dir, "entity/test/ship.entity.json"))
	if err != nil || string(data) != string(client) {
		t.Fatalf("client definition: %s, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "models/entity/test/ship.geo.json")); err != nil {
		t.Fatal(err)
	}
	pack, ok := BuildBehaviourPack(reg)
	if !ok {
		t.Fatal("no behaviour pack")
	}
	data, err = pack.ReadFile("entities/test/ship.json")
	if err != nil || string(data) != string(definition) {
		t.Fatalf("behaviour definition: %s, %v", data, err)
	}
}

func TestEntityResourceRejectsUnsafePathsAndIdentity(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "models/../escape", "models\\escape"} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("unsafe path accepted")
				}
			}()
			writeEntityFile(t.TempDir(), name, nil)
		})
	}
	t.Run("identity", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("mismatched identity accepted")
			}
		}()
		checkEntityDefinition([]byte(`{"minecraft:client_entity":{"description":{"identifier":"other:ship"}}}`), "minecraft:client_entity", "test:ship")
	})
}
