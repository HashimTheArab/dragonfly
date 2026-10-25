package stackrequest

import (
	"errors"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type testContainer struct {
	items      []item.Stack
	networkIDs []int32
	accepts    func(int, item.Stack) bool
}

func (c *testContainer) apply(changes []Change, nextID *int32) {
	for _, change := range changes {
		if change.Container != c {
			continue
		}
		c.items[change.Slot] = change.After
		c.networkIDs[change.Slot] = *nextID
		*nextID++
	}
}

func (c *testContainer) Item(slot int) (item.Stack, bool) {
	if slot < 0 || slot >= len(c.items) {
		return item.Stack{}, false
	}
	return c.items[slot], true
}

func (c *testContainer) StackNetworkID(slot int) (int32, bool) {
	if slot < 0 || slot >= len(c.networkIDs) {
		return 0, false
	}
	return c.networkIDs[slot], true
}

func (c *testContainer) Size() int { return len(c.items) }

func (c *testContainer) Accepts(slot int, stack item.Stack) bool {
	return c.accepts == nil || c.accepts(slot, stack)
}

type testResolver struct {
	containers map[byte]Container
}

type nonComparableContainer []item.Stack

func (c nonComparableContainer) Item(slot int) (item.Stack, bool) {
	if slot < 0 || slot >= len(c) {
		return item.Stack{}, false
	}
	return c[slot], true
}
func (c nonComparableContainer) StackNetworkID(int) (int32, bool) { return 10, true }
func (c nonComparableContainer) Size() int                        { return len(c) }
func (c nonComparableContainer) Accepts(int, item.Stack) bool     { return true }

type nonComparableResolver struct{ container nonComparableContainer }

func (r nonComparableResolver) Resolve(protocol.FullContainerName) (Container, bool) {
	return r.container, true
}
func (nonComparableResolver) StackNetworkID(item.Stack) int32 { return 11 }

func (r testResolver) Resolve(name protocol.FullContainerName) (Container, bool) {
	c, ok := r.containers[name.ContainerID]
	return c, ok
}

func (testResolver) StackNetworkID(stack item.Stack) int32 {
	if stack.Empty() {
		return 0
	}
	return int32(100 + stack.Count())
}

type testHooks struct{}

func (testHooks) Take(Container, int, item.Stack) error  { return nil }
func (testHooks) Place(Container, int, item.Stack) error { return nil }
func (testHooks) Drop(_ Container, _ int, stack item.Stack) (int, error) {
	return stack.Count(), nil
}
func (testHooks) Destroy(_ Container, _ int, stack item.Stack) (int, error) {
	return stack.Count(), nil
}

type rejectingDestroyHooks struct{ testHooks }

func (rejectingDestroyHooks) Destroy(Container, int, item.Stack) (int, error) {
	return 0, errors.New("destroy denied")
}

type ignoringDestroyHooks struct{ testHooks }

func (ignoringDestroyHooks) Destroy(Container, int, item.Stack) (int, error) { return 0, nil }

type cancellingDropHooks struct{ testHooks }

func (cancellingDropHooks) Drop(Container, int, item.Stack) (int, error) { return 0, nil }

type rejectingTakeHooks struct{ testHooks }

func (rejectingTakeHooks) Take(Container, int, item.Stack) error {
	return errors.New("take cancelled")
}

type rejectingPlaceHooks struct{ testHooks }

func (rejectingPlaceHooks) Place(Container, int, item.Stack) error {
	return errors.New("place cancelled")
}

func stackSlot(container byte, slot byte, networkID int32) protocol.StackRequestSlotInfo {
	return protocol.StackRequestSlotInfo{
		Container:      protocol.FullContainerName{ContainerID: container},
		Slot:           slot,
		StackNetworkID: networkID,
	}
}

// container builds a container holding stacks. An occupied slot n carries stack
// network ID 10+n; an empty slot carries 0, as an empty slot does on the wire.
func container(stacks ...item.Stack) *testContainer {
	c := &testContainer{items: stacks, networkIDs: make([]int32, len(stacks))}
	for i, stack := range stacks {
		if !stack.Empty() {
			c.networkIDs[i] = int32(10 + i)
		}
	}
	return c
}

// empty builds a container of n empty slots, as used for UI and cursor containers.
func empty(n int) *testContainer {
	return &testContainer{items: make([]item.Stack, n), networkIDs: make([]int32, n)}
}

func resolverOf(containers map[byte]Container) testResolver {
	return testResolver{containers: containers}
}

// handle plans actions as request -3 against resolver, the default for tests
// that do not exercise request identity or expiry.
func handle(resolver Resolver, hooks Hooks, extension Extension, actions ...protocol.StackRequestAction) *Plan {
	return New().Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   actions,
	}, resolver, hooks, extension, time.Unix(1, 0))
}

