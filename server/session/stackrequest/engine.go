// Package stackrequest validates and plans protocol item stack requests without
// mutating live inventories. Consumers decide how and when successful plans are
// committed.
package stackrequest

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

var (
	// ErrInvalidCount indicates that an action requested a non-positive count.
	ErrInvalidCount = errors.New("invalid item count")
	// ErrInsufficientCount indicates that a source stack did not contain the requested count.
	ErrInsufficientCount = errors.New("insufficient item count")
	// ErrIncompatibleStack indicates that an item cannot be placed into the destination stack.
	ErrIncompatibleStack = errors.New("incompatible item stack")
	// ErrStackOverflow indicates that a transfer would exceed the destination stack limit.
	ErrStackOverflow = errors.New("item stack overflow")
	// ErrSameSlot indicates that a transfer used one physical slot as both source and destination.
	ErrSameSlot = errors.New("source and destination are the same slot")
	// ErrInvalidSlot indicates that a container or slot could not be resolved.
	ErrInvalidSlot = errors.New("invalid inventory slot")
	// ErrInvalidItem indicates that a container rejected a planned slot value.
	ErrInvalidItem = errors.New("invalid item for inventory slot")
	// ErrInvalidContainerIdentity indicates that a resolver returned an unsafe container handle.
	ErrInvalidContainerIdentity = errors.New("invalid container identity")
	// ErrStackNetworkID indicates that a request referred to stale stack identity.
	ErrStackNetworkID = errors.New("stack network ID mismatch")
	// ErrUnsupportedAction indicates that neither the engine nor its extension handled an action.
	ErrUnsupportedAction = errors.New("unsupported stack request action")
	// ErrInvalidResult indicates that a Create action referred to no pending craft result.
	ErrInvalidResult = errors.New("invalid pending craft result")
	// ErrTooManyUnacknowledgedChanges indicates that prediction history exceeded its safe bound.
	ErrTooManyUnacknowledgedChanges = errors.New("too many unacknowledged stack request changes")
)

const (
	// createdOutputSlot is the client UI slot holding the output of a craft in
	// the ContainerCreatedOutput container.
	createdOutputSlot = 50
	// historyExpiry is how long a request's predicted stack identities stay
	// resolvable before the client is assumed to have caught up.
	historyExpiry = 5 * time.Second
	// maxTrackedRequests bounds prediction history so a client that never
	// acknowledges a change cannot grow it without limit.
	maxTrackedRequests = 256
)

// RejectError identifies the request action that caused planning to fail.
type RejectError struct {
	ActionIndex int
	Action      protocol.StackRequestAction
	Cause       error
}

func (e *RejectError) Error() string {
	return fmt.Sprintf("action %d (%T): %v", e.ActionIndex, e.Action, e.Cause)
}

// Unwrap returns the validation error that rejected the action.
func (e *RejectError) Unwrap() error { return e.Cause }

// Container is a live, read-only inventory view. Implementations must be
// comparable, and equal values must refer to the same underlying inventory.
// This identity is used to keep aliases such as HotBar and Inventory coherent.
type Container interface {
	Item(slot int) (item.Stack, bool)
	StackNetworkID(slot int) (int32, bool)
	Size() int
	Accepts(slot int, stack item.Stack) bool
}

// Resolver applies consumer-owned container access policy and resolves a full
// protocol container name to its underlying inventory.
type Resolver interface {
	Resolve(protocol.FullContainerName) (Container, bool)
	StackNetworkID(item.Stack) int32
}

// Hooks exposes Dragonfly-style cancellable inventory action hooks. Hooks run
// while planning, before any live inventory mutation occurs. Drop and Destroy
// return how many of the offered items the consumer permits.
type Hooks interface {
	Take(Container, int, item.Stack) error
	Place(Container, int, item.Stack) error
	Drop(Container, int, item.Stack) (int, error)
	Destroy(Container, int, item.Stack) (int, error)
}

// Extension plans an action not owned by the generic engine.
type Extension func(*Plan, protocol.StackRequestAction) error

// permit reports how many of the offered items a consumer allows to leave a slot.
type permit func(Container, int, item.Stack) (int, error)

// noHooks is the policy used when a consumer supplies none: everything is allowed.
type noHooks struct{}

