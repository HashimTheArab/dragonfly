package ddui

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Controller owns data-driven UI forms shown over a single client connection.
// It writes client-bound packets through the function passed to NewController and
// identifies the server-bound packets that belong to those forms.
type Controller struct {
	mu             sync.Mutex
	forms          map[uint32]*activeForm
	write          func(packet.Packet)
	nextFormID     atomic.Uint32
	nextInstanceID atomic.Uint32
	nextBindingID  atomic.Uint64
	nextOrder      atomic.Uint64
}

type activeForm struct {
	form                Form
	formID              uint32
	instanceID          uint32
	bindingID           uint64
	order               uint64
	property            string
	propertyUpdateCount uint32
	unbind              func()
	closed              atomic.Bool
	updateMu            sync.Mutex
	sendMu              sync.Mutex
	published           bool
	pathUpdateCounts    map[string]uint32
	pending             []pendingDDUIUpdate
}

type pendingDDUIUpdate struct {
	update              UpdateNotification
	propertyUpdateCount uint32
	pathUpdateCount     uint32
}

// NewController creates a controller that writes client-bound DDUI packets using write.
// IDs start at random offsets so a proxy's local forms are unlikely to collide with
// data-driven UI forms owned by the upstream server.
func NewController(write func(packet.Packet)) *Controller {
	if write == nil {
		panic("ddui: nil packet writer")
	}
	c := &Controller{forms: make(map[uint32]*activeForm), write: write}
	c.nextFormID.Store(rand.Uint32())
	c.nextInstanceID.Store(rand.Uint32())
	c.nextBindingID.Store(rand.Uint64())
	return c
}

// Show sends f to the client and registers it as an active form.
func (c *Controller) Show(f Form) {
	instanceID := nextNonZeroUint32(&c.nextInstanceID)
	formID := nextNonZeroUint32(&c.nextFormID)
	screenID := f.ScreenID()

	property := deriveProperty(screenID, instanceID)
	bindingID := nextNonZeroUint64(&c.nextBindingID)

	af := &activeForm{
		form:                f,
		formID:              formID,
		instanceID:          instanceID,
		bindingID:           bindingID,
		order:               c.nextOrder.Add(1),
		property:            property,
		propertyUpdateCount: 1,
		pathUpdateCounts:    make(map[string]uint32),
	}

	onUpdate := func(update UpdateNotification) {
		af.sendMu.Lock()
		defer af.sendMu.Unlock()
		if af.closed.Load() {
			return
		}
		propertyUpdateCount, pathUpdateCount := af.recordUpdate(update.Path)
		if !af.published {
			af.pending = append(af.pending, pendingDDUIUpdate{
				update:              update,
				propertyUpdateCount: propertyUpdateCount,
				pathUpdateCount:     pathUpdateCount,
			})
			return
		}
		c.sendDataStoreUpdate(property, update, propertyUpdateCount, pathUpdateCount)
	}
	if bound, ok := f.(BindingForm); ok {
		af.unbind = bound.BindSendFrom(bindingID, onUpdate)
	} else {
		af.unbind = f.BindSend(onUpdate)
	}
	if af.unbind == nil {
		af.unbind = func() {}
	}
	desc := f.Describe()

	c.mu.Lock()
	c.forms[instanceID] = af
	c.write(&packet.ClientBoundDataStore{
		Updates: []protocol.DataStoreChangeEntry{
			{
				ChangeType: protocol.DataStoreChangeTypeChange,
				Change: protocol.DataStoreChange{
					DataStoreName: "minecraft",
					Property:      property,
					UpdateCount:   1,
					NewValue:      serializeForm(screenID, desc),
				},
			},
		},
	})
	c.write(&packet.ClientBoundDataDrivenUIShowScreen{
		ScreenID:       screenID,
		FormID:         formID,
		DataInstanceID: protocol.Option(instanceID),
	})
	c.mu.Unlock()

	af.sendMu.Lock()
	if !af.closed.Load() {
		af.published = true
		for _, update := range af.pending {
			c.sendDataStoreUpdate(property, update.update, update.propertyUpdateCount, update.pathUpdateCount)
		}
	}
	af.pending = nil
	af.sendMu.Unlock()
}

func (c *Controller) sendDataStoreUpdate(property string, update UpdateNotification, propertyUpdateCount, pathUpdateCount uint32) {
	c.write(&packet.ClientBoundDataStore{
		Updates: []protocol.DataStoreChangeEntry{
			{
				ChangeType: protocol.DataStoreChangeTypeUpdate,
				Update:     serializeUpdate(property, update, propertyUpdateCount, pathUpdateCount),
			},
		},
	})
}