func TestEngine_DropThenInvalidTransferPublishesNoChangesOrEffects(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 5), item.Stack{})
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
		protocol.ContainerCursor:    empty(1),
	})

	drop := &protocol.DropStackRequestAction{
		Count:  2,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	invalid := &protocol.TakeStackRequestAction{}
	invalid.Count = 64
	invalid.Source = stackSlot(protocol.ContainerInventory, 1, 0)
	invalid.Destination = stackSlot(protocol.ContainerCursor, 0, 0)

	plan := handle(resolver, testHooks{}, nil, drop, invalid)

	if !errors.Is(plan.Err(), ErrInsufficientCount) {
		t.Fatalf("plan error = %v, want ErrInsufficientCount", plan.Err())
	}
	var reject *RejectError
	if !errors.As(plan.Err(), &reject) || reject.ActionIndex != 1 {
		t.Fatalf("reject context = %#v, want action index 1", reject)
	}
	if len(plan.Changes()) != 0 {
		t.Fatalf("rejected plan published %d changes", len(plan.Changes()))
	}
	if len(plan.Effects()) != 0 {
		t.Fatalf("rejected plan published %d effects", len(plan.Effects()))
	}
	if plan.Response().Status != protocol.ItemStackResponseStatusError {
		t.Fatalf("response status = %d, want error", plan.Response().Status)
	}
	if got, _ := main.Item(0); got.Count() != 5 {
		t.Fatalf("planning mutated source count to %d", got.Count())
	}
}

func TestEngine_NegativeRequestIDUsesUnderlyingContainerIdentityAcrossAliases(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 2))
	ui := empty(1)
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerHotBar:                     main,
		protocol.ContainerCombinedHotBarAndInventory: main,
		protocol.ContainerCursor:                     ui,
	})
	engine := New()

	take := &protocol.TakeStackRequestAction{}
	take.Count = 1
	take.Source = stackSlot(protocol.ContainerHotBar, 0, 10)
	take.Destination = stackSlot(protocol.ContainerCursor, 0, 0)
	first := engine.Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{take},
	}, resolver, testHooks{}, nil, time.Unix(1, 0))
	if first.Err() != nil {
		t.Fatalf("first plan: %v", first.Err())
	}
	nextID := int32(11)
	main.apply(first.Changes(), &nextID)
	ui.apply(first.Changes(), &nextID)
	engine.Commit(first)

	drop := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerCombinedHotBarAndInventory, 0, -3),
	}
	second := engine.Handle(protocol.ItemStackRequest{
		RequestID: -5,
		Actions:   []protocol.StackRequestAction{drop},
	}, resolver, testHooks{}, nil, time.Unix(2, 0))
	if second.Err() != nil {
		t.Fatalf("alias plan: %v", second.Err())
	}
}

func TestEngine_ResponsePreservesFirstChangeOrder(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1), item.NewStack(block.Dirt{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
	})
	swap := &protocol.SwapStackRequestAction{
		Source:      stackSlot(protocol.ContainerInventory, 1, 11),
		Destination: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	engine := New()
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{swap},
	}, resolver, testHooks{}, nil, time.Unix(1, 0))
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	nextID := int32(20)
	main.apply(plan.Changes(), &nextID)
	engine.Commit(plan)

	response := plan.Response()
	if len(response.ContainerInfo) != 1 {
		t.Fatalf("response containers = %d, want 1", len(response.ContainerInfo))
	}
	slots := response.ContainerInfo[0].SlotInfo
	if len(slots) != 2 || slots[0].Slot != 1 || slots[1].Slot != 0 {
		t.Fatalf("response slot order = %#v, want [1, 0]", slots)
	}
	if slots[0].StackNetworkID != 20 || slots[1].StackNetworkID != 21 {
		t.Fatalf("response network IDs = [%d, %d], want [20, 21]",
			slots[0].StackNetworkID, slots[1].StackNetworkID)
	}
}

