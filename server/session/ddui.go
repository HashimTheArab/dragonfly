package session

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/df-mc/dragonfly/server/player/ddui"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// DDUIFormHandler holds shared state for all active data-driven UI forms.
type DDUIFormHandler struct {
	mu             sync.Mutex
	forms          map[uint32]*activeDDUIForm
	nextFormID     atomic.Uint32
	nextInstanceID atomic.Uint32
}

type activeDDUIForm struct {
	form                ddui.Form
	formID              uint32
	instanceID          uint32
	property            string
	propertyUpdateCount uint32
	unbind              func()
	closed              atomic.Bool
	updateMu            sync.Mutex
	sendMu              sync.Mutex
	published           bool
	pending             []ddui.UpdateNotification
}

// SendDDUIForm sends f to the client via s, registering it as an active form.
func (h *DDUIFormHandler) SendDDUIForm(f ddui.Form, s *Session) {
	instanceID := h.nextInstanceID.Add(1)
	formID := h.nextFormID.Add(1)

	property := deriveProperty(f.ScreenID(), instanceID)

	af := &activeDDUIForm{
		form:                f,
		formID:              formID,
		instanceID:          instanceID,
		property:            property,
		propertyUpdateCount: 1,
	}

	af.unbind = f.BindSend(func(update ddui.UpdateNotification) {
		af.sendMu.Lock()
		defer af.sendMu.Unlock()
		if af.closed.Load() {
			return
		}
		if !af.published {
			af.pending = append(af.pending, update)
			return
		}
		sendDataStoreUpdate(s, property, update)
	})

	h.mu.Lock()
	h.forms[instanceID] = af
	s.writePacket(&packet.ClientBoundDataStore{
		Updates: []protocol.DataStoreChangeEntry{
			{
				ChangeType: protocol.DataStoreChangeTypeChange,
				Change: protocol.DataStoreChange{
					DataStoreName: "minecraft",
					Property:      property,
					UpdateCount:   1,
					NewValue:      serializeForm(f.ScreenID(), f.Describe()),
				},
			},
		},
	})
	s.writePacket(&packet.ClientBoundDataDrivenUIShowScreen{
		ScreenID:       f.ScreenID(),
		FormID:         formID,
		DataInstanceID: protocol.Option(instanceID),
	})
	h.mu.Unlock()

	af.sendMu.Lock()
	if !af.closed.Load() {
		af.published = true
		for _, update := range af.pending {
			sendDataStoreUpdate(s, property, update)
		}
	}
	af.pending = nil
	af.sendMu.Unlock()
}

func sendDataStoreUpdate(s *Session, property string, update ddui.UpdateNotification) {
	s.writePacket(&packet.ClientBoundDataStore{
		Updates: []protocol.DataStoreChangeEntry{
			{
				ChangeType: protocol.DataStoreChangeTypeUpdate,
				Update:     serializeUpdate(property, update),
			},
		},
	})
}

// CloseDDUIForms closes all active DDUI forms, calling OnClose on each.
func (h *DDUIFormHandler) CloseDDUIForms(s *Session) {
	h.mu.Lock()
	active := make([]*activeDDUIForm, 0, len(h.forms))
	for _, af := range h.forms {
		active = append(active, af)
	}
	h.forms = make(map[uint32]*activeDDUIForm)
	h.mu.Unlock()

	if len(active) == 0 {
		return
	}

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

	s.writePacket(&packet.ClientBoundDataDrivenUICloseScreen{})

	for _, af := range claimed {
		af.form.OnClose(ddui.CloseReasonProgrammaticAll)
		sendDataStoreCleanup(s, af)
	}
}

func (h *DDUIFormHandler) discardDDUIForms() {
	h.mu.Lock()
	active := h.forms
	h.forms = make(map[uint32]*activeDDUIForm)
	h.mu.Unlock()

	for _, af := range active {
		if !af.claim() {
			continue
		}
		af.unbind()
	}
}

func (af *activeDDUIForm) claim() bool {
	if !af.closed.CompareAndSwap(false, true) {
		return false
	}
	af.sendMu.Lock()
	af.sendMu.Unlock()
	return true
}

func (af *activeDDUIForm) handleUpdate(path string, value ddui.UpdateValue) (ddui.UpdateResult, bool) {
	af.updateMu.Lock()
	defer af.updateMu.Unlock()
	if af.closed.Load() {
		return ddui.UpdateResult{}, false
	}
	result := af.form.HandleUpdate(path, value)
	if result.Close {
		af.closed.Store(true)
	}
	return result, true
}

