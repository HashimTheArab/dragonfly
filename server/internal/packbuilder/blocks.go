package packbuilder

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	_ "unsafe" // Imported for compiler directives.

	"github.com/df-mc/dragonfly/server/world"
)

// buildBlocks builds all the block-related files for the resource pack. This includes textures, geometries, language
// entries and terrain texture atlas.
func buildBlocks(reg world.BlockRegistry, dir string) (count int, lang []string) {
	if err := os.MkdirAll(filepath.Join(dir, "models/blocks"), os.ModePerm); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "textures/blocks"), os.ModePerm); err != nil {
		panic(err)
	}

	textureData := make(map[string]any)
	var flipbooks []map[string]any
	for identifier, blk := range reg.CustomBlocks() {
		b, ok := blk.(world.CustomBlockBuildable)
		if !ok {
			continue
		}

		lang = append(lang, fmt.Sprintf("tile.%s.name=%s", identifier, b.Name()))
		textures := b.Textures()
		for name, texture := range textures {
			textureData[name] = map[string]string{"textures": "textures/blocks/" + name}
			buildBlockTexture(dir, name, texture)
		}
		if animated, ok := b.(world.CustomBlockAnimated); ok {
			for name, flipbook := range animated.Flipbooks() {
				if _, ok := textures[name]; !ok {
					panic(fmt.Sprintf("flipbook of %s animates texture %s, which the block does not have", identifier, name))
				}
				flipbooks = append(flipbooks, flipbook.Encode(name, "textures/blocks/"+name))
			}
		}
		if b.Geometry() != nil {
			// Geometry files are named by the whole identifier: blocks of two namespaces may share a name.
			namespace, name, _ := strings.Cut(identifier, ":")
			if err := os.MkdirAll(filepath.Join(dir, "models/blocks", namespace), os.ModePerm); err != nil {
				panic(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "models/blocks", namespace, name+".geo.json"), b.Geometry(), 0666); err != nil {
				panic(err)
			}
		}
		count++
	}
	if len(flipbooks) > 0 {
		// Sorted, so that the pack, and with it its UUID, only changes when a flipbook does.
		slices.SortFunc(flipbooks, func(a, b map[string]any) int {
			return strings.Compare(a["atlas_tile"].(string), b["atlas_tile"].(string))
		})
		buildBlockFlipbooks(dir, flipbooks)
	}

	buildBlockAtlas(dir, map[string]any{
		"resource_pack_name": "vanilla",
		"texture_name":       "atlas.terrain",
		"padding":            8,
		"num_mip_levels":     4,
		"texture_data":       textureData,
	})
	return
}

// buildBlockTexture creates a PNG file for the block from the provided image and name and writes it to the pack.
func buildBlockTexture(dir, name string, img image.Image) {
	texture, err := os.Create(filepath.Join(dir, fmt.Sprintf("textures/blocks/%s.png", name)))
	if err != nil {
		panic(err)
	}
	if err := png.Encode(texture, img); err != nil {
		_ = texture.Close()
		panic(err)
	}
	if err := texture.Close(); err != nil {
		panic(err)
	}
}

// buildBlockFlipbooks writes the animations of block textures to the pack.
func buildBlockFlipbooks(dir string, flipbooks []map[string]any) {
	b, err := json.Marshal(flipbooks)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "textures/flipbook_textures.json"), b, 0666); err != nil {
		panic(err)
	}
}

// buildBlockAtlas creates the identifier to texture mapping and writes it to the pack.
func buildBlockAtlas(dir string, atlas map[string]any) {
	b, err := json.Marshal(atlas)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "textures/terrain_texture.json"), b, 0666); err != nil {
		panic(err)
	}
}