func TestEngine_ExtensionStagesIntoSameRejectedJournal(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 2), item.Stack{})
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
		protocol.ContainerCursor:    empty(1),
	})
	minedSlot := stackSlot(protocol.ContainerInventory, 0, 10)
	extension := func(plan *Plan, action protocol.StackRequestAction) error {
		if _, ok := action.(*protocol.MineBlockStackRequestAction); !ok {
			return ErrUnsupportedAction
		}
		stack, err := plan.ItemAt(minedSlot.Container, int(minedSlot.Slot))
		if err != nil {
			return err
		}
		plan.Defer("mining effect")
		return plan.SetAt(minedSlot.Container, int(minedSlot.Slot), stack.Grow(-1))
	}
	invalid := &protocol.TakeStackRequestAction{}
	invalid.Count = 64
	invalid.Source = stackSlot(protocol.ContainerInventory, 1, 0)
	invalid.Destination = stackSlot(protocol.ContainerCursor, 0, 0)
	plan := handle(resolver, testHooks{}, extension, &protocol.MineBlockStackRequestAction{}, invalid)

	if !errors.Is(plan.Err(), ErrInsufficientCount) {
		t.Fatalf("plan error = %v, want ErrInsufficientCount", plan.Err())
	}
	if len(plan.Changes()) != 0 {
		t.Fatal("rejected extension plan published changes")
	}
	if len(plan.Effects()) != 0 {
		t.Fatal("rejected extension plan published effects")
	}
}

func TestEngine_CurrentRequestIDResolvesStagedExtensionResult(t *testing.T) {
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory:     empty(1),
		protocol.ContainerCreatedOutput: empty(1),
	})
	resultSlot := stackSlot(protocol.ContainerCreatedOutput, 0, 0)
	extension := func(plan *Plan, action protocol.StackRequestAction) error {
		if _, ok := action.(*protocol.CraftCreativeStackRequestAction); !ok {
			return ErrUnsupportedAction
		}
		return plan.SetAt(resultSlot.Container, int(resultSlot.Slot), item.NewStack(block.Stone{}, 1))
	}
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = stackSlot(protocol.ContainerCreatedOutput, 0, -3)
	place.Destination = stackSlot(protocol.ContainerInventory, 0, 0)
	plan := handle(resolver, testHooks{}, extension, &protocol.CraftCreativeStackRequestAction{}, place)

	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
}

func TestEngine_ExtensionReadsGenericChangesFromShadowState(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1))
	ui := empty(1)
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory:     main,
		protocol.ContainerCraftingInput: ui,
	})
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = stackSlot(protocol.ContainerInventory, 0, 10)
	place.Destination = stackSlot(protocol.ContainerCraftingInput, 0, 0)
	extension := func(plan *Plan, action protocol.StackRequestAction) error {
		if _, ok := action.(*protocol.MineBlockStackRequestAction); !ok {
			return ErrUnsupportedAction
		}
		stack, err := plan.ItemAt(protocol.FullContainerName{ContainerID: protocol.ContainerCraftingInput}, 0)
		if err != nil {
			return err
		}
		if stack.Empty() || stack.Item() != (block.Stone{}) {
			return errors.New("extension observed stale live slot")
		}
		return nil
	}
	plan := handle(resolver, testHooks{}, extension, place, &protocol.MineBlockStackRequestAction{})
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
}