func (noHooks) Take(Container, int, item.Stack) error  { return nil }
func (noHooks) Place(Container, int, item.Stack) error { return nil }
func (noHooks) Drop(_ Container, _ int, stack item.Stack) (int, error) {
	return stack.Count(), nil
}
func (noHooks) Destroy(_ Container, _ int, stack item.Stack) (int, error) {
	return stack.Count(), nil
}

// Change is the final planned value of one inventory slot.
type Change struct {
	Container Container
	Slot      int
	After     item.Stack
	// StackNetworkID is the planned identity of After.
	StackNetworkID int32
}

// Effect is an ordered side effect produced by a successful plan.
type Effect any

// DropEffect requests that Stack is dropped only when the plan is committed.
type DropEffect struct {
	Stack    item.Stack
	Randomly bool
}

type slotKey struct {
	container Container
	slot      int
}

type responseKey struct {
	name protocol.FullContainerName
	slot int
}

// responseSlot is one protocol alias reporting the change at change index.
type responseSlot struct {
	responseKey
	container Container
	change    int
}

type historyChange struct {
	networkID int32
	timestamp time.Time
}

// Engine holds cross-request prediction history. One Engine is used per client session.
type Engine struct {
	history map[int32]map[slotKey]historyChange
}

// New creates an empty request engine.
func New() *Engine {
	return &Engine{history: make(map[int32]map[slotKey]historyChange)}
}

// Plan accumulates the staged slot changes, side effects and response of one
// request. It is complete once Handle returns.
type Plan struct {
	requestID int32
	err       error
	engine    *Engine
	resolver  Resolver
	hooks     Hooks
	now       time.Time

	changes []Change
	index   map[slotKey]int
	effects []Effect
	results []item.Stack

	responses     []responseSlot
	responseIndex map[responseKey]struct{}
	ignoreDestroy bool
}

// Handle validates and plans request against resolver without mutating live containers.
func (e *Engine) Handle(request protocol.ItemStackRequest, resolver Resolver, hooks Hooks, extension Extension, now time.Time) *Plan {
	e.prune(now)
	if hooks == nil {
		hooks = noHooks{}
	}
	p := &Plan{
		requestID:     request.RequestID,
		engine:        e,
		resolver:      resolver,
		hooks:         hooks,
		now:           now,
		index:         make(map[slotKey]int),
		responseIndex: make(map[responseKey]struct{}),
	}
	if len(e.history) > maxTrackedRequests {
		p.err = ErrTooManyUnacknowledgedChanges
		return p
	}
	for actionIndex, action := range request.Actions {
		if err := p.handle(action, extension); err != nil {
			p.err = &RejectError{ActionIndex: actionIndex, Action: action, Cause: err}
			break
		}
	}
	return p
}

// Item returns the shadow value of a request slot. Staged changes take
// precedence over the resolver's live value.
func (p *Plan) Item(info protocol.StackRequestSlotInfo) (item.Stack, error) {
	_, _, stack, err := p.resolve(info)
	return stack, err
}

// ItemAt returns the shadow value at slot without applying wire stack-network-ID validation.
// Extension handlers use ItemAt for server-derived recipe and workstation slots.
func (p *Plan) ItemAt(name protocol.FullContainerName, slot int) (item.Stack, error) {
	_, stack, _, err := p.lookup(name, slot)
	return stack, err
}

// SetAt stages stack at slot without applying wire stack-network-ID validation.
func (p *Plan) SetAt(name protocol.FullContainerName, slot int, stack item.Stack) error {
	container, _, _, err := p.lookup(name, slot)
	if err != nil {
		return err
	}
	return p.set(container, name, slot, stack)
}

// Defer appends an ordered consumer-defined effect to the request journal.
// Effects are only exposed when the complete plan succeeds.
func (p *Plan) Defer(effect Effect) {
	p.effects = append(p.effects, effect)
}

// IgnoreDestroyActions makes Destroy actions later in this request informational.
// Extensions use it when they consume an input themselves but the client still
// emits its normal follow-up Destroy action, such as for beacon payment.
func (p *Plan) IgnoreDestroyActions() {
	p.ignoreDestroy = true
}

