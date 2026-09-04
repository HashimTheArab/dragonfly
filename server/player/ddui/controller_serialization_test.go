package ddui

import (
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestDataStoreSerializationPreservesControlTypes(t *testing.T) {
	desc := FormDescriptor{
		Title:          "Settings",
		HasCloseButton: true,
		CloseButton:    ElementDescriptor{Visible: false, Label: "Dismiss"},
		Elements: []ElementDescriptor{
			{Kind: ElementDropdown, Options: []DropdownOption{{Label: "Easy", Description: "Low", Value: 3}}, IntValue: 3},
			{Kind: ElementSlider, Min: 0.25, Max: 2.75, Step: 0.25, FloatValue: 1.5},
		},
	}

	root := serializeCustomForm(desc)
	closeButton := mapValue(t, root, "closeButton")
	if got := boolValue(t, closeButton, "visible"); got {
		t.Fatal("hidden close button was serialized as visible")
	}

	layout := mapValue(t, root, "layout")
	dropdown := mapValue(t, layout, "0")
	items := mapValue(t, dropdown, "items")
	item := mapValue(t, items, "0")
	if got := intValue(t, item, "value"); got != 3 {
		t.Fatalf("dropdown item value = %d, want 3", got)
	}
	if got := stringValue(t, item, "description"); got != "Low" {
		t.Fatalf("dropdown item description = %q, want %q", got, "Low")
	}

	slider := mapValue(t, layout, "1")
	for _, key := range []string{"minValue", "maxValue", "step", "value"} {
		if got := propertyValue(t, slider, key).Type; got != protocol.DataStorePropertyTypeDouble {
			t.Fatalf("slider %s type = %d, want double", key, got)
		}
	}
}

func TestMessageBoxSerializationOmitsUnsetButtons(t *testing.T) {
	desc := NewMessageBox("Confirm", Body("Continue?")).Describe()
	root := serializeMessageBox(desc)
	if _, ok := findMapValue(root, "button1"); ok {
		t.Fatal("unset first button was serialized")
	}
	if _, ok := findMapValue(root, "button2"); ok {
		t.Fatal("unset second button was serialized")
	}
}

func TestDataStoreUpdateCarriesMonotonicCounts(t *testing.T) {
	af := &activeForm{
		propertyUpdateCount: 1,
		pathUpdateCounts:    make(map[string]uint32),
	}

	property, path := af.recordUpdate("layout[0].text")
	if property != 2 || path != 1 {
		t.Fatalf("first update counts = (%d, %d), want (2, 1)", property, path)
	}
	property, path = af.recordUpdate("layout[0].text")
	if property != 3 || path != 2 {
		t.Fatalf("second update counts = (%d, %d), want (3, 2)", property, path)
	}
	property, path = af.recordUpdate("title")
	if property != 4 || path != 1 {
		t.Fatalf("different-path update counts = (%d, %d), want (4, 1)", property, path)
	}

	update := serializeUpdate("custom_form_data_1", UpdateNotification{
		Path:  "title",
		Value: UpdateValue{Kind: UpdateKindString, String: "Updated"},
	}, property, path)
	if update.PropertyUpdateCount != 4 || update.PathUpdateCount != 1 {
		t.Fatalf("serialized counts = (%d, %d), want (4, 1)", update.PropertyUpdateCount, update.PathUpdateCount)
	}
}

func TestServerBoundDataStoreRejectsInvalidOwnershipAndTypes(t *testing.T) {
	var packets []packet.Packet
	controller := NewController(func(pk packet.Packet) { packets = append(packets, pk) })
	value := NewObservable("before", true)
	controller.Show(New("Settings", TextField("Name", value)))
	property := packets[0].(*packet.ClientBoundDataStore).Updates[0].Change.Property

	packetFor := func(property string, controlType uint32) *packet.ServerBoundDataStore {
		return &packet.ServerBoundDataStore{Update: protocol.DataStoreUpdate{
			DataStoreName: "minecraft",
			Property:      property,
			Path:          "layout[0].text",
			ControlType:   controlType,
			StringValue:   "changed",
		}}
	}

	if !controller.HandleDataStore(packetFor(property, 99)) {
		t.Fatal("invalid update to an owned property was not claimed")
	}
	if got := value.Get(); got != "before" {
		t.Fatalf("invalid control type changed value to %q", got)
	}

	if controller.HandleDataStore(packetFor("other_data_1", protocol.DataStoreControlString)) {
		t.Fatal("foreign property was claimed")
	}
	if got := value.Get(); got != "before" {
		t.Fatalf("wrong property changed value to %q", got)
	}

	if !controller.HandleDataStore(packetFor(property, protocol.DataStoreControlString)) {
		t.Fatal("valid update to an owned property was not claimed")
	}
	if got := value.Get(); got != "changed" {
		t.Fatalf("valid update changed value to %q", got)
	}
}

func TestServerBoundDataStoreOnlyUpdatesTopForm(t *testing.T) {
	var packets []packet.Packet
	controller := NewController(func(pk packet.Packet) { packets = append(packets, pk) })
	oldValue := NewObservable("old", true)
	newValue := NewObservable("new", true)
	controller.Show(New("Old", TextField("Name", oldValue)))
	oldProperty := packets[0].(*packet.ClientBoundDataStore).Updates[0].Change.Property
	controller.Show(New("New", TextField("Name", newValue)))

	if !controller.HandleDataStore(&packet.ServerBoundDataStore{Update: protocol.DataStoreUpdate{
		DataStoreName: "minecraft",
		Property:      oldProperty,
		Path:          "layout[0].text",
		ControlType:   protocol.DataStoreControlString,
		StringValue:   "ignored",
	}}) {
		t.Fatal("non-top owned form update was not claimed")
	}
	if oldValue.Get() != "old" || newValue.Get() != "new" {
		t.Fatalf("non-top form was updated: old=%q new=%q", oldValue.Get(), newValue.Get())
	}
}

func TestDataStoreControlConversionRejectsUnknownTypes(t *testing.T) {
	if _, ok := dataStoreControlToUpdateValue(protocol.DataStoreUpdate{ControlType: 99}); ok {
		t.Fatal("unknown data-store control type was accepted")
	}
}

func TestCloseDDUIFormsPublishesCleanupBeforeCallbacks(t *testing.T) {
	var packets []packet.Packet
	callbackQueueLength := 0
	controller := NewController(func(pk packet.Packet) { packets = append(packets, pk) })
	form := New("Settings", Handler(func(int) {
		callbackQueueLength = len(packets)
	}))
	controller.Show(form)
	controller.CloseAll()
	if callbackQueueLength != 4 {
		t.Fatalf("callback observed %d queued packets, want show packets plus close and cleanup", callbackQueueLength)
	}
}

func TestMessageBoxSelectionPublishesCloseBeforeCallback(t *testing.T) {
	var packets []packet.Packet
	callbackQueueLength := 0
	controller := NewController(func(pk packet.Packet) { packets = append(packets, pk) })
	box := NewMessageBox("Confirm", Button1("Yes"), Handler(func(int) {
		callbackQueueLength = len(packets)
	}))
	controller.Show(box)
	property := packets[0].(*packet.ClientBoundDataStore).Updates[0].Change.Property
	if !controller.HandleDataStore(&packet.ServerBoundDataStore{Update: protocol.DataStoreUpdate{
		DataStoreName: "minecraft",
		Property:      property,
		Path:          "button1.onClick",
		ControlType:   protocol.DataStoreControlDouble,
	}}) {
		t.Fatal("message-box selection was not claimed")
	}
	if callbackQueueLength != 4 {
		t.Fatalf("callback observed %d queued packets, want show packets plus close and cleanup", callbackQueueLength)
	}
}

func propertyValue(t *testing.T, value protocol.DataStorePropertyValue, key string) protocol.DataStorePropertyValue {
	t.Helper()
	for _, entry := range value.MapValue {
		if entry.Key == key {
			return entry.Value
		}
	}
	t.Fatalf("data-store map is missing %q", key)
	return protocol.DataStorePropertyValue{}
}

func findMapValue(value protocol.DataStorePropertyValue, key string) (protocol.DataStorePropertyValue, bool) {
	for _, entry := range value.MapValue {
		if entry.Key == key {
			return entry.Value, true
		}
	}
	return protocol.DataStorePropertyValue{}, false
}

func mapValue(t *testing.T, value protocol.DataStorePropertyValue, key string) protocol.DataStorePropertyValue {
	t.Helper()
	value = propertyValue(t, value, key)
	if value.Type != protocol.DataStorePropertyTypeMap {
		t.Fatalf("data-store value %q has type %d, want map", key, value.Type)
	}
	return value
}

func boolValue(t *testing.T, value protocol.DataStorePropertyValue, key string) bool {
	t.Helper()
	value = propertyValue(t, value, key)
	if value.Type != protocol.DataStorePropertyTypeBool {
		t.Fatalf("data-store value %q has type %d, want bool", key, value.Type)
	}
	return value.BoolValue
}

func intValue(t *testing.T, value protocol.DataStorePropertyValue, key string) int64 {
	t.Helper()
	value = propertyValue(t, value, key)
	if value.Type != protocol.DataStorePropertyTypeInt64 {
		t.Fatalf("data-store value %q has type %d, want int64", key, value.Type)
	}
	return value.Int64Value
}

func stringValue(t *testing.T, value protocol.DataStorePropertyValue, key string) string {
	t.Helper()
	value = propertyValue(t, value, key)
	if value.Type != protocol.DataStorePropertyTypeString {
		t.Fatalf("data-store value %q has type %d, want string", key, value.Type)
	}
	return value.StringValue
}