func TestEngine_ExtensionMayIgnoreDestroyBeforeSlotValidation(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
	})
	extension := func(plan *Plan, action protocol.StackRequestAction) error {
		if _, ok := action.(*protocol.BeaconPaymentStackRequestAction); !ok {
			return ErrUnsupportedAction
		}
		if err := plan.SetAt(protocol.FullContainerName{ContainerID: protocol.ContainerInventory}, 0, item.Stack{}); err != nil {
			return err
		}
		plan.IgnoreDestroyActions()
		return nil
	}
	destroy := &protocol.DestroyStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := handle(resolver, testHooks{}, extension, &protocol.BeaconPaymentStackRequestAction{}, destroy)
	if plan.Err() != nil {
		t.Fatalf("ignored destroy: %v", plan.Err())
	}
}

func TestEngine_DestroyUsesConsumerPolicy(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
	})
	destroy := &protocol.DestroyStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := handle(resolver, rejectingDestroyHooks{}, nil, destroy)
	if plan.Err() == nil {
		t.Fatal("destroy policy rejection was ignored")
	}
}

func TestEngine_DestroyPolicyMayIgnoreActionWithoutRejecting(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
	})
	destroy := &protocol.DestroyStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := handle(resolver, ignoringDestroyHooks{}, nil, destroy)
	if plan.Err() != nil || len(plan.Changes()) != 0 {
		t.Fatalf("ignored destroy = error %v, changes %d", plan.Err(), len(plan.Changes()))
	}
}

func TestEngine_TransferHonoursTakeAndPlaceCancellation(t *testing.T) {
	newPlan := func(hooks Hooks) *Plan {
		resolver := resolverOf(map[byte]Container{
			protocol.ContainerInventory: container(item.NewStack(block.Stone{}, 1), item.Stack{}),
		})
		place := &protocol.PlaceStackRequestAction{}
		place.Count = 1
		place.Source = stackSlot(protocol.ContainerInventory, 0, 10)
		place.Destination = stackSlot(protocol.ContainerInventory, 1, 0)
		return handle(resolver, hooks, nil, place)
	}
	if plan := newPlan(rejectingTakeHooks{}); plan.Err() == nil {
		t.Fatal("source take cancellation was ignored")
	}
	if plan := newPlan(rejectingPlaceHooks{}); plan.Err() == nil {
		t.Fatal("destination place cancellation was ignored")
	}
}

func TestEngine_RejectsDestinationThatDoesNotAcceptStack(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1))
	destination := empty(1)
	destination.accepts = func(int, item.Stack) bool { return false }
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
		protocol.ContainerArmor:     destination,
	})
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = stackSlot(protocol.ContainerInventory, 0, 10)
	place.Destination = stackSlot(protocol.ContainerArmor, 0, 0)
	plan := handle(resolver, testHooks{}, nil, place)
	if !errors.Is(plan.Err(), ErrInvalidItem) {
		t.Fatalf("destination validation error = %v, want ErrInvalidItem", plan.Err())
	}
	if len(plan.Changes()) != 0 {
		t.Fatalf("rejected plan published changes: %#v", plan.Changes())
	}
}

func TestEngine_DropEffectPreservesRandomFlag(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
	})
	drop := &protocol.DropStackRequestAction{
		Count:    1,
		Source:   stackSlot(protocol.ContainerInventory, 0, 10),
		Randomly: true,
	}
	plan := handle(resolver, testHooks{}, nil, drop)
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	effect, ok := plan.Effects()[0].(DropEffect)
	if !ok || !effect.Randomly {
		t.Fatalf("drop effect = %#v, want random flag", plan.Effects()[0])
	}
}

func TestEngine_CancelledDropStagesClientCorrectionWithoutEffect(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
	})
	drop := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := handle(resolver, cancellingDropHooks{}, nil, drop)
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	changes := plan.Changes()
	if len(changes) != 1 || changes[0].After.Count() != 1 {
		t.Fatalf("cancelled drop changes = %#v, want one unchanged-count correction", changes)
	}
	if len(plan.Effects()) != 0 {
		t.Fatalf("cancelled drop effects = %#v, want none", plan.Effects())
	}
}

