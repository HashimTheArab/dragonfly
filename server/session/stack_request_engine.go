package session

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/session/stackrequest"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// stackRequestContainer adapts an inventory to stackrequest.Container. It is a
// comparable value so that the same inventory yields an equal handle in every
// request, which is what the engine keys its prediction history on.
type stackRequestContainer struct {
	inv     *inventory.Inventory
	offhand bool
}

func (c stackRequestContainer) Item(slot int) (item.Stack, bool) {
	slot, ok := c.storageSlot(slot)
	if !ok {
		return item.Stack{}, false
	}
	stack, err := c.inv.Item(slot)
	return stack, err == nil
}

func (c stackRequestContainer) StackNetworkID(slot int) (int32, bool) {
	stack, ok := c.Item(slot)
	if !ok {
		return 0, false
	}
	return item_id(stack), true
}

func (c stackRequestContainer) Size() int {
	if c.offhand {
		return 2
	}
	return c.inv.Size()
}

func (c stackRequestContainer) Accepts(slot int, stack item.Stack) bool {
	slot, ok := c.storageSlot(slot)
	return ok && c.inv.ValidItem(slot, stack)
}

// storageSlot maps a wire slot to its storage slot. The offhand occupies wire
// slot 1 of a two-slot container but storage slot 0 of a one-slot inventory.
func (c stackRequestContainer) storageSlot(slot int) (int, bool) {
	if c.offhand {
		return 0, slot == 1
	}
	return slot, true
}

func (c stackRequestContainer) setItem(slot int, stack item.Stack) error {
	slot, ok := c.storageSlot(slot)
	if !ok {
		return inventory.ErrSlotOutOfRange
	}
	return c.inv.SetItem(slot, stack)
}

type stackRequestResolver struct {
	s  *Session
	tx *world.Tx
}

func (r stackRequestResolver) Resolve(name protocol.FullContainerName) (stackrequest.Container, bool) {
	inv, ok := r.s.invByID(int32(name.ContainerID), r.tx)
	if !ok || inv == nil {
		return nil, false
	}
	return stackRequestContainer{inv: inv, offhand: inv == r.s.offHand}, true
}

func (stackRequestResolver) StackNetworkID(stack item.Stack) int32 { return item_id(stack) }

type stackRequestHooks struct {
	s  *Session
	tx *world.Tx
	c  Controllable
	// rewards holds containers whose result slot was taken from, pending the
	// experience payout that commitStackRequestPlan makes.
	rewards []stackRequestContainer
}

// stackRequestEffect is a session-owned side effect that runs only after the
// complete inventory plan has validated and its slot changes have been applied.
type stackRequestEffect func()

// openedWindow reports whether c is the container window the player has open.
func (h *stackRequestHooks) openedWindow(c stackRequestContainer) bool {
	return c.inv == h.s.openedWindow.Load() && h.s.containerOpened.Load()
}

func (h *stackRequestHooks) Take(container stackrequest.Container, slot int, stack item.Stack) error {
	c := container.(stackRequestContainer)
	if err := call(event.C(inventory.Holder(h.c)), slot, stack, c.inv.Handler().HandleTake); err != nil {
		return err
	}
	if !stack.Empty() && h.openedWindow(c) && slot == c.inv.Size()-1 {
		h.rewards = append(h.rewards, c)
	}
	return nil
}

func (h *stackRequestHooks) Place(container stackrequest.Container, slot int, stack item.Stack) error {
	c := container.(stackRequestContainer)
	return call(event.C(inventory.Holder(h.c)), slot, stack, c.inv.Handler().HandlePlace)
}

func (h *stackRequestHooks) Drop(container stackrequest.Container, slot int, stack item.Stack) (int, error) {
	c := container.(stackRequestContainer)
	if err := call(event.C(inventory.Holder(h.c)), slot, stack, c.inv.Handler().HandleDrop); err != nil {
		return 0, err
	}
	if !h.c.PrepareItemDrop(stack) {
		return 0, nil
	}
	return stack.Count(), nil
}

func (h *stackRequestHooks) Destroy(_ stackrequest.Container, _ int, stack item.Stack) (int, error) {
	if !h.c.GameMode().CreativeInventory() {
		return 0, fmt.Errorf("can only destroy items in gamemode creative/spectator")
	}
	return stack.Count(), nil
}

func commitStackRequestPlan(plan *stackrequest.Plan, engine *stackrequest.Engine, hooks *stackRequestHooks) error {
	for _, effect := range plan.Effects() {
		switch effect.(type) {
		case stackrequest.DropEffect, stackRequestEffect:
		default:
			return fmt.Errorf("unsupported stack request effect %T", effect)
		}
	}
	type appliedChange struct {
		container stackRequestContainer
		slot      int
		before    item.Stack
	}
	applied := make([]appliedChange, 0, len(plan.Changes()))
	rollback := func() {
		for i := len(applied) - 1; i >= 0; i-- {
			change := applied[i]
			_ = change.container.setItem(change.slot, change.before)
		}
	}
	for _, change := range plan.Changes() {
		container, ok := change.Container.(stackRequestContainer)
		if !ok {
			rollback()
			return fmt.Errorf("unexpected stack request container %T", change.Container)
		}
		before, ok := container.Item(change.Slot)
		if !ok {
			rollback()
			return fmt.Errorf("read stack request slot %d before commit", change.Slot)
		}
		if err := container.setItem(change.Slot, change.After); err != nil {
			rollback()
			return err
		}
		after, ok := container.Item(change.Slot)
		if !ok || !after.Equal(change.After) {
			_ = container.setItem(change.Slot, before)
			rollback()
			return fmt.Errorf("inventory rejected stack request slot %d", change.Slot)
		}
		applied = append(applied, appliedChange{container: container, slot: change.Slot, before: before})
	}
	for _, effect := range plan.Effects() {
		switch effect := effect.(type) {
		case stackrequest.DropEffect:
			hooks.c.SpawnItemDrop(effect.Stack)
		case stackRequestEffect:
			effect()
		}
	}
	for _, reward := range hooks.rewards {
		if !hooks.openedWindow(reward) {
			continue
		}
		if smelter, ok := hooks.tx.Block(*hooks.s.openedPos.Load()).(smelter); ok {
			for _, orb := range entity.NewExperienceOrbs(entity.EyePosition(hooks.c), smelter.ResetExperience()) {
				hooks.tx.AddEntity(orb)
			}
		}
	}
	engine.Commit(plan)
	return nil
}
