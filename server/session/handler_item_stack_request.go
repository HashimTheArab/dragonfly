package session

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/session/stackrequest"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"time"
)

// ItemStackRequestHandler handles the ItemStackRequest packet. It handles the actions done within the
// inventory.
type ItemStackRequestHandler struct {
	engine              *stackrequest.Engine
	plan                *stackrequest.Plan
	planErr             error
	statefulWorkstation bool
}

// Handle ...
func (h *ItemStackRequestHandler) Handle(p packet.Packet, s *Session, tx *world.Tx, c Controllable) error {
	pk := p.(*packet.ItemStackRequest)

	s.inTransaction.Store(true)
	defer s.inTransaction.Store(false)

	for _, req := range pk.Requests {
		if err := h.handleRequest(req, s, tx, c); err != nil {
			// Item stacks being out of sync isn't uncommon, so don't disconnect. The rejected plan has not
			// mutated live inventories.
			s.conf.Log.Debug("process packet: ItemStackRequest: resolve item stack request: " + err.Error())
		}
	}
	return nil
}

// handleRequest resolves a single item stack request from the client. The plan
// is only applied to live inventories once every action in it has validated.
func (h *ItemStackRequestHandler) handleRequest(req protocol.ItemStackRequest, s *Session, tx *world.Tx, c Controllable) error {
	// plan and planErr are scoped to the request being resolved.
	defer func() {
		h.plan, h.planErr, h.statefulWorkstation = nil, nil, false
	}()

	hooks := &stackRequestHooks{s: s, tx: tx, c: c}
	plan := h.engine.Handle(req, stackRequestResolver{s: s, tx: tx}, hooks, func(plan *stackrequest.Plan, action protocol.StackRequestAction) error {
		h.plan = plan
		err := h.handleExtension(action, req.FilterStrings, s, tx, c)
		if err == nil {
			err = h.planErr
		}
		return err
	}, time.Now())
	err := plan.Err()
	if err == nil {
		err = commitStackRequestPlan(plan, h.engine, hooks)
	}
	if err != nil {
		// A rejected plan has not touched live inventories, so the client only
		// needs to be told to revert what it predicted.
		h.rejectRequest(req.RequestID, s)
		return err
	}
	s.writePacket(&packet.ItemStackResponse{Responses: []protocol.ItemStackResponse{plan.Response()}})
	return nil
}

func (h *ItemStackRequestHandler) handleExtension(action protocol.StackRequestAction, filterStrings []string, s *Session, tx *world.Tx, c Controllable) error {
	switch action := action.(type) {
	case *protocol.BeaconPaymentStackRequestAction:
		if err := h.claimStatefulWorkstation(); err != nil {
			return err
		}
		return h.handleBeaconPayment(action, s, tx)
	case *protocol.CraftRecipeStackRequestAction:
		if s.containerOpened.Load() {
			switch tx.Block(*s.openedPos.Load()).(type) {
			case block.SmithingTable:
				return h.handleSmithing(action, s, tx)
			case block.Stonecutter:
				return h.handleStonecutting(action, s, tx)
			case block.EnchantingTable:
				if err := h.claimStatefulWorkstation(); err != nil {
					return err
				}
				return h.handleEnchant(action, s, tx, c)
			}
		}
		return h.handleCraft(action, s, tx)
	case *protocol.AutoCraftRecipeStackRequestAction:
		return h.handleAutoCraft(action, s, tx)
	case *protocol.CraftRecipeOptionalStackRequestAction:
		if err := h.claimStatefulWorkstation(); err != nil {
			return err
		}
		return h.handleCraftRecipeOptional(action, s, filterStrings, c, tx)
	case *protocol.CraftLoomRecipeStackRequestAction:
		return h.handleLoomCraft(action, s, tx)
	case *protocol.CraftGrindstoneRecipeStackRequestAction:
		if err := h.claimStatefulWorkstation(); err != nil {
			return err
		}
		return h.handleGrindstoneCraft(s, tx, c)
	case *protocol.CraftCreativeStackRequestAction:
		return h.handleCreativeCraft(action, s, tx, c)
	case *protocol.MineBlockStackRequestAction:
		return h.handleMineBlock(action, s, tx)
	default:
		return fmt.Errorf("unhandled stack request action %#v", action)
	}
}

// claimStatefulWorkstation rejects repeated operations whose non-inventory
// state is committed as one effect. This prevents a second operation from
// validating against the same pre-request XP, seed or block state.
func (h *ItemStackRequestHandler) claimStatefulWorkstation() error {
	if h.statefulWorkstation {
		return fmt.Errorf("multiple stateful workstation operations in one request")
	}
	h.statefulWorkstation = true
	return nil
}

// rejectRequest tells the client its request failed so that it reverts the
// changes it predicted.
func (*ItemStackRequestHandler) rejectRequest(id int32, s *Session) {
	s.writePacket(&packet.ItemStackResponse{Responses: []protocol.ItemStackResponse{{
		Status:    protocol.ItemStackResponseStatusError,
		RequestID: id,
	}}})
}

// handleMineBlock handles the action associated with a block being mined by the player. This seems to be a workaround
// by Mojang to deal with the durability changes client-side.
func (h *ItemStackRequestHandler) handleMineBlock(a *protocol.MineBlockStackRequestAction, s *Session, tx *world.Tx) error {
	slot := protocol.StackRequestSlotInfo{
		Container:      protocol.FullContainerName{ContainerID: protocol.ContainerInventory},
		Slot:           byte(a.HotbarSlot),
		StackNetworkID: a.StackNetworkID,
	}
	// Update the slots through ItemStackResponses, don't actually do anything special with this action.
	i, err := h.plan.Item(slot)
	if err != nil {
		return err
	}
	h.setItemInSlot(slot, i, s, tx)
	return nil
}

// The three methods below forward the workstation handlers onto h.plan. They
// keep their *Session and *world.Tx parameters, now unused, so that those
// handlers stay untouched; both are read off the plan instead.

// createResults creates a new craft result and adds it to the list of pending craft results.
func (h *ItemStackRequestHandler) createResults(_ *Session, _ *world.Tx, result ...item.Stack) error {
	return h.plan.CreateResults(result...)
}

// itemInSlot looks for the item in the slot as indicated by the slot info passed.
func (h *ItemStackRequestHandler) itemInSlot(slot protocol.StackRequestSlotInfo, _ *Session, _ *world.Tx) (item.Stack, error) {
	return h.plan.ItemAt(slot.Container, int(slot.Slot))
}

// setItemInSlot stages an item stack in the slot of a container present in the slot info.
// It cannot report failure, so the first error is held in planErr and rejects the
// request once the action returns.
func (h *ItemStackRequestHandler) setItemInSlot(slot protocol.StackRequestSlotInfo, stack item.Stack, _ *Session, _ *world.Tx) {
	if err := h.plan.SetAt(slot.Container, int(slot.Slot), stack); err != nil && h.planErr == nil {
		h.planErr = err
	}
}

// call uses an event.Context, slot and item.Stack to call the event handler function passed. An error is returned if
// the event.Context was cancelled either before or after the call.
func call(ctx *inventory.Context, slot int, it item.Stack, f func(ctx *inventory.Context, slot int, it item.Stack)) error {
	if ctx.Cancelled() {
		return fmt.Errorf("action was cancelled")
	}
	f(ctx, slot, it)
	if ctx.Cancelled() {
		return fmt.Errorf("action was cancelled")
	}
	return nil
}
