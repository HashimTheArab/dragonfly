# End Portals — Design / Implementation Plan

Date: 2026-05-09
Status: Draft (pre-implementation)
Branch base: `pr-1122` (uses the new `tx.BlocksWithin` index added in this branch — see "Scan/index" below)

## Goal

Add functional Overworld ↔ End portal travel to dragonfly, mirroring the
recently-merged Nether portal feature where it makes sense and diverging where
the End demands it.

## Non-goals

- Stronghold generation, dragon, dragon egg, end crystals.
- `end_gateway` block and outer-island travel.
- Eye-of-ender thrown projectile (the locator entity).
- Custom End generator changes (end city, choruses, outer islands).
- Return-portal "find or create matching portal in Overworld". Returning
  always lands at the Overworld world spawn.

## Scope (locked from brainstorm)

- New blocks: `EndPortal`, `EndPortalFrame`.
- New item: `EnderEye` (used only on a frame; no projectile, no stronghold-finder).
- Activation: per-eye placement runs a deterministic 12-frame ring check; the
  12th eye placement spawns 9 `end_portal` blocks in the interior.
- Travel: instantaneous, no 4-second wait, in both directions.
- First-entry spawn (Overworld → End): regenerate a 5×5 obsidian platform at
  (100, 48, 0), clear 5×5×4 air above, place the entity at (100, 49, 0). On
  every entry. This wipes anything already there — vanilla parity.
- Return spawn (End → Overworld): place the entity at the Overworld's
  `World.Spawn()`.
- No block-entity NBT for `end_portal` / `end_portal_frame`. Block states only.
- Frames are unbreakable in survival; portal blocks are unbreakable in survival.
  Both are creative-only break (matches vanilla).
- Once activated, `end_portal` blocks persist even if a frame is removed —
  no neighbour-driven deactivation. (Vanilla parity.)

## Scan / index decision

**No new scan or POI/index facility is needed for this feature.**

The branch already exposes `tx.BlocksWithin(pos, radius, blocks...)` (in
`server/world/block_search.go` and `server/world/tx.go`), used by
`portal.FindNetherPortal`. The End feature does not need it because:

- Activation is purely local — the placed eye gives us the starting frame; the
  `cardinal_direction` state tells us which side of the ring it sits on; from
  that we deterministically compute the 11 other frame positions and the 9
  interior positions in O(1). No flood-fill, no radius scan.
- Destination spawning is at fixed coordinates ((100, 49, 0) or
  `World.Spawn()`). No "find existing portal at translated coords" lookup.
- Frames are unbreakable, so we never run a deactivation flood-fill.

If stronghold-generated portals or a "find nearest end portal" feature is
added later, `tx.BlocksWithin(pos, r, endPortal())` will be the right tool —
it just isn't needed now.

## Architecture

Symmetric with nether ("Approach A"). Side-by-side packages, separate algorithms.

```
server/
  block/
    end_portal.go              (new)
    end_portal_frame.go        (new)
  item/
    ender_eye.go               (new)
  world/
    portal/
      end.go                   (new)
      end_test.go              (new)
  entity/
    travel.go                  (modified: target-aware Instantaneous + per-target spawn)
    travel_test.go             (modified: cover End travel)
  player/
    conf.go                    (modified: Instantaneous closure honors target dim)
```

### Data flow — Activation (Overworld)

1. Player right-clicks a frame with an `EnderEye`.
2. `EnderEye.UseOnBlock` checks the target is an `EndPortalFrame` with `Eye == false`.
3. Sets `Eye = true` on that frame, plays `BlockEndPortalFrameFill` sound,
   consumes one item.
4. Calls `portal.ActivateEndPortal(tx, framePos)`.
5. `ActivateEndPortal`:
   - Reads the frame's `Facing` state.
   - Computes the ring center: `framePos + 2 * Facing.Face().Direction()`
     (a frame faces inward toward the center; walking 2 blocks in that
     direction lands on the center).
   - Validates the 12 ring positions: each must be `EndPortalFrame` with
     `Eye == true` and `Facing` pointing toward the center.
   - If all 12 valid: places `EndPortal{}` at the 9 interior positions and
     plays `BlockEndPortalSpawn` sound. Returns `true`.
   - If any frame is missing an eye or mis-facing: returns `false`. No-op.