// lookup resolves name and slot to a container and the stack it will hold,
// preferring a value staged earlier in this request over the live one. The
// returned change is non-nil exactly when that value came from the plan.
func (p *Plan) lookup(name protocol.FullContainerName, slot int) (Container, item.Stack, *Change, error) {
	container, ok := p.resolver.Resolve(name)
	if !ok || container == nil {
		return nil, item.Stack{}, nil, ErrInvalidSlot
	}
	if !validContainerIdentity(container) {
		return nil, item.Stack{}, nil, ErrInvalidContainerIdentity
	}
	if slot < 0 || slot >= container.Size() {
		return nil, item.Stack{}, nil, ErrInvalidSlot
	}
	if i, staged := p.index[slotKey{container: container, slot: slot}]; staged {
		return container, p.changes[i].After, &p.changes[i], nil
	}
	stack, ok := container.Item(slot)
	if !ok {
		return nil, item.Stack{}, nil, ErrInvalidSlot
	}
	return container, stack, nil, nil
}

func (p *Plan) handle(action protocol.StackRequestAction, extension Extension) error {
	switch action := action.(type) {
	case *protocol.TakeStackRequestAction:
		return p.transfer(action.Source, action.Destination, int(action.Count))
	case *protocol.PlaceStackRequestAction:
		return p.transfer(action.Source, action.Destination, int(action.Count))
	case *protocol.SwapStackRequestAction:
		return p.swap(action.Source, action.Destination)
	case *protocol.DropStackRequestAction:
		return p.drop(action.Source, int(action.Count), action.Randomly)
	case *protocol.DestroyStackRequestAction:
		if p.ignoreDestroy {
			return nil
		}
		return p.remove(action.Source, int(action.Count))
	case *protocol.CreateStackRequestAction:
		return p.create(int(action.ResultsSlot))
	case *protocol.ConsumeStackRequestAction, *protocol.CraftResultsDeprecatedStackRequestAction:
		// These remain deliberately informational, matching Dragonfly's existing handler.
		return nil
	default:
		if extension != nil {
			return extension(p, action)
		}
		return ErrUnsupportedAction
	}
}

// CreateResults adds server-derived craft results to the request. A single
// result is created immediately; multi-result recipes wait for explicit Create actions.
func (p *Plan) CreateResults(results ...item.Stack) error {
	start := len(p.results)
	p.results = append(p.results, results...)
	if len(results) == 1 {
		return p.create(start)
	}
	return nil
}

func (p *Plan) create(resultSlot int) error {
	if resultSlot < 0 || resultSlot >= len(p.results) || p.results[resultSlot].Empty() {
		return ErrInvalidResult
	}
	result := p.results[resultSlot]
	p.results[resultSlot] = item.Stack{}
	return p.SetAt(protocol.FullContainerName{ContainerID: protocol.ContainerCreatedOutput}, createdOutputSlot, result)
}

// resolve looks up a request slot and validates the stack network ID the client
// predicted for it against the identity the slot actually carries.
func (p *Plan) resolve(info protocol.StackRequestSlotInfo) (Container, int, item.Stack, error) {
	slot := int(info.Slot)
	container, stack, staged, err := p.lookup(info.Container, slot)
	if err != nil {
		return nil, 0, item.Stack{}, err
	}
	key := slotKey{container: container, slot: slot}

	actual := int32(0)
	if staged != nil {
		actual = staged.StackNetworkID
	} else {
		var ok bool
		if actual, ok = container.StackNetworkID(slot); !ok {
			return nil, 0, item.Stack{}, ErrStackNetworkID
		}
	}

	expected := info.StackNetworkID
	switch {
	case expected >= 0:
		p.engine.acknowledge(key)
	case expected == p.requestID && staged != nil:
		// The client is predicting a change this same request already staged,
		// so the slot carries the identity it expects by construction.
		return container, slot, stack, nil
	default:
		change, ok := p.engine.history[expected][key]
		if !ok {
			return nil, 0, item.Stack{}, ErrStackNetworkID
		}
		expected = change.networkID
	}
	if actual != expected {
		return nil, 0, item.Stack{}, ErrStackNetworkID
	}
	return container, slot, stack, nil
}

// validContainerIdentity reports whether container may be used as a map key
// without panicking, and is not a typed nil masquerading as a live container.
func validContainerIdentity(container Container) bool {
	t := reflect.TypeOf(container)
	if !t.Comparable() {
		return false
	}
	switch t.Kind() {
	case reflect.Chan, reflect.Pointer:
		return !reflect.ValueOf(container).IsNil()
	default:
		return true
	}
}

// acknowledge drops the predictions for key: the client has caught up with the
// identity the slot actually carries.
func (e *Engine) acknowledge(key slotKey) {
	for requestID, changes := range e.history {
		delete(changes, key)
		if len(changes) == 0 {
			delete(e.history, requestID)
		}
	}
}

