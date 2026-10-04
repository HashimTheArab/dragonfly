package packbuilder

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/customblock"
	"github.com/df-mc/dragonfly/server/world"
)

// testBlock is a buildable custom block with one geometry and texture.
type testBlock struct {
	identifier string
	geometry   []byte
	textures   []string
}

func (b testBlock) EncodeBlock() (string, map[string]any) { return b.identifier, nil }
func (testBlock) Model() world.BlockModel                 { return nil }
func (testBlock) Hash() (uint64, uint64)                  { return 0, 0 }
func (testBlock) Properties() customblock.Properties      { return customblock.Properties{Cube: true} }
func (b testBlock) Name() string                          { return b.identifier }
func (b testBlock) Geometry() []byte                      { return b.geometry }

func (b testBlock) Textures() map[string]image.Image {
	textures := make(map[string]image.Image, len(b.textures))
	for _, name := range b.textures {
		textures[name] = image.NewRGBA(image.Rect(0, 0, 16, 32))
	}
	return textures
}

// animatedBlock is a testBlock whose textures carry flipbooks.
type animatedBlock struct {
	testBlock
	flipbooks map[string]customblock.Flipbook
}

func (b animatedBlock) Flipbooks() map[string]customblock.Flipbook { return b.flipbooks }

func buildTestBlocks(t *testing.T, blocks ...world.Block) string {
	t.Helper()
	reg := world.NewBlockRegistry()
	for _, b := range blocks {
		reg.RegisterBlock(b)
	}
	dir := t.TempDir()
	buildBlocks(reg, dir)
	return dir
}

// Two Experiences may each declare a block with the same short name, so geometry files are
// named by the whole identifier rather than overwriting one another.
func TestBuildBlocks_GeometryNamedByIdentifier(t *testing.T) {
	dir := buildTestBlocks(t,
		testBlock{identifier: "first:drive", geometry: []byte(`{"first":true}`)},
		testBlock{identifier: "second:drive", geometry: []byte(`{"second":true}`)},
	)
	for path, want := range map[string]string{
		"models/blocks/first/drive.geo.json":  `{"first":true}`,
		"models/blocks/second/drive.geo.json": `{"second":true}`,
	} {
		got, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%s = %s, want %s", path, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "models/blocks/drive.geo.json")); !os.IsNotExist(err) {
		t.Errorf("short-named geometry file exists (err = %v)", err)
	}
}

// Flipbook entries follow the vanilla resource pack's textures/flipbook_textures.json: the
// texture path, its terrain atlas key and the animation fields the flipbook sets.
func TestBuildBlocks_FlipbookTextures(t *testing.T) {
	dir := buildTestBlocks(t,
		animatedBlock{
			testBlock: testBlock{identifier: "test:controller", textures: []string{"test.lights", "test.side"}},
			flipbooks: map[string]customblock.Flipbook{
				"test.lights": {TicksPerFrame: 2, Frames: []int{0, 1, 1}, BlendFrames: true},
			},
		},
		animatedBlock{
			testBlock: testBlock{identifier: "test:anim", textures: []string{"test.anim"}},
			flipbooks: map[string]customblock.Flipbook{"test.anim": {}},
		},
	)
	raw, err := os.ReadFile(filepath.Join(dir, "textures/flipbook_textures.json"))
	if err != nil {
		t.Fatalf("read flipbook_textures.json: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode flipbook_textures.json: %v", err)
	}
	want := []map[string]any{
		{
			"flipbook_texture": "textures/blocks/test.anim",
			"atlas_tile":       "test.anim",
			"blend_frames":     false,
		},
		{
			"flipbook_texture": "textures/blocks/test.lights",
			"atlas_tile":       "test.lights",
			"ticks_per_frame":  float64(2),
			"frames":           []any{float64(0), float64(1), float64(1)},
			"blend_frames":     true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("flipbook_textures.json = %v, want %v", got, want)
	}
}

// A pack without animated textures carries no flipbook file, as before.
func TestBuildBlocks_NoFlipbooksWithoutAnimation(t *testing.T) {
	dir := buildTestBlocks(t, testBlock{identifier: "test:still", textures: []string{"test.still"}})
	if _, err := os.Stat(filepath.Join(dir, "textures/flipbook_textures.json")); !os.IsNotExist(err) {
		t.Errorf("flipbook_textures.json exists (err = %v)", err)
	}
}

// A flipbook must animate one of the block's own textures; otherwise it names an atlas key
// the pack never defines.
func TestBuildBlocks_FlipbookForUnknownTexturePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("buildBlocks did not panic")
		}
	}()
	buildTestBlocks(t, animatedBlock{
		testBlock: testBlock{identifier: "test:anim", textures: []string{"test.anim"}},
		flipbooks: map[string]customblock.Flipbook{"test.missing": {}},
	})
}
