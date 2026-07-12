package block

import (
	"image/color"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestHangingSignEncodeBlock(t *testing.T) {
	tests := []struct {
		name string
		sign HangingSign
		want map[string]any
	}{
		{
			name: "wall mounted",
			sign: HangingSign{Wood: OakWood(), Attach: WallHangingAttachment(cube.North)},
			want: map[string]any{
				"attached_bit":          uint8(0),
				"facing_direction":      int32(cube.North.Face()),
				"ground_sign_direction": int32(0),
				"hanging":               uint8(0),
			},
		},
		{
			name: "ceiling hanging",
			sign: HangingSign{Wood: SpruceWood(), Attach: CeilingHangingAttachment(cube.East)},
			want: map[string]any{
				"attached_bit":          uint8(0),
				"facing_direction":      int32(cube.East.Face()),
				"ground_sign_direction": int32(0),
				"hanging":               uint8(1),
			},
		},
		{
			name: "attached ceiling",
			sign: HangingSign{Wood: DarkOakWood(), Attach: AttachedCeilingHangingAttachment(7)},
			want: map[string]any{
				"attached_bit":          uint8(1),
				"facing_direction":      int32(0),
				"ground_sign_direction": int32(7),
				"hanging":               uint8(1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, states := tt.sign.EncodeBlock()
			if wantName := "minecraft:" + tt.sign.Wood.String() + "_hanging_sign"; name != wantName {
				t.Fatalf("EncodeBlock name = %q, want %q", name, wantName)
			}
			for key, want := range tt.want {
				if got := states[key]; got != want {
					t.Fatalf("state %s = %#v, want %#v", key, got, want)
				}
			}
		})
	}
}

func TestWallHangingSignFacingMatchesClickedFace(t *testing.T) {
	for _, face := range cube.HorizontalFaces() {
		if got := wallHangingSignFacing(face); got != face.Direction() {
			t.Fatalf("wallHangingSignFacing(%v) = %v, want %v", face, got, face.Direction())
		}
	}
}

func TestWallMountedHangingSignSurvivesWithDirectWallSupport(t *testing.T) {
	w := world.Config{DisableLighting: true}.New()
	defer w.Close()

	var testErr error
	<-w.Do(func(tx *world.Tx) {
		signPos := cube.Pos{1, 1, 0}
		supportPos := signPos.Side(cube.FaceWest)
		sign := HangingSign{Wood: OakWood(), Attach: WallHangingAttachment(cube.East)}
		tx.SetBlock(supportPos, Dirt{}, nil)
		tx.SetBlock(signPos, sign, nil)

		sign.NeighbourUpdateTick(signPos, supportPos, tx)
		if _, ok := tx.Block(signPos).(HangingSign); !ok {
			testErr = errString("wall-mounted hanging sign broke despite direct wall support")
		}
	}).Done()
	if testErr != nil {
		t.Fatal(testErr)
	}
}

func TestHangingSignUseOnBlockPlacesWallSignOnClickedFace(t *testing.T) {
	w := world.Config{DisableLighting: true}.New()
	defer w.Close()

	var testErr error
	<-w.Do(func(tx *world.Tx) {
		supportPos := cube.Pos{0, 1, 0}
		signPos := supportPos.Side(cube.FaceEast)
		tx.SetBlock(supportPos, Dirt{}, nil)

		ctx := &item.UseContext{}
		_ = (HangingSign{Wood: OakWood()}).UseOnBlock(supportPos, cube.FaceEast, mgl64.Vec3{}, tx, nil, ctx)
		sign, ok := tx.Block(signPos).(HangingSign)
		if !ok {
			testErr = errString("hanging sign was not placed on clicked east face")
			return
		}
		if sign.Attach != WallHangingAttachment(cube.East) {
			testErr = errString("wall hanging sign attachment did not face the clicked face")
		}
	}).Done()
	if testErr != nil {
		t.Fatal(testErr)
	}
}

func TestHangingSignNBTUsesHangingSignIDAndSignText(t *testing.T) {
	sign := HangingSign{
		Wood:  OakWood(),
		Front: SignText{Text: "front", BaseColour: color.RGBA{R: 1, G: 2, B: 3, A: 255}, Glowing: true, Owner: "front-owner"},
		Back:  SignText{Text: "back", BaseColour: color.RGBA{R: 4, G: 5, B: 6, A: 255}, Owner: "back-owner"},
		Waxed: true,
	}
	nbt := sign.EncodeNBT()
	if got := nbt["id"]; got != "HangingSign" {
		t.Fatalf("EncodeNBT id = %#v, want HangingSign", got)
	}

	decoded := (HangingSign{}).DecodeNBT(nbt).(HangingSign)
	if decoded.Front.Text != sign.Front.Text || decoded.Back.Text != sign.Back.Text {
		t.Fatalf("decoded text = front %q back %q, want front %q back %q", decoded.Front.Text, decoded.Back.Text, sign.Front.Text, sign.Back.Text)
	}
	if !decoded.Waxed {
		t.Fatalf("decoded Waxed = false, want true")
	}
}

func TestHangingAttachmentHashValuesAreUnique(t *testing.T) {
	seen := map[uint8]HangingAttachment{}
	for _, direction := range cube.Directions() {
		for _, attachment := range []HangingAttachment{
			WallHangingAttachment(direction),
			CeilingHangingAttachment(direction),
		} {
			hash := attachment.Uint8()
			if previous, ok := seen[hash]; ok {
				t.Fatalf("attachment hash %d used by %#v and %#v", hash, previous, attachment)
			}
			seen[hash] = attachment
		}
	}
	for o := cube.Orientation(0); o <= 15; o++ {
		attachment := AttachedCeilingHangingAttachment(o)
		hash := attachment.Uint8()
		if previous, ok := seen[hash]; ok {
			t.Fatalf("attachment hash %d used by %#v and %#v", hash, previous, attachment)
		}
		seen[hash] = attachment
	}
}
