package packbuilder

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/df-mc/dragonfly/server/item/category"
	"github.com/df-mc/dragonfly/server/world"
)

// testItem is a custom item whose texture is an image of the width passed.
type testItem struct {
	identifier string
	width      int
}

func (i testItem) EncodeItem() (string, int16) { return i.identifier, 0 }
func (i testItem) Name() string                { return i.identifier }
func (testItem) Category() category.Category   { return category.Items() }
func (i testItem) Texture() image.Image        { return image.NewRGBA(image.Rect(0, 0, i.width, 16)) }

// Item textures are named by the whole identifier, as geometry files are: items of two
// namespaces may share a name, and each keeps its own texture.
func TestBuildItems_TexturesNamedByIdentifier(t *testing.T) {
	dir := t.TempDir()
	buildItemsOf(dir, []world.CustomItem{testItem{"first:cell", 16}, testItem{"second:cell", 32}})
	for path, width := range map[string]int{
		"textures/items/first/cell.png":  16,
		"textures/items/second/cell.png": 32,
	} {
		f, err := os.Open(filepath.Join(dir, path))
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		config, err := png.DecodeConfig(f)
		f.Close()
		if err != nil || config.Width != width {
			t.Errorf("%s is %d wide (%v), want %d", path, config.Width, err, width)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "textures/item_texture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var atlas struct {
		TextureData map[string]struct {
			Textures string `json:"textures"`
		} `json:"texture_data"`
	}
	if err := json.Unmarshal(raw, &atlas); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"first:cell":  "textures/items/first/cell.png",
		"second:cell": "textures/items/second/cell.png",
	} {
		if got := atlas.TextureData[id].Textures; got != want {
			t.Errorf("%s's texture is %q, want %q", id, got, want)
		}
	}
}