func TestEngine_PositiveStackIDAcknowledgesPriorPredictionReference(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 2))
	ui := empty(1)
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
		protocol.ContainerCursor:    ui,
	})
	engine := New()
	take := &protocol.TakeStackRequestAction{}
	take.Count = 1
	take.Source = stackSlot(protocol.ContainerInventory, 0, 10)
	take.Destination = stackSlot(protocol.ContainerCursor, 0, 0)
	first := engine.Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{take},
	}, resolver, testHooks{}, nil, time.Unix(1, 0))
	nextID := int32(11)
	main.apply(first.Changes(), &nextID)
	ui.apply(first.Changes(), &nextID)
	engine.Commit(first)

	positive := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 11),
	}
	if plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -5,
		Actions:   []protocol.StackRequestAction{positive},
	}, resolver, testHooks{}, nil, time.Unix(2, 0)); plan.Err() != nil {
		t.Fatalf("positive acknowledgement plan: %v", plan.Err())
	}

	stale := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, -3),
	}
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -7,
		Actions:   []protocol.StackRequestAction{stale},
	}, resolver, testHooks{}, nil, time.Unix(3, 0))
	if !errors.Is(plan.Err(), ErrStackNetworkID) {
		t.Fatalf("stale prediction error = %v, want ErrStackNetworkID", plan.Err())
	}
}

func TestEngine_BoundsUnacknowledgedPredictionHistory(t *testing.T) {
	engine := New()
	for request := int32(-1); request >= -257; request-- {
		c := container(item.NewStack(block.Stone{}, 1))
		resolver := resolverOf(map[byte]Container{
			protocol.ContainerInventory: c,
		})
		drop := &protocol.DropStackRequestAction{
			Count:  1,
			Source: stackSlot(protocol.ContainerInventory, 0, 10),
		}
		plan := engine.Handle(protocol.ItemStackRequest{
			RequestID: request,
			Actions:   []protocol.StackRequestAction{drop},
		}, resolver, testHooks{}, nil, time.Unix(1, 0))
		if plan.Err() != nil {
			t.Fatalf("seed request %d: %v", request, plan.Err())
		}
		nextID := int32(11)
		c.apply(plan.Changes(), &nextID)
		engine.Commit(plan)
	}
	c := container(item.NewStack(block.Stone{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: c,
	})
	drop := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -258,
		Actions:   []protocol.StackRequestAction{drop},
	}, resolver, testHooks{}, nil, time.Unix(2, 0))
	if !errors.Is(plan.Err(), ErrTooManyUnacknowledgedChanges) {
		t.Fatalf("history bound error = %v, want ErrTooManyUnacknowledgedChanges", plan.Err())
	}
}

func TestEngine_RejectsNonComparableContainerIdentity(t *testing.T) {
	resolver := nonComparableResolver{container: nonComparableContainer{
		item.NewStack(block.Stone{}, 1),
	}}
	drop := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := handle(resolver, testHooks{}, nil, drop)
	if !errors.Is(plan.Err(), ErrInvalidContainerIdentity) {
		t.Fatalf("container identity error = %v, want ErrInvalidContainerIdentity", plan.Err())
	}
}

func TestEngine_RejectsTypedNilContainerIdentity(t *testing.T) {
	var nilContainer *testContainer
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: nilContainer,
	})
	drop := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := handle(resolver, testHooks{}, nil, drop)
	if !errors.Is(plan.Err(), ErrInvalidContainerIdentity) {
		t.Fatalf("container identity error = %v, want ErrInvalidContainerIdentity", plan.Err())
	}
}

