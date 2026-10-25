package packbuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/tailscale/hujson"
	"golang.org/x/mod/sumdb/dirhash"
)

// buildEntities adds registered client entities and their resource files.
func buildEntities(dir string, registries []world.EntityRegistry) int {
	count := 0
	for _, registry := range registries {
		for _, t := range registry.Types() {
			custom, ok := t.(world.ResourcePackEntityType)
			if !ok {
				continue
			}
			id := world.EntityNetworkIdentifier(t)
			resources := custom.Resources()
			checkEntityDefinition(resources.ClientEntity, "minecraft:client_entity", id)
			writeEntityFile(dir, "entity/"+entityResourceName(id)+".entity.json", resources.ClientEntity)
			for name, data := range resources.Assets {
				if name == "manifest.json" || name == "pack_icon.png" || strings.HasPrefix(name, "entity/") {
					panic("reserved entity resource path: " + name)
				}
				writeEntityFile(dir, name, data)
			}
			count++
		}
	}
	return count
}

// BuildBehaviourPack builds optional custom entity behaviour definitions.
func BuildBehaviourPack(registry world.EntityRegistry) (*resource.Pack, bool) {
	dir, err := os.MkdirTemp("", "dragonfly_behaviour_pack-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	count := 0
	for _, t := range registry.Types() {
		custom, ok := t.(world.ResourcePackEntityType)
		if !ok {
			continue
		}
		definition := custom.Resources().Definition
		if len(definition) == 0 {
			continue
		}
		id := world.EntityNetworkIdentifier(t)
		checkEntityDefinition(definition, "minecraft:entity", id)
		writeEntityFile(dir, "entities/"+entityResourceName(id)+".json", definition)
		count++
	}
	if count == 0 {
		return nil, false
	}
	hash, err := dirhash.HashDir(dir, "", dirhash.Hash1)
	if err != nil {
		panic(err)
	}
	var header, module [16]byte
	copy(header[:], hash)
	copy(module[:], hash[16:])
	buildManifestType(dir, header, module, "data")
	return resource.MustReadPath(dir), true
}

// checkEntityDefinition refuses an invalid or mismatched entity identity.
func checkEntityDefinition(data []byte, key, id string) {
	standard, err := hujson.Standardize(bytes.Clone(data))
	if err != nil {
		panic(fmt.Errorf("entity %s: %w", id, err))
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(standard, &document); err != nil {
		panic(fmt.Errorf("entity %s: %w", id, err))
	}
	var entity struct {
		Description struct {
			Identifier string `json:"identifier"`
		} `json:"description"`
	}
	if err := json.Unmarshal(document[key], &entity); err != nil {
		panic(fmt.Errorf("entity %s: %w", id, err))
	}
	if entity.Description.Identifier != id {
		panic("entity resource identifier does not match " + id)
	}
}

// entityResourceName preserves the namespace without allowing path traversal.
func entityResourceName(id string) string {
	namespace, name, ok := strings.Cut(id, ":")
	if !ok || namespace == "" || name == "" || strings.ContainsAny(namespace+name, "/\\") ||
		namespace == "." || namespace == ".." || name == "." || name == ".." {
		panic("invalid entity resource identifier: " + id)
	}
	return namespace + "/" + name
}

// writeEntityFile refuses paths outside the pack and collisions with other assets.
func writeEntityFile(dir, name string, data []byte) {
	if name == "." || !filepath.IsLocal(name) || strings.Contains(name, "\\") || path.Clean(name) != name {
		panic("invalid entity resource path: " + name)
	}
	file := filepath.Join(dir, filepath.FromSlash(name))
	if existing, err := os.ReadFile(file); err == nil && bytes.Equal(existing, data) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		panic(err)
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		panic(err)
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		panic(err)
	}
	if closeErr != nil {
		panic(closeErr)
	}
}