func sendDataStoreCleanup(s *Session, af *activeDDUIForm) {
	s.writePacket(&packet.ClientBoundDataStore{
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

func serializeForm(screenID string, desc ddui.FormDescriptor) protocol.DataStorePropertyValue {
	if screenID == "minecraft:message_box" {
		return serializeMessageBox(desc)
	}
	return serializeCustomForm(desc)
}

func serializeCustomForm(desc ddui.FormDescriptor) protocol.DataStorePropertyValue {
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

func serializeUpdate(property string, update ddui.UpdateNotification) protocol.DataStoreUpdate {
	u := protocol.DataStoreUpdate{
		DataStoreName:       "minecraft",
		Property:            property,
		Path:                update.Path,
		PropertyUpdateCount: 1,
		PathUpdateCount:     1,
	}
	switch update.Value.Kind {
	case ddui.UpdateKindFloat:
		u.ControlType = protocol.DataStoreControlDouble
		u.DoubleValue = update.Value.Float
	case ddui.UpdateKindBool:
		u.ControlType = protocol.DataStoreControlBoolean
		u.BoolValue = update.Value.Bool
	case ddui.UpdateKindString:
		u.ControlType = protocol.DataStoreControlString
		u.StringValue = update.Value.String
	}
	return u
}

func serializeMessageBox(desc ddui.FormDescriptor) protocol.DataStorePropertyValue {
	btn1 := []protocol.DataStoreMapEntry{
		dsEntry("button_visible", dsBool(true)),
		dsEntry("label", dsStr(desc.Button1.Label)),
		dsEntry("onClick", dsInt(0)),
		dsEntry("visible", dsBool(true)),
	}
	if desc.Button1.Tooltip != "" {
		btn1 = append(btn1,
			dsEntry("tooltip", dsStr(desc.Button1.Tooltip)),
			dsEntry("tooltip_visible", dsBool(true)),
		)
	}

	btn2 := []protocol.DataStoreMapEntry{
		dsEntry("button_visible", dsBool(true)),
		dsEntry("label", dsStr(desc.Button2.Label)),
		dsEntry("onClick", dsInt(0)),
		dsEntry("visible", dsBool(true)),
	}
	if desc.Button2.Tooltip != "" {
		btn2 = append(btn2,
			dsEntry("tooltip", dsStr(desc.Button2.Tooltip)),
			dsEntry("tooltip_visible", dsBool(true)),
		)
	}

	return dsMap(
		dsEntry("body", dsStr(desc.Body)),
		dsEntry("button1", dsMap(btn1...)),
		dsEntry("button2", dsMap(btn2...)),
		dsEntry("title", dsStr(desc.Title)),
	)
}

func serializeElement(e ddui.ElementDescriptor) protocol.DataStorePropertyValue {
	switch e.Kind {
	case ddui.ElementSpacer:
		return dsMap(
			dsEntry("spacer_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ddui.ElementDivider:
		return dsMap(
			dsEntry("divider_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ddui.ElementLabel:
		return dsMap(
			dsEntry("label_visible", dsBool(e.Visible)),
			dsEntry("text", dsStr(e.StringValue)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ddui.ElementHeader:
		return dsMap(
			dsEntry("header_visible", dsBool(e.Visible)),
			dsEntry("text", dsStr(e.StringValue)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ddui.ElementTextField:
		return dsMap(
			dsEntry("description", dsStr(e.Description)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("text", dsStr(e.StringValue)),
			dsEntry("textfield_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ddui.ElementDropdown:
		return dsMap(
			dsEntry("description", dsStr(e.Description)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("dropdown_visible", dsBool(e.Visible)),
			dsEntry("items", serializeDropdownItems(e.Options)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("value", dsInt(int64(e.IntValue))),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ddui.ElementToggle:
		return dsMap(
			dsEntry("description", dsStr(e.Description)),
			dsEntry("disabled", dsBool(e.Disabled)),
			dsEntry("label", dsStr(e.Label)),
			dsEntry("toggled", dsBool(e.BoolValue)),
			dsEntry("toggle_visible", dsBool(e.Visible)),
			dsEntry("visible", dsBool(e.Visible)),
		)
	case ddui.ElementSlider:
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
	case ddui.ElementButton:
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

func serializeDropdownItems(opts []ddui.DropdownOption) protocol.DataStorePropertyValue {
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