func TestEngine_CreateConsumesPendingCraftResult(t *testing.T) {
	main := empty(1)
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory:     main,
		protocol.ContainerCreatedOutput: empty(54),
	})
	extension := func(plan *Plan, action protocol.StackRequestAction) error {
		if _, ok := action.(*protocol.CraftCreativeStackRequestAction); !ok {
			return ErrUnsupportedAction
		}
		return plan.CreateResults(
			item.NewStack(block.Stone{}, 1),
			item.NewStack(block.Dirt{}, 1),
		)
	}
	create := &protocol.CreateStackRequestAction{ResultsSlot: 1}
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = stackSlot(protocol.ContainerCreatedOutput, 50, -3)
	place.Destination = stackSlot(protocol.ContainerInventory, 0, 0)
	plan := handle(resolver, testHooks{}, extension, &protocol.CraftCreativeStackRequestAction{}, create, place)
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	var destination item.Stack
	for _, change := range plan.Changes() {
		if change.Container == main && change.Slot == 0 {
			destination = change.After
		}
	}
	if destination.Empty() || destination.Item() != (block.Dirt{}) {
		t.Fatalf("created destination = %#v, want dirt", destination)
	}
}

func TestEngine_DoesNotTrackRequestsWithoutSlotChanges(t *testing.T) {
	engine := New()
	for request := int32(-1); request >= -300; request-- {
		plan := engine.Handle(protocol.ItemStackRequest{
			RequestID: request,
			Actions:   []protocol.StackRequestAction{&protocol.ConsumeStackRequestAction{}},
		}, testResolver{}, testHooks{}, nil, time.Unix(1, 0))
		if plan.Err() != nil {
			t.Fatalf("no-op request %d: %v", request, plan.Err())
		}
		engine.Commit(plan)
	}
	main := container(item.NewStack(block.Stone{}, 1))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerInventory: main,
	})
	drop := &protocol.DropStackRequestAction{
		Count:  1,
		Source: stackSlot(protocol.ContainerInventory, 0, 10),
	}
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -301,
		Actions:   []protocol.StackRequestAction{drop},
	}, resolver, testHooks{}, nil, time.Unix(2, 0))
	if plan.Err() != nil {
		t.Fatalf("request after no-op history: %v", plan.Err())
	}
}

func TestEngine_ResponseRetainsEveryTouchedContainerAlias(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 2))
	ui := empty(1)
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerHotBar:                     main,
		protocol.ContainerCombinedHotBarAndInventory: main,
		protocol.ContainerCursor:                     ui,
	})
	take := &protocol.TakeStackRequestAction{}
	take.Count = 1
	take.Source = stackSlot(protocol.ContainerHotBar, 0, 10)
	take.Destination = stackSlot(protocol.ContainerCursor, 0, 0)
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = stackSlot(protocol.ContainerCursor, 0, -3)
	place.Destination = stackSlot(protocol.ContainerCombinedHotBarAndInventory, 0, -3)
	engine := New()
	plan := engine.Handle(protocol.ItemStackRequest{
		RequestID: -3,
		Actions:   []protocol.StackRequestAction{take, place},
	}, resolver, testHooks{}, nil, time.Unix(1, 0))
	if plan.Err() != nil {
		t.Fatalf("plan: %v", plan.Err())
	}
	nextID := int32(20)
	main.apply(plan.Changes(), &nextID)
	ui.apply(plan.Changes(), &nextID)
	engine.Commit(plan)
	seen := map[byte]bool{}
	for _, container := range plan.Response().ContainerInfo {
		seen[container.Container.ContainerID] = true
	}
	for _, container := range []byte{
		protocol.ContainerHotBar,
		protocol.ContainerCursor,
		protocol.ContainerCombinedHotBarAndInventory,
	} {
		if !seen[container] {
			t.Fatalf("response omitted touched container alias %d", container)
		}
	}
}

func TestEngine_RejectsTransferToSamePhysicalSlot(t *testing.T) {
	main := container(item.NewStack(block.Stone{}, 2))
	resolver := resolverOf(map[byte]Container{
		protocol.ContainerHotBar:                     main,
		protocol.ContainerCombinedHotBarAndInventory: main,
	})
	place := &protocol.PlaceStackRequestAction{}
	place.Count = 1
	place.Source = stackSlot(protocol.ContainerHotBar, 0, 10)
	place.Destination = stackSlot(protocol.ContainerCombinedHotBarAndInventory, 0, 10)
	plan := handle(resolver, testHooks{}, nil, place)
	if !errors.Is(plan.Err(), ErrSameSlot) {
		t.Fatalf("same-slot error = %v, want ErrSameSlot", plan.Err())
	}
}
