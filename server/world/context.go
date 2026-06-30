package world

import (
	"iter"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/df-mc/dragonfly/server/world/redstone"
	"github.com/go-gl/mathgl/mgl64"
)

// Context is the owner-scoped context passed to world callbacks and scheduled
// world work. It proves that the callback is already running on the world's
// owner, so world operations should be performed directly through the context
// instead of scheduling and waiting for another world transaction.
type Context struct {
	*event.Context[*Tx]
}

func newContext(tx *Tx) *Context {
	return &Context{Context: event.C(tx)}
}

// Tx returns the transaction backing the context. The returned transaction is
// only valid for the duration of the callback.
func (ctx *Context) Tx() *Tx { return ctx.Val() }

// Defer schedules f to run after the current owner callback completes.
func (ctx *Context) Defer(f func(ctx *Context)) *Task {
	return ctx.Tx().deferTask(func(ctx *Context) error {
		f(ctx)
		return nil
	})
}

func (ctx *Context) Range() cube.Range                             { return ctx.Tx().Range() }
func (ctx *Context) SetBlock(pos cube.Pos, b Block, opts *SetOpts) { ctx.Tx().SetBlock(pos, b, opts) }
func (ctx *Context) Block(pos cube.Pos) Block                      { return ctx.Tx().Block(pos) }
func (ctx *Context) Liquid(pos cube.Pos) (Liquid, bool)            { return ctx.Tx().Liquid(pos) }
func (ctx *Context) SetLiquid(pos cube.Pos, b Liquid)              { ctx.Tx().SetLiquid(pos, b) }
func (ctx *Context) BuildStructure(pos cube.Pos, s Structure)      { ctx.Tx().BuildStructure(pos, s) }
func (ctx *Context) ScheduleBlockUpdate(pos cube.Pos, b Block, delay time.Duration) {
	ctx.Tx().ScheduleBlockUpdate(pos, b, delay)
}
func (ctx *Context) HighestLightBlocker(x, z int) int       { return ctx.Tx().HighestLightBlocker(x, z) }
func (ctx *Context) HighestBlock(x, z int) int              { return ctx.Tx().HighestBlock(x, z) }
func (ctx *Context) Light(pos cube.Pos) uint8               { return ctx.Tx().Light(pos) }
func (ctx *Context) SkyLight(pos cube.Pos) uint8            { return ctx.Tx().SkyLight(pos) }
func (ctx *Context) SetBiome(pos cube.Pos, b Biome)         { ctx.Tx().SetBiome(pos, b) }
func (ctx *Context) Biome(pos cube.Pos) Biome               { return ctx.Tx().Biome(pos) }
func (ctx *Context) Temperature(pos cube.Pos) float64       { return ctx.Tx().Temperature(pos) }
func (ctx *Context) RainingAt(pos cube.Pos) bool            { return ctx.Tx().RainingAt(pos) }
func (ctx *Context) SnowingAt(pos cube.Pos) bool            { return ctx.Tx().SnowingAt(pos) }
func (ctx *Context) ThunderingAt(pos cube.Pos) bool         { return ctx.Tx().ThunderingAt(pos) }
func (ctx *Context) Raining() bool                          { return ctx.Tx().Raining() }
func (ctx *Context) Thundering() bool                       { return ctx.Tx().Thundering() }
func (ctx *Context) AddParticle(pos mgl64.Vec3, p Particle) { ctx.Tx().AddParticle(pos, p) }
func (ctx *Context) PlayEntityAnimation(e Entity, a EntityAnimation) {
	ctx.Tx().PlayEntityAnimation(e, a)
}
func (ctx *Context) PlaySound(pos mgl64.Vec3, s Sound)   { ctx.Tx().PlaySound(pos, s) }
func (ctx *Context) AddEntity(e *EntityHandle) Entity    { return ctx.Tx().AddEntity(e) }
func (ctx *Context) RemoveEntity(e Entity) *EntityHandle { return ctx.Tx().RemoveEntity(e) }
func (ctx *Context) EntitiesWithin(box cube.BBox) iter.Seq[Entity] {
	return ctx.Tx().EntitiesWithin(box)
}
func (ctx *Context) Entities() iter.Seq[Entity]      { return ctx.Tx().Entities() }
func (ctx *Context) Players() iter.Seq[Entity]       { return ctx.Tx().Players() }
func (ctx *Context) Viewers(pos mgl64.Vec3) []Viewer { return ctx.Tx().Viewers(pos) }
func (ctx *Context) Sleepers() iter.Seq[Sleeper]     { return ctx.Tx().Sleepers() }
func (ctx *Context) BroadcastSleepingIndicator()     { ctx.Tx().BroadcastSleepingIndicator() }
func (ctx *Context) BroadcastSleepingReminder(sleeper Sleeper) {
	ctx.Tx().BroadcastSleepingReminder(sleeper)
}
func (ctx *Context) RedstonePower(pos cube.Pos, face cube.Face, accountForDust bool) int {
	return ctx.Tx().RedstonePower(pos, face, accountForDust)
}
func (ctx *Context) CurrentTick() int64        { return ctx.Tx().CurrentTick() }
func (ctx *Context) Redstone() *redstone.State { return ctx.Tx().Redstone() }