// CloseAll closes all active DDUI forms, calling OnClose on each.
func (c *Controller) CloseAll() {
	c.mu.Lock()
	active := make([]*activeForm, 0, len(c.forms))
	for _, af := range c.forms {
		active = append(active, af)
	}
	c.forms = make(map[uint32]*activeForm)
	c.mu.Unlock()

	if len(active) == 0 {
		return
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].order > active[j].order
	})

	claimed := active[:0]
	for _, af := range active {
		if !af.claim() {
			continue
		}
		af.unbind()
		claimed = append(claimed, af)
	}
	if len(claimed) == 0 {
		return
	}

	c.write(&packet.ClientBoundDataDrivenUICloseScreen{})

	for _, af := range claimed {
		c.sendDataStoreCleanup(af)
	}
	for _, af := range claimed {
		af.form.OnClose(CloseReasonProgrammaticAll)
	}
}

// Discard releases all active forms without writing packets or invoking close handlers.
func (c *Controller) Discard() {
	c.mu.Lock()
	active := c.forms
	c.forms = make(map[uint32]*activeForm)
	c.mu.Unlock()

	for _, af := range active {
		if !af.claim() {
			continue
		}
		af.unbind()
	}
}

// HandleDataStore handles a client data-store update if it belongs to a form
// owned by c. It reports whether the packet was claimed, including malformed
// updates to an owned property that should not be handled elsewhere.
func (c *Controller) HandleDataStore(pk *packet.ServerBoundDataStore) bool {
	if pk.Update.DataStoreName != "minecraft" {
		return false
	}

	c.mu.Lock()
	var af, top *activeForm
	for _, candidate := range c.forms {
		if top == nil || candidate.order > top.order {
			top = candidate
		}
		if candidate.property == pk.Update.Property {
			af = candidate
		}
	}
	c.mu.Unlock()
	if af == nil {
		return false
	}
	if af != top {
		return true
	}

	value, valid := dataStoreControlToUpdateValue(pk.Update)
	if !valid {
		return true
	}
	result, accepted := af.handleUpdate(pk.Update.Path, value)
	if !accepted {
		return true
	}
	if !result.Close {
		if result.Complete != nil {
			result.Complete()
		}
		return true
	}

	af.sendMu.Lock()
	af.pending = nil
	af.sendMu.Unlock()
	c.mu.Lock()
	if c.forms[af.instanceID] == af {
		delete(c.forms, af.instanceID)
	}
	c.mu.Unlock()
	af.unbind()
	c.write(&packet.ClientBoundDataDrivenUICloseScreen{
		FormID: protocol.Option(af.formID),
	})
	c.sendDataStoreCleanup(af)
	if result.Complete == nil {
		af.form.OnClose(Closed)
	} else {
		result.Complete()
	}
	return true
}

// HandleScreenClosed handles a client screen-closed packet if it belongs to a
// form owned by c and reports whether the packet was claimed.
func (c *Controller) HandleScreenClosed(pk *packet.ServerBoundDataDrivenScreenClosed) bool {
	c.mu.Lock()
	var af *activeForm
	for _, candidate := range c.forms {
		if candidate.formID == pk.FormID {
			af = candidate
			delete(c.forms, candidate.instanceID)
			break
		}
	}
	c.mu.Unlock()
	if af == nil {
		return false
	}
	if !af.claim() {
		return true
	}

	af.unbind()
	c.sendDataStoreCleanup(af)
	af.form.OnClose(closeReason(pk.CloseReason))
	return true
}

func dataStoreControlToUpdateValue(u protocol.DataStoreUpdate) (UpdateValue, bool) {
	switch u.ControlType {
	case protocol.DataStoreControlDouble:
		return UpdateValue{Kind: UpdateKindFloat, Float: u.DoubleValue}, true
	case protocol.DataStoreControlBoolean:
		return UpdateValue{Kind: UpdateKindBool, Bool: u.BoolValue}, true
	case protocol.DataStoreControlString:
		return UpdateValue{Kind: UpdateKindString, String: u.StringValue}, true
	default:
		return UpdateValue{}, false
	}
}

func closeReason(reason string) int {
	switch reason {
	case packet.DataDrivenScreenCloseReasonProgrammaticClose:
		return CloseReasonProgrammatic
	case packet.DataDrivenScreenCloseReasonProgrammaticCloseAll:
		return CloseReasonProgrammaticAll
	case packet.DataDrivenScreenCloseReasonClientCanceled:
		return Closed
	case packet.DataDrivenScreenCloseReasonUserBusy:
		return Busy
	case packet.DataDrivenScreenCloseReasonInvalidForm:
		return Invalid
	default:
		return Closed
	}
}

func (af *activeForm) claim() bool {
	if !af.closed.CompareAndSwap(false, true) {
		return false
	}
	af.sendMu.Lock()
	af.pending = nil
	af.sendMu.Unlock()
	return true
}

