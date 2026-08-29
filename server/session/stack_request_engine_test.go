package session

import (
	"errors"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/session/stackrequest"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestStackRequestResolverPreservesUnderlyingInventoryIdentityAcrossAliases(t *testing.T) {
	s := &Session{
		inv:     inventory.New(36, nil),
		ui:      inventory.New(54, nil),
		offHand: inventory.New(1, nil),
	}
	resolver := stackRequestResolver{s: s}
	hotbar, ok := resolver.Resolve(protocol.FullContainerName{ContainerID: protocol.ContainerHotBar})
	if !ok {
		t.Fatal("hotbar container did not resolve")
	}
	combined, ok := resolver.Resolve(protocol.FullContainerName{
		ContainerID: protocol.ContainerCombinedHotBarAndInventory,
	})
	if !ok {
		t.Fatal("combined inventory container did not resolve")
	}
	if hotbar != combined {
		t.Fatal("aliases resolved to distinct container handles")
	}
}

func TestCommitStackRequestPlanAppliesFinalSlotValues(t *testing.T) {
	inv := inventory.New(36, nil)
	stack := item.NewStack(block.Stone{}, 2)
	if err := inv.SetItem(0, stack); err != nil {
		t.Fatal(err)
	}
	s := &Session{inv: inv, ui: inventory.New(54, nil), offHand: inventory.New(1, nil)}
	resolver := stackRequestResolver{s: s}
	engine := stackrequest.New()
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = protocol.StackRequestSlotInfo{
		Container:      protocol.FullContainerName{ContainerID: protocol.ContainerInventory},
		Slot:           0,
		StackNetworkID: item_id(stack),
	}
	place.Destination = protocol.StackRequestSlotInfo{
		Container: protocol.FullContainerName{ContainerID: protocol.ContainerInventory},
		Slot:      1,
	}
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{place},
	}, resolver, nil, nil, time.Unix(1, 0))
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	if err := commitStackRequestPlan(plan, engine, &stackRequestHooks{s: s}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	source, _ := inv.Item(0)
	destination, _ := inv.Item(1)
	if source.Count() != 1 || destination.Count() != 1 {
		t.Fatalf("committed counts = (%d, %d), want (1, 1)", source.Count(), destination.Count())
	}
}

func TestStackRequestResolverMapsOffhandWireSlotOneToStorageSlotZero(t *testing.T) {
	offhand := inventory.New(1, nil)
	if err := offhand.SetItem(0, item.NewStack(block.Stone{}, 1)); err != nil {
		t.Fatal(err)
	}
	s := &Session{
		inv:     inventory.New(36, nil),
		ui:      inventory.New(54, nil),
		offHand: offhand,
	}
	resolver := stackRequestResolver{s: s}
	container, ok := resolver.Resolve(protocol.FullContainerName{ContainerID: protocol.ContainerOffhand})
	if !ok {
		t.Fatal("offhand container did not resolve")
	}
	stack, ok := container.Item(1)
	if !ok || stack.Empty() {
		t.Fatal("wire offhand slot 1 did not resolve storage slot 0")
	}
	if _, ok := container.Item(0); ok {
		t.Fatal("wire offhand slot 0 was accepted")
	}
}

// A client under latency refers to a slot by the request ID that last changed
// it. That only resolves if a container yields an equal handle in every request.
func TestStackRequestPlanResolvesPriorRequestIDAcrossRequests(t *testing.T) {
	inv := inventory.New(36, nil)
	stack := item.NewStack(block.Stone{}, 2)
	if err := inv.SetItem(0, stack); err != nil {
		t.Fatal(err)
	}
	s := &Session{inv: inv, ui: inventory.New(54, nil), offHand: inventory.New(1, nil)}
	engine := stackrequest.New()

	move := func(from, to byte, networkID int32, requestID int32) *stackrequest.Plan {
		place := &protocol.PlaceStackRequestAction{}
		place.Count = 1
		place.Source = protocol.StackRequestSlotInfo{
			Container:      protocol.FullContainerName{ContainerID: protocol.ContainerInventory},
			Slot:           from,
			StackNetworkID: networkID,
		}
		place.Destination = protocol.StackRequestSlotInfo{
			Container: protocol.FullContainerName{ContainerID: protocol.ContainerInventory},
			Slot:      to,
		}
		return engine.Handle(protocol.ItemStackRequest{
			RequestID: requestID,
			Actions:   []protocol.StackRequestAction{place},
		}, stackRequestResolver{s: s}, nil, nil, time.Unix(1, 0))
	}

	first := move(0, 1, item_id(stack), -3)
	if first.Err() != nil {
		t.Fatalf("first plan: %v", first.Err())
	}
	if err := commitStackRequestPlan(first, engine, &stackRequestHooks{s: s}); err != nil {
		t.Fatalf("first commit: %v", err)
	}
	if second := move(1, 2, -3, -5); second.Err() != nil {
		t.Fatalf("slot referred to by prior request ID did not resolve: %v", second.Err())
	}
}