### Data flow — Travel (Overworld → End)

1. Entity steps onto an `EndPortal` block.
2. `EndPortal.EntityInside` calls `t.TravelThroughPortal(tx, world.End)`.
3. `PortalTravelComputer.EnterPortal`:
   - Looks up destination via `tx.World().PortalDestination(world.End)`.
   - Source dim = Overworld, target dim = End. `Instantaneous(source, target)`
     returns `true` for any End-involved travel → bypass the 4-second timer,
     start travel immediately (queued for end-of-tick like nether).
4. Cross-tx travel:
   - Remove entity from source tx.
   - Exec on destination world:
     - Call new `portal.GenerateEndSpawnPlatform(tx)`:
       - For each (x,z) in [-2,2]×[-2,2] around (100, 48, 0):
         - Set y=48 to obsidian.
         - Set y=49..52 to air (clears anything sitting there).
     - Spawn the entity at (100.5, 49, 0.5).
5. `finishTravel` runs `HandlePortalTravel` hook and teleports.

### Data flow — Travel (End → Overworld)

1. Entity steps onto an `EndPortal` block in the End.
2. `EndPortal.EntityInside` calls `t.TravelThroughPortal(tx, world.Overworld)`.
   The portal block returns its **target** dimension based on the dimension
   it currently sits in (see `EndPortal.Portal()` below). End-side end_portal
   targets Overworld.
3. `PortalTravelComputer.EnterPortal`: instant (source == End). Travel begins.
4. Cross-tx travel:
   - Remove entity.
   - Exec on Overworld:
     - Spawn the entity at `tx.World().Spawn().Vec3Middle()`.
5. Finish.

## Components

### `block.EndPortal`

```go
type EndPortal struct {
    transparent
    // No state. Fixed light 15. Indestructible in survival.
}
```