func (af *activeForm) handleUpdate(path string, value UpdateValue) (UpdateResult, bool) {
	af.updateMu.Lock()
	defer af.updateMu.Unlock()
	if af.closed.Load() {
		return UpdateResult{}, false
	}
	if bound, ok := af.form.(BindingForm); ok {
		result := bound.HandleUpdateFrom(af.bindingID, path, value)
		if result.Close {
			af.closed.Store(true)
		}
		return result, true
	}
	result := af.form.HandleUpdate(path, value)
	if result.Close {
		af.closed.Store(true)
	}
	return result, true
}

func (af *activeForm) recordUpdate(path string) (propertyUpdateCount, pathUpdateCount uint32) {
	af.propertyUpdateCount++
	af.pathUpdateCounts[path]++
	return af.propertyUpdateCount, af.pathUpdateCounts[path]
}

func (c *Controller) sendDataStoreCleanup(af *activeForm) {
	c.write(&packet.ClientBoundDataStore{
		Updates: []protocol.DataStoreChangeEntry{
			{
				ChangeType: protocol.DataStoreChangeTypeChange,
				Change: protocol.DataStoreChange{
					DataStoreName: "minecraft",
					Property:      af.property,
					UpdateCount:   af.propertyUpdateCount + 1,
					NewValue:      protocol.DataStorePropertyValue{Type: protocol.DataStorePropertyTypeNone},
				},
			},
		},
	})
}

func deriveProperty(screenID string, instanceID uint32) string {
	base := strings.TrimPrefix(screenID, "minecraft:")
	base = strings.ReplaceAll(base, ":", "_")
	return base + "_data_" + strconv.FormatUint(uint64(instanceID), 10)
}

func nextNonZeroUint32(counter *atomic.Uint32) uint32 {
	for {
		if id := counter.Add(1); id != 0 {
			return id
		}
	}
}

func nextNonZeroUint64(counter *atomic.Uint64) uint64 {
	for {
		if id := counter.Add(1); id != 0 {
			return id
		}
	}
}

func serializeForm(screenID string, desc FormDescriptor) protocol.DataStorePropertyValue {
	if screenID == "minecraft:message_box" {
		return serializeMessageBox(desc)
	}
	return serializeCustomForm(desc)
}

func serializeCustomForm(desc FormDescriptor) protocol.DataStorePropertyValue {
	entries := make([]protocol.DataStoreMapEntry, 0, 3)

	if desc.HasCloseButton {
		entries = append(entries, dsEntry("closeButton", dsMap(
			dsEntry("button_visible", dsBool(desc.CloseButton.Visible)),
			dsEntry("label", dsStr(desc.CloseButton.Label)),
			dsEntry("onClick", dsInt(0)),
			dsEntry("visible", dsBool(desc.CloseButton.Visible)),
		)))
	}

	layoutEntries := make([]protocol.DataStoreMapEntry, 0, len(desc.Elements)+1)
	for i, elem := range desc.Elements {
		layoutEntries = append(layoutEntries, dsEntry(strconv.Itoa(i), serializeElement(elem)))
	}
	layoutEntries = append(layoutEntries, dsEntry("length", dsInt(int64(len(desc.Elements)))))

	entries = append(entries,
		dsEntry("layout", dsMap(layoutEntries...)),
		dsEntry("title", dsStr(desc.Title)),
	)
	return dsMap(entries...)
}

func serializeUpdate(property string, update UpdateNotification, propertyUpdateCount, pathUpdateCount uint32) protocol.DataStoreUpdate {
	u := protocol.DataStoreUpdate{
		DataStoreName:       "minecraft",
		Property:            property,
		Path:                update.Path,
		PropertyUpdateCount: propertyUpdateCount,
		PathUpdateCount:     pathUpdateCount,
	}
	switch update.Value.Kind {
	case UpdateKindFloat:
		u.ControlType = protocol.DataStoreControlDouble
		u.DoubleValue = update.Value.Float
	case UpdateKindBool:
		u.ControlType = protocol.DataStoreControlBoolean
		u.BoolValue = update.Value.Bool
	case UpdateKindString:
		u.ControlType = protocol.DataStoreControlString
		u.StringValue = update.Value.String
	}
	return u
}

func serializeMessageBox(desc FormDescriptor) protocol.DataStorePropertyValue {
	entries := []protocol.DataStoreMapEntry{
		dsEntry("body", dsStr(desc.Body)),
	}
	if desc.HasButton1 {
		entries = append(entries, dsEntry("button1", serializeMessageBoxButton(desc.Button1)))
	}
	if desc.HasButton2 {
		entries = append(entries, dsEntry("button2", serializeMessageBoxButton(desc.Button2)))
	}
	entries = append(entries, dsEntry("title", dsStr(desc.Title)))
	return dsMap(entries...)
}