func (p *Plan) set(container Container, name protocol.FullContainerName, slot int, stack item.Stack) error {
	if (!stack.Empty() && stack.Count() > stack.MaxCount()) || !container.Accepts(slot, stack) {
		return ErrInvalidItem
	}
	key := slotKey{container: container, slot: slot}
	networkID := p.resolver.StackNetworkID(stack)
	change, ok := p.index[key]
	if ok {
		p.changes[change].After = stack
		p.changes[change].StackNetworkID = networkID
	} else {
		change = len(p.changes)
		p.index[key] = change
		p.changes = append(p.changes, Change{
			Container:      container,
			Slot:           slot,
			After:          stack,
			StackNetworkID: networkID,
		})
	}

	// Every alias the request touched is reported, in first-touch order.
	response := responseKey{name: name, slot: slot}
	if _, ok := p.responseIndex[response]; !ok {
		p.responseIndex[response] = struct{}{}
		p.responses = append(p.responses, responseSlot{responseKey: response, container: container, change: change})
	}
	return nil
}

func (p *Plan) transfer(from, to protocol.StackRequestSlotInfo, count int) error {
	if count <= 0 {
		return ErrInvalidCount
	}
	sourceContainer, sourceSlot, source, err := p.resolve(from)
	if err != nil {
		return err
	}
	destinationContainer, destinationSlot, destination, err := p.resolve(to)
	if err != nil {
		return err
	}
	if sourceContainer == destinationContainer && sourceSlot == destinationSlot {
		return ErrSameSlot
	}
	if source.Count() < count {
		return ErrInsufficientCount
	}
	if !source.Comparable(destination) {
		return ErrIncompatibleStack
	}
	if !destination.Empty() && destination.Count()+count > destination.MaxCount() {
		return ErrStackOverflow
	}
	moved := source.Grow(count - source.Count())
	if destination.Empty() {
		destination = source.Grow(-source.Count())
	}
	sourceAfter, destinationAfter := source.Grow(-count), destination.Grow(count)
	if !sourceContainer.Accepts(sourceSlot, sourceAfter) || !destinationContainer.Accepts(destinationSlot, destinationAfter) {
		return ErrInvalidItem
	}
	if err := p.hooks.Take(sourceContainer, sourceSlot, moved); err != nil {
		return err
	}
	if err := p.hooks.Place(destinationContainer, destinationSlot, moved); err != nil {
		return err
	}
	if err := p.set(sourceContainer, from.Container, sourceSlot, sourceAfter); err != nil {
		return err
	}
	if err := p.set(destinationContainer, to.Container, destinationSlot, destinationAfter); err != nil {
		return err
	}
	return nil
}

func (p *Plan) swap(sourceInfo, destinationInfo protocol.StackRequestSlotInfo) error {
	sourceContainer, sourceSlot, source, err := p.resolve(sourceInfo)
	if err != nil {
		return err
	}
	destinationContainer, destinationSlot, destination, err := p.resolve(destinationInfo)
	if err != nil {
		return err
	}
	if !sourceContainer.Accepts(sourceSlot, destination) || !destinationContainer.Accepts(destinationSlot, source) {
		return ErrInvalidItem
	}
	if err := p.hooks.Take(sourceContainer, sourceSlot, source); err != nil {
		return err
	}
	if err := p.hooks.Place(sourceContainer, sourceSlot, destination); err != nil {
		return err
	}
	if err := p.hooks.Take(destinationContainer, destinationSlot, destination); err != nil {
		return err
	}
	if err := p.hooks.Place(destinationContainer, destinationSlot, source); err != nil {
		return err
	}
	if err := p.set(sourceContainer, sourceInfo.Container, sourceSlot, destination); err != nil {
		return err
	}
	if err := p.set(destinationContainer, destinationInfo.Container, destinationSlot, source); err != nil {
		return err
	}
	return nil
}