- `Model() world.BlockModel` → `model.Solid` (full block collision; entities
  collide unless they're inside, in which case `EntityInside` fires).
  Vanilla actually uses a slim collision (12/16 height) but full block is
  acceptable; revisit if it visibly breaks placement.
- `Portal() world.Dimension` → returns the **target** dimension based on the
  block's current world. We don't have access to the world from the block
  struct, so the cleaner shape is: have `Portal()` return `world.End` always
  (the dimension this portal *leads to* on the Overworld side), and let
  `PortalTravelComputer` derive the actual destination via
  `tx.World().PortalDestination(target)` — which already returns the Overworld
  when called from the End. The existing nether code does the same.
  → **Action**: confirm `World.PortalDestination(End)` returns Overworld when
  the source is the End. Reading `world.go:1035-1043` it does (it returns
  `conf.PortalDestination(End)` which the server wires to the End world; if
  source *is* End, the conf's switch returns the *configured* End world, which
  equals the source — and `enterPortal` short-circuits on `destination ==
  source`). This is a bug for our case: End→Overworld would be a no-op.
  → **Fix**: in `server/server.go:createWorld`, when the source dim is End,
  `PortalDestination(world.End)` should return the **Overworld** (since the
  end_portal block in the End is the return path). Two equivalent options:
  - (a) Have `EndPortal.Portal()` return Overworld when called from the End,
    End when called from Overworld. Block can't see its world directly, but
    `EntityInside` has the tx — the block dispatches the target there, not
    via `Portal()`.
  - (b) Override the End world's `PortalDestination` so `End → End` returns
    the Overworld instead.
  → **Decision**: (a). Move target selection into `EntityInside`:
    ```go
    func (EndPortal) EntityInside(_ cube.Pos, tx *world.Tx, e world.Entity) {
        target := world.End
        if tx.World().Dimension() == world.End {
            target = world.Overworld
        }
        if t, ok := e.(portalTraveller); ok {
            t.TravelThroughPortal(tx, target)
        }
    }
    ```
  This avoids a server.go config change and keeps the asymmetry localized.
  `Portal()` (the interface method used by `Ent.checkPortalInsiders`) keeps
  returning `world.End` for cache/POI semantics.
- `LightEmissionLevel() uint8` → 15.
- `EncodeBlock` → `("minecraft:end_portal", nil)`.
- `BreakInfo()` → unbreakable in survival (creative-only). Mirror bedrock's
  hardness=-1 / resistance=18000000. Reuse the same pattern as Bedrock-style
  unbreakable blocks; if dragonfly currently uses a sentinel hardness for that
  purpose, follow it.
- `HasLiquidDrops()` → false.
- No `Frame(dim)` interface — End frames don't reuse the obsidian-frame path.

### `block.EndPortalFrame`

```go
type EndPortalFrame struct {
    solid
    // Eye reports whether an eye of ender is inserted.
    Eye bool
    // Facing is the cardinal direction the frame faces (the eye sits on
    // the side opposite this).
    Facing cube.Direction
}
```

- `Model()` → custom box model: 0–13/16 height when `!Eye`, 0–16/16 when
  `Eye`. Reuse existing `model.Box` or add a thin variant. Inspect
  `block/end_rod.go` and similar for the existing per-block model pattern.
- `EncodeBlock` → `("minecraft:end_portal_frame", {"end_portal_eye_bit": Eye,
  "minecraft:cardinal_direction": Facing.String()})`.
- `EncodeItem` → only obtainable in creative; the item form has no states.
- `BreakInfo` → unbreakable in survival.
- `LightEmissionLevel` → 0 (Bedrock parity; Java emits 1 but Bedrock does not).
- `Activate(...)` (use the existing `block.Activatable` pattern):
  - If `Eye == true`, no-op.
  - If held item is not `item.EnderEye`, no-op.
  - Set `Eye = true`, set the block, play sound, consume one item, call
    `portal.ActivateEndPortal(tx, pos)`.
- Placement: faces opposite the placer, `cardinal_direction = placer.Facing().Opposite()`.

### `item.EnderEye`

```go
type EnderEye struct{}
```

- `EncodeItem` → `("minecraft:ender_eye", 0)`.
- `MaxCount` → 64.
- `UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, w *world.Tx,
  user item.User, ctx *item.UseContext) bool`:
  - If the block at `pos` is `EndPortalFrame{Eye: false}`, delegate to its
    `Activate(...)` and return true (consume one).
  - Else return false (no-op; we don't implement the thrown locator).

Item registration goes in the same place vanilla items are registered. (Find
the registry init list — I'll locate it during impl.)

### `portal/end.go`

Public surface mirrors `nether.go` *only where it adds value*:

```go
// End represents an end portal — purely a result of the activation scan;
// not retained anywhere.
type End struct {
    tx        *world.Tx
    framePos  cube.Pos       // any frame on the ring (the one that triggered the scan)
    center    cube.Pos       // 3x3 interior center
    frames    []cube.Pos     // 12 frame positions
    interior  []cube.Pos     // 9 interior positions
    completed bool           // true if all 12 frames have eyes and face inward
}

// ActivateEndPortal runs the ring scan for the frame at pos. If the ring is
// complete (12 valid frames with eyes), places end_portal blocks in the
// interior and returns true. Idempotent — does nothing if any interior
// position already holds an end_portal block.
func ActivateEndPortal(tx *world.Tx, framePos cube.Pos) bool

// EndPortalFromPos performs the scan without mutating the world. Useful for
// tests and for asserting the ring shape.
func EndPortalFromPos(tx *world.Tx, framePos cube.Pos) (End, bool)

// GenerateEndSpawnPlatform builds the 5x5 obsidian platform at
// (100, 48, 0) and clears 5x5x4 above it. Idempotent except for the air
// clear, which always overwrites. Used by the travel computer when
// arriving in the End.
func GenerateEndSpawnPlatform(tx *world.Tx)

// EndSpawnPosition returns the entity arrival position in the End:
// (100.5, 49, 0.5).
func EndSpawnPosition() mgl64.Vec3
```

No `Find*`, no `Create*`, no `Deactivate*`, no `Framed`/`Activated` accessors —
unlike `Nether`. The End ring is either complete or not; partially-built rings
don't carry persistent state.

### Ring scan algorithm (in `end.go`)

Given the placed frame at `framePos` with `Facing`:

1. The ring center is at `framePos + 2 * facingOffset(Facing)`. Note: each
   frame faces *toward* the center; the eye sits on the inward face. So
   moving 2 blocks in `Facing` direction from any ring frame lands at the
   center. Caveat: ambiguous for corner frames? **Resolved**: corners are
   not part of the ring. The ring has 12 frames in 4 sets of 3 along the
   four cardinal sides; corners (`±2, ±2` from center) are *not* frames.
   So every frame has a uniquely-defined inward direction.
2. Compute the 12 expected frame positions: for each of N/E/S/W, three
   positions along that side at offsets `[-1, 0, 1]` along the side's
   tangent. Each side is 2 blocks from the center in the side's direction.
   For example:
   - N side: frames at `center + (-1, 0, -2)`, `center + (0, 0, -2)`,
     `center + (1, 0, -2)`. Each must have `Facing = South` (toward center).
   - Same pattern rotated for E, S, W.
3. Compute the 9 interior positions:
   `center + (dx, 0, dz)` for `dx, dz in [-1, 0, 1]`.
4. Validate: all 12 frame positions are `EndPortalFrame{Eye: true,
   Facing: <expected>}`. If any fails, return `(End{...completed:false}, false)`.
5. Otherwise return `(End{...completed:true}, true)`.

`ActivateEndPortal` calls the scan; if `completed`, sets each interior to
`EndPortal{}` (skipping positions already holding `EndPortal{}` so it stays
idempotent if the player breaks-and-replaces an eye in creative).

### `entity/travel.go` changes

The current shape is:

```go
Instantaneous func() bool
```

Change to:

```go
Instantaneous func(source, target world.Dimension) bool
```

Update call sites:

- `enterPortal` already knows `source := tx.World()` and `target` parameter.
  Pass both.
- `NewPortalTravelComputer` default returns `func(_, _ world.Dimension) bool {
  return true }` (matches existing behavior).
- `player/conf.go`:
  ```go
  Instantaneous: func(source, target world.Dimension) bool {
      if pdata.gameMode == world.GameModeCreative {
          return true
      }
      return source == world.End || target == world.End
  }
  ```

Add a destination-aware spawn pre-step. Rather than overload the computer
with switch-on-dim logic, add an optional hook:

```go
// SpawnFor returns the arrival position for an entity travelling from source
// to target. If nil, the default behavior is used (find/create nether portal
// for Nether targets, identity translate otherwise).
SpawnFor func(tx *world.Tx, source, target world.Dimension, fallback mgl64.Vec3) mgl64.Vec3
```

Default behavior (current code) stays in `travel`/`travelQueued`. For End
targets we don't go through `SpawnFor` — instead, both `travel` and
`travelQueued` get a small switch on `destinationDim`:

```go
switch destinationDim {
case world.Nether:
    if netherPortal, ok := portal.FindOrCreateNetherPortal(tx, pos, 128); ok {
        spawn = netherPortal.Spawn().Vec3Middle()
    }
case world.End:
    portal.GenerateEndSpawnPlatform(tx)
    spawn = portal.EndSpawnPosition()
case world.Overworld:
    if sourceDim == world.End {
        spawn = tx.World().Spawn().Vec3Middle()
    }
    // else: identity translate (already done above)
}
```

`SpawnFor` is overkill — drop it, keep the switch. Travel computer stays
focused on timing; per-dimension spawn details are inline because they're
short and dimension-coupled by nature.

## Block / item registration

- Register `EndPortal{}` and all `EndPortalFrame` state combinations in the
  same place obsidian/portal are registered today (find via
  `grep -rn "RegisterBlock(Obsidian{})" server/` during impl).
- Register `EnderEye{}` in the item init list (find via existing item
  registrations, e.g. `world.RegisterItem(item.Apple{})` patterns).

## Edge cases

| Case | Behavior |
|---|---|
| Player breaks frame after activation (creative) | `end_portal` blocks remain. No deactivation flood-fill. |
| Player places eye into a frame that's part of an *invalid* ring (wrong-facing neighbours, missing one frame) | Eye is consumed, frame turns to `Eye: true`, **portal does not activate**. Vanilla parity — eyes don't validate the ring. |
| Player places eye on the 12th frame, but one of the other 11 was broken since being filled | Activation scan fails → no portal blocks. Vanilla edge-case. |
| Two adjacent rings share frames | Impossible — rings are 5×5 footprint with frames on the perimeter; two rings can't share frame positions without overlap. |
| Entity carrying a passenger | Bedrock skips the travel for passengered entities. Match — early-return in `EntityInside` if the entity has riders. (Need to look up the rider check API.) |
| Travel during the same tick from two players touching the same portal | Existing nether travel computer handles this with `awaitingTravel`/`travelling` flags; same code path applies. |
| Spawn-platform generation overwrites a player's nearby builds | Vanilla parity — accept it. |
| End-side end_portal in a world without an Overworld destination configured | `PortalDestination(Overworld)` returns the source world; `enterPortal` short-circuits on `destination == source` and the entity stays put. Same as nether. |
| Item.EnderEye used on something that's not a frame | Returns false from `UseOnBlock`, no consumption. |
| Frame with `Eye` already true, used with another eye | `Activate` short-circuits; no consumption. |

## Testing

`server/world/portal/end_test.go`:

- `TestRingScanComplete` — build a 12-frame ring (3 frames on each side
  facing inward, all `Eye: true`), call `ActivateEndPortal`, assert 9
  `EndPortal` blocks placed.
- `TestRingScanMissingEye` — same but one frame has `Eye: false`, assert no
  blocks placed.
- `TestRingScanWrongFacing` — one frame faces outward, assert no blocks.
- `TestRingScanCornerNotFrame` — corner positions hold `EndPortalFrame`
  blocks too. Assert scan still succeeds (corners aren't part of the 12, so
  whatever sits there is irrelevant).
- `TestActivateIdempotent` — call `ActivateEndPortal` twice, assert second
  call doesn't re-set already-`EndPortal` interior blocks.
- `TestSpawnPlatformGenerated` — call `GenerateEndSpawnPlatform`, assert
  the 5×5 obsidian + 5×5×4 air clearance.

`server/entity/travel_test.go` (extending existing table):

- Overworld → End instant travel: no 4-second wait, entity arrives at
  (100.5, 49, 0.5), platform present.
- End → Overworld instant travel: entity arrives at world spawn.
- Survival player from non-creative: still instant for End travel.

`server/block/` tests follow existing patterns (state encoding round-trip).

## Implementation phases

1. **Blocks (no behavior)** — register `EndPortal` and `EndPortalFrame` with
   correct states, encoding, light, hardness. Confirm `block_states.nbt`
   round-trips. No travel, no activation. Compile + state-tests green.
2. **Item + activation** — add `EnderEye`, `EndPortalFrame.Activate`,
   `portal.ActivateEndPortal`, ring-scan algorithm + tests.
3. **Travel mechanics** — widen `PortalTravelComputer.Instantaneous`
   signature, update player conf, add End-spawn switch in `travel` /
   `travelQueued`, add `EndPortal.EntityInside`, add
   `portal.GenerateEndSpawnPlatform` + `EndSpawnPosition`. Travel tests.
4. **End-side return** — confirm asymmetric `EntityInside` target dispatch
   works (Overworld→End vs End→Overworld). Add the End→Overworld test.
5. **Polish** — collision-box height for frame block, sounds (frame fill,
   portal spawn), `Solidifies`/`Replaceable` audit, comparator output (15
   when eye), waterlogging on frame.

## Open questions to resolve during impl

- Exact unbreakable-block hardness pattern in dragonfly (search for an
  existing `bedrock.go`-style hardness=-1 block). Bedrock is the only
  existing parallel.
- Sound IDs for `BlockEndPortalFrameFill` and `BlockEndPortalSpawn` —
  verify they exist in `world/sound`.
- Whether `EndPortalFrame` should implement `world.NBTer` for safety even
  though it has no NBT — probably not, but confirm against the registration
  pipeline's expectations.
- Comparator output (15 when eye, 0 otherwise) — defer if there's no
  comparator framework yet; not blocking for travel correctness.