func serializeMessageBoxButton(button ButtonDescriptor) protocol.DataStorePropertyValue {
	entries := []protocol.DataStoreMapEntry{
		dsEntry("button_visible", dsBool(true)),
		dsEntry("label", dsStr(button.Label)),
		dsEntry("onClick", dsInt(0)),
		dsEntry("visible", dsBool(true)),
	}
	if button.Tooltip != "" {
		entries = append(entries,
			dsEntry("tooltip", dsStr(button.Tooltip)),
			dsEntry("tooltip_visible", dsBool(true)),
		)
	}
	return dsMap(entries...)
}

func serializeElement(e ElementDescriptor) protocol.DataStorePropertyValue {
	switch e.Kind {
	case ElementSpacer:
		return dsMap(
			dsEntry("spacer_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementDivider:
		return dsMap(
			dsEntry("divider_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementLabel:
		return dsMap(
			dsEntry("label_visible", dsBool(e.Visible)),
			dsEntry("text", dsStr(e.StringValue)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementHeader:
		return dsMap(
			dsEntry("header_visible", dsBool(e.Visible)),
			dsEntry("text", dsStr(e.StringValue)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementTextField:
		return dsMap(
			dsEntry("description", dsStr(e.Description)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("text", dsStr(e.StringValue)),
			dsEntry("textfield_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementDropdown:
		return dsMap(
			dsEntry("description", dsStr(e.Description)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("dropdown_visible", dsBool(e.Visible)),
			dsEntry("items", serializeDropdownItems(e.Options)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("value", dsInt(int64(e.IntValue))),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementToggle:
		return dsMap(
			dsEntry("description", dsStr(e.Description)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("toggled", dsBool(e.BoolValue)),
			dsEntry("toggle_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementSlider:
		return dsMap(
			dsEntry("description", dsStr(e.Description)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("maxValue", dsFloat(e.Max)),
			dsEntry("minValue", dsFloat(e.Min)),
			dsEntry("slider_visible", dsBool(e.Visible)),
			dsEntry("step", dsFloat(e.Step)),
			dsEntry("value", dsFloat(e.FloatValue)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ElementButton:
		entries := []protocol.DataStoreMapEntry{
			dsEntry("button_visible", dsBool(e.Visible)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("onClick", dsInt(0)),
			dsEntry("visible", dsBool(e.Visible)),
		}
		if e.Tooltip != "" {
			entries = append(entries,
				dsEntry("tooltip", dsStr(e.Tooltip)),
				dsEntry("tooltip_visible", dsBool(true)),
			)
		}
		return dsMap(entries...)
	}
	return dsMap()
}

func serializeDropdownItems(opts []DropdownOption) protocol.DataStorePropertyValue {
	entries := make([]protocol.DataStoreMapEntry, 0, len(opts)+1)
	for i, opt := range opts {
		item := []protocol.DataStoreMapEntry{
			dsEntry("label", dsStr(opt.Label)),
			dsEntry("value", dsInt(int64(opt.Value))),
		}
		if opt.Description != "" {
			item = append(item, dsEntry("description", dsStr(opt.Description)))
		}
		entries = append(entries, dsEntry(strconv.Itoa(i), dsMap(item...)))
	}
	entries = append(entries, dsEntry("length", dsInt(int64(len(opts)))))
	return dsMap(entries...)
}

func dsMap(entries ...protocol.DataStoreMapEntry) protocol.DataStorePropertyValue {
	return protocol.DataStorePropertyValue{
		Type:     protocol.DataStorePropertyTypeMap,
		MapValue: entries,
	}
}

func dsBool(v bool) protocol.DataStorePropertyValue {
	return protocol.DataStorePropertyValue{Type: protocol.DataStorePropertyTypeBool, BoolValue: v}
}

func dsInt(v int64) protocol.DataStorePropertyValue {
	return protocol.DataStorePropertyValue{Type: protocol.DataStorePropertyTypeInt64, Int64Value: v}
}

func dsFloat(v float64) protocol.DataStorePropertyValue {
	return protocol.DataStorePropertyValue{Type: protocol.DataStorePropertyTypeDouble, DoubleValue: v}
}

func dsStr(v string) protocol.DataStorePropertyValue {
	return protocol.DataStorePropertyValue{Type: protocol.DataStorePropertyTypeString, StringValue: v}
}

func dsEntry(key string, value protocol.DataStorePropertyValue) protocol.DataStoreMapEntry {
	return protocol.DataStoreMapEntry{Key: key, Value: value}
}