// withdraw stages the removal of count items from the slot in info, returning
// the stack that actually left it. allowed decides how many items the consumer
// permits; an empty return means it permitted none.
func (p *Plan) withdraw(info protocol.StackRequestSlotInfo, count int, allowed permit) (item.Stack, error) {
	if count <= 0 {
		return item.Stack{}, ErrInvalidCount
	}
	container, slot, source, err := p.resolve(info)
	if err != nil {
		return item.Stack{}, err
	}
	if source.Count() < count {
		return item.Stack{}, ErrInsufficientCount
	}
	permitted, err := allowed(container, slot, source.Grow(count-source.Count()))
	if err != nil {
		return item.Stack{}, err
	}
	if permitted < 0 || permitted > count {
		return item.Stack{}, ErrInvalidCount
	}
	if permitted == 0 {
		return item.Stack{}, nil
	}
	if err := p.set(container, info.Container, slot, source.Grow(-permitted)); err != nil {
		return item.Stack{}, err
	}
	return source.Grow(permitted - source.Count()), nil
}

func (p *Plan) drop(info protocol.StackRequestSlotInfo, count int, randomly bool) error {
	dropped, err := p.withdraw(info, count, p.hooks.Drop)
	if err != nil {
		return err
	}
	if dropped.Empty() {
		// A cancelled player-level drop still needs a slot correction. Giving the
		// unchanged stack a fresh identity tells the client to discard its local
		// removal prediction, matching Dragonfly's previous Grow(0) behaviour.
		container, slot, source, err := p.resolve(info)
		if err != nil {
			return err
		}
		return p.set(container, info.Container, slot, source.Grow(0))
	}
	p.Defer(DropEffect{Stack: dropped, Randomly: randomly})
	return nil
}

func (p *Plan) remove(info protocol.StackRequestSlotInfo, count int) error {
	_, err := p.withdraw(info, count, p.hooks.Destroy)
	return err
}

// Commit records the successfully applied plan in prediction history. Consumers
// call Commit after applying Changes to their live containers so their final
// stack network IDs can be observed.
func (e *Engine) Commit(p *Plan) {
	if p == nil || p.err != nil {
		return
	}
	changes := make(map[slotKey]historyChange, len(p.changes))
	for _, change := range p.changes {
		networkID, ok := change.Container.StackNetworkID(change.Slot)
		if !ok {
			continue
		}
		changes[slotKey{container: change.Container, slot: change.Slot}] = historyChange{
			networkID: networkID,
			timestamp: p.now,
		}
	}
	e.history[p.requestID] = changes
}

func (e *Engine) prune(now time.Time) {
	for requestID, changes := range e.history {
		for key, change := range changes {
			if now.Sub(change.timestamp) > historyExpiry {
				delete(changes, key)
			}
		}
		if len(changes) == 0 {
			delete(e.history, requestID)
		}
	}
}

// Err returns the error that rejected the plan, if any.
func (p *Plan) Err() error { return p.err }

// Changes returns ordered final slot changes for a successful plan.
func (p *Plan) Changes() []Change {
	if p.err != nil {
		return nil
	}
	return slices.Clone(p.changes)
}

// Effects returns ordered side effects for a successful plan.
func (p *Plan) Effects() []Effect {
	if p.err != nil {
		return nil
	}
	return slices.Clone(p.effects)
}

// Response returns the request's terminal response. For a successful committed
// plan, response containers and slots follow first-change order.
func (p *Plan) Response() protocol.ItemStackResponse {
	if p.err != nil {
		return protocol.ItemStackResponse{Status: protocol.ItemStackResponseStatusError, RequestID: p.requestID}
	}
	response := protocol.ItemStackResponse{Status: protocol.ItemStackResponseStatusOK, RequestID: p.requestID}
	containerIndex := make(map[protocol.FullContainerName]int)
	for _, slot := range p.responses {
		index, ok := containerIndex[slot.name]
		if !ok {
			index = len(response.ContainerInfo)
			containerIndex[slot.name] = index
			response.ContainerInfo = append(response.ContainerInfo, protocol.StackResponseContainerInfo{
				Container: slot.name,
			})
		}
		change := p.changes[slot.change]
		actual, ok := slot.container.Item(slot.slot)
		if !ok {
			actual = change.After
		}
		networkID, _ := slot.container.StackNetworkID(slot.slot)
		response.ContainerInfo[index].SlotInfo = append(
			response.ContainerInfo[index].SlotInfo,
			protocol.StackResponseSlotInfo{
				Slot:                 byte(slot.slot),
				HotbarSlot:           byte(slot.slot),
				Count:                byte(actual.Count()),
				StackNetworkID:       networkID,
				DurabilityCorrection: int32(actual.MaxDurability() - actual.Durability()),
			},
		)
	}
	return response
}
