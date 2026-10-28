package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// PlacementSource exposes the world state needed to predict a block's
// UseOnBlock behaviour.
type PlacementSource interface {
	world.LiquidSource
	BlockLoaded(cube.Pos) (world.Block, bool)
	Range() cube.Range
	Dimension() world.Dimension
}

// PlacementPose is the player state read by orientation-dependent placement rules.
// PredictPlacement accepts a nil pose when the user is unknown.
type PlacementPose struct {
	Position  mgl64.Vec3
	Rotation  cube.Rotation
	EyeHeight float64
}

// PredictedBlockChange is a block write produced by placement behaviour.
type PredictedBlockChange struct {
	Pos   cube.Pos
	Block world.Block
	// Layer is the Bedrock block layer written by the placement. Primary
	// blocks use layer zero; displaced or cleared liquids use layer one.
	Layer uint8
}

// PredictedBlockAt returns the last block written to pos on layer in the ordered
// changes. The bool reports whether a matching write exists, even if its block
// is nil or Air. If no write matches, it returns nil, false. It reads changes
// without modifying them or consulting world state.
func PredictedBlockAt(changes []PredictedBlockChange, pos cube.Pos, layer uint8) (world.Block, bool) {
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		if change.Pos == pos && change.Layer == layer {
			return change.Block, true
		}
	}
	return nil, false
}

// PredictPlacement runs placed's actual placement behaviour against src and
// returns its block writes in order. The bool reports whether every world and
// player-state query made by the placement rule was known. Running the
// canonical implementation keeps prediction in step with placement changes
// made in this package.
//
// It mirrors the two branches (*player.Player).UseItemOnBlock dispatches on: a
// block implementing item.UsableOnBlock has its own UseOnBlock run, and any
// other block is placed at the first replaceable position at or beside the
// click. Most full blocks, such as Cobblestone and Dirt, take the second
// branch.
//
// pose may be nil only when the placed block does not consult player position,
// rotation or eye height; a placement rule that does makes the result unknown.
// Non-block side effects such as sounds, particles and scheduled updates are
// intentionally discarded.
func PredictPlacement(src PlacementSource, pose *PlacementPose, clickedPos cube.Pos, face cube.Face, clickPos mgl64.Vec3, placed world.Block) (changes []PredictedBlockChange, known bool) {
	return predictPlacementWithLimits(src, pose, clickedPos, face, clickPos, placed, placementLimits{})
}

// placementLimits are the canonical Player.PlaceBlock admission checks applied
// when planning an executable placement. Ordinary prediction leaves them unset.
type placementLimits struct {
	// reach is the largest eye distance to each placed cell's centre. Zero
	// disables the check.
	reach  float64
	actors []world.PlacementActor
}

// predictPlacementWithLimits predicts placement and refuses the whole placement
// when any canonical PlaceBlock call fails limits.
func predictPlacementWithLimits(src PlacementSource, pose *PlacementPose, clickedPos cube.Pos, face cube.Face, clickPos mgl64.Vec3, placed world.Block, limits placementLimits) (changes []PredictedBlockChange, known bool) {
	if src == nil || placed == nil {
		return nil, false
	}

	view := &placementView{
		src:    src,
		known:  true,
		limits: limits,
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, ok := recovered.(placementUserUnsupported); !ok {
				panic(recovered)
			}
			changes, known = view.changes, false
		}
	}()
	complete := world.RunBlockTransaction(view, func(tx *world.Tx) {
		u, ctx := &placementUser{pose: pose, tx: tx, view: view}, &item.UseContext{}
		if usable, ok := placed.(item.UsableOnBlock); ok {
			usable.UseOnBlock(clickedPos, face, clickPos, tx, u, ctx)
			return
		}
		pos, _, ok := firstReplaceable(tx, clickedPos, face, placed)
		if !ok {
			return
		}
		place(tx, pos, placed, u, ctx)
	})
	if view.refused {
		// A plan must not publish only part of a multi-block placement when a
		// later canonical PlaceBlock call is refused.
		return nil, complete && view.known
	}
	return view.changes, complete && view.known
}