func TestCommitStackRequestPlanRunsDeferredSessionEffect(t *testing.T) {
	s := &Session{
		inv:     inventory.New(36, nil),
		ui:      inventory.New(54, nil),
		offHand: inventory.New(1, nil),
	}
	engine := stackrequest.New()
	called := false
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{&protocol.MineBlockStackRequestAction{}},
	}, stackRequestResolver{s: s}, nil, func(plan *stackrequest.Plan, _ protocol.StackRequestAction) error {
		plan.Defer(stackRequestEffect(func() { called = true }))
		return nil
	}, time.Unix(1, 0))
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	if called {
		t.Fatal("deferred effect ran during planning")
	}
	if err := commitStackRequestPlan(plan, engine, &stackRequestHooks{s: s}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if !called {
		t.Fatal("deferred effect did not run during commit")
	}
}

func TestStackRequestPlanRejectsInventoryValidator(t *testing.T) {
	main := inventory.New(36, nil)
	offhand := inventory.New(1, nil)
	offhand.SlotValidatorFunc(func(item.Stack, int) bool { return false })
	stack := item.NewStack(block.Stone{}, 1)
	if err := main.SetItem(0, stack); err != nil {
		t.Fatal(err)
	}
	s := &Session{inv: main, ui: inventory.New(54, nil), offHand: offhand}
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = protocol.StackRequestSlotInfo{
		Container:      protocol.FullContainerName{ContainerID: protocol.ContainerInventory},
		Slot:           0,
		StackNetworkID: item_id(stack),
	}
	place.Destination = protocol.StackRequestSlotInfo{
		Container: protocol.FullContainerName{ContainerID: protocol.ContainerOffhand},
		Slot:      1,
	}
	plan := stackrequest.New().Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{place},
	}, stackRequestResolver{s: s}, nil, nil, time.Unix(1, 0))
	if !errors.Is(plan.Err(), stackrequest.ErrInvalidItem) {
		t.Fatalf("plan error = %v, want ErrInvalidItem", plan.Err())
	}
}

func TestCommitStackRequestPlanRollsBackRejectedWrite(t *testing.T) {
	main := inventory.New(1, nil)
	destination := inventory.New(1, nil)
	stack := item.NewStack(block.Stone{}, 1)
	if err := main.SetItem(0, stack); err != nil {
		t.Fatal(err)
	}
	s := &Session{inv: main, ui: destination, offHand: inventory.New(1, nil)}
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = protocol.StackRequestSlotInfo{
		Container:      protocol.FullContainerName{ContainerID: protocol.ContainerInventory},
		Slot:           0,
		StackNetworkID: item_id(stack),
	}
	place.Destination = protocol.StackRequestSlotInfo{
		Container: protocol.FullContainerName{ContainerID: protocol.ContainerCursor},
		Slot:      0,
	}
	engine := stackrequest.New()
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{place},
	}, stackRequestResolver{s: s}, nil, nil, time.Unix(1, 0))
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	destination.SlotValidatorFunc(func(item.Stack, int) bool { return false })
	if err := commitStackRequestPlan(plan, engine, &stackRequestHooks{s: s}); err == nil {
		t.Fatal("commit succeeded after destination began rejecting the stack")
	}
	gotSource, _ := main.Item(0)
	gotDestination, _ := destination.Item(0)
	if gotSource.Empty() || !gotDestination.Empty() {
		t.Fatalf("rollback slots = source %#v destination %#v", gotSource, gotDestination)
	}
}

func TestItemStackRequestHandlerRejectsRepeatedStatefulWorkstationActions(t *testing.T) {
	h := &ItemStackRequestHandler{}
	plan := stackrequest.New().Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions: []protocol.StackRequestAction{
			&protocol.MineBlockStackRequestAction{},
			&protocol.MineBlockStackRequestAction{},
		},
	}, stackRequestResolver{}, nil, func(plan *stackrequest.Plan, _ protocol.StackRequestAction) error {
		h.plan = plan
		return h.claimStatefulWorkstation()
	}, time.Unix(1, 0))
	if plan.Err() == nil {
		t.Fatal("repeated stateful workstation action was accepted")
	}
	var reject *stackrequest.RejectError
	if !errors.As(plan.Err(), &reject) || reject.ActionIndex != 1 {
		t.Fatalf("reject = %#v, want action index 1", reject)
	}
}

func TestStackRequestHooksDoNotRewardTakingEmptySmelterResult(t *testing.T) {
	opened := inventory.New(3, nil)
	s := &Session{}
	s.openedWindow.Store(opened)
	s.containerOpened.Store(true)
	hooks := &stackRequestHooks{s: s}
	container := stackRequestContainer{inv: opened}
	if err := hooks.Take(container, opened.Size()-1, item.Stack{}); err != nil {
		t.Fatalf("take: %v", err)
	}
	if len(hooks.rewards) != 0 {
		t.Fatalf("empty result queued %d smelter rewards", len(hooks.rewards))
	}
}