type placementView struct {
	src     PlacementSource
	changes []PredictedBlockChange
	known   bool
	limits  placementLimits
	refused bool
}

func (v *placementView) Range() cube.Range          { return v.src.Range() }
func (v *placementView) Dimension() world.Dimension { return v.src.Dimension() }

func (v *placementView) Block(pos cube.Pos) world.Block {
	if b, ok := PredictedBlockAt(v.changes, pos, 0); ok {
		return b
	}
	b, ok := v.src.BlockLoaded(pos)
	if !ok {
		v.known = false
		return Air{}
	}
	return b
}

func (v *placementView) BlockLoaded(pos cube.Pos) (world.Block, bool) {
	if b, ok := PredictedBlockAt(v.changes, pos, 0); ok {
		return b, true
	}
	b, ok := v.src.BlockLoaded(pos)
	if !ok {
		v.known = false
	}
	return b, ok
}

func (v *placementView) Liquid(pos cube.Pos) (world.Liquid, bool) {
	if liquid, ok := v.Block(pos).(world.Liquid); ok {
		return liquid, true
	}
	if b, ok := PredictedBlockAt(v.changes, pos, 1); ok {
		liquid, liquidOK := b.(world.Liquid)
		return liquid, liquidOK
	}
	primary, ok := v.src.BlockLoaded(pos)
	if !ok {
		v.known = false
		return nil, false
	}
	// PlacementSource.Liquid returns either layer. If the source primary block
	// is itself liquid, it was already handled above; consulting the source
	// after a staged primary replacement would otherwise resurrect that stale
	// layer-zero liquid.
	if _, primaryLiquid := primary.(world.Liquid); primaryLiquid {
		return nil, false
	}
	return v.src.Liquid(pos)
}

func (v *placementView) SetBlock(pos cube.Pos, b world.Block, opts *world.SetOpts) {
	if pos.OutOfBounds(v.Range()) {
		return
	}
	if b == nil {
		b = Air{}
	}
	if opts != nil && opts.DisableLiquidDisplacement {
		v.setPrimaryBlock(pos, b)
		return
	}
	oldPrimary := v.Block(pos)
	oldSecondary, hasSecondary := v.secondaryLiquid(pos, oldPrimary)

	if _, air := b.(Air); air && hasSecondary {
		// Real world transactions move a displaced liquid back to the primary
		// layer when the containing block is removed.
		v.setSecondaryLiquid(pos, nil)
		v.setPrimaryBlock(pos, oldSecondary)
		return
	}
	oldPrimaryLiquid, primaryWasLiquid := oldPrimary.(world.Liquid)
	if displacer, ok := b.(world.LiquidDisplacer); ok {
		if primaryWasLiquid && displacer.CanDisplace(oldPrimaryLiquid) {
			v.setSecondaryLiquid(pos, oldPrimaryLiquid)
		}
	} else {
		if hasSecondary {
			v.setSecondaryLiquid(pos, nil)
		}
	}
	v.setPrimaryBlock(pos, b)
}

func (v *placementView) SetLiquid(pos cube.Pos, liquid world.Liquid) {
	if pos.OutOfBounds(v.Range()) {
		return
	}
	primary := v.Block(pos)
	_, hasSecondary := v.secondaryLiquid(pos, primary)
	if liquid == nil {
		if _, primaryLiquid := primary.(world.Liquid); primaryLiquid {
			v.setPrimaryBlock(pos, Air{})
		}
		if hasSecondary {
			v.setSecondaryLiquid(pos, nil)
		}
		return
	}

	replaceable, canReplace := primary.(Replaceable)
	displacer, canDisplace := primary.(world.LiquidDisplacer)
	if (!canReplace || !replaceable.ReplaceableBy(liquid)) && (!canDisplace || !displacer.CanDisplace(liquid)) {
		return
	}
	_, primaryIsAir := primary.(Air)
	_, primaryIsLiquid := primary.(world.Liquid)
	if primaryIsAir || primaryIsLiquid {
		if hasSecondary {
			v.setSecondaryLiquid(pos, nil)
		}
		v.setPrimaryBlock(pos, liquid)
		return
	}
	v.setSecondaryLiquid(pos, liquid)
}

func (v *placementView) secondaryLiquid(pos cube.Pos, primary world.Block) (world.Liquid, bool) {
	if b, ok := PredictedBlockAt(v.changes, pos, 1); ok {
		liquid, liquidOK := b.(world.Liquid)
		return liquid, liquidOK
	}
	if _, primaryLiquid := primary.(world.Liquid); primaryLiquid {
		return nil, false
	}
	sourcePrimary, ok := v.src.BlockLoaded(pos)
	if !ok {
		v.known = false
		return nil, false
	}
	if _, primaryLiquid := sourcePrimary.(world.Liquid); primaryLiquid {
		return nil, false
	}
	liquid, ok := v.src.Liquid(pos)
	return liquid, ok
}

func (v *placementView) setPrimaryBlock(pos cube.Pos, b world.Block) {
	b, _ = NormaliseLayerBlock(b, 0)
	v.changes = append(v.changes, PredictedBlockChange{Pos: pos, Block: b})
}

func (v *placementView) setSecondaryLiquid(pos cube.Pos, liquid world.Liquid) {
	b, _ := NormaliseLayerBlock(liquid, 1)
	v.changes = append(v.changes, PredictedBlockChange{Pos: pos, Block: b, Layer: 1})
}

type placementUser struct {
	pose *PlacementPose
	tx   *world.Tx
	view *placementView
}

type placementUserUnsupported struct{}

func (u *placementUser) H() *world.EntityHandle {
	u.unsupported()
	return nil
}

func (u *placementUser) Close() error {
	u.unsupported()
	return nil
}

func (u *placementUser) Position() mgl64.Vec3 {
	if u.pose == nil {
		u.view.known = false
		return mgl64.Vec3{}
	}
	return u.pose.Position
}

func (u *placementUser) Rotation() cube.Rotation {
	if u.pose == nil {
		u.view.known = false
		return cube.Rotation{}
	}
	return u.pose.Rotation
}

// EyeHeight supplies the eye offset used by vertical placement rules.
func (u *placementUser) EyeHeight() float64 {
	if u.pose == nil {
		u.view.known = false
		return 0
	}
	return u.pose.EyeHeight
}

func (u *placementUser) HeldItems() (item.Stack, item.Stack) {
	u.unsupported()
	return item.Stack{}, item.Stack{}
}

func (u *placementUser) SetHeldItems(item.Stack, item.Stack) { u.unsupported() }
func (u *placementUser) UsingItem() bool {
	u.unsupported()
	return false
}
func (u *placementUser) ReleaseItem() { u.unsupported() }
func (u *placementUser) UseItem()     { u.unsupported() }

func (u *placementUser) PlaceBlock(pos cube.Pos, b world.Block, ctx *item.UseContext) {
	limits := u.view.limits
	if limits.reach > 0 && !placementWithinReach(u.Position().Add(mgl64.Vec3{0, u.EyeHeight()}), pos.Vec3Centre(), limits.reach) {
		u.view.refused = true
		return
	}
	if len(limits.actors) > 0 && (ctx == nil || !ctx.IgnoreBBox) {
		model := world.NewPlacementModel(u.tx, pos, b)
		for _, actor := range limits.actors {
			if model.Obstructed(actor) {
				u.view.refused = true
				return
			}
		}
	}
	u.tx.SetBlock(pos, b, nil)
	if ctx != nil {
		ctx.CountSub++
	}
}

func (u *placementUser) unsupported() {
	u.view.known = false
	panic(placementUserUnsupported{})
}
